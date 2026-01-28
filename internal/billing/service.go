package billing

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/securizon/internal/email"
	"github.com/securizon/internal/tenant"
	"github.com/stripe/stripe-go/v74"
	"github.com/stripe/stripe-go/v74/webhook"
	"github.com/stripe/stripe-go/v74/usage"
)

type BillingService struct {
	stripeKey        string
	webhookSecret    string
	tenantStore      tenant.Store
	usageService     *UsageService
	emailService     *email.Service
	plans            map[string]Plan
	featureTiers     map[string]FeatureTier
}

type Plan struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Description      string            `json:"description"`
	StripePriceID    string            `json:"stripe_price_id"`
	MonthlyPrice     int64             `json:"monthly_price"` // in cents
	AnnualPrice      int64             `json:"annual_price"`
	Features         []Feature         `json:"features"`
	Limits           PlanLimits        `json:"limits"`
	Metadata         map[string]string `json:"metadata"`
}

type Feature struct {
	Name string `json:"name"`
}

type PlanLimits struct {
	MaxAssets       int `json:"max_assets"`
	MaxUsers        int `json:"max_users"`
	MaxFindings     int `json:"max_findings"`
}

type FeatureTier struct {
	Feature      string  `json:"feature"`
	Unit         string  `json:"unit"` // "asset", "finding", "user", "gb"
	BasePrice    int64   `json:"base_price"` // in cents
	UnitPrice    int64   `json:"unit_price"` // price per unit over included
	Included     int64   `json:"included"`   // included in base plan
	Max          int64   `json:"max"`        // maximum allowed
}

type UsageRecord struct {
	TenantID  string    `json:"tenant_id"`
	Feature   string    `json:"feature"`
	Quantity  int64     `json:"quantity"`
	Timestamp time.Time `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata"`
}

type InvoiceEstimate struct {
	TenantID     string        `json:"tenant_id"`
	PeriodStart  time.Time     `json:"period_start"`
	PeriodEnd    time.Time     `json:"period_end"`
	Items        []InvoiceItem `json:"items"`
	Currency     string        `json:"currency"`
	Subtotal     int64         `json:"subtotal"`
	Discount     int64         `json:"discount"`
	Tax          int64         `json:"tax"`
	Total        int64         `json:"total"`
}

type InvoiceItem struct {
	Description string `json:"description"`
	Quantity    int64  `json:"quantity"`
	UnitPrice   int64  `json:"unit_price"`
	Amount      int64  `json:"amount"`
	Type        string `json:"type"` // "plan" or "usage"
}

func NewBillingService(stripeKey, webhookSecret string, tenantStore tenant.Store) *BillingService {
	stripe.Key = stripeKey
	
	return &BillingService{
		stripeKey:     stripeKey,
		webhookSecret: webhookSecret,
		tenantStore:   tenantStore,
		usageService:  NewUsageService(),
		emailService:  email.NewService(),
		plans:         loadPlans(),
		featureTiers:  loadFeatureTiers(),
	}
}

// RecordUsage records usage for a tenant and creates Stripe usage records
func (bs *BillingService) RecordUsage(ctx context.Context, usageRecord *UsageRecord) error {
	// Get tenant
	tenant, err := bs.tenantStore.GetTenant(ctx, usageRecord.TenantID)
	if err != nil {
		return fmt.Errorf("failed to get tenant: %v", err)
	}
	
	// Check if feature is metered
	_, exists := bs.featureTiers[usageRecord.Feature]
	if !exists {
		// Feature is not metered, included in plan
		return nil
	}
	
	// Get Stripe subscription item ID for this feature
	subItemID, err := bs.getSubscriptionItemID(ctx, tenant, usageRecord.Feature)
	if err != nil {
		return fmt.Errorf("failed to get subscription item: %v", err)
	}
	
	// Record usage in Stripe
	params := &stripe.UsageRecordParams{
		SubscriptionItem: stripe.String(subItemID),
		Quantity:         stripe.Int64(usageRecord.Quantity),
		Timestamp:        stripe.Int64(usageRecord.Timestamp.Unix()),
		Action:           stripe.String("set"), // or "increment"
	}
	
	if _, err := usage.New(params); err != nil {
		return fmt.Errorf("failed to record usage in Stripe: %v", err)
	}
	
	// Update local usage tracking
	if err := bs.usageService.RecordUsage(ctx, usageRecord); err != nil {
		log.Printf("Failed to record usage locally: %v", err)
	}
	
	return nil
}

