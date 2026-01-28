package tenant

import (
    "context"
    "fmt"
    "time"
)

// Tenant represents a customer organization
type Tenant struct {
    ID                 string                 `json:"id"`
    Name               string                 `json:"name"`
    Slug               string                 `json:"slug"`
    Plan               string                 `json:"plan"`
    Status             TenantStatus           `json:"status"`
    CreatedAt          time.Time              `json:"created_at"`
    UpdatedAt          time.Time              `json:"updated_at"`
    Metadata           map[string]interface{} `json:"metadata"`
    Limits             TenantLimits           `json:"limits"`
    Features           TenantFeatures         `json:"features"`
    Subscription       SubscriptionInfo       `json:"subscription"`
    BillingInfo        BillingInfo            `json:"billing_info"`
    ContactInfo        ContactInfo            `json:"contact_info"`
}

type TenantStatus string

const (
    TenantStatusActive      TenantStatus = "active"
    TenantStatusSuspended   TenantStatus = "suspended"
    TenantStatusPending     TenantStatus = "pending"
    TenantStatusCancelled   TenantStatus = "cancelled"
    TenantStatusOnboarding  TenantStatus = "onboarding"
)

type TenantLimits struct {
    MaxAssets          int `json:"max_assets"`
    MaxUsers           int `json:"max_users"`
    MaxFindings        int `json:"max_findings"`
    MaxAPICalls        int `json:"max_api_calls"`
    DataRetentionDays  int `json:"data_retention_days"`
    MaxCollections     int `json:"max_collections"` // API collections
    MaxPlaybooks       int `json:"max_playbooks"`
    MaxIntegrations    int `json:"max_integrations"`
}

type TenantFeatures struct {
    RealTimeMonitoring bool `json:"real_time_monitoring"`
    AttackPathAnalysis bool `json:"attack_path_analysis"`
    AutoRemediation    bool `json:"auto_remediation"`
    APIAccess          bool `json:"api_access"`
    SSO                bool `json:"sso"`
    CustomPolicies     bool `json:"custom_policies"`
    ComplianceReports  bool `json:"compliance_reports"`
    AdvancedAnalytics  bool `json:"advanced_analytics"`
}

type IsolationLevel string

const (
    IsolationShared     IsolationLevel = "shared"     // Shared database, schema-based isolation
    IsolationDedicated  IsolationLevel = "dedicated"  // Dedicated database per tenant
    IsolationEnterprise IsolationLevel = "enterprise" // Fully isolated infrastructure
)

// Placeholder structs for Subscription, Billing, Contact, and User
type SubscriptionInfo struct{}
type BillingInfo struct{}
type ContactInfo struct{}
type User struct {
    ID   string
    Role string
}

// TenantContext carries tenant information through requests
type TenantContext struct {
    TenantID       string
    Slug           string
    Plan           string
    Features       TenantFeatures
    IsolationLevel IsolationLevel
    DatabaseName   string // For dedicated isolation
    KafkaPrefix    string
    RequestID      string
    UserID         string
    UserRole       string
}

// Context key for tenant context
type contextKey string

const TenantContextKey contextKey = "tenant_context"

func NewTenantContext(tenant *Tenant, user *User) *TenantContext {
    return &TenantContext{
        TenantID:       tenant.ID,
        Slug:           tenant.Slug,
        Plan:           tenant.Plan,
        Features:       tenant.Features,
        IsolationLevel: determineIsolationLevel(tenant.Plan),
        DatabaseName:   getDatabaseName(tenant),
        KafkaPrefix:    fmt.Sprintf("tenant_%s", tenant.Slug),
        UserID:         user.ID,
        UserRole:       user.Role,
    }
}

func WithTenantContext(ctx context.Context, tenantCtx *TenantContext) context.Context {
    return context.WithValue(ctx, TenantContextKey, tenantCtx)
}

func GetTenantContext(ctx context.Context) (*TenantContext, error) {
    value := ctx.Value(TenantContextKey)
    if value == nil {
        return nil, fmt.Errorf("no tenant context found")
    }

    tenantCtx, ok := value.(*TenantContext)
    if !ok {
        return nil, fmt.Errorf("invalid tenant context type")
    }

    return tenantCtx, nil
}

func determineIsolationLevel(plan string) IsolationLevel {
    switch plan {
    case "enterprise":
        return IsolationEnterprise
    case "dedicated":
        return IsolationDedicated
    default:
        return IsolationShared
    }
}

func getDatabaseName(tenant *Tenant) string {
    isolation := determineIsolationLevel(tenant.Plan)
    if isolation == IsolationDedicated || isolation == IsolationEnterprise {
        return fmt.Sprintf("db_%s", tenant.Slug)
    }
    return "db_shared"
}
