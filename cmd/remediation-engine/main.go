package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/securazion/remediation-engine/internal/api"
	"github.com/securazion/remediation-engine/internal/executor"
	"github.com/securazion/remediation-engine/internal/kafka"
	"github.com/securazion/remediation-engine/internal/playbook"
	"github.com/securazion/remediation-engine/internal/store"
	"github.com/securazion/remediation-engine/internal/workflow"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize dependencies
	db := store.NewPostgresStore()
	kafkaProducer, err := kafka.NewProducer()
	if err != nil {
		log.Fatal("Failed to create Kafka producer:", err)
	}
	defer kafkaProducer.Close()

	// Load playbooks
	playbookManager := playbook.NewManager()
	if err := playbookManager.LoadFromDirectory("/etc/securazion/playbooks"); err != nil {
		log.Fatal("Failed to load playbooks:", err)
	}

	// Create executor with different runners for each cloud provider
	exec := executor.NewExecutor(db, kafkaProducer)
	exec.RegisterRunner("aws", executor.NewAWSRunner())
	exec.RegisterRunner("azure", executor.NewAzureRunner())
	exec.RegisterRunner("gcp", executor.NewGCPRunner())
	exec.RegisterRunner("script", executor.NewScriptRunner())

	// Create approval workflow manager
	approvalManager := workflow.NewApprovalManager(db, kafkaProducer)

	// Create remediation engine
	engine := NewRemediationEngine(exec, approvalManager, playbookManager, db)
	go engine.Start(ctx)

	// Start HTTP server for API
	server := api.NewServer(engine, approvalManager, playbookManager)
	go func() {
		if err := server.Start(":8080"); err != nil {
			log.Fatal("HTTP server failed:", err)
		}
	}()

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down remediation engine...")
	server.Stop(ctx)
}
