package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prompt-general/securizon/internal/collector"
	"github.com/prompt-general/securizon/internal/config"
	"github.com/prompt-general/securizon/internal/kafka"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load configuration
	cfg := config.Load()
	
	// Initialize Kafka producer
	producer, err := kafka.NewProducer(cfg.Kafka)
	if err != nil {
		log.Fatal("Failed to create Kafka producer:", err)
	}
	defer producer.Close()

	// Create collector manager
	collectorMgr := collector.NewManager(ctx, cfg, producer)

	// Start collection routines
	collectorMgr.Start()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	
	log.Println("Shutting down collector...")
	collectorMgr.Stop()
	log.Println("Collector stopped")
}
