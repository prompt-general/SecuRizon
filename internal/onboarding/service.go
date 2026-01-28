package onboarding

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/securazion/onboarding/internal/email"
    "github.com/securazion/onboarding/internal/kafka"
    "github.com/securazion/onboarding/internal/neo4j"
    "github.com/securazion/onboarding/internal/stripe"
    "github.com/securazion/onboarding/internal/tenant"
    "github.com/securazion/onboarding/internal/user"
    "github.com/securazion/onboarding/internal/workflow"
)

type OnboardingService struct {
    tenantStore    tenant.Store
    userStore      user.Store
    neo4jAdmin     *neo4j.AdminClient
    kafkaAdmin     *kafka.AdminClient
    stripeClient   *stripe.Client
    emailService   *email.Service
    workflowEngine *workflow.Engine
    config         OnboardingConfig
}

type OnboardingConfig struct {
    DefaultPlan             string        `yaml:"default_plan"`
    TrialDays               int           `yaml:"trial_days"`
    ProvisioningTimeout     time.Duration `yaml:"provisioning_timeout"`
    WelcomeEmailTemplate    string        `yaml:"welcome_email_template"`
    OnboardingChecklist     []string      `yaml:"onboarding_checklist"`
    AutoConnectClouds       bool          `yaml:"auto_connect_clouds"`
    RequiredIntegrations    []string      `yaml:"required_integrations"`
}

type OnboardingStep string

const (
    StepAccountCreated       OnboardingStep = "account_created"
    StepEmailVerified        OnboardingStep = "email_verified"
    StepPlanSelected         OnboardingStep = "plan_selected"
    StepPaymentProcessed     OnboardingStep = "payment_processed"
    StepResourcesProvisioned OnboardingStep = "resources_provisioned"
    StepFirstIntegration     OnboardingStep = "first_integration"
    StepDataCollected        OnboardingStep = "data_collected"
    StepFirstFinding         OnboardingStep = "first_finding"
    StepDashboardViewed      OnboardingStep = "dashboard_viewed"
    StepOnboardingComplete   OnboardingStep = "onboarding_complete"
)

// Placeholder structs for Request/Response
type OnboardingRequest struct {
    Email       string
    CompanyName string
    Plan        string
}

type OnboardingResponse struct {
    Tenant     *tenant.Tenant
    User       *user.User
    WorkflowID string
    NextSteps  []string
}

type OnboardingProgress struct {
    TenantID       string                 `json:"tenant_id"`
    Steps          map[OnboardingStep]StepProgress `json:"steps"`
    CompletedSteps int                    `json:"completed_steps"`
    TotalSteps     int                    `json:"total_steps"`
    Percentage     int                    `json:"percentage"`
    CurrentStep    OnboardingStep         `json:"current_step"`
}

type StepProgress struct {
    Completed   bool       `json:"completed"`
    CompletedAt *time.Time `json:"completed_at,omitempty"`
}

func NewOnboardingService(
    tenantStore tenant.Store,
    userStore user.Store,
    neo4jAdmin *neo4j.AdminClient,
    kafkaAdmin *kafka.AdminClient,
    stripeClient *stripe.Client,
    emailService *email.Service,
    workflowEngine *workflow.Engine,
) *OnboardingService {
    return &OnboardingService{
        tenantStore:    tenantStore,
        userStore:      userStore,
        neo4jAdmin:     neo4jAdmin,
        kafkaAdmin:     kafkaAdmin,
        stripeClient:   stripeClient,
        emailService:   emailService,
        workflowEngine: workflowEngine,
        config: OnboardingConfig{
            DefaultPlan:          "starter",
            TrialDays:            14,
            ProvisioningTimeout:  10 * time.Minute,
            WelcomeEmailTemplate: "welcome.html",
            OnboardingChecklist: []string{
                "verify_email",
                "add_team_members",
                "connect_cloud_account",
                "review_first_findings",
                "setup_notifications",
                "explore_dashboard",
            },
        },
    }
}

// Helper methods implementations placeholder...
func (os *OnboardingService) createTenant(ctx context.Context, req *OnboardingRequest) (*tenant.Tenant, error) {
    // Implement tenant creation logic
    return &tenant.Tenant{ID: "new-tenant-id", Name: req.CompanyName, Status: tenant.TenantStatusPending}, nil
}

func (os *OnboardingService) createAdminUser(ctx context.Context, t *tenant.Tenant, req *OnboardingRequest) (*user.User, error) {
    // Implement admin user creation logic
    return &user.User{ID: "admin-user", Email: req.Email}, nil
}

func (os *OnboardingService) getPriceID(plan string) string { return "price_id_placeholder" }

