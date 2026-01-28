package support

import (
	"context"
	"time"
)

type TicketStore interface {
	CreateTicket(ctx context.Context, ticket *Ticket) error
	GetTicket(ctx context.Context, id string) (*Ticket, error)
	UpdateTicket(ctx context.Context, ticket *Ticket) error
	AddComment(ctx context.Context, comment *Comment) error
	GetTicketsApproachingSLA(ctx context.Context) ([]*Ticket, error)
}

type SupportConfig struct {
	DefaultResponseTime map[TicketPriority]time.Duration
	AutoCloseDays       int
}

type CreateTicketRequest struct {
	TenantID    string         `json:"tenant_id"`
	UserID      string         `json:"user_id"`
	Subject     string         `json:"subject"`
	Description string         `json:"description"`
	Priority    TicketPriority `json:"priority"`
	Type        TicketType     `json:"type"`
	Category    string         `json:"category"`
	Tags        []string       `json:"tags"`
}

type AddCommentRequest struct {
	TicketID   string       `json:"ticket_id"`
	UserID     string       `json:"user_id"`
	Content    string       `json:"content"`
	IsInternal bool         `json:"is_internal"`
	Status     TicketStatus `json:"status"`
	UserRole   string       `json:"user_role"`
}

type Comment struct {
	ID         string                 `json:"id"`
	TicketID   string                 `json:"ticket_id"`
	UserID     string                 `json:"user_id"`
	Content    string                 `json:"content"`
	IsInternal bool                   `json:"is_internal"`
	Metadata   map[string]interface{} `json:"metadata"`
	CreatedAt  time.Time              `json:"created_at"`
}

type Article struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Content  string `json:"content"`
	Category string `json:"category"`
	Plan     string `json:"plan"`
}
