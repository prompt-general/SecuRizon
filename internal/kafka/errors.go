package kafka

import "errors"

var (
	// ErrProducerClosed is returned when trying to send on a closed producer
	ErrProducerClosed = errors.New("producer is closed")

	// ErrInvalidBrokers is returned when no brokers are configured
	ErrInvalidBrokers = errors.New("no kafka brokers configured")

	// ErrInvalidTopic is returned when topic is empty
	ErrInvalidTopic = errors.New("kafka topic cannot be empty")
)
