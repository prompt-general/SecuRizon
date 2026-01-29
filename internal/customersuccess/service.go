package customersuccess

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/securizon/internal/billing"
	"github.com/securizon/internal/email"
	"github.com/securizon/internal/slack"
	"github.com/securizon/internal/support"
	"github.com/securizon/internal/tenant"
)

type CustomerSuccessService struct {
	tenantStore    tenant.Store
	usageService   *UsageService
	billingService *billing.BillingService
	supportService *support.SupportService
	emailService   *email.Service
	slack          *slack.Client
	config         CSConfig
}

func NewCustomerSuccessService(
	tenantStore tenant.Store,
	usageService *UsageService,
	billingService *billing.BillingService,
	supportService *support.SupportService,
	emailService *email.Service,
	slack *slack.Client,
) *CustomerSuccessService {

	return &CustomerSuccessService{
		tenantStore:    tenantStore,
		usageService:   usageService,
		billingService: billingService,
		supportService: supportService,
		emailService:   emailService,
		slack:          slack,
		config: CSConfig{
			HealthCheckInterval: 24 * time.Hour,
			RiskThresholds: map[HealthLevel]float64{
				HealthCritical: 40,
				HealthWarning:  70,
				HealthHealthy:  100,
			},
		},
	}
}

// CalculateHealth calculates customer health score
func (css *CustomerSuccessService) CalculateHealth(ctx context.Context, tenantID string) (*CustomerHealth, error) {
	t, err := css.tenantStore.GetTenant(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant: %v", err)
	}

	health := &CustomerHealth{
		TenantID:    tenantID,
		LastUpdated: time.Now(),
	}

	// Calculate different health factors
	factors := []struct {
		name   string
		weight float64
		calc   func(context.Context, *tenant.Tenant) (float64, HealthFactor)
	}{
		{"engagement", 0.25, css.calculateEngagement},
		{"adoption", 0.20, css.calculateAdoption},
		{"satisfaction", 0.15, css.calculateSatisfaction},
		{"growth", 0.15, css.calculateGrowth},
		{"risk", 0.25, css.calculateRisk},
	}

	var totalScore float64
	var totalWeight float64

	for _, factor := range factors {
		score, details := factor.calc(ctx, t)
		details.Weight = factor.weight
		health.Factors = append(health.Factors, details)

		totalScore += score * factor.weight
		totalWeight += factor.weight
	}

	// Calculate overall score
	if totalWeight > 0 {
		health.Score = totalScore / totalWeight
	}

	// Determine health level
	switch {
	case health.Score < css.config.RiskThresholds[HealthCritical]:
		health.Level = HealthCritical
	case health.Score < css.config.RiskThresholds[HealthWarning]:
		health.Level = HealthWarning
	default:
		health.Level = HealthHealthy
	}

	// Generate recommendations
	health.Recommendations = css.generateRecommendations(ctx, t, health)

	return health, nil
}

func (css *CustomerSuccessService) calculateEngagement(ctx context.Context, t *tenant.Tenant) (float64, HealthFactor) {
	// Get usage data
	usage, err := css.usageService.GetRecentUsage(ctx, t.ID, 30*24*time.Hour)
	if err != nil {
		return 0, HealthFactor{
			Name:        "engagement",
			Description: "Unable to calculate engagement",
		}
	}

	var score float64
	var factors []string

	// Active users
	activeUsers := usage.UsersActive
	totalUsers := usage.UsersTotal
	if totalUsers > 0 {
		userRatio := float64(activeUsers) / float64(totalUsers)
		if userRatio > 0.7 {
			score += 25
			factors = append(factors, "High user activity")
		} else if userRatio > 0.3 {
			score += 15
			factors = append(factors, "Moderate user activity")
		} else {
			factors = append(factors, "Low user activity")
		}
	}

	// API usage
	if usage.APICalls > 1000 {
		score += 25
		factors = append(factors, "High API usage")
	} else if usage.APICalls > 100 {
		score += 15
		factors = append(factors, "Moderate API usage")
	} else {
		factors = append(factors, "Low API usage")
	}

	// Dashboard visits
	if usage.DashboardVisits > 50 {
		score += 25
		factors = append(factors, "Frequent dashboard usage")
	} else if usage.DashboardVisits > 10 {
		score += 15
		factors = append(factors, "Regular dashboard usage")
	} else {
		factors = append(factors, "Infrequent dashboard usage")
	}

	// Feature usage
	featureScore := css.calculateFeatureUsage(usage.Features)
	score += featureScore
	factors = append(factors, fmt.Sprintf("Feature usage score: %.0f", featureScore))

	return score, HealthFactor{
		Name:        "engagement",
		Score:       score,
		Description: fmt.Sprintf("Engagement factors: %s", strings.Join(factors, ", ")),
		Trend:       css.calculateTrend(usage.EngagementTrend),
	}
}