// CalculateInvoice calculates estimated invoice for a tenant
func (bs *BillingService) CalculateInvoice(ctx context.Context, tenantID string, periodStart, periodEnd time.Time) (*InvoiceEstimate, error) {
	tenant, err := bs.tenantStore.GetTenant(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant: %v", err)
	}
	
	invoice := &InvoiceEstimate{
		TenantID:     tenantID,
		PeriodStart:  periodStart,
		PeriodEnd:    periodEnd,
		Items:        make([]InvoiceItem, 0),
		Currency:     "usd",
	}
	
	// Base plan charge
	plan := bs.plans[tenant.Plan]
	invoice.Items = append(invoice.Items, InvoiceItem{
		Description: fmt.Sprintf("%s Plan", plan.Name),
		Quantity:    1,
		UnitPrice:   plan.MonthlyPrice,
		Amount:      plan.MonthlyPrice,
		Type:        "plan",
	})
	
	// Metered usage charges
	usage, err := bs.usageService.GetUsage(ctx, tenantID, periodStart, periodEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage: %v", err)
	}
	
	for feature, quantity := range usage {
		tier, exists := bs.featureTiers[feature]
		if !exists {
			continue
		}
		
		// Calculate overage
		overage := quantity - tier.Included
		if overage > 0 {
			charge := overage * tier.UnitPrice
			invoice.Items = append(invoice.Items, InvoiceItem{
				Description: fmt.Sprintf("%s Overage (%d units)", feature, overage),
				Quantity:    overage,
				UnitPrice:   tier.UnitPrice,
				Amount:      charge,
				Type:        "usage",
			})
		}
	}
	
	// Calculate totals
	for _, item := range invoice.Items {
		invoice.Subtotal += item.Amount
	}
	
	// Apply discounts
	invoice.Discount = bs.calculateDiscounts(tenant)
	invoice.Tax = bs.calculateTax(invoice.Subtotal - invoice.Discount)
	invoice.Total = invoice.Subtotal - invoice.Discount + invoice.Tax
	
	return invoice, nil
}

// HandleWebhook processes Stripe webhook events
func (bs *BillingService) HandleWebhook(ctx context.Context, payload []byte, signature string) error {
	event, err := webhook.ConstructEvent(payload, signature, bs.webhookSecret)
	if err != nil {
		return fmt.Errorf("webhook signature verification failed: %v", err)
	}
	
	switch event.Type {
	case "invoice.payment_succeeded":
		return bs.handleInvoicePaymentSucceeded(ctx, event.Data.Object)
	case "invoice.payment_failed":
		return bs.handleInvoicePaymentFailed(ctx, event.Data.Object)
	case "customer.subscription.updated":
		// Stripe Go SDK uses pointers for event data, need to cast
		var subscription stripe.Subscription
		// In a real app, unmarshal event.Data.Raw into subscription
		// For now, we assume it works or use a helper
		return bs.handleSubscriptionUpdated(ctx, &subscription)
	case "customer.subscription.deleted":
		return bs.handleSubscriptionDeleted(ctx, event.Data.Object)
	case "checkout.session.completed":
		return bs.handleCheckoutCompleted(ctx, event.Data.Object)
	}
	
	return nil
}

func (bs *BillingService) handleSubscriptionUpdated(ctx context.Context, subscription *stripe.Subscription) error {
	// Get tenant from metadata
	tenantID := subscription.Metadata["tenant_id"]
	if tenantID == "" {
		return fmt.Errorf("subscription missing tenant_id metadata")
	}
	
	tenant, err := bs.tenantStore.GetTenant(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("failed to get tenant: %v", err)
	}
	
	// Update tenant subscription info
	tenant.Subscription = tenant.SubscriptionInfo{
		StripeSubscriptionID: subscription.ID,
		Status:               string(subscription.Status),
		CurrentPeriodStart:   time.Unix(subscription.CurrentPeriodStart, 0),
		CurrentPeriodEnd:     time.Unix(subscription.CurrentPeriodEnd, 0),
		CancelAtPeriodEnd:    subscription.CancelAtPeriodEnd,
	}
	
	// If plan changed, update tenant plan
	if len(subscription.Items.Data) > 0 {
		priceID := subscription.Items.Data[0].Price.ID
		newPlan := bs.getPlanByPriceID(priceID)
		if newPlan != "" && newPlan != tenant.Plan {
			tenant.Plan = newPlan
			// Update tenant limits based on new plan
			tenant.Limits = bs.getPlanLimits(newPlan)
		}
	}
	
	// Handle cancellation
	if subscription.Status == stripe.SubscriptionStatusCanceled || 
	   subscription.Status == stripe.SubscriptionStatusUnpaid {
		
		// Downgrade to free plan or suspend
		tenant.Status = tenant.TenantStatusSuspended
		
		// Send notification
		go bs.sendSubscriptionCancelledEmail(tenant)
	}
	
	return bs.tenantStore.UpdateTenant(ctx, tenant)
}

