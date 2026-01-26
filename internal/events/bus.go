package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/securizon/pkg/models"
)

// EventBus interface defines event bus operations
type EventBus interface {
	// Publishing
	PublishEvent(ctx context.Context, topic string, event models.BaseEvent) error
	PublishBatch(ctx context.Context, topic string, batch models.EventBatch) error
	
	// Subscribing
	Subscribe(ctx context.Context, topic string, handler EventHandler) error
	SubscribeGroup(ctx context.Context, topic, group string, handler EventHandler) error
	
	// Topic management
	CreateTopic(ctx context.Context, topic string, partitions int, replicationFactor int) error
	DeleteTopic(ctx context.Context, topic string) error
	ListTopics(ctx context.Context) ([]string, error)
	
	// Health and maintenance
	Ping(ctx context.Context) error
	Close() error
}

// EventHandler processes incoming events
type EventHandler interface {
	Handle(ctx context.Context, event models.BaseEvent) error
	GetName() string
}

// EventHandlerFunc is a function adapter for EventHandler
type EventHandlerFunc func(ctx context.Context, event models.BaseEvent) error

func (h EventHandlerFunc) Handle(ctx context.Context, event models.BaseEvent) error {
	return h(ctx, event)
}

func (h EventHandlerFunc) GetName() string {
	return fmt.Sprintf("HandlerFunc-%p", h)
}

// KafkaEventBus implements EventBus using Kafka
type KafkaEventBus struct {
	brokers []string
	config  KafkaConfig
	producer *kafka.Writer
	consumers map[string]*kafka.Reader
}

// KafkaConfig represents Kafka configuration
type KafkaConfig struct {
	Brokers            []string `json:"brokers"`
	ClientID           string   `json:"client_id"`
	ConsumerGroup      string   `json:"consumer_group"`
	BatchSize          int      `json:"batch_size"`
	BatchTimeout       time.Duration `json:"batch_timeout"`
	CommitInterval     time.Duration `json:"commit_interval"`
	HeartbeatInterval  time.Duration `json:"heartbeat_interval"`
	SessionTimeout     time.Duration `json:"session_timeout"`
	RebalanceTimeout   time.Duration `json:"rebalance_timeout"`
	StartOffset        int64    `json:"start_offset"` // -1 for latest, -2 for earliest
	MinBytes           int      `json:"min_bytes"`
	MaxBytes           int      `json:"max_bytes"`
	MaxWait            time.Duration `json:"max_wait"`
	CompressionType    string   `json:"compression_type"`
	SecurityProtocol   string   `json:"security_protocol"`
	SASLMechanism      string   `json:"sasl_mechanism"`
	SASLUsername       string   `json:"sasl_username"`
	SASLPassword       string   `json:"sasl_password"`
}

// DefaultKafkaConfig returns default Kafka configuration
func DefaultKafkaConfig() KafkaConfig {
	return KafkaConfig{
		Brokers:           []string{"localhost:9092"},
		ClientID:          "securizon-events",
		ConsumerGroup:     "securizon-group",
		BatchSize:         100,
		BatchTimeout:      10 * time.Millisecond,
		CommitInterval:    time.Second,
		HeartbeatInterval: 3 * time.Second,
		SessionTimeout:    30 * time.Second,
		RebalanceTimeout:  30 * time.Second,
		StartOffset:       -1, // Latest
		MinBytes:          1,
		MaxBytes:          1e6, // 1MB
		MaxWait:           500 * time.Millisecond,
		CompressionType:   "gzip",
		SecurityProtocol:  "PLAINTEXT",
	}
}

// NewKafkaEventBus creates a new Kafka event bus
func NewKafkaEventBus(config KafkaConfig) (*KafkaEventBus, error) {
	// Create producer
	producerConfig := kafka.WriterConfig{
		Brokers:          config.Brokers,
		Topic:            "", // Will be set per message
		Balancer:         &kafka.LeastBytes{},
		BatchSize:        config.BatchSize,
		BatchTimeout:     config.BatchTimeout,
		Compression:      kafka.Compression(config.CompressionType),
		AllowAutoTopicCreation: false,
		Transport: &kafka.Transport{
			TLS: nil, // Configure TLS if needed
			SASL: nil, // Configure SASL if needed
		},
	}

	producer := kafka.NewWriter(producerConfig)

	return &KafkaEventBus{
		brokers:  config.Brokers,
		config:   config,
		producer: producer,
		consumers: make(map[string]*kafka.Reader),
	}, nil
}

