package event

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/securazion/event-processor/internal/graph"
	"github.com/securazion/event-processor/internal/policy"
	"github.com/securazion/event-processor/internal/risk"
	"github.com/securazion/event-processor/internal/kafka"
)

type EventProcessor struct {
	cfg          *config.Config
	graphClient  *graph.Client
	policyEngine *policy.Engine
	riskEngine   *risk.Engine
	consumer     kafka.Consumer
	producer     kafka.Producer
	workers      []*Worker
	mu           sync.RWMutex
	stats        ProcessorStats
}

type ProcessorStats struct {
	EventsProcessed int64
	FindingsCreated int64
	PolicyChecks    int64
	AvgProcessTime  time.Duration
	Errors          int64
}

func NewEventProcessor(cfg *config.Config) (*EventProcessor, error) {
	// Initialize dependencies
	graphClient, err := graph.NewClient(cfg.Neo4j)
	if err != nil {
		return nil, fmt.Errorf("failed to create graph client: %v", err)
	}
	
	policyEngine := policy.NewEngine(cfg.Policies)
	riskEngine := risk.NewEngine(cfg.Risk)
	
	consumer, err := kafka.NewConsumer("event-processor-group")
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer: %v", err)
	}
	
	producer, err := kafka.NewProducer()
	if err != nil {
		return nil, fmt.Errorf("failed to create producer: %v", err)
	}
	
	return &EventProcessor{
		cfg:          cfg,
		graphClient:  graphClient,
		policyEngine: policyEngine,
		riskEngine:   riskEngine,
		consumer:     consumer,
		producer:     producer,
	}, nil
}

func (ep *EventProcessor) Start(ctx context.Context, workerCount int) error {
	// Subscribe to topics
	topics := []string{
		"asset.upserts",
		"asset.relationships", 
		"security.events",
	}
	
	ep.consumer.Subscribe(ctx, topics)
	
	// Start worker pool
	ep.workers = make([]*Worker, workerCount)
	for i := 0; i < workerCount; i++ {
		worker := NewWorker(i, ep)
		ep.workers[i] = worker
		go worker.Start(ctx)
	}
	
	log.Printf("Started event processor with %d workers", workerCount)
	return nil
}

func (ep *EventProcessor) processEvent(eventData []byte) error {
	startTime := time.Now()
	
	// Parse event header first to determine type
	var header struct {
		EventType string `json:"event_type"`
	}
	
	if err := json.Unmarshal(eventData, &header); err != nil {
		return fmt.Errorf("failed to parse event header: %v", err)
	}
	
	// Route to appropriate handler
	switch header.EventType {
	case "asset.upsert":
		return ep.handleAssetUpsert(eventData)
	case "asset.relationship":
		return ep.handleRelationship(eventData)
	case "security.event":
		return ep.handleSecurityEvent(eventData)
	default:
		return fmt.Errorf("unknown event type: %s", header.EventType)
	}
	
	ep.updateStats(time.Since(startTime))
	return nil
}

func (ep *EventProcessor) handleAssetUpsert(eventData []byte) error {
	var event models.AssetUpsertEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal asset event: %v", err)
	}
	
	// 1. Update graph with asset
	if err := ep.graphClient.UpsertAsset(event.Asset); err != nil {
		return fmt.Errorf("failed to upsert asset: %v", err)
	}
	
	// 2. Run policies against this asset
	findings := ep.policyEngine.EvaluateAsset(event.Asset)
	
	// 3. Calculate risk scores for findings
	for i := range findings {
		finding := &findings[i]
		finding.RiskScore = ep.riskEngine.Calculate(finding, event.Asset)
		
		// 4. Store finding in graph
		if err := ep.graphClient.CreateFinding(finding); err != nil {
			log.Printf("Failed to create finding: %v", err)
			continue
		}
		
		// 5. Emit finding event
		findingEvent := models.FindingEvent{
			Header: models.EventHeader{
				EventID:   generateUUID(),
				EventType: "finding.created",
				Timestamp: time.Now().UnixMilli(),
			},
			Finding: *finding,
		}
		
		eventData, err := json.Marshal(findingEvent)
		if err != nil {
			log.Printf("Failed to marshal finding event: %v", err)
			continue
		}
		
		ep.producer.Produce("findings", finding.ID, eventData)
		ep.stats.FindingsCreated++
	}
	
	// 6. Trigger attack path re-evaluation if asset is high-risk
	if event.Asset.RiskScore > 70 {
		go ep.reevaluateAttackPaths(event.Asset.ID)
	}
	
	return nil
}

func (ep *EventProcessor) handleSecurityEvent(eventData []byte) error {
	var event models.SecurityEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal security event: %v", err)
	}
	
	// Check for suspicious activities
	suspicious := ep.detectSuspiciousActivity(event)
	if suspicious {
		// Create alert
		alert := ep.createAlertFromEvent(event)
		
		// Update graph with alert
		if err := ep.graphClient.CreateAlert(alert); err != nil {
			log.Printf("Failed to create alert: %v", err)
		}
		
		// Emit alert
		alertData, _ := json.Marshal(alert)
		ep.producer.Produce("alerts", alert.ID, alertData)
	}
	
	return nil
}

func (ep *EventProcessor) reevaluateAttackPaths(assetID string) {
	// Find all attack paths involving this asset
	paths := ep.graphClient.FindAttackPaths(assetID, 3) // max 3 hops
	
	for _, path := range paths {
		// Recalculate cumulative risk
		newRisk := ep.calculatePathRisk(path)
		
		// Update if risk changed significantly
		if math.Abs(newRisk-path.CumulativeRisk) > 10 {
			ep.graphClient.UpdatePathRisk(path.ID, newRisk)
			
			// Notify if path becomes critical
			if newRisk > 80 {
				ep.emitCriticalPathAlert(path)
			}
		}
	}
}
