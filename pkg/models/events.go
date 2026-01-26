package models

import (
	"time"
	"github.com/google/uuid"
)

// EventType represents the type of security event
type EventType string

const (
	EventTypeAssetCreated     EventType = "asset.created"
	EventTypeAssetUpdated     EventType = "asset.updated"
	EventTypeAssetDeleted     EventType = "asset.deleted"
	EventTypeRelationshipCreated EventType = "relationship.created"
	EventTypeRelationshipUpdated EventType = "relationship.updated"
	EventTypeRelationshipDeleted EventType = "relationship.deleted"
	EventTypeFindingCreated   EventType = "finding.created"
	EventTypeFindingUpdated   EventType = "finding.updated"
	EventTypeFindingResolved  EventType = "finding.resolved"
	EventTypePolicyViolation  EventType = "policy.violation"
	EventTypeThreatDetected   EventType = "threat.detected"
	EventTypeRiskScoreChanged EventType = "risk.score_changed"
)

// EventSeverity represents the severity of an event
type EventSeverity string

const (
	EventSeverityLow      EventSeverity = "low"
	EventSeverityMedium   EventSeverity = "medium"
	EventSeverityHigh     EventSeverity = "high"
	EventSeverityCritical EventSeverity = "critical"
)

// BaseEvent represents the base structure for all events
type BaseEvent struct {
	ID          string        `json:"id"`
	Type        EventType     `json:"type"`
	Severity    EventSeverity `json:"severity"`
	Timestamp   time.Time     `json:"timestamp"`
	Provider    Provider      `json:"provider"`
	Environment Environment   `json:"environment"`
	Source      string        `json:"source"` // Source system/service
	Actor       string        `json:"actor,omitempty"` // Who performed the action
	AssetID     string        `json:"asset_id,omitempty"`
	Description string        `json:"description"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	RawData     []byte        `json:"raw_data,omitempty"` // Original event data
}

// AssetEvent represents events related to assets
type AssetEvent struct {
	BaseEvent
	Asset      Asset   `json:"asset"`
	OldAsset   *Asset  `json:"old_asset,omitempty"` // For update events
	Changes    []FieldChange `json:"changes,omitempty"`
}

// RelationshipEvent represents events related to relationships
type RelationshipEvent struct {
	BaseEvent
	Relationship     Relationship `json:"relationship"`
	OldRelationship  *Relationship `json:"old_relationship,omitempty"` // For update events
	FromAsset        Asset         `json:"from_asset"`
	ToAsset          Asset         `json:"to_asset"`
}

// FindingEvent represents events related to findings
type FindingEvent struct {
	BaseEvent
	Finding    Finding `json:"finding"`
	OldFinding *Finding `json:"old_finding,omitempty"` // For update events
	Asset      Asset   `json:"asset"`
}

// PolicyViolationEvent represents policy violation events
type PolicyViolationEvent struct {
	BaseEvent
	PolicyID      string                 `json:"policy_id"`
	PolicyName    string                 `json:"policy_name"`
	PolicyCategory string                `json:"policy_category"`
	Severity      float64                `json:"severity"`
	Asset         Asset                  `json:"asset"`
	ViolationDetails map[string]interface{} `json:"violation_details"`
	Remediation   string                 `json:"remediation,omitempty"`
}

// ThreatEvent represents threat detection events
type ThreatEvent struct {
	BaseEvent
	ThreatType     string                 `json:"threat_type"`
	ThreatID       string                 `json:"threat_id"`
	Confidence     float64                `json:"confidence"` // 0.0-1.0
	Indicators     []ThreatIndicator      `json:"indicators"`
	AffectedAssets []Asset                `json:"affected_assets"`
	MITRETTP       string                 `json:"mitre_ttp,omitempty"`
}

// ThreatIndicator represents a threat indicator
type ThreatIndicator struct {
	Type        string                 `json:"type"` // IP, Domain, Hash, etc.
	Value       string                 `json:"value"`
	Confidence  float64                `json:"confidence"`
	Source      string                 `json:"source"`
	FirstSeen   time.Time              `json:"first_seen"`
	LastSeen    time.Time              `json:"last_seen"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// RiskScoreChangeEvent represents risk score change events
type RiskScoreChangeEvent struct {
	BaseEvent
	AssetID       string  `json:"asset_id"`
	OldRiskScore  float64 `json:"old_risk_score"`
	NewRiskScore  float64 `json:"new_risk_score"`
	RiskDelta     float64 `json:"risk_delta"`
	Reason        string  `json:"reason"` // Why the risk score changed
	Contributors  []RiskContributor `json:"contributors,omitempty"`
}

// RiskContributor represents a contributor to risk score change
type RiskContributor struct {
	Type        string  `json:"type"` // finding, exposure, threat, etc.
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Impact      float64 `json:"impact"` // Positive or negative impact
	Description string  `json:"description"`
}

// FieldChange represents a change in a field
type FieldChange struct {
	Field    string      `json:"field"`
	OldValue interface{} `json:"old_value"`
	NewValue interface{} `json:"new_value"`
}

// NewBaseEvent creates a new base event
func NewBaseEvent(eventType EventType, provider Provider, environment Environment, source, description string) BaseEvent {
	return BaseEvent{
		ID:          uuid.New().String(),
		Type:        eventType,
		Severity:    EventSeverityMedium, // Default severity
		Timestamp:   time.Now(),
		Provider:    provider,
		Environment: environment,
		Source:      source,
		Description: description,
		Metadata:    make(map[string]interface{}),
	}
}

// WithSeverity sets the event severity
func (e *BaseEvent) WithSeverity(severity EventSeverity) *BaseEvent {
	e.Severity = severity
	return e
}

// WithActor sets the event actor
func (e *BaseEvent) WithActor(actor string) *BaseEvent {
	e.Actor = actor
	return e
}

// WithAssetID sets the asset ID
func (e *BaseEvent) WithAssetID(assetID string) *BaseEvent {
	e.AssetID = assetID
	return e
}

// WithMetadata adds metadata to the event
func (e *BaseEvent) WithMetadata(key string, value interface{}) *BaseEvent {
	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{})
	}
	e.Metadata[key] = value
	return e
}

