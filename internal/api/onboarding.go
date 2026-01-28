package api

import (
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "github.com/securazion/api/internal/onboarding"
    "github.com/securazion/api/internal/tenant"
    "github.com/securazion/api/internal/validator"
)

type OnboardingHandler struct {
    onboardingService *onboarding.OnboardingService
    tenantStore       tenant.Store
    validator         *validator.Validator
}

// POST /api/v1/onboarding/start
func (h *OnboardingHandler) StartOnboarding(w http.ResponseWriter, r *http.Request) {
    var req onboarding.OnboardingRequest
    if err := h.decodeAndValidate(r, &req); err != nil {
        h.respondError(w, http.StatusBadRequest, err)
        return
    }

    // Check if email already exists
    existing, err := h.tenantStore.GetTenantByEmail(r.Context(), req.Email)
    if err == nil && existing != nil {
        h.respondError(w, http.StatusConflict, "Email already registered")
        return
    }

    // Start onboarding
    resp, err := h.onboardingService.StartOnboarding(r.Context(), &req)
    if err != nil {
        h.respondError(w, http.StatusInternalServerError, err)
        return
    }

    h.respondJSON(w, http.StatusCreated, resp)
}

// GET /api/v1/onboarding/progress
func (h *OnboardingHandler) GetOnboardingProgress(w http.ResponseWriter, r *http.Request) {
    tenantCtx, err := tenant.GetTenantContext(r.Context())
    if err != nil {
        h.respondError(w, http.StatusUnauthorized, "Tenant context required")
        return
    }

    progress, err := h.onboardingService.TrackOnboardingProgress(r.Context(), tenantCtx.TenantID)
    if err != nil {
        h.respondError(w, http.StatusInternalServerError, err)
        return
    }

    h.respondJSON(w, http.StatusOK, progress)
}

// POST /api/v1/onboarding/complete-step
func (h *OnboardingHandler) CompleteOnboardingStep(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Step onboarding.OnboardingStep `json:"step"`
    }

    if err := h.decodeAndValidate(r, &req); err != nil {
        h.respondError(w, http.StatusBadRequest, err)
        return
    }

    tenantCtx, err := tenant.GetTenantContext(r.Context())
    if err != nil {
        h.respondError(w, http.StatusUnauthorized, "Tenant context required")
        return
    }

    // Update tenant metadata with completed step
    t, err := h.tenantStore.GetTenant(r.Context(), tenantCtx.TenantID)
    if err != nil {
        h.respondError(w, http.StatusNotFound, "Tenant not found")
        return
    }

    if t.Metadata == nil {
        t.Metadata = make(map[string]interface{})
    }

    stepKey := fmt.Sprintf("onboarding_step_%s_completed_at", req.Step)
    t.Metadata[stepKey] = time.Now()

    if err := h.tenantStore.UpdateTenant(r.Context(), t); err != nil {
        h.respondError(w, http.StatusInternalServerError, err)
        return
    }

    // If this is the "dashboard_viewed" step, mark onboarding as complete
    if req.Step == onboarding.StepDashboardViewed {
        t.Metadata["onboarding_completed_at"] = time.Now()
        h.tenantStore.UpdateTenant(r.Context(), t)
    }

    h.respondJSON(w, http.StatusOK, map[string]interface{}{
        "step":      req.Step,
        "completed": true,
        "timestamp": time.Now(),
    })
}

// GET /api/v1/onboarding/checklist
func (h *OnboardingHandler) GetOnboardingChecklist(w http.ResponseWriter, r *http.Request) {
    tenantCtx, err := tenant.GetTenantContext(r.Context())
    if err != nil {
        h.respondError(w, http.StatusUnauthorized, "Tenant context required")
        return
    }

    checklist := []struct {
        ID          string `json:"id"`
        Title       string `json:"title"`
        Description string `json:"description"`
        Completed   bool   `json:"completed"`
        ActionURL   string `json:"action_url"`
        Priority    int    `json:"priority"`
    }{
        {
            ID:          "verify_email",
            Title:       "Verify your email",
            Description: "Confirm your email address to secure your account",
            ActionURL:   fmt.Sprintf("/%s/settings/email", tenantCtx.Slug),
            Priority:    1,
        },
        {
            ID:          "add_team_members",
            Title:       "Invite your team",
            Description: "Add team members to collaborate on security findings",
            ActionURL:   fmt.Sprintf("/%s/settings/team", tenantCtx.Slug),
            Priority:    2,
        },
        {
            ID:          "connect_cloud_account",
            Title:       "Connect your cloud",
            Description: "Connect AWS, Azure, or GCP to start monitoring",
            ActionURL:   fmt.Sprintf("/%s/integrations/cloud", tenantCtx.Slug),
            Priority:    1,
        },
        {
            ID:          "review_first_findings",
            Title:       "Review findings",
            Description: "Check out the security findings from your environment",
            ActionURL:   fmt.Sprintf("/%s/findings", tenantCtx.Slug),
            Priority:    3,
        },
        {
            ID:          "setup_notifications",
            Title:       "Set up notifications",
            Description: "Configure Slack, email, or webhook notifications",
            ActionURL:   fmt.Sprintf("/%s/settings/notifications", tenantCtx.Slug),
            Priority:    2,
        },
        {
            ID:          "explore_dashboard",
            Title:       "Explore dashboard",
            Description: "Take a tour of the SecuRizon dashboard",
            ActionURL:   fmt.Sprintf("/%s/dashboard", tenantCtx.Slug),
            Priority:    3,
        },
    }

    // Mark completed items
    progress, err := h.onboardingService.TrackOnboardingProgress(r.Context(), tenantCtx.TenantID)
    if err == nil {
        for i, item := range checklist {
            switch item.ID {
            case "verify_email":
                checklist[i].Completed = progress.Steps[onboarding.StepEmailVerified].Completed
            case "connect_cloud_account":
                checklist[i].Completed = progress.Steps[onboarding.StepFirstIntegration].Completed
            case "review_first_findings":
                checklist[i].Completed = progress.Steps[onboarding.StepFirstFinding].Completed
            case "explore_dashboard":
                checklist[i].Completed = progress.Steps[onboarding.StepDashboardViewed].Completed
            }
        }
    }

    h.respondJSON(w, http.StatusOK, map[string]interface{}{
        "checklist": checklist,
        "progress":  progress,
    })
}


// Helper Helpers
func (h *OnboardingHandler) decodeAndValidate(r *http.Request, v interface{}) error {
    if err := json.NewDecoder(r.Body).Decode(v); err != nil {
        return err
    }
    // Perform validation if needed... 
    return nil
}

func (h *OnboardingHandler) respondError(w http.ResponseWriter, code int, message interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    json.NewEncoder(w).Encode(map[string]interface{}{"error": message})
}

func (h *OnboardingHandler) respondJSON(w http.ResponseWriter, code int, payload interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    json.NewEncoder(w).Encode(payload)
}
