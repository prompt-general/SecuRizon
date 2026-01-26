package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/securizon/internal/events"
)

func main() {
	var (
		configFile = flag.String("config", "config/aws-collector.yaml", "Configuration file path")
		region     = flag.String("region", "us-east-1", "AWS region")
		interval   = flag.Duration("interval", 5*time.Minute, "Collection interval")
	)
	flag.Parse()

	log.Printf("Starting AWS collector for region %s", *region)

	// Initialize collector
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize event bus
	eventBus, err := events.NewKafkaEventBus(events.DefaultKafkaConfig())
	if err != nil {
		log.Fatalf("Failed to initialize event bus: %v", err)
	}
	defer eventBus.Close()

	// Start collection
	if err := startCollection(ctx, *region, *interval, eventBus); err != nil {
		log.Fatalf("Failed to start collection: %v", err)
	}

	// Wait for shutdown signal
	waitForShutdown(ctx, cancel)
}

func startCollection(ctx context.Context, region string, interval time.Duration, eventBus events.EventBus) error {
	log.Printf("Starting AWS resource collection for %s", region)
	
	// Collection implementation
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			// Collect AWS resources
			if err := collectAWSResources(ctx, region, eventBus); err != nil {
				log.Printf("Error collecting AWS resources: %v", err)
			}
		}
	}
}

func collectAWSResources(ctx context.Context, region string, eventBus events.EventBus) error {
	// AWS resource collection implementation
	log.Printf("Collecting AWS resources from %s", region)
	return nil
}

func waitForShutdown(ctx context.Context, cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutdown signal received, stopping AWS collector...")
	cancel()
}