// WithRawData sets the raw event data
func (e *BaseEvent) WithRawData(data []byte) *BaseEvent {
	e.RawData = data
	return e
}

// EventFilter represents a filter for events
type EventFilter struct {
	Types         []EventType      `json:"types,omitempty"`
	Severities    []EventSeverity  `json:"severities,omitempty"`
	Providers     []Provider       `json:"providers,omitempty"`
	Environments  []Environment    `json:"environments,omitempty"`
	AssetIDs      []string         `json:"asset_ids,omitempty"`
	Sources       []string         `json:"sources,omitempty"`
	Actors        []string         `json:"actors,omitempty"`
	StartTime     *time.Time       `json:"start_time,omitempty"`
	EndTime       *time.Time       `json:"end_time,omitempty"`
	MinSeverity   *EventSeverity   `json:"min_severity,omitempty"`
	MaxSeverity   *EventSeverity   `json:"max_severity,omitempty"`
	Limit         int              `json:"limit,omitempty"`
	Offset        int              `json:"offset,omitempty"`
}

// EventQuery represents a query for events
type EventQuery struct {
	EventFilter
	TextSearch string `json:"text_search,omitempty"`
	SortBy     string `json:"sort_by,omitempty"`
	SortOrder  string `json:"sort_order,omitempty"` // asc, desc
}

// EventBatch represents a batch of events for bulk processing
type EventBatch struct {
	Events    []BaseEvent `json:"events"`
	BatchID   string      `json:"batch_id"`
	Timestamp time.Time   `json:"timestamp"`
	Source    string      `json:"source"`
	Size      int         `json:"size"`
}

// NewEventBatch creates a new event batch
func NewEventBatch(source string, events ...BaseEvent) EventBatch {
	return EventBatch{
		Events:    events,
		BatchID:   uuid.New().String(),
		Timestamp: time.Now(),
		Source:    source,
		Size:      len(events),
	}
}
