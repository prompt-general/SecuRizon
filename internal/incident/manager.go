package incident

import (
    "context"
    "fmt"
    "time"
)

type IncidentManager struct {
    incidentStore IncidentStore
    notifier      Notifier
    escalation    EscalationManager
    slaTracker    SLATracker
}

type Incident struct {
    ID               string            `json:"id"`
    Title            string            `json:"title"`
    Description      string            `json:"description"`
    Severity         string            `json:"severity"` // critical, high, medium, low
    Status           string            `json:"status"`   // open, investigating, mitigated, resolved, closed
    CreatedAt        time.Time         `json:"created_at"`
    DetectedAt       time.Time         `json:"detected_at"`
    MitigatedAt      *time.Time        `json:"mitigated_at,omitempty"`
    ResolvedAt       *time.Time        `json:"resolved_at,omitempty"`
    ClosedAt         *time.Time        `json:"closed_at,omitempty"`
    AssignedTo       string            `json:"assigned_to,omitempty"`
    RelatedFindings  []string          `json:"related_findings"`
    RelatedAssets    []string          `json:"related_assets"`
    Tags             map[string]string `json:"tags"`
    Timeline         []TimelineEvent   `json:"timeline"`
    SLATimers        SLATimers         `json:"sla_timers"`
}

type SLATimers struct {
    ResponseSLA    time.Duration `json:"response_sla"`
    MitigationSLA  time.Duration `json:"mitigation_sla"`
    ResolutionSLA  time.Duration `json:"resolution_sla"`
    ResponseDueAt  *time.Time    `json:"response_due_at,omitempty"`
    MitigationDueAt *time.Time   `json:"mitigation_due_at,omitempty"`
    ResolutionDueAt *time.Time   `json:"resolution_due_at,omitempty"`
}

func (im *IncidentManager) CreateIncident(ctx context.Context, incident Incident) (string, error) {
    // Set timers based on severity
    im.setSLATimers(&incident)

    // Create incident in store
    incidentID, err := im.incidentStore.CreateIncident(ctx, incident)
    if err != nil {
        return "", fmt.Errorf("failed to create incident: %v", err)
    }

    // Notify relevant teams
    im.notifyTeams(ctx, incident)

    // Start SLA monitoring
    go im.monitorSLA(ctx, incidentID, incident.SLATimers)

    return incidentID, nil
}

func (im *IncidentManager) setSLATimers(incident *Incident) {
    now := time.Now()
    switch incident.Severity {
    case "critical":
        incident.SLATimers = SLATimers{
            ResponseSLA:    5 * time.Minute,
            MitigationSLA:  30 * time.Minute,
            ResolutionSLA:  4 * time.Hour,
            ResponseDueAt:  timePtr(now.Add(5 * time.Minute)),
            MitigationDueAt: timePtr(now.Add(30 * time.Minute)),
            ResolutionDueAt: timePtr(now.Add(4 * time.Hour)),
        }
    case "high":
        incident.SLATimers = SLATimers{
            ResponseSLA:   15 * time.Minute,
            MitigationSLA: 2 * time.Hour,
            ResolutionSLA: 24 * time.Hour,
        }
    case "medium":
        incident.SLATimers = SLATimers{
            ResponseSLA:   1 * time.Hour,
            MitigationSLA: 8 * time.Hour,
            ResolutionSLA: 7 * 24 * time.Hour,
        }
    default:
        incident.SLATimers = SLATimers{
            ResponseSLA:   4 * time.Hour,
            MitigationSLA: 24 * time.Hour,
            ResolutionSLA: 30 * 24 * time.Hour,
        }
    }
}

func (im *IncidentManager) monitorSLA(ctx context.Context, incidentID string, timers SLATimers) {
    // Check response SLA
    if timers.ResponseDueAt != nil {
        timer := time.NewTimer(time.Until(*timers.ResponseDueAt))
        <-timer.C
        incident, err := im.incidentStore.GetIncident(ctx, incidentID)
        if err != nil {
            return
        }
        if incident.Status == "open" {
            im.handleSLABreach(ctx, incidentID, "response")
        }
    }
    // Additional timers for mitigation and resolution could be added similarly
}

func (im *IncidentManager) handleSLABreach(ctx context.Context, incidentID string, slaType string) {
    incident, err := im.incidentStore.GetIncident(ctx, incidentID)
    if err != nil {
        return
    }
    event := TimelineEvent{
        Timestamp: time.Now(),
        Type:      "sla_breach",
        Details:   fmt.Sprintf("%s SLA breached", slaType),
        Actor:     "system",
    }
    im.incidentStore.AddTimelineEvent(ctx, incidentID, event)
    im.escalation.EscalateIncident(ctx, incident, slaType)
    im.notifier.NotifySLABreach(ctx, incident, slaType)
}

// Helper to get pointer to time
func timePtr(t time.Time) *time.Time { return &t }

// Placeholder interfaces – to be implemented elsewhere
type IncidentStore interface {
    CreateIncident(ctx context.Context, incident Incident) (string, error)
    GetIncident(ctx context.Context, id string) (Incident, error)
    AddTimelineEvent(ctx context.Context, incidentID string, event TimelineEvent) error
}

type Notifier interface {
    NotifySLABreach(ctx context.Context, incident Incident, slaType string) error
}

type EscalationManager interface {
    EscalateIncident(ctx context.Context, incident Incident, slaType string) error
}

type SLATracker interface {}

type TimelineEvent struct {
    Timestamp time.Time `json:"timestamp"`
    Type      string    `json:"type"`
    Details   string    `json:"details"`
    Actor     string    `json:"actor"`
}