// Usage-based billing for different features
func (bs *BillingService) syncUsageToStripe(ctx context.Context) error {
	// Get all active tenants
	tenants, err := bs.tenantStore.ListActiveTenants(ctx)
	if err != nil {
		return fmt.Errorf("failed to list tenants: %v", err)
	}
	
	for _, tenant := range tenants {
		// Skip if no Stripe subscription
		if tenant.Subscription.StripeSubscriptionID == "" {
			continue
		}
		
		// Calculate usage for the current billing period
		periodStart := tenant.Subscription.CurrentPeriodStart
		periodEnd := time.Now()
		
		usage, err := bs.usageService.GetUsage(ctx, tenant.ID, periodStart, periodEnd)
		if err != nil {
			log.Printf("Failed to get usage for tenant %s: %v", tenant.ID, err)
			continue
		}
		
		// Record usage in Stripe for each metered feature
		for feature, quantity := range usage {
			if err := bs.RecordUsage(ctx, &UsageRecord{
				TenantID:  tenant.ID,
				Feature:   feature,
				Quantity:  quantity,
				Timestamp: time.Now(),
			}); err != nil {
				log.Printf("Failed to record usage for tenant %s feature %s: %v", 
					tenant.ID, feature, err)
			}
		}
	}
	
	return nil
}

// Helper methods

func loadPlans() map[string]Plan {
	return map[string]Plan{
		"starter": {
			ID: "starter",
			Name: "Starter",
			MonthlyPrice: 2900,
			Limits: PlanLimits{MaxAssets: 100},
		},
		"pro": {
			ID: "pro",
			Name: "Pro",
			MonthlyPrice: 9900,
			Limits: PlanLimits{MaxAssets: 1000},
		},
	}
}

func loadFeatureTiers() map[string]FeatureTier {
	return map[string]FeatureTier{
		"assets": {
			Feature: "assets",
			Unit: "asset",
			UnitPrice: 10, // 10 cents per asset
			Included: 100,
		},
	}
}

func (bs *BillingService) getSubscriptionItemID(ctx context.Context, t *tenant.Tenant, feature string) (string, error) {
	// In a real implementation, this would query Stripe or a local cache
	// to find the subscription item ID for the given feature price
	return "si_fake123", nil
}

func (bs *BillingService) getPlanByPriceID(priceID string) string {
	// Reverse lookup
	return "pro" 
}

func (bs *BillingService) getPlanLimits(planID string) tenant.TenantLimits {
	// Convert PlanLimits to TenantLimits
	return tenant.TenantLimits{MaxAssets: 1000}
}

func (bs *BillingService) calculateDiscounts(t *tenant.Tenant) int64 {
	return 0
}

func (bs *BillingService) calculateTax(amount int64) int64 {
	return 0
}

func (bs *BillingService) sendSubscriptionCancelledEmail(t *tenant.Tenant) {
	bs.emailService.SendSubscriptionCancelledEmail(t)
}

func (bs *BillingService) handleInvoicePaymentSucceeded(ctx context.Context, obj map[string]interface{}) error {
	return nil
}

func (bs *BillingService) handleInvoicePaymentFailed(ctx context.Context, obj map[string]interface{}) error {
	return nil
}

func (bs *BillingService) handleSubscriptionDeleted(ctx context.Context, obj map[string]interface{}) error {
	return nil
}

func (bs *BillingService) handleCheckoutCompleted(ctx context.Context, obj map[string]interface{}) error {
	return nil
}
