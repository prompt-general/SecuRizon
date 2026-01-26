package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/securizon/pkg/models"
)

// EventProcessor processes events from the event bus
type EventProcessor struct {
	bus           EventBus
	graphStore    GraphStore
	riskEngine    RiskEngine
	policyEngine  PolicyEngine
	handlers      map[models.EventType][]EventHandler
	mu            sync.RWMutex
	metrics       *ProcessorMetrics
	config        ProcessorConfig
}

// GraphStore interface for graph operations
type GraphStore interface {
	CreateAsset(ctx context.Context, asset models.Asset) error
	GetAsset(ctx context.Context, id string) (models.Asset, error)
	UpdateAsset(ctx context.Context, asset models.Asset) error
	DeleteAsset(ctx context.Context, id string) error
	CreateRelationship(ctx context.Context, rel models.Relationship) error
	UpdateRelationship(ctx context.Context, rel models.Relationship) error
	DeleteRelationship(ctx context.Context, id string) error
	CreateFinding(ctx context.Context, finding models.Finding) error
	UpdateFinding(ctx context.Context, finding models.Finding) error
	GetAssetFindings(ctx context.Context, assetID string) ([]models.Finding, error)
	GetAssetRisk(ctx context.Context, assetID string) (models.RiskScore, error)
	UpdateAssetRisk(ctx context.Context, risk models.RiskScore) error
}

// RiskEngine interface for risk calculations
type RiskEngine interface {
	CalculateRisk(asset models.Asset, findings []models.Finding, threats []models.ThreatEvent) models.RiskScore
	RecalculateRisk(assetID string) (models.RiskScore, error)
	UpdateRiskScore(assetID string, score models.RiskScore) error
}

// PolicyEngine interface for policy evaluation
type PolicyEngine interface {
	EvaluateAsset(ctx context.Context, asset models.Asset) ([]models.Finding, error)
	EvaluatePolicy(ctx context.Context, policyID string, asset models.Asset) (*models.Finding, error)
	GetPolicies(ctx context.Context, filter PolicyFilter) ([]Policy, error)
}

// Policy represents a security policy
type Policy struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Category    string                 `json:"category"`
	Severity    float64                `json:"severity"`
	Enabled     bool                   `json:"enabled"`
	Rules       []PolicyRule           `json:"rules"`
	Remediation string                 `json:"remediation"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// PolicyRule represents a single policy rule
type PolicyRule struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Field       string                 `json:"field"`
	Operator    string                 `json:"operator"`
	Value       interface{}            `json:"value"`
	Severity    float64                `json:"severity"`
	Description string                 `json:"description"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// PolicyFilter represents a filter for policies
type PolicyFilter struct {
	Categories []string `json:"categories,omitempty"`
	Enabled    *bool    `json:"enabled,omitempty"`
	MinSeverity float64 `json:"min_severity,omitempty"`
	MaxSeverity float64 `json:"max_severity,omitempty"`
}

// ProcessorConfig represents event processor configuration
type ProcessorConfig struct {
	WorkerCount       int           `json:"worker_count"`
	BatchSize         int           `json:"batch_size"`
	BatchTimeout      time.Duration `json:"batch_timeout"`
	RetryAttempts     int           `json:"retry_attempts"`
	RetryDelay        time.Duration `json:"retry_delay"`
	EnableMetrics     bool          `json:"enable_metrics"`
	MetricsInterval   time.Duration `json:"metrics_interval"`
	DeadLetterTopic   string        `json:"dead_letter_topic"`
	EnableDLQ         bool          `json:"enable_dlq"`
}

