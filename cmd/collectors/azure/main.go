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
		configFile = flag.String("config", "config/azure-collector.yaml", "Configuration file path")
		subscription = flag.String("subscription", "", "Azure subscription ID")
		interval   = flag.Duration("interval", 5*time.Minute, "Collection interval")
	)
	flag.Parse()

	log.Printf("Starting Azure collector for subscription %s", *subscription)

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
	if err := startCollection(ctx, *subscription, *interval, eventBus); err != nil {
		log.Fatalf("Failed to start collection: %v", err)
	}

	// Wait for shutdown signal
	waitForShutdown(ctx, cancel)
}

func startCollection(ctx context.Context, subscription string, interval time.Duration, eventBus events.EventBus) error {
	log.Printf("Starting Azure resource collection for %s", subscription)
	
	// Collection implementation
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			// Collect Azure resources
			if err := collectAzureResources(ctx, subscription, eventBus); err != nil {
				log.Printf("Error collecting Azure resources: %v", err)
			}
		}
	}
}

func collectAzureResources(ctx context.Context, subscription string, eventBus events.EventBus) error {
	// Azure resource collection implementation
	log.Printf("Collecting Azure resources from %s", subscription)
	return nil
}

func waitForShutdown(ctx context.Context, cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutdown signal received, stopping Azure collector...")
	cancel()
}