func (os *OnboardingService) StartOnboarding(ctx context.Context, req *OnboardingRequest) (*OnboardingResponse, error) {
    tenant, err := os.createTenant(ctx, req)
    if err != nil {
        return nil, fmt.Errorf("failed to create tenant: %v", err)
    }
    
    adminUser, err := os.createAdminUser(ctx, tenant, req)
    if err != nil {
        return nil, fmt.Errorf("failed to create admin user: %v", err)
    }
    
    var stripeCustomerID string
    if req.Plan != "trial" && os.stripeClient != nil {
        customer, err := os.stripeClient.CreateCustomer(ctx, &stripe.CustomerParams{
            Email: req.Email,
            Name:  req.CompanyName,
            Metadata: map[string]string{
                "tenant_id": tenant.ID,
                "plan":      req.Plan,
            },
        })
        if err != nil {
            log.Printf("Failed to create Stripe customer: %v", err)
        } else {
            stripeCustomerID = customer.ID
            subscription, err := os.stripeClient.CreateSubscription(ctx, &stripe.SubscriptionParams{
                Customer: stripeCustomerID,
                Items: []*stripe.SubscriptionItemsParams{{Price: os.getPriceID(req.Plan)}},
                TrialPeriodDays: int64(os.config.TrialDays),
            })
            if err != nil {
                log.Printf("Failed to create subscription: %v", err)
            } else {
                tenant.Subscription = tenant.SubscriptionInfo{
                    StripeCustomerID:     stripeCustomerID,
                    StripeSubscriptionID: subscription.ID,
                    Plan:                 req.Plan,
                    Status:               "trialing",
                    TrialEndsAt:          time.Unix(subscription.TrialEnd, 0),
                }
            }
        }
    }
    
    if stripeCustomerID != "" {
        tenant.BillingInfo = tenant.BillingInfo{StripeCustomerID: stripeCustomerID}
    }
    
    if err := os.tenantStore.UpdateTenant(ctx, tenant); err != nil {
        log.Printf("Failed to update tenant with billing info: %v", err)
    }
    
    go os.provisionTenantResources(context.Background(), tenant, adminUser)
    go os.sendWelcomeEmail(tenant, adminUser)
    
    workflowID := ""
    if os.workflowEngine != nil {
        workflowID, err = os.workflowEngine.CreateOnboardingWorkflow(ctx, tenant.ID, adminUser.ID)
        if err != nil {
            log.Printf("Failed to create onboarding workflow: %v", err)
        }
    }
    
    return &OnboardingResponse{
        Tenant:     tenant,
        User:       adminUser,
        WorkflowID: workflowID,
        NextSteps:  os.config.OnboardingChecklist,
    }, nil
}


func (os *OnboardingService) provisionTenantResources(ctx context.Context, t *tenant.Tenant, adminUser *user.User) error {
    ctx, cancel := context.WithTimeout(ctx, os.config.ProvisioningTimeout)
    defer cancel()
    tenantCtx := tenant.NewTenantContext(t, &tenant.User{ID: adminUser.ID, Role: "admin"})
    ctx = tenant.WithTenantContext(ctx, tenantCtx)

    steps := []struct {
        name string
        fn   func(context.Context) error
    }{
        {"create_database", os.createTenantDatabase},
        {"create_kafka_topics", os.createKafkaTopics},
        {"initialize_graph_schema", os.initializeGraphSchema},
        {"setup_default_policies", os.setupDefaultPolicies},
        {"create_default_playbooks", os.createDefaultPlaybooks},
        {"configure_notifications", os.configureNotifications},
    }

    for _, step := range steps {
        log.Printf("Provisioning step %s for tenant %s", step.name, t.ID)
        start := time.Now()
        if err := step.fn(ctx); err != nil {
            log.Printf("Failed step %s for tenant %s: %v", step.name, t.ID, err)
            // Ideally record failure, continuing here
            continue
        }
        _ = start
    }

    t.Status = tenant.TenantStatusActive
    t.UpdatedAt = time.Now()
    if err := os.tenantStore.UpdateTenant(ctx, t); err != nil {
        return fmt.Errorf("failed to update tenant status: %v", err)
    }
    go os.sendProvisioningCompleteEmail(t, adminUser)
    return nil
}

func (os *OnboardingService) createTenantDatabase(ctx context.Context) error {
    tenantCtx, err := tenant.GetTenantContext(ctx)
    if err != nil {
        return err
    }
    if tenantCtx.IsolationLevel == tenant.IsolationDedicated || tenantCtx.IsolationLevel == tenant.IsolationEnterprise {
        dbName := fmt.Sprintf("securazion_tenant_%s", tenantCtx.Slug)
        if os.neo4jAdmin != nil {
            if err := os.neo4jAdmin.CreateDatabase(ctx, dbName); err != nil {
                return fmt.Errorf("failed to create database: %v", err)
            }
            if err := os.neo4jAdmin.CreateDatabaseUser(ctx, dbName, tenantCtx.Slug); err != nil {
                return fmt.Errorf("failed to create database user: %v", err)
            }
        }
        tenantCtx.DatabaseName = dbName
    }
    return nil
}