// ProcessorMetrics represents processor metrics
type ProcessorMetrics struct {
	EventsProcessed    int64     `json:"events_processed"`
	EventsFailed       int64     `json:"events_failed"`
	EventsRetried      int64     `json:"events_retried"
	AverageLatency     time.Duration `json:"average_latency"`
	LastProcessed      time.Time `json:"last_processed"`
	EventsByType       map[models.EventType]int64 `json:"events_by_type"`
	ErrorsByType       map[string]int64 `json:"errors_by_type"`
	WorkerUtilization  map[int]float64 `json:"worker_utilization"`
	mu                 sync.RWMutex
}

// DefaultProcessorConfig returns default processor configuration
func DefaultProcessorConfig() ProcessorConfig {
	return ProcessorConfig{
		WorkerCount:     10,
		BatchSize:       100,
		BatchTimeout:    5 * time.Second,
		RetryAttempts:   3,
		RetryDelay:      1 * time.Second,
		EnableMetrics:   true,
		MetricsInterval: 30 * time.Second,
		DeadLetterTopic: "events.dlq",
		EnableDLQ:       true,
	}
}

// NewEventProcessor creates a new event processor
func NewEventProcessor(bus EventBus, graphStore GraphStore, riskEngine RiskEngine, policyEngine PolicyEngine, config ProcessorConfig) *EventProcessor {
	processor := &EventProcessor{
		bus:          bus,
		graphStore:   graphStore,
		riskEngine:   riskEngine,
		policyEngine: policyEngine,
		handlers:     make(map[models.EventType][]EventHandler),
		config:       config,
		metrics:      &ProcessorMetrics{
			EventsByType: make(map[models.EventType]int64),
			ErrorsByType: make(map[string]int64),
			WorkerUtilization: make(map[int]float64),
		},
	}

	// Register default handlers
	processor.registerDefaultHandlers()

	return processor
}

// registerDefaultHandlers registers default event handlers
func (p *EventProcessor) registerDefaultHandlers() {
	// Asset event handlers
	p.RegisterHandler(models.EventTypeAssetCreated, EventHandlerFunc(p.handleAssetCreated))
	p.RegisterHandler(models.EventTypeAssetUpdated, EventHandlerFunc(p.handleAssetUpdated))
	p.RegisterHandler(models.EventTypeAssetDeleted, EventHandlerFunc(p.handleAssetDeleted))

	// Relationship event handlers
	p.RegisterHandler(models.EventTypeRelationshipCreated, EventHandlerFunc(p.handleRelationshipCreated))
	p.RegisterHandler(models.EventTypeRelationshipUpdated, EventHandlerFunc(p.handleRelationshipUpdated))
	p.RegisterHandler(models.EventTypeRelationshipDeleted, EventHandlerFunc(p.handleRelationshipDeleted))

	// Finding event handlers
	p.RegisterHandler(models.EventTypeFindingCreated, EventHandlerFunc(p.handleFindingCreated))
	p.RegisterHandler(models.EventTypeFindingUpdated, EventHandlerFunc(p.handleFindingUpdated))
	p.RegisterHandler(models.EventTypeFindingResolved, EventHandlerFunc(p.handleFindingResolved))

	// Policy violation handlers
	p.RegisterHandler(models.EventTypePolicyViolation, EventHandlerFunc(p.handlePolicyViolation))

	// Threat event handlers
	p.RegisterHandler(models.EventTypeThreatDetected, EventHandlerFunc(p.handleThreatDetected))

	// Risk score change handlers
	p.RegisterHandler(models.EventTypeRiskScoreChanged, EventHandlerFunc(p.handleRiskScoreChanged))
}

// RegisterHandler registers a handler for an event type
func (p *EventProcessor) RegisterHandler(eventType models.EventType, handler EventHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.handlers[eventType] == nil {
		p.handlers[eventType] = make([]EventHandler, 0)
	}
	p.handlers[eventType] = append(p.handlers[eventType], handler)
}

// Start starts the event processor
func (p *EventProcessor) Start(ctx context.Context) error {
	log.Printf("Starting event processor with %d workers", p.config.WorkerCount)

	// Subscribe to all topics
	topics := []string{
		TopicAssetUpserts,
		TopicAssetRelationships,
		TopicSecurityEvents,
		TopicPolicyViolations,
		TopicRiskScores,
		TopicThreatIntel,
		TopicFindings,
	}

	for _, topic := range topics {
		handler := EventHandlerFunc(p.handleEvent)
		if err := p.bus.SubscribeGroup(ctx, topic, fmt.Sprintf("processor-%s", topic), handler); err != nil {
			return fmt.Errorf("failed to subscribe to topic %s: %w", topic, err)
		}
	}

	// Start metrics collection if enabled
	if p.config.EnableMetrics {
		go p.collectMetrics(ctx)
	}

	log.Printf("Event processor started successfully")
	return nil
}

// handleEvent is the main event handler
func (p *EventProcessor) handleEvent(ctx context.Context, event models.BaseEvent) error {
	start := time.Now()
	defer func() {
		latency := time.Since(start)
		p.updateMetrics(event.Type, latency, nil)
	}()

	// Get handlers for this event type
	p.mu.RLock()
	handlers := p.handlers[event.Type]
	p.mu.RUnlock()

	if len(handlers) == 0 {
		log.Printf("No handlers registered for event type: %s", event.Type)
		return nil
	}

	// Execute all handlers for this event type
	var errors []error
	for _, handler := range handlers {
		if err := handler.Handle(ctx, event); err != nil {
			log.Printf("Handler %s failed for event %s: %v", handler.GetName(), event.ID, err)
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("handlers failed: %v", errors)
	}

	return nil
}

// Asset event handlers

func (p *EventProcessor) handleAssetCreated(ctx context.Context, event models.BaseEvent) error {
	var assetEvent models.AssetEvent
	if err := p.unmarshalEvent(event, &assetEvent); err != nil {
		return err
	}

	// Create asset in graph store
	if err := p.graphStore.CreateAsset(ctx, assetEvent.Asset); err != nil {
		return fmt.Errorf("failed to create asset: %w", err)
	}

	// Evaluate policies for new asset
	findings, err := p.policyEngine.EvaluateAsset(ctx, assetEvent.Asset)
	if err != nil {
		log.Printf("Failed to evaluate policies for asset %s: %v", assetEvent.Asset.GetID(), err)
	}

	// Create findings
	for _, finding := range findings {
		if err := p.graphStore.CreateFinding(ctx, finding); err != nil {
			log.Printf("Failed to create finding %s: %v", finding.ID, err)
		}
	}

	// Calculate initial risk score
	if len(findings) > 0 {
		risk := p.riskEngine.CalculateRisk(assetEvent.Asset, findings, []models.ThreatEvent{})
		if err := p.graphStore.UpdateAssetRisk(ctx, risk); err != nil {
			log.Printf("Failed to update risk for asset %s: %v", assetEvent.Asset.GetID(), err)
		}
	}

	log.Printf("Processed asset creation: %s", assetEvent.Asset.GetID())
	return nil
}

func (p *EventProcessor) handleAssetUpdated(ctx context.Context, event models.BaseEvent) error {
	var assetEvent models.AssetEvent
	if err := p.unmarshalEvent(event, &assetEvent); err != nil {
		return err
	}

	// Update asset in graph store
	if err := p.graphStore.UpdateAsset(ctx, assetEvent.Asset); err != nil {
		return fmt.Errorf("failed to update asset: %w", err)
	}

	// Re-evaluate policies
	findings, err := p.policyEngine.EvaluateAsset(ctx, assetEvent.Asset)
	if err != nil {
		log.Printf("Failed to evaluate policies for asset %s: %v", assetEvent.Asset.GetID(), err)
	}

	// Update findings (this would be more sophisticated in practice)
	// For now, we'll just create new findings
	for _, finding := range findings {
		if err := p.graphStore.CreateFinding(ctx, finding); err != nil {
			log.Printf("Failed to create finding %s: %v", finding.ID, err)
		}
	}

	// Recalculate risk score
	risk := p.riskEngine.CalculateRisk(assetEvent.Asset, findings, []models.ThreatEvent{})
	if err := p.graphStore.UpdateAssetRisk(ctx, risk); err != nil {
		log.Printf("Failed to update risk for asset %s: %v", assetEvent.Asset.GetID(), err)
	}

	log.Printf("Processed asset update: %s", assetEvent.Asset.GetID())
	return nil
}

func (p *EventProcessor) handleAssetDeleted(ctx context.Context, event models.BaseEvent) error {
	var assetEvent models.AssetEvent
	if err := p.unmarshalEvent(event, &assetEvent); err != nil {
		return err
	}

	// Delete asset from graph store
	if err := p.graphStore.DeleteAsset(ctx, assetEvent.Asset.GetID()); err != nil {
		return fmt.Errorf("failed to delete asset: %w", err)
	}

	log.Printf("Processed asset deletion: %s", assetEvent.Asset.GetID())
	return nil
}

// Relationship event handlers

func (p *EventProcessor) handleRelationshipCreated(ctx context.Context, event models.BaseEvent) error {
	var relEvent models.RelationshipEvent
	if err := p.unmarshalEvent(event, &relEvent); err != nil {
		return err
	}

	// Create relationship in graph store
	if err := p.graphStore.CreateRelationship(ctx, relEvent.Relationship); err != nil {
		return fmt.Errorf("failed to create relationship: %w", err)
	}

	log.Printf("Processed relationship creation: %s", relEvent.Relationship.ID)
	return nil
}

func (p *EventProcessor) handleRelationshipUpdated(ctx context.Context, event models.BaseEvent) error {
	var relEvent models.RelationshipEvent
	if err := p.unmarshalEvent(event, &relEvent); err != nil {
		return err
	}

	// Update relationship in graph store
	if err := p.graphStore.UpdateRelationship(ctx, relEvent.Relationship); err != nil {
		return fmt.Errorf("failed to update relationship: %w", err)
	}

	log.Printf("Processed relationship update: %s", relEvent.Relationship.ID)
	return nil
}

func (p *EventProcessor) handleRelationshipDeleted(ctx context.Context, event models.BaseEvent) error {
	var relEvent models.RelationshipEvent
	if err := p.unmarshalEvent(event, &relEvent); err != nil {
		return err
	}

	// Delete relationship from graph store
	if err := p.graphStore.DeleteRelationship(ctx, relEvent.Relationship.ID); err != nil {
		return fmt.Errorf("failed to delete relationship: %w", err)
	}

	log.Printf("Processed relationship deletion: %s", relEvent.Relationship.ID)
	return nil
}

// Finding event handlers

func (p *EventProcessor) handleFindingCreated(ctx context.Context, event models.BaseEvent) error {
	var findingEvent models.FindingEvent
	if err := p.unmarshalEvent(event, &findingEvent); err != nil {
		return err
	}

	// Create finding in graph store
	if err := p.graphStore.CreateFinding(ctx, findingEvent.Finding); err != nil {
		return fmt.Errorf("failed to create finding: %w", err)
	}

	log.Printf("Processed finding creation: %s", findingEvent.Finding.ID)
	return nil
}

func (p *EventProcessor) handleFindingUpdated(ctx context.Context, event models.BaseEvent) error {
	var findingEvent models.FindingEvent
	if err := p.unmarshalEvent(event, &findingEvent); err != nil {
		return err
	}

	// Update finding in graph store
	if err := p.graphStore.UpdateFinding(ctx, findingEvent.Finding); err != nil {
		return fmt.Errorf("failed to update finding: %w", err)
	}

	log.Printf("Processed finding update: %s", findingEvent.Finding.ID)
	return nil
}

func (p *EventProcessor) handleFindingResolved(ctx context.Context, event models.BaseEvent) error {
	var findingEvent models.FindingEvent
	if err := p.unmarshalEvent(event, &findingEvent); err != nil {
		return err
	}

	// Update finding status to resolved
	findingEvent.Finding.Status = "resolved"
	if err := p.graphStore.UpdateFinding(ctx, findingEvent.Finding); err != nil {
		return fmt.Errorf("failed to resolve finding: %w", err)
	}

	// Recalculate risk for the asset
	risk, err := p.riskEngine.RecalculateRisk(findingEvent.Finding.AssetID)
	if err != nil {
		log.Printf("Failed to recalculate risk for asset %s: %v", findingEvent.Finding.AssetID, err)
	} else {
		if err := p.graphStore.UpdateAssetRisk(ctx, risk); err != nil {
			log.Printf("Failed to update risk for asset %s: %v", findingEvent.Finding.AssetID, err)
		}
	}

	log.Printf("Processed finding resolution: %s", findingEvent.Finding.ID)
	return nil
}

// Other event handlers

func (p *EventProcessor) handlePolicyViolation(ctx context.Context, event models.BaseEvent) error {
	var violationEvent models.PolicyViolationEvent
	if err := p.unmarshalEvent(event, &violationEvent); err != nil {
		return err
	}

	// Create finding from policy violation
	finding := models.Finding{
		BaseAsset: models.NewBaseAsset(
			violationEvent.Provider,
			models.AssetTypeFinding,
			violationEvent.Environment,
			fmt.Sprintf("Policy Violation: %s", violationEvent.PolicyName),
		),
		PolicyID:      violationEvent.PolicyID,
		Severity:      violationEvent.Severity,
		Status:        "open",
		Description:   violationEvent.Description,
		Recommendation: violationEvent.Remediation,
		AssetID:       violationEvent.Asset.GetID(),
	}

	if err := p.graphStore.CreateFinding(ctx, finding); err != nil {
		return fmt.Errorf("failed to create finding from policy violation: %w", err)
	}

	log.Printf("Processed policy violation: %s", violationEvent.PolicyID)
	return nil
}

func (p *EventProcessor) handleThreatDetected(ctx context.Context, event models.BaseEvent) error {
	var threatEvent models.ThreatEvent
	if err := p.unmarshalEvent(event, &threatEvent); err != nil {
		return err
	}

	// Recalculate risk for affected assets
	for _, asset := range threatEvent.AffectedAssets {
		risk, err := p.riskEngine.RecalculateRisk(asset.GetID())
		if err != nil {
			log.Printf("Failed to recalculate risk for asset %s: %v", asset.GetID(), err)
			continue
		}

		if err := p.graphStore.UpdateAssetRisk(ctx, risk); err != nil {
			log.Printf("Failed to update risk for asset %s: %v", asset.GetID(), err)
		}
	}

	log.Printf("Processed threat detection: %s", threatEvent.ThreatID)
	return nil
}

func (p *EventProcessor) handleRiskScoreChanged(ctx context.Context, event models.BaseEvent) error {
	var riskEvent models.RiskScoreChangeEvent
	if err := p.unmarshalEvent(event, &riskEvent); err != nil {
		return err
	}

	// Update risk score in graph store
	risk := models.RiskScore{
		AssetID:        riskEvent.AssetID,
		Score:          riskEvent.NewRiskScore,
		LastCalculated: time.Now(),
	}

	if err := p.graphStore.UpdateAssetRisk(ctx, risk); err != nil {
		return fmt.Errorf("failed to update risk score: %w", err)
	}

	log.Printf("Processed risk score change for asset %s: %.2f -> %.2f", 
		riskEvent.AssetID, riskEvent.OldRiskScore, riskEvent.NewRiskScore)
	return nil
}

// Helper methods

func (p *EventProcessor) unmarshalEvent(event models.BaseEvent, target interface{}) error {
	if event.RawData == nil {
		return fmt.Errorf("event has no raw data")
	}
	
	if err := json.Unmarshal(event.RawData, target); err != nil {
		return fmt.Errorf("failed to unmarshal event data: %w", err)
	}
	
	return nil
}

func (p *EventProcessor) updateMetrics(eventType models.EventType, latency time.Duration, err error) {
	if !p.config.EnableMetrics {
		return
	}

	p.metrics.mu.Lock()
	defer p.metrics.mu.Unlock()

	p.metrics.EventsProcessed++
	p.metrics.EventsByType[eventType]++
	
	if err != nil {
		p.metrics.EventsFailed++
		p.metrics.ErrorsByType[string(eventType)]++
	}

	// Update average latency (simple moving average)
	if p.metrics.AverageLatency == 0 {
		p.metrics.AverageLatency = latency
	} else {
		p.metrics.AverageLatency = (p.metrics.AverageLatency + latency) / 2
	}

	p.metrics.LastProcessed = time.Now()
}

func (p *EventProcessor) collectMetrics(ctx context.Context) {
	ticker := time.NewTicker(p.config.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.logMetrics()
		}
	}
}

func (p *EventProcessor) logMetrics() {
	p.metrics.mu.RLock()
	metrics := *p.metrics
	p.metrics.mu.RUnlock()

	log.Printf("Event Processor Metrics: Processed=%d, Failed=%d, AvgLatency=%v, LastProcessed=%v",
		metrics.EventsProcessed,
		metrics.EventsFailed,
		metrics.AverageLatency,
		metrics.LastProcessed,
	)
}

// GetMetrics returns current processor metrics
func (p *EventProcessor) GetMetrics() ProcessorMetrics {
	p.metrics.mu.RLock()
	defer p.metrics.mu.RUnlock()
	return *p.metrics
}
