package kafka

import (
	"fmt"
	"log"
	"net"
	"strconv"

	"github.com/segmentio/kafka-go"
)

// TopicConfig defines Kafka topic configuration
type TopicConfig struct {
	Name              string
	Partitions        int
	ReplicationFactor int
	RetentionMs       int64
	CleanupPolicy     string
	KeyField          string
}

// Topics defines all Kafka topics used in Securizon
var Topics = map[string]TopicConfig{
	// Raw events from collectors
	"raw.aws.events": {
		Name:              "raw.aws.events",
		Partitions:        12,
		ReplicationFactor: 3,
		RetentionMs:       604800000, // 7 days
		CleanupPolicy:     "delete",
	},
	"raw.azure.events": {
		Name:              "raw.azure.events",
		Partitions:        8,
		ReplicationFactor: 3,
		RetentionMs:       604800000, // 7 days
		CleanupPolicy:     "delete",
	},
	"raw.gcp.events": {
		Name:              "raw.gcp.events",
		Partitions:        8,
		ReplicationFactor: 3,
		RetentionMs:       604800000, // 7 days
		CleanupPolicy:     "delete",
	},

	// Normalized events
	"asset.upserts": {
		Name:              "asset.upserts",
		Partitions:        16,
		ReplicationFactor: 3,
		RetentionMs:       2592000000, // 30 days
		CleanupPolicy:     "delete",
		KeyField:          "asset_id",
	},
	"asset.relationships": {
		Name:              "asset.relationships",
		Partitions:        16,
		ReplicationFactor: 3,
		RetentionMs:       2592000000, // 30 days
		CleanupPolicy:     "delete",
		KeyField:          "source_id",
	},
	"security.events": {
		Name:              "security.events",
		Partitions:        24,
		ReplicationFactor: 3,
		RetentionMs:       1209600000, // 14 days
		CleanupPolicy:     "delete",
		KeyField:          "resource_id",
	},

	// Findings and alerts
	"findings": {
		Name:              "findings",
		Partitions:        12,
		ReplicationFactor: 3,
		RetentionMs:       7776000000, // 90 days
		CleanupPolicy:     "delete",
		KeyField:          "finding_id",
	},
	"alerts": {
		Name:              "alerts",
		Partitions:        8,
		ReplicationFactor: 3,
		RetentionMs:       2592000000, // 30 days
		CleanupPolicy:     "delete",
		KeyField:          "alert_id",
	},

	// Attack paths
	"attackpaths": {
		Name:              "attackpaths",
		Partitions:        8,
		ReplicationFactor: 3,
		RetentionMs:       2592000000, // 30 days
		CleanupPolicy:     "delete",
	},

	// Dead letter queue
	"events.dlq": {
		Name:              "events.dlq",
		Partitions:        4,
		ReplicationFactor: 3,
		RetentionMs:       2592000000, // 30 days
		CleanupPolicy:     "delete",
	},

	// Metrics and telemetry
	"metrics": {
		Name:              "metrics",
		Partitions:        4,
		ReplicationFactor: 3,
		RetentionMs:       604800000, // 7 days
		CleanupPolicy:     "delete",
	},

	// Remediation actions
	"remediation.requests": {
		Name:              "remediation.requests",
		Partitions:        8,
		ReplicationFactor: 3,
		RetentionMs:       604800000, // 7 days
		CleanupPolicy:     "delete",
	},
	"remediation.results": {
		Name:              "remediation.results",
		Partitions:        8,
		ReplicationFactor: 3,
		RetentionMs:       2592000000, // 30 days
		CleanupPolicy:     "delete",
	},
}

// TopicManager handles Kafka topic creation and management
type TopicManager struct {
	brokers []string
}

// NewTopicManager creates a new topic manager
func NewTopicManager(brokers []string) *TopicManager {
	return &TopicManager{
		brokers: brokers,
	}
}

// CreateTopics creates all Kafka topics if they don't exist
func (tm *TopicManager) CreateTopics() error {
	conn, err := kafka.Dial("tcp", tm.brokers[0])
	if err != nil {
		return fmt.Errorf("failed to connect to Kafka broker: %v", err)
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		return fmt.Errorf("failed to get controller: %v", err)
	}

	controllerConn, err := kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if err != nil {
		return fmt.Errorf("failed to connect to controller: %v", err)
	}
	defer controllerConn.Close()

	for topicName, config := range Topics {
		topicConfigs := []kafka.TopicConfig{
			{
				Topic:             config.Name,
				NumPartitions:     config.Partitions,
				ReplicationFactor: config.ReplicationFactor,
				ConfigEntries: []kafka.ConfigEntry{
					{
						ConfigName:  "retention.ms",
						ConfigValue: fmt.Sprintf("%d", config.RetentionMs),
					},
					{
						ConfigName:  "cleanup.policy",
						ConfigValue: config.CleanupPolicy,
					},
				},
			},
		}

		err := controllerConn.CreateTopics(topicConfigs...)
		if err != nil {
			// Topic might already exist, log warning but continue
			log.Printf("Warning creating topic %s: %v", topicName, err)
		} else {
			log.Printf("Created topic: %s", topicName)
		}
	}

	return nil
}

// ListTopics lists all Kafka topics
func (tm *TopicManager) ListTopics() ([]string, error) {
	conn, err := kafka.Dial("tcp", tm.brokers[0])
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Kafka broker: %v", err)
	}
	defer conn.Close()

	partitions, err := conn.ReadPartitions()
	if err != nil {
		return nil, fmt.Errorf("failed to read partitions: %v", err)
	}

	topicMap := make(map[string]bool)
	for _, partition := range partitions {
		topicMap[partition.Topic] = true
	}

	topics := make([]string, 0, len(topicMap))
	for topic := range topicMap {
		topics = append(topics, topic)
	}

	return topics, nil
}

// GetTopicConfig retrieves the configuration for a specific topic
func GetTopicConfig(topicName string) (TopicConfig, error) {
	config, exists := Topics[topicName]
	if !exists {
		return TopicConfig{}, fmt.Errorf("topic %s not found in configuration", topicName)
	}
	return config, nil
}