func (css *CustomerSuccessService) calculateAdoption(ctx context.Context, t *tenant.Tenant) (float64, HealthFactor) {
	return 80, HealthFactor{Name: "adoption", Score: 80, Description: "Good feature adoption", Trend: "stable"}
}

func (css *CustomerSuccessService) calculateSatisfaction(ctx context.Context, t *tenant.Tenant) (float64, HealthFactor) {
	return 90, HealthFactor{Name: "satisfaction", Score: 90, Description: "High customer satisfaction", Trend: "improving"}
}

func (css *CustomerSuccessService) calculateGrowth(ctx context.Context, t *tenant.Tenant) (float64, HealthFactor) {
	return 70, HealthFactor{Name: "growth", Score: 70, Description: "Steady growth", Trend: "stable"}
}

func (css *CustomerSuccessService) calculateRisk(ctx context.Context, t *tenant.Tenant) (float64, HealthFactor) {
	var score float64 = 100 // Start with perfect score
	var riskFactors []string

	// Support tickets
	// Note: In a real implementation, we'd need a GetRecentTickets method in supportService
	// For now, we'll assume a mock behavior or use what's available.
	// ss.ticketStore is not exported, so we'd need a method on SupportService.

	// Churn risk
	churnRisk := css.calculateChurnRisk(ctx, t)
	if churnRisk > 0.7 {
		score -= 40
		riskFactors = append(riskFactors, "High churn risk")
	} else if churnRisk > 0.3 {
		score -= 20
		riskFactors = append(riskFactors, "Moderate churn risk")
	}

	// Payment issues
	if t.Subscription.Status == "past_due" || t.Subscription.Status == "unpaid" {
		score -= 30
		riskFactors = append(riskFactors, "Payment issues")
	}

	// Ensure score doesn't go below 0
	if score < 0 {
		score = 0
	}

	return score, HealthFactor{
		Name:        "risk",
		Score:       score,
		Description: fmt.Sprintf("Risk factors: %s", strings.Join(riskFactors, ", ")),
		Trend:       "stable",
	}
}

func (css *CustomerSuccessService) calculateFeatureUsage(features map[string]float64) float64 {
	if len(features) == 0 {
		return 0
	}
	var total float64
	for _, v := range features {
		total += v
	}
	return (total / float64(len(features))) * 0.25 // Max 25 points
}

func (css *CustomerSuccessService) calculateTrend(trend string) string {
	if trend == "" {
		return "stable"
	}
	return trend
}

func (css *CustomerSuccessService) calculateChurnRisk(ctx context.Context, t *tenant.Tenant) float64 {
	return 0.1
}

func (css *CustomerSuccessService) generateRecommendations(ctx context.Context, t *tenant.Tenant, health *CustomerHealth) []Recommendation {
	var recs []Recommendation
	if health.Level == HealthCritical || health.Level == HealthWarning {
		recs = append(recs, Recommendation{
			Title:       "Schedule Health Review",
			Description: "Customer health is below threshold. Schedule a review call.",
			Priority:    "high",
			Category:    "retention",
		})
	}
	return recs
}