// PublishEvent publishes a single event
func (bus *KafkaEventBus) PublishEvent(ctx context.Context, topic string, event models.BaseEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	message := kafka.Message{
		Topic: topic,
		Key:   []byte(event.ID),
		Value: data,
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte(string(event.Type))},
			{Key: "provider", Value: []byte(string(event.Provider))},
			{Key: "environment", Value: []byte(string(event.Environment))},
			{Key: "severity", Value: []byte(string(event.Severity))},
			{Key: "timestamp", Value: []byte(event.Timestamp.Format(time.RFC3339))},
		},
		Time: time.Now(),
	}

	return bus.producer.WriteMessages(ctx, message)
}

// PublishBatch publishes a batch of events
func (bus *KafkaEventBus) PublishBatch(ctx context.Context, topic string, batch models.EventBatch) error {
	messages := make([]kafka.Message, len(batch.Events))
	
	for i, event := range batch.Events {
		data, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to marshal event %s: %w", event.ID, err)
		}

		messages[i] = kafka.Message{
			Topic: topic,
			Key:   []byte(event.ID),
			Value: data,
			Headers: []kafka.Header{
				{Key: "event_type", Value: []byte(string(event.Type))},
				{Key: "provider", Value: []byte(string(event.Provider))},
				{Key: "environment", Value: []byte(string(event.Environment))},
				{Key: "severity", Value: []byte(string(event.Severity))},
				{Key: "timestamp", Value: []byte(event.Timestamp.Format(time.RFC3339))},
			},
			Time: time.Now(),
		}
	}

	return bus.producer.WriteMessages(ctx, messages...)
}

// Subscribe subscribes to a topic
func (bus *KafkaEventBus) Subscribe(ctx context.Context, topic string, handler EventHandler) error {
	return bus.SubscribeGroup(ctx, topic, "", handler)
}

// SubscribeGroup subscribes to a topic with a consumer group
func (bus *KafkaEventBus) SubscribeGroup(ctx context.Context, topic, group string, handler EventHandler) error {
	consumerConfig := kafka.ReaderConfig{
		Brokers:          bus.config.Brokers,
		GroupID:          group,
		Topic:            topic,
		MinBytes:         bus.config.MinBytes,
		MaxBytes:         bus.config.MaxBytes,
		MaxWait:          bus.config.MaxWait,
		ReadBackoffMin:   100 * time.Millisecond,
		ReadBackoffMax:   1 * time.Second,
		StartOffset:      bus.config.StartOffset,
		CommitInterval:   bus.config.CommitInterval,
		HeartbeatInterval: bus.config.HeartbeatInterval,
		SessionTimeout:   bus.config.SessionTimeout,
		RebalanceTimeout: bus.config.RebalanceTimeout,
	}

	consumer := kafka.NewReader(consumerConfig)
	
	// Store consumer for cleanup
	consumerKey := fmt.Sprintf("%s:%s", topic, group)
	bus.consumers[consumerKey] = consumer

	// Start consuming in a goroutine
	go func() {
		defer func() {
			if err := consumer.Close(); err != nil {
				log.Printf("Error closing consumer for %s: %v", topic, err)
			}
			delete(bus.consumers, consumerKey)
		}()

		for {
			select {
			case <-ctx.Done():
				log.Printf("Stopping consumer for %s: %v", topic, ctx.Err())
				return
			default:
				message, err := consumer.ReadMessage(ctx)
				if err != nil {
					log.Printf("Error reading message from %s: %v", topic, err)
					continue
				}

				// Parse event
				var event models.BaseEvent
				if err := json.Unmarshal(message.Value, &event); err != nil {
					log.Printf("Error unmarshaling event from %s: %v", topic, err)
					continue
				}

				// Handle event
				if err := handler.Handle(ctx, event); err != nil {
					log.Printf("Error handling event %s in %s: %v", event.ID, handler.GetName(), err)
					// Continue processing other events
				}
			}
		}
	}()

	log.Printf("Subscribed to topic %s with handler %s", topic, handler.GetName())
	return nil
}

