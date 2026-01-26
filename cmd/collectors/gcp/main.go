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
		configFile = flag.String("config", "config/gcp-collector.yaml", "Configuration file path")
		project    = flag.String("project", "", "GCP project ID")
		interval   = flag.Duration("interval", 5*time.Minute, "Collection interval")
	)
	flag.Parse()

	log.Printf("Starting GCP collector for project %s", *project)

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
	if err := startCollection(ctx, *project, *interval, eventBus); err != nil {
		log.Fatalf("Failed to start collection: %v", err)
	}

	// Wait for shutdown signal
	waitForShutdown(ctx, cancel)
}

func startCollection(ctx context.Context, project string, interval time.Duration, eventBus events.EventBus) error {
	log.Printf("Starting GCP resource collection for %s", project)
	
	// Collection implementation
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			// Collect GCP resources
			if err := collectGCPResources(ctx, project, eventBus); err != nil {
				log.Printf("Error collecting GCP resources: %v", err)
			}
		}
	}
}

func collectGCPResources(ctx context.Context, project string, eventBus events.EventBus) error {
	// GCP resource collection implementation
	log.Printf("Collecting GCP resources from %s", project)
	return nil
}

func waitForShutdown(ctx context.Context, cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutdown signal received, stopping GCP collector...")
	cancel()
}
