package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/securazion/event-ingestion/internal/api"
	"github.com/securazion/event-ingestion/internal/kafka"
	"github.com/securazion/event-ingestion/internal/validator"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize Kafka consumer and producer
	consumer, err := kafka.NewConsumer("event-ingestion-group")
	if err != nil {
		log.Fatal("Failed to create Kafka consumer:", err)
	}
	defer consumer.Close()
	
	producer, err := kafka.NewProducer()
	if err != nil {
		log.Fatal("Failed to create Kafka producer:", err)
	}
	defer producer.Close()

	// Create event validator
	validator := validator.NewEventValidator()
	
	// Create HTTP server for external event ingestion
	server := api.NewServer(validator, producer)
	
	// Start consuming from collector topics
	go consumeCollectorEvents(ctx, consumer, validator, producer)
	
	// Start HTTP server
	go func() {
		if err := server.Start(":8080"); err != nil {
			log.Fatal("HTTP server failed:", err)
		}
	}()
	
	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	
	log.Println("Shutting down event ingestion service...")
	server.Stop(ctx)
}

func consumeCollectorEvents(ctx context.Context, consumer kafka.Consumer, 
	validator *validator.EventValidator, producer kafka.Producer) {
	
	topics := []string{
		"raw.aws.events",
		"raw.azure.events", 
		"raw.gcp.events",
		"raw.saas.events",
	}
	
	consumer.Subscribe(ctx, topics)
	
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-consumer.Messages():
			// Validate and normalize the event
			normalized, err := validator.ValidateAndNormalize(msg.Value)
			if err != nil {
				log.Printf("Invalid event: %v", err)
				// Send to DLQ
				producer.Produce("events.dlq", msg.Key, msg.Value)
				continue
			}
			
			// Route to appropriate topic based on event type
			topic := determineTopic(normalized)
			if err := producer.Produce(topic, msg.Key, normalized); err != nil {
				log.Printf("Failed to produce event: %v", err)
			}
		}
	}
}
