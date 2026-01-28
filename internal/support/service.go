package support

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/securizon/internal/email"
	"github.com/securizon/internal/slack"
	"github.com/securizon/internal/tenant"
	"github.com/securizon/internal/user"
	"github.com/securizon/internal/zendesk"
)

type SupportService struct {
	ticketStore  TicketStore
	tenantStore  tenant.Store
	userStore    user.Store
	zendesk      *zendesk.Client
	slack        *slack.Client
	emailService *email.Service
	config       SupportConfig
}

type Ticket struct {
	ID            string                 `json:"id"`
	TenantID      string                 `json:"tenant_id"`
	UserID        string                 `json:"user_id"`
	Subject       string                 `json:"subject"`
	Description   string                 `json:"description"`
	Priority      TicketPriority         `json:"priority"`
	Status        TicketStatus           `json:"status"`
	Type          TicketType             `json:"type"`
	Category      string                 `json:"category"`
	AssignedTo    string                 `json:"assigned_to,omitempty"`
	Tags          []string               `json:"tags"`
	Metadata      map[string]interface{} `json:"metadata"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
	ClosedAt      *time.Time             `json:"closed_at,omitempty"`
	FirstResponse *time.Time             `json:"first_response_at,omitempty"`
}

type TicketPriority string

const (
	PriorityLow    TicketPriority = "low"
	PriorityNormal TicketPriority = "normal"
	PriorityHigh   TicketPriority = "high"
	PriorityUrgent TicketPriority = "urgent"
)

type TicketStatus string

const (
	StatusNew      TicketStatus = "new"
	StatusOpen     TicketStatus = "open"
	StatusPending  TicketStatus = "pending"
	StatusResolved TicketStatus = "resolved"
	StatusClosed   TicketStatus = "closed"
)

type TicketType string

const (
	TypeQuestion   TicketType = "question"
	TypeIncident   TicketType = "incident"
	TypeProblem    TicketType = "problem"
	TypeFeatureReq TicketType = "feature_request"
	TypeBilling    TicketType = "billing"
)

func NewSupportService(
	ticketStore TicketStore,
	tenantStore tenant.Store,
	userStore user.Store,
	zendesk *zendesk.Client,
	slack *slack.Client,
	emailService *email.Service,
) *SupportService {

	return &SupportService{
		ticketStore:  ticketStore,
		tenantStore:  tenantStore,
		userStore:    userStore,
		zendesk:      zendesk,
		slack:        slack,
		emailService: emailService,
		config: SupportConfig{
			DefaultResponseTime: map[TicketPriority]time.Duration{
				PriorityLow:    48 * time.Hour,
				PriorityNormal: 24 * time.Hour,
				PriorityHigh:   4 * time.Hour,
				PriorityUrgent: 1 * time.Hour,
			},
			AutoCloseDays: 7,
		},
	}
}

// CreateTicket creates a new support ticket
func (ss *SupportService) CreateTicket(ctx context.Context, req *CreateTicketRequest) (*Ticket, error) {
	// Get tenant and user
	tenant, err := ss.tenantStore.GetTenant(ctx, req.TenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant: %v", err)
	}

	user, err := ss.userStore.GetUser(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %v", err)
	}

	// Create ticket
	ticket := &Ticket{
		ID:          generateUUID(),
		TenantID:    req.TenantID,
		UserID:      req.UserID,
		Subject:     req.Subject,
		Description: req.Description,
		Priority:    req.Priority,
		Status:      StatusNew,
		Type:        req.Type,
		Category:    req.Category,
		Tags:        append(req.Tags, fmt.Sprintf("tenant:%s", tenant.Slug)),
		Metadata: map[string]interface{}{
			"plan":        tenant.Plan,
			"user_email":  user.Email,
			"user_name":   user.Name,
			"tenant_name": tenant.Name,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Store ticket
	if err := ss.ticketStore.CreateTicket(ctx, ticket); err != nil {
		return nil, fmt.Errorf("failed to store ticket: %v", err)
	}

	// Create in Zendesk if enabled
	if ss.zendesk != nil && tenant.Plan != "starter" {
		zdTicket, err := ss.createZendeskTicket(ctx, ticket, tenant, user)
		if err != nil {
			log.Printf("Failed to create Zendesk ticket: %v", err)
		} else {
			ticket.Metadata["zendesk_id"] = zdTicket.ID
		}
	}

	// Notify support team via Slack
	if ss.slack != nil {
		go ss.notifySlack(ticket, tenant, user)
	}

	// Send confirmation email to user
	go ss.sendTicketConfirmation(ticket, user)

	// Start SLA timer
	go ss.startSLATimer(ctx, ticket)

	return ticket, nil
}

// AddComment adds a comment to a ticket
func (ss *SupportService) AddComment(ctx context.Context, req *AddCommentRequest) (*Comment, error) {
	// Get ticket
	ticket, err := ss.ticketStore.GetTicket(ctx, req.TicketID)
	if err != nil {
		return nil, fmt.Errorf("failed to get ticket: %v", err)
	}

	// Get user
	user, err := ss.userStore.GetUser(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %v", err)
	}

	// Create comment
	comment := &Comment{
		ID:         generateUUID(),
		TicketID:   req.TicketID,
		UserID:     req.UserID,
		Content:    req.Content,
		IsInternal: req.IsInternal,
		Metadata: map[string]interface{}{
			"user_email": user.Email,
			"user_name":  user.Name,
			"user_role":  req.UserRole,
		},
		CreatedAt: time.Now(),
	}

	// Store comment
	if err := ss.ticketStore.AddComment(ctx, comment); err != nil {
		return nil, fmt.Errorf("failed to store comment: %v", err)
	}

	// Update ticket status if needed
	if req.Status != "" && req.Status != ticket.Status {
		if err := ss.updateTicketStatus(ctx, ticket, req.Status, req.UserID); err != nil {
			log.Printf("Failed to update ticket status: %v", err)
		}
	}

	// If this is the first response, record it
	if ticket.FirstResponse == nil && !req.IsInternal {
		ticket.FirstResponse = &comment.CreatedAt
		ss.ticketStore.UpdateTicket(ctx, ticket)

		// Calculate response time
		responseTime := comment.CreatedAt.Sub(ticket.CreatedAt)
		ss.recordSLA(ctx, ticket, responseTime)
	}

	// Sync to Zendesk if applicable
	if zdID, ok := ticket.Metadata["zendesk_id"].(string); ok && ss.zendesk != nil {
		go ss.addZendeskComment(zdID, comment, user, req.IsInternal)
	}

	// Notify relevant parties
	go ss.notifyCommentAdded(ticket, comment, user, req.IsInternal)

	return comment, nil
}

// GetKnowledgeBaseArticles returns relevant articles for a query
func (ss *SupportService) GetKnowledgeBaseArticles(ctx context.Context, query string, category string, limit int) ([]Article, error) {
	// Search vector database for relevant articles
	articles, err := ss.searchArticles(ctx, query, category, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search articles: %v", err)
	}

	// Get tenant context for personalized results
	tenantCtx, _ := tenant.GetTenantContext(ctx)
	if tenantCtx != nil {
		// Filter articles based on tenant plan
		articles = ss.filterArticlesByPlan(articles, tenantCtx.Plan)
	}

	return articles, nil
}

// AutoSuggestTickets suggests similar tickets when creating a new one
func (ss *SupportService) AutoSuggestTickets(ctx context.Context, subject, description string) ([]Ticket, error) {
	// Use NLP to find similar tickets
	similarTickets, err := ss.findSimilarTickets(ctx, subject, description)
	if err != nil {
		return nil, fmt.Errorf("failed to find similar tickets: %v", err)
	}

	// Get tenant context
	tenantCtx, err := tenant.GetTenantContext(ctx)
	if err != nil {
		return similarTickets, nil
	}

	// Filter to tenant's tickets only
	var filtered []Ticket
	for _, ticket := range similarTickets {
		if ticket.TenantID == tenantCtx.TenantID {
			filtered = append(filtered, ticket)
		}
	}

	return filtered, nil
}

// SLA monitoring
func (ss *SupportService) monitorSLAs(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		// Find tickets approaching SLA deadlines
		tickets, err := ss.ticketStore.GetTicketsApproachingSLA(ctx)
		if err != nil {
			log.Printf("Failed to get tickets approaching SLA: %v", err)
			continue
		}

		for _, ticket := range tickets {
			// Calculate time remaining
			deadline := ss.calculateDeadline(ticket)
			remaining := time.Until(deadline)

			if remaining < 1*time.Hour && remaining > 0 {
				// Send escalation notification
				go ss.escalateTicket(ctx, ticket)
			} else if remaining < 0 {
				// SLA breached
				go ss.handleSLABreach(ctx, ticket)
			}
		}

		// Auto-close resolved tickets
		ss.autoCloseTickets(ctx)
	}
}

func (ss *SupportService) escalateTicket(ctx context.Context, ticket *Ticket) {
	// Get tenant
	tenant, err := ss.tenantStore.GetTenant(ctx, ticket.TenantID)
	if err != nil {
		log.Printf("Failed to get tenant for escalation: %v", err)
		return
	}

	// Determine escalation level based on priority and tenant plan
	escalationLevel := ss.getEscalationLevel(ticket.Priority, tenant.Plan)

	// Send escalation notifications
	switch escalationLevel {
	case 1:
		// Notify support team lead
		ss.slack.SendMessage("#support-leads",
			fmt.Sprintf("Ticket %s approaching SLA: %s", ticket.ID, ticket.Subject))
	case 2:
		// Notify engineering on-call
		ss.slack.SendMessage("#engineering-oncall",
			fmt.Sprintf("URGENT: Ticket %s approaching SLA: %s", ticket.ID, ticket.Subject))
	case 3:
		// Notify customer success manager
		if csm, ok := tenant.Metadata["csm_email"].(string); ok && csm != "" {
			ss.emailService.Send(csm,
				fmt.Sprintf("Critical ticket approaching SLA for %s", tenant.Name),
				ss.renderEscalationTemplate(ticket, tenant))
		}
	}

	// Update ticket metadata
	ticket.Metadata["escalated_at"] = time.Now()
	ticket.Metadata["escalation_level"] = escalationLevel
	ss.ticketStore.UpdateTicket(ctx, ticket)
}

// Helper methods

func (ss *SupportService) createZendeskTicket(ctx context.Context, t *Ticket, ten *tenant.Tenant, u *user.User) (*zendesk.Ticket, error) {
	return ss.zendesk.CreateTicket(ctx, t)
}

func (ss *SupportService) notifySlack(t *Ticket, ten *tenant.Tenant, u *user.User) {
	ss.slack.SendMessage("#support", fmt.Sprintf("New ticket from %s: %s", ten.Name, t.Subject))
}

func (ss *SupportService) sendTicketConfirmation(t *Ticket, u *user.User) {
	// ss.emailService.Send(...)
}

func (ss *SupportService) startSLATimer(ctx context.Context, t *Ticket) {
	// Logic to track SLA
}

func (ss *SupportService) updateTicketStatus(ctx context.Context, t *Ticket, status TicketStatus, userID string) error {
	t.Status = status
	t.UpdatedAt = time.Now()
	if status == StatusClosed || status == StatusResolved {
		now := time.Now()
		t.ClosedAt = &now
	}
	return ss.ticketStore.UpdateTicket(ctx, t)
}

func (ss *SupportService) recordSLA(ctx context.Context, t *Ticket, duration time.Duration) {
	// Logic to record SLA metrics
}

func (ss *SupportService) addZendeskComment(zdID string, c *Comment, u *user.User, internal bool) {
	ss.zendesk.AddComment(context.Background(), zdID, c)
}

func (ss *SupportService) notifyCommentAdded(t *Ticket, c *Comment, u *user.User, internal bool) {
	// Logic to notify user or support team
}

func (ss *SupportService) searchArticles(ctx context.Context, query, category string, limit int) ([]Article, error) {
	return []Article{}, nil
}

func (ss *SupportService) filterArticlesByPlan(articles []Article, plan string) []Article {
	return articles
}

func (ss *SupportService) findSimilarTickets(ctx context.Context, subject, description string) ([]Ticket, error) {
	return []Ticket{}, nil
}

func (ss *SupportService) calculateDeadline(t *Ticket) time.Time {
	duration := ss.config.DefaultResponseTime[t.Priority]
	return t.CreatedAt.Add(duration)
}

func (ss *SupportService) autoCloseTickets(ctx context.Context) {
	// Logic to auto-close resolved tickets after X days
}

func (ss *SupportService) getEscalationLevel(p TicketPriority, plan string) int {
	if p == PriorityUrgent {
		return 3
	}
	if p == PriorityHigh {
		return 2
	}
	return 1
}

func (ss *SupportService) renderEscalationTemplate(t *Ticket, ten *tenant.Tenant) string {
	return fmt.Sprintf("Ticket %s escalated", t.ID)
}

func (ss *SupportService) handleSLABreach(ctx context.Context, t *Ticket) {
	// Logic for SLA breach
}
