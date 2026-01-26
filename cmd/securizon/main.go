package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/securizon/internal/api"
	"github.com/securizon/internal/events"
	"github.com/securizon/internal/graph"
	"github.com/securizon/internal/risk"
	"github.com/securizon/pkg/models"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	var (
		configFile = flag.String("config", "config/config.yaml", "Configuration file path")
		version    = flag.Bool("version", false, "Show version information")
		help       = flag.Bool("help", false, "Show help information")
	)
	flag.Parse()

	if *help {
		showHelp()
		return
	}

	if *version {
		showVersion()
		return
	}

	log.Printf("Starting SecuRizon v%s (commit: %s, built: %s)", version, commit, date)

	// Load configuration
	config, err := loadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize components
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize graph store
	graphStore, err := graph.NewNeo4jStore(config.Graph)
	if err != nil {
		log.Fatalf("Failed to initialize graph store: %v", err)
	}
	defer graphStore.Close()

	// Initialize event bus
	eventBus, err := events.NewKafkaEventBus(config.Events)
	if err != nil {
		log.Fatalf("Failed to initialize event bus: %v", err)
	}
	defer eventBus.Close()

	// Initialize risk engine
	riskEngine := risk.NewEngine(config.Risk, graphStore, nil, nil)

	// Initialize API gateway
	gateway := api.NewGateway(config.API, graphStore, riskEngine, eventBus)

	// Start services
	if err := startServices(ctx, config, eventBus, gateway); err != nil {
		log.Fatalf("Failed to start services: %v", err)
	}

	// Wait for shutdown signal
	waitForShutdown(ctx, cancel, gateway)
}

func showHelp() {
	fmt.Printf(`SecuRizon - Real-time Security Posture Management Platform

Usage:
  securizon [flags]

Flags:
  -config string
        Configuration file path (default "config/config.yaml")
  -version
        Show version information
  -help
        Show this help message

Examples:
  securizon                                    # Start with default config
  securizon -config config/production.yaml     # Start with production config
  securizon -version                           # Show version

For more information, visit: https://github.com/prompt-general/SecuRizon
`)
}

func showVersion() {
	fmt.Printf("SecuRizon version %s\n", version)
	fmt.Printf("Commit: %s\n", commit)
	fmt.Printf("Built: %s\n", date)
}

func loadConfig(path string) (*Config, error) {
	// Configuration loading implementation
	return &Config{}, nil
}

func startServices(ctx context.Context, config *Config, eventBus events.EventBus, gateway *api.Gateway) error {
	// Start event processor
	// Start API gateway
	return nil
}

func waitForShutdown(ctx context.Context, cancel context.CancelFunc, gateway *api.Gateway) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutdown signal received, stopping services...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := gateway.Stop(shutdownCtx); err != nil {
		log.Printf("Error during gateway shutdown: %v", err)
	}

	cancel()
	log.Println("SecuRizon stopped")
}

type Config struct {
	Graph   graph.GraphConfig    `yaml:"graph"`
	Events  events.KafkaConfig   `yaml:"events"`
	Risk    risk.EngineConfig    `yaml:"risk"`
	API     api.GatewayConfig    `yaml:"api"`
}
