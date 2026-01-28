package health

import (
    "context"
    "encoding/json"
    "net/http"
    "sync"
    "time"

    "github.com/securazion/remediation-engine/internal/kafka"
    "github.com/securazion/remediation-engine/internal/store"
)

type HealthStatus string

const (
    StatusHealthy   HealthStatus = "healthy"
    StatusDegraded  HealthStatus = "degraded"
    StatusUnhealthy HealthStatus = "unhealthy"
)

type HealthCheck interface {
    Name() string
    Check(ctx context.Context) HealthResult
}

type HealthResult struct {
    Name     string       `json:"name"`
    Status   HealthStatus `json:"status"`
    Message  string       `json:"message,omitempty"`
    Duration time.Duration `json:"duration,omitempty"`
    Error    error        `json:"error,omitempty"`
}

type HealthChecker struct {
    checks []HealthCheck
    mu     sync.RWMutex
}

func NewHealthChecker() *HealthChecker {
    return &HealthChecker{checks: make([]HealthCheck, 0)}
}

func (hc *HealthChecker) Register(check HealthCheck) {
    hc.mu.Lock()
    defer hc.mu.Unlock()
    hc.checks = append(hc.checks, check)
}

func (hc *HealthChecker) Check(ctx context.Context) map[string]HealthResult {
    hc.mu.RLock()
    checks := make([]HealthCheck, len(hc.checks))
    copy(checks, hc.checks)
    hc.mu.RUnlock()

    results := make(map[string]HealthResult)
    var wg sync.WaitGroup
    var mu sync.Mutex
    for _, c := range checks {
        wg.Add(1)
        go func(ch HealthCheck) {
            defer wg.Done()
            start := time.Now()
            res := ch.Check(ctx)
            res.Duration = time.Since(start)
            mu.Lock()
            results[ch.Name()] = res
            mu.Unlock()
        }(c)
    }
    wg.Wait()
    return results
}

func (hc *HealthChecker) OverallStatus(results map[string]HealthResult) HealthStatus {
    hasDegraded := false
    for _, r := range results {
        switch r.Status {
        case StatusUnhealthy:
            return StatusUnhealthy
        case StatusDegraded:
            hasDegraded = true
        }
    }
    if hasDegraded {
        return StatusDegraded
    }
    return StatusHealthy
}

func (hc *HealthChecker) HTTPHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
        defer cancel()
        results := hc.Check(ctx)
        overall := hc.OverallStatus(results)
        resp := map[string]interface{}{
            "status":    overall,
            "timestamp": time.Now().UTC().Format(time.RFC3339),
            "checks":    results,
        }
        w.Header().Set("Content-Type", "application/json")
        statusCode := http.StatusOK
        if overall == StatusUnhealthy {
            statusCode = http.StatusServiceUnavailable
        }
        w.WriteHeader(statusCode)
        json.NewEncoder(w).Encode(resp)
    }
}

// Example health checks
type DatabaseHealthCheck struct { db store.Store }
func (d *DatabaseHealthCheck) Name() string { return "database" }
func (d *DatabaseHealthCheck) Check(ctx context.Context) HealthResult {
    start := time.Now()
    err := d.db.Ping(ctx)
    duration := time.Since(start)
    res := HealthResult{Name: d.Name(), Duration: duration}
    if err != nil {
        res.Status = StatusUnhealthy
        res.Message = "Database connection failed"
        res.Error = err
    } else if duration > 100*time.Millisecond {
        res.Status = StatusDegraded
        res.Message = "Database responding slowly"
    } else {
        res.Status = StatusHealthy
        res.Message = "Database connection healthy"
    }
    return res
}

type KafkaHealthCheck struct { producer kafka.Producer; topic string }
func (k *KafkaHealthCheck) Name() string { return "kafka" }
func (k *KafkaHealthCheck) Check(ctx context.Context) HealthResult {
    start := time.Now()
    err := k.producer.Produce(k.topic, "health-check", []byte("test"))
    duration := time.Since(start)
    res := HealthResult{Name: k.Name(), Duration: duration}
    if err != nil {
        res.Status = StatusUnhealthy
        res.Message = "Kafka producer failed"
        res.Error = err
    } else if duration > 500*time.Millisecond {
        res.Status = StatusDegraded
        res.Message = "Kafka responding slowly"
    } else {
        res.Status = StatusHealthy
        res.Message = "Kafka connection healthy"
    }
    return res
}
