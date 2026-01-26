package kafka

import (
	"context"
	"log"
	"sync"

	"github.com/segmentio/kafka-go"

	"github.com/prompt-general/securizon/internal/config"
)

// Producer defines the interface for Kafka message production
type Producer interface {
	Send(ctx context.Context, topic string, key []byte, value []byte) error
	Close() error
}

// kafkaProducer implements the Producer interface
type kafkaProducer struct {
	writer *kafka.Writer
	mu     sync.Mutex
	closed bool
}

// NewProducer creates a new Kafka producer
func NewProducer(cfg config.KafkaConfig) (Producer, error) {
	if len(cfg.Brokers) == 0 {
		log.Fatal("No Kafka brokers configured")
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Topic:        cfg.Topic,
		Compression:  kafka.Gzip,
		RequiredAcks: kafka.RequireAll,
	}

	return &kafkaProducer{
		writer: writer,
		closed: false,
	}, nil
}

// Send sends a message to Kafka
func (p *kafkaProducer) Send(ctx context.Context, topic string, key []byte, value []byte) error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return ErrProducerClosed
	}
	p.mu.Unlock()

	message := kafka.Message{
		Key:   key,
		Value: value,
	}

	if topic != "" {
		message.Topic = topic
	}

	return p.writer.WriteMessages(ctx, message)
}

// Close closes the producer
func (p *kafkaProducer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true
	return p.writer.Close()
}