// CreateTopic creates a new topic
func (bus *KafkaEventBus) CreateTopic(ctx context.Context, topic string, partitions int, replicationFactor int) error {
	conn, err := kafka.Dial("tcp", bus.config.Brokers[0])
	if err != nil {
		return fmt.Errorf("failed to connect to Kafka: %w", err)
	}
	defer conn.Close()

	topicConfig := kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     partitions,
		ReplicationFactor: replicationFactor,
	}

	err = conn.CreateTopics(topicConfig)
	if err != nil {
		return fmt.Errorf("failed to create topic %s: %w", topic, err)
	}

	log.Printf("Created topic %s with %d partitions and replication factor %d", topic, partitions, replicationFactor)
	return nil
}

// DeleteTopic deletes a topic
func (bus *KafkaEventBus) DeleteTopic(ctx context.Context, topic string) error {
	conn, err := kafka.Dial("tcp", bus.config.Brokers[0])
	if err != nil {
		return fmt.Errorf("failed to connect to Kafka: %w", err)
	}
	defer conn.Close()

	err = conn.DeleteTopics(topic)
	if err != nil {
		return fmt.Errorf("failed to delete topic %s: %w", topic, err)
	}

	log.Printf("Deleted topic %s", topic)
	return nil
}

// ListTopics lists all topics
func (bus *KafkaEventBus) ListTopics(ctx context.Context) ([]string, error) {
	conn, err := kafka.Dial("tcp", bus.config.Brokers[0])
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Kafka: %w", err)
	}
	defer conn.Close()

	partitions, err := conn.ReadPartitions()
	if err != nil {
		return nil, fmt.Errorf("failed to read partitions: %w", err)
	}

	// Extract unique topic names
	topics := make(map[string]bool)
	for _, partition := range partitions {
		topics[partition.Topic] = true
	}

	topicList := make([]string, 0, len(topics))
	for topic := range topics {
		topicList = append(topicList, topic)
	}

	return topicList, nil
}

// Ping checks Kafka connectivity
func (bus *KafkaEventBus) Ping(ctx context.Context) error {
	conn, err := kafka.Dial("tcp", bus.config.Brokers[0])
	if err != nil {
		return fmt.Errorf("failed to connect to Kafka: %w", err)
	}
	defer conn.Close()

	// Simple connectivity check
	_, err = conn.Controller()
	return err
}

// Close closes the event bus
func (bus *KafkaEventBus) Close() error {
	var errors []error

	// Close producer
	if err := bus.producer.Close(); err != nil {
		errors = append(errors, fmt.Errorf("failed to close producer: %w", err))
	}

	// Close all consumers
	for key, consumer := range bus.consumers {
		if err := consumer.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close consumer %s: %w", key, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors closing event bus: %v", errors)
	}

	return nil
}

// Event topics
const (
	TopicAssetUpserts       = "asset.upserts"
	TopicAssetRelationships = "asset.relationships"
	TopicSecurityEvents     = "security.events"
	TopicPolicyViolations   = "policy.violations"
	TopicRiskScores         = "risk.scores"
	TopicThreatIntel        = "threat.intel"
	TopicFindings           = "findings"
	TopicAuditLogs          = "audit.logs"
)

// GetAllTopics returns all predefined topics
func GetAllTopics() []string {
	return []string{
		TopicAssetUpserts,
		TopicAssetRelationships,
		TopicSecurityEvents,
		TopicPolicyViolations,
		TopicRiskScores,
		TopicThreatIntel,
		TopicFindings,
		TopicAuditLogs,
	}
}

// InitializeTopics creates all predefined topics
func (bus *KafkaEventBus) InitializeTopics(ctx context.Context, partitions int, replicationFactor int) error {
	topics := GetAllTopics()
	
	for _, topic := range topics {
		if err := bus.CreateTopic(ctx, topic, partitions, replicationFactor); err != nil {
			log.Printf("Warning: failed to create topic %s: %v", topic, err)
		}
	}
	
	return nil
}