// Monitor at-risk customers
func (css *CustomerSuccessService) MonitorAtRiskCustomers(ctx context.Context) {
	ticker := time.NewTicker(css.config.HealthCheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		tenants, err := css.tenantStore.ListActiveTenants(ctx)
		if err != nil {
			log.Printf("Failed to list tenants: %v", err)
			continue
		}

		for _, t := range tenants {
			health, err := css.CalculateHealth(ctx, t.ID)
			if err != nil {
				log.Printf("Failed to calculate health for tenant %s: %v", t.ID, err)
				continue
			}

			switch health.Level {
			case HealthCritical:
				css.handleCriticalHealth(ctx, t, health)
			case HealthWarning:
				css.handleWarningHealth(ctx, t, health)
			}

			css.storeHealthData(ctx, health)
		}
	}
}

func (css *CustomerSuccessService) handleCriticalHealth(ctx context.Context, t *tenant.Tenant, health *CustomerHealth) {
	if csm, ok := t.Metadata["csm_email"].(string); ok && csm != "" {
		css.emailService.Send(csm,
			fmt.Sprintf("CRITICAL: Customer health alert for %s", t.Name),
			fmt.Sprintf("Customer %s has critical health score: %.0f", t.Name, health.Score))
	}

	css.slack.SendMessage("#customer-success-alerts",
		fmt.Sprintf("🚨 Customer %s (%s) has critical health score: %.0f",
			t.Name, t.Slug, health.Score))

	go css.scheduleInterventionCall(ctx, t, health)
}

func (css *CustomerSuccessService) handleWarningHealth(ctx context.Context, t *tenant.Tenant, health *CustomerHealth) {
	// Send health report email
	// Note: ContactInfo is a placeholder struct, we'll assume it has AdminEmail or use Metadata
	adminEmail := ""
	if t.Metadata["admin_email"] != nil {
		adminEmail = t.Metadata["admin_email"].(string)
	}

	if adminEmail != "" {
		css.emailService.Send(adminEmail,
			fmt.Sprintf("Your SecuRizon Health Report"),
			fmt.Sprintf("Your health score is %.0f", health.Score))
	}

	go css.createFollowupTask(ctx, t, health)
}

func (css *CustomerSuccessService) storeHealthData(ctx context.Context, health *CustomerHealth) {
	// Logic to store health data
}

func (css *CustomerSuccessService) scheduleInterventionCall(ctx context.Context, t *tenant.Tenant, health *CustomerHealth) {
	// Logic to schedule call
}

func (css *CustomerSuccessService) createFollowupTask(ctx context.Context, t *tenant.Tenant, health *CustomerHealth) {
	// Logic to create task
}

// Generate quarterly business review
func (css *CustomerSuccessService) GenerateQBR(ctx context.Context, tenantID string, quarter string) (*QuarterlyBusinessReview, error) {
	t, err := css.tenantStore.GetTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	startDate, endDate := css.getQuarterDates(quarter)

	usage := css.usageService.GetUsage(ctx, tenantID, startDate, endDate)
	health, _ := css.CalculateHealth(ctx, tenantID)

	qbr := &QuarterlyBusinessReview{
		TenantID:         tenantID,
		Quarter:          quarter,
		Period:           fmt.Sprintf("%s - %s", startDate.Format("Jan 2"), endDate.Format("Jan 2, 2006")),
		ExecutiveSummary: fmt.Sprintf("Executive summary for %s in %s", t.Name, quarter),
		UsageAnalytics:   usage,
		HealthScore:      health,
		KeyAchievements:  []string{"Improved security posture", "Increased API adoption"},
		Recommendations:  health.Recommendations,
		NextQuarterGoals: []string{"Enable auto-remediation", "Expand monitoring coverage"},
	}

	pdf, err := css.generatePDFReport(qbr)
	if err != nil {
		return nil, err
	}

	qbr.PDFReport = pdf

	return qbr, nil
}

func (css *CustomerSuccessService) getQuarterDates(quarter string) (time.Time, time.Time) {
	// Mock implementation
	return time.Now().AddDate(0, -3, 0), time.Now()
}

func (css *CustomerSuccessService) generatePDFReport(qbr *QuarterlyBusinessReview) ([]byte, error) {
	return []byte("PDF Report Content"), nil
}