func (os *OnboardingService) createKafkaTopics(ctx context.Context) error {
    if os.kafkaAdmin == nil {
        return nil
    }
    tenantCtx, err := tenant.GetTenantContext(ctx)
    if err != nil { return err }
    topics := []struct {
        name       string
        partitions int32
        replicas   int16
    }{
        {fmt.Sprintf("%s.assets", tenantCtx.KafkaPrefix), 6, 2},
        {fmt.Sprintf("%s.events", tenantCtx.KafkaPrefix), 12, 2},
        {fmt.Sprintf("%s.findings", tenantCtx.KafkaPrefix), 6, 2},
        {fmt.Sprintf("%s.remediation", tenantCtx.KafkaPrefix), 4, 2},
        {fmt.Sprintf("%s.metrics", tenantCtx.KafkaPrefix), 2, 2},
    }
    for _, topic := range topics {
        if err := os.kafkaAdmin.CreateTopic(ctx, topic.name, topic.partitions, topic.replicas); err != nil {
            return fmt.Errorf("failed to create topic %s: %v", topic.name, err)
        }
    }
    return nil
}

func (os *OnboardingService) sendWelcomeEmail(t *tenant.Tenant, adminUser *user.User) error {
    if os.emailService == nil { return nil }
    data := map[string]interface{}{
        "TenantName":    t.Name,
        "AdminName":     adminUser.Name, // Assuming Name exists on User
        "DashboardURL":  fmt.Sprintf("https://app.securazion.com/%s", t.Slug),
        "SupportEmail":  "support@securazion.com",
        "TrialDays":     os.config.TrialDays,
        "NextSteps":     os.config.OnboardingChecklist,
    }
    return os.emailService.SendTemplate(adminUser.Email, "Welcome to SecuRizon!", os.config.WelcomeEmailTemplate, data)
}

func (os *OnboardingService) sendProvisioningCompleteEmail(t *tenant.Tenant, adminUser *user.User) error {
    // Implementation placeholder
    return nil 
}

// Method stubs needed for compilation / logic flow
func (os *OnboardingService) initializeGraphSchema(ctx context.Context) error { return nil }
func (os *OnboardingService) setupDefaultPolicies(ctx context.Context) error { return nil }
func (os *OnboardingService) createDefaultPlaybooks(ctx context.Context) error { return nil }
func (os *OnboardingService) configureNotifications(ctx context.Context) error { return nil }
func (os *OnboardingService) recordProvisioningStep(ctx context.Context, name, status, errMsg string, duration time.Duration) {}

// Check functions
func (os *OnboardingService) checkFirstIntegration(t *tenant.Tenant) bool { return false }
func (os *OnboardingService) checkDataCollected(t *tenant.Tenant) bool { return false }
func (os *OnboardingService) checkFirstFinding(t *tenant.Tenant) bool { return false }
func (os *OnboardingService) checkDashboardViewed(t *tenant.Tenant) bool { return false }
func getCompletionTime(t *tenant.Tenant, step OnboardingStep) *time.Time { return nil }

func (os *OnboardingService) TrackOnboardingProgress(ctx context.Context, tenantID string) (*OnboardingProgress, error) {
    progress := &OnboardingProgress{
        TenantID: tenantID,
        Steps:    make(map[OnboardingStep]StepProgress),
    }
    t, err := os.tenantStore.GetTenant(ctx, tenantID)
    if err != nil {
        return nil, err
    }
    steps := []struct {
        step     OnboardingStep
        checkFn  func(*tenant.Tenant) bool
    }{
        {StepAccountCreated, func(t *tenant.Tenant) bool { return true }},
        {StepEmailVerified, func(t *tenant.Tenant) bool { return t.Metadata["email_verified"] == true }},
        {StepPlanSelected, func(t *tenant.Tenant) bool { return t.Plan != "" }},
        {StepPaymentProcessed, func(t *tenant.Tenant) bool { return t.Subscription.Status != "" }},
        {StepResourcesProvisioned, func(t *tenant.Tenant) bool { return t.Status == tenant.TenantStatusActive }},
        {StepFirstIntegration, os.checkFirstIntegration},
        {StepDataCollected, os.checkDataCollected},
        {StepFirstFinding, os.checkFirstFinding},
        {StepDashboardViewed, os.checkDashboardViewed},
    }
    for _, s := range steps {
        completed := s.checkFn(t)
        progress.Steps[s.step] = StepProgress{
            Completed:   completed,
            CompletedAt: getCompletionTime(t, s.step),
        }
        if completed {
            progress.CompletedSteps++
        }
    }
    progress.TotalSteps = len(steps)
    if progress.TotalSteps > 0 {
        progress.Percentage = (progress.CompletedSteps * 100) / progress.TotalSteps
    }
    // Determine current step
    allSteps := []OnboardingStep{
        StepAccountCreated, StepEmailVerified, StepPlanSelected,
        StepPaymentProcessed, StepResourcesProvisioned, StepFirstIntegration,
        StepDataCollected, StepFirstFinding, StepDashboardViewed,
    }
    for _, step := range allSteps {
        if !progress.Steps[step].Completed {
            progress.CurrentStep = step
            break
        }
    }
    if progress.CurrentStep == "" {
        progress.CurrentStep = StepOnboardingComplete
    }
    return progress, nil
}
