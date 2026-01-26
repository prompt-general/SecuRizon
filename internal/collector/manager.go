package collector

import (
	"context"
	"log"
	"sync"

	"github.com/prompt-general/securizon/internal/config"
	"github.com/prompt-general/securizon/internal/kafka"
)

// Manager manages the lifecycle of collectors
type Manager struct {
	ctx      context.Context
	cancel   context.CancelFunc
	cfg      *config.Config
	producer kafka.Producer
	wg       sync.WaitGroup
	running  bool
	mu       sync.Mutex
}

// NewManager creates a new collector manager
func NewManager(ctx context.Context, cfg *config.Config, producer kafka.Producer) *Manager {
	childCtx, cancel := context.WithCancel(ctx)
	return &Manager{
		ctx:      childCtx,
		cancel:   cancel,
		cfg:      cfg,
		producer: producer,
		running:  false,
	}
}

// Start begins the collection routines
func (m *Manager) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()

	log.Println("Starting collector manager...")
	
	// Start AWS collector routine
	m.wg.Add(1)
	go m.runAWSCollector()
}

// Stop gracefully shuts down the collector
func (m *Manager) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	m.mu.Unlock()

	log.Println("Stopping collector manager...")
	m.cancel()
	m.wg.Wait()
	log.Println("Collector manager stopped")
}

// runAWSCollector runs the AWS collection routine
func (m *Manager) runAWSCollector() {
	defer m.wg.Done()
	
	log.Println("AWS collector routine started")
	
	<-m.ctx.Done()
	
	log.Println("AWS collector routine stopped")
}
