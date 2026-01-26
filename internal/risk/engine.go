package risk

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/securizon/pkg/models"
)

// Engine implements the risk scoring engine
type Engine struct {
	config           EngineConfig
	graphStore       GraphStore
	threatIntel      ThreatIntelProvider
	policyEngine     PolicyEngine
	cache            *RiskCache
	metrics          *EngineMetrics
	mu               sync.RWMutex
}

// GraphStore interface for graph operations
type GraphStore interface {
	GetAsset(ctx context.Context, id string) (models.Asset, error)
	GetAssetFindings(ctx context.Context, assetID string) ([]models.Finding, error)
	GetAssetRisk(ctx context.Context, assetID string) (models.RiskScore, error)
	UpdateAssetRisk(ctx context.Context, risk models.RiskScore) error
	ListAssets(ctx context.Context, filter models.AssetFilter) ([]models.Asset, error)
	GetNeighbors(ctx context.Context, assetID string, direction string, maxDepth int) ([]models.Asset, []models.Relationship, error)
}

// ThreatIntelProvider interface for threat intelligence
type ThreatIntelProvider interface {
	GetThreatsForAsset(ctx context.Context, asset models.Asset) ([]models.ThreatEvent, error)
	GetThreatsByIndicator(ctx context.Context, indicator string) ([]models.ThreatEvent, error)
	IsIndicatorMalicious(ctx context.Context, indicator string) (bool, float64, error)
}

// PolicyEngine interface for policy evaluation
type PolicyEngine interface {
	EvaluateAsset(ctx context.Context, asset models.Asset) ([]models.Finding, error)
	GetPolicies(ctx context.Context, filter models.PolicyFilter) ([]models.Policy, error)
}

// EngineConfig represents risk engine configuration
type EngineConfig struct {
	// Risk calculation weights
	BaseSeverityWeight    float64 `json:"base_severity_weight"`
	ExposureWeight        float64 `json:"exposure_weight"`
	EnvironmentWeight     float64 `json:"environment_weight"`
	ThreatIntelWeight     float64 `json:"threat_intel_weight"`
	
	// Risk thresholds
	CriticalThreshold     float64 `json:"critical_threshold"`
	HighThreshold         float64 `json:"high_threshold"`
	MediumThreshold       float64 `json:"medium_threshold"`
	
	// Cache configuration
	CacheEnabled          bool          `json:"cache_enabled"`
	CacheTTL              time.Duration `json:"cache_ttl"`
	CacheSize             int           `json:"cache_size"`
	
	// Calculation settings
	EnablePropagation     bool          `json:"enable_propagation"`
	PropagationDepth      int           `json:"propagation_depth"`
	DecayFactor           float64       `json:"decay_factor"`
	
	// Performance settings
	BatchSize             int           `json:"batch_size"`
	CalculationTimeout    time.Duration `json:"calculation_timeout"`
	EnableMetrics         bool          `json:"enable_metrics"`
	MetricsInterval       time.Duration `json:"metrics_interval"`
}

// DefaultEngineConfig returns default engine configuration
func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		BaseSeverityWeight:  1.0,
		ExposureWeight:      1.0,
		EnvironmentWeight:   1.0,
		ThreatIntelWeight:   1.0,
		
		CriticalThreshold:   80.0,
		HighThreshold:       60.0,
		MediumThreshold:     40.0,
		
		CacheEnabled:        true,
		CacheTTL:            5 * time.Minute,
		CacheSize:           10000,
		
		EnablePropagation:   true,
		PropagationDepth:    3,
		DecayFactor:         0.5,
		
		BatchSize:           100,
		CalculationTimeout:  30 * time.Second,
		EnableMetrics:       true,
		MetricsInterval:     60 * time.Second,
	}
}

// RiskCache caches risk calculations
type RiskCache struct {
	entries map[string]*CacheEntry
	mu      sync.RWMutex
	maxSize int
	ttl     time.Duration
}

// CacheEntry represents a cached risk score
type CacheEntry struct {
	RiskScore  models.RiskScore
	ExpiresAt  time.Time
	AccessedAt time.Time
}

// NewRiskCache creates a new risk cache
func NewRiskCache(maxSize int, ttl time.Duration) *RiskCache {
	cache := &RiskCache{
		entries: make(map[string]*CacheEntry),
		maxSize: maxSize,
		ttl:     ttl,
	}
	
	// Start cleanup goroutine
	go cache.cleanup()
	
	return cache
}

// Get retrieves a cached risk score
func (c *RiskCache) Get(assetID string) (models.RiskScore, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	entry, exists := c.entries[assetID]
	if !exists || time.Now().After(entry.ExpiresAt) {
		return models.RiskScore{}, false
	}
	
	entry.AccessedAt = time.Now()
	return entry.RiskScore, true
}

// Set stores a risk score in cache
func (c *RiskCache) Set(assetID string, risk models.RiskScore) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Evict if cache is full
	if len(c.entries) >= c.maxSize {
		c.evictLRU()
	}
	
	c.entries[assetID] = &CacheEntry{
		RiskScore:  risk,
		ExpiresAt:  time.Now().Add(c.ttl),
		AccessedAt: time.Now(),
	}
}

// evictLRU evicts the least recently used entry
func (c *RiskCache) evictLRU() {
	var oldestKey string
	var oldestTime time.Time
	
	for key, entry := range c.entries {
		if oldestKey == "" || entry.AccessedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.AccessedAt
		}
	}
	
	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

// cleanup removes expired entries
func (c *RiskCache) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, entry := range c.entries {
			if now.After(entry.ExpiresAt) {
				delete(c.entries, key)
			}
		}
		c.mu.Unlock()
	}
}

// EngineMetrics represents risk engine metrics
type EngineMetrics struct {
	CalculationsPerformed int64     `json:"calculations_performed"`
	CalculationsFailed    int64     `json:"calculations_failed"`
	CacheHits            int64     `json:"cache_hits"`
	CacheMisses          int64     `json:"cache_misses"`
	AverageCalculationTime time.Duration `json:"average_calculation_time"`
	LastCalculation      time.Time `json:"last_calculation"`
	RiskDistribution     map[models.RiskLevel]int64 `json:"risk_distribution"`
	CalculationErrors    map[string]int64 `json:"calculation_errors"`
	mu                   sync.RWMutex
}

// NewEngine creates a new risk engine
func NewEngine(config EngineConfig, graphStore GraphStore, threatIntel ThreatIntelProvider, policyEngine PolicyEngine) *Engine {
	engine := &Engine{
		config:      config,
		graphStore:  graphStore,
		threatIntel: threatIntel,
		policyEngine: policyEngine,
		metrics: &EngineMetrics{
			RiskDistribution: make(map[models.RiskLevel]int64),
			CalculationErrors: make(map[string]int64),
		},
	}
	
	if config.CacheEnabled {
		engine.cache = NewRiskCache(config.CacheSize, config.CacheTTL)
	}
	
	return engine
}

// CalculateRisk calculates risk score for an asset
func (e *Engine) CalculateRisk(ctx context.Context, asset models.Asset, findings []models.Finding, threats []models.ThreatEvent) (models.RiskScore, error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		e.updateMetrics(duration, nil)
	}()

	// Check cache first
	if e.cache != nil {
		if cached, found := e.cache.Get(asset.GetID()); found {
			e.metrics.mu.Lock()
			e.metrics.CacheHits++
			e.metrics.mu.Unlock()
			return cached, nil
		}
		e.metrics.mu.Lock()
		e.metrics.CacheMisses++
		e.metrics.mu.Unlock()
	}

	// Calculate base severity from findings
	baseSeverity := e.calculateBaseSeverity(findings)
	
	// Calculate exposure multiplier
	exposureMult := e.calculateExposureMultiplier(asset)
	
	// Calculate environment multiplier
	environmentMult := e.calculateEnvironmentMultiplier(asset)
	
	// Calculate threat intelligence multiplier
	threatIntelMult := e.calculateThreatIntelMultiplier(threats)
	
	// Apply weights
	weightedBaseSeverity := baseSeverity * e.config.BaseSeverityWeight
	weightedExposure := (exposureMult - 1.0) * e.config.ExposureWeight
	weightedEnvironment := (environmentMult - 1.0) * e.config.EnvironmentWeight
	weightedThreatIntel := (threatIntelMult - 1.0) * e.config.ThreatIntelWeight
	
	// Calculate final risk score
	riskScore := weightedBaseSeverity * (1.0 + weightedExposure + weightedEnvironment + weightedThreatIntel)
	
	// Ensure score is within bounds
	riskScore = math.Min(100, math.Max(0, riskScore))
	
	risk := models.RiskScore{
		AssetID:         asset.GetID(),
		Score:           riskScore,
		BaseSeverity:    baseSeverity,
		ExposureMult:    exposureMult,
		EnvironmentMult: environmentMult,
		ThreatIntelMult: threatIntelMult,
		LastCalculated:  time.Now(),
		Contributors:    e.buildContributors(findings, threats),
	}
	
	// Cache the result
	if e.cache != nil {
		e.cache.Set(asset.GetID(), risk)
	}
	
	// Update risk distribution
	e.updateRiskDistribution(risk.Score)
	
	return risk, nil
}

// RecalculateRisk recalculates risk for an asset
func (e *Engine) RecalculateRisk(ctx context.Context, assetID string) (models.RiskScore, error) {
	// Get asset
	asset, err := e.graphStore.GetAsset(ctx, assetID)
	if err != nil {
		return models.RiskScore{}, fmt.Errorf("failed to get asset %s: %w", assetID, err)
	}
	
	// Get findings
	findings, err := e.graphStore.GetAssetFindings(ctx, assetID)
	if err != nil {
		return models.RiskScore{}, fmt.Errorf("failed to get findings for asset %s: %w", assetID, err)
	}
	
	// Get threats
	var threats []models.ThreatEvent
	if e.threatIntel != nil {
		threats, err = e.threatIntel.GetThreatsForAsset(ctx, asset)
		if err != nil {
			log.Printf("Failed to get threats for asset %s: %v", assetID, err)
		}
	}
	
	// Calculate risk
	risk, err := e.CalculateRisk(ctx, asset, findings, threats)
	if err != nil {
		return models.RiskScore{}, err
	}
	
	// Update in graph store
	if err := e.graphStore.UpdateAssetRisk(ctx, risk); err != nil {
		return models.RiskScore{}, fmt.Errorf("failed to update risk for asset %s: %w", assetID, err)
	}
	
	// Propagate risk to connected assets if enabled
	if e.config.EnablePropagation {
		go e.propagateRisk(ctx, assetID, risk.Score)
	}
	
	return risk, nil
}

// UpdateRiskScore updates risk score for an asset
func (e *Engine) UpdateRiskScore(ctx context.Context, assetID string, score models.RiskScore) error {
	// Update in graph store
	if err := e.graphStore.UpdateAssetRisk(ctx, score); err != nil {
		return fmt.Errorf("failed to update risk for asset %s: %w", assetID, err)
	}
	
	// Update cache
	if e.cache != nil {
		e.cache.Set(assetID, score)
	}
	
	// Update risk distribution
	e.updateRiskDistribution(score.Score)
	
	return nil
}

// BatchRecalculateRisk recalculates risk for multiple assets
func (e *Engine) BatchRecalculateRisk(ctx context.Context, assetIDs []string) ([]models.RiskScore, error) {
	results := make([]models.RiskScore, 0, len(assetIDs))
	
	// Process in batches
	for i := 0; i < len(assetIDs); i += e.config.BatchSize {
		end := i + e.config.BatchSize
		if end > len(assetIDs) {
			end = len(assetIDs)
		}
		
		batch := assetIDs[i:end]
		batchResults := make([]models.RiskScore, len(batch))
		
		// Process batch in parallel
		var wg sync.WaitGroup
		for j, assetID := range batch {
			wg.Add(1)
			go func(idx int, id string) {
				defer wg.Done()
				risk, err := e.RecalculateRisk(ctx, id)
				if err != nil {
					log.Printf("Failed to recalculate risk for asset %s: %v", id, err)
					return
				}
				batchResults[idx] = risk
			}(j, assetID)
		}
		wg.Wait()
		
		// Filter out empty results
		for _, result := range batchResults {
			if result.AssetID != "" {
				results = append(results, result)
			}
		}
	}
	
	return results, nil
}

// propagateRisk propagates risk to connected assets
func (e *Engine) propagateRisk(ctx context.Context, assetID string, riskScore float64) {
	if !e.config.EnablePropagation {
		return
	}
	
	// Get neighbors
	neighbors, _, err := e.graphStore.GetNeighbors(ctx, assetID, "both", e.config.PropagationDepth)
	if err != nil {
		log.Printf("Failed to get neighbors for asset %s: %v", assetID, err)
		return
	}
	
	// Calculate propagated risk for each neighbor
	for _, neighbor := range neighbors {
		// Apply decay factor
		propagatedRisk := riskScore * e.config.DecayFactor
		
		// Get current risk
		currentRisk, err := e.graphStore.GetAssetRisk(ctx, neighbor.GetID())
		if err != nil {
			log.Printf("Failed to get current risk for neighbor %s: %v", neighbor.GetID(), err)
			continue
		}
		
		// Combine risks (take maximum)
		newRisk := math.Max(currentRisk.Score, propagatedRisk)
		
		// Update if significantly different
		if math.Abs(newRisk-currentRisk.Score) > 1.0 {
			updatedRisk := currentRisk
			updatedRisk.Score = newRisk
			updatedRisk.LastCalculated = time.Now()
			
			if err := e.graphStore.UpdateAssetRisk(ctx, updatedRisk); err != nil {
				log.Printf("Failed to update propagated risk for neighbor %s: %v", neighbor.GetID(), err)
			}
		}
	}
}

// calculateBaseSeverity calculates base severity from findings
func (e *Engine) calculateBaseSeverity(findings []models.Finding) float64 {
	if len(findings) == 0 {
		return 0
	}
	
	totalSeverity := 0.0
	maxSeverity := 0.0
	
	for _, finding := range findings {
		totalSeverity += finding.Severity
		if finding.Severity > maxSeverity {
			maxSeverity = finding.Severity
		}
	}
	
	// Weight towards maximum severity but consider all findings
	avgSeverity := totalSeverity / float64(len(findings))
	weightedSeverity := (maxSeverity * 0.7) + (avgSeverity * 0.3)
	
	return weightedSeverity
}

// calculateExposureMultiplier calculates exposure based on asset characteristics
func (e *Engine) calculateExposureMultiplier(asset models.Asset) float64 {
	baseMultiplier := 1.0
	
	switch a := asset.(type) {
	case *models.Compute:
		if a.InternetExposed {
			baseMultiplier *= 2.0
		}
		if len(a.ExposedPorts) > 0 {
			baseMultiplier *= 1.2
		}
	case *models.Data:
		if a.ExternalSharing {
			baseMultiplier *= 1.8
		}
		if a.DataSensitivity == models.DataSensitivityPublic {
			baseMultiplier *= 1.3
		}
	case *models.SaaS:
		if a.ExternalSharing {
			baseMultiplier *= 1.6
		}
		if a.Public {
			baseMultiplier *= 1.4
		}
	case *models.Identity:
		if a.PrivilegeLevel == models.PrivilegeLevelAdmin {
			baseMultiplier *= 1.8
		}
	}
	
	return math.Min(2.0, baseMultiplier)
}

// calculateEnvironmentMultiplier calculates environment-based risk
func (e *Engine) calculateEnvironmentMultiplier(asset models.Asset) float64 {
	switch asset.GetEnvironment() {
	case models.EnvironmentProduction:
		return 1.5
	case models.EnvironmentStaging:
		return 1.2
	case models.EnvironmentTesting:
		return 1.1
	default:
		return 1.0
	}
}

// calculateThreatIntelMultiplier calculates threat intelligence multiplier
func (e *Engine) calculateThreatIntelMultiplier(threats []models.ThreatEvent) float64 {
	if len(threats) == 0 {
		return 1.0
	}
	
	multiplier := 1.0
	for _, threat := range threats {
		// Higher confidence threats have more impact
		confidenceMultiplier := 1.0 + (threat.Confidence * 0.5)
		multiplier += confidenceMultiplier * 0.2
	}
	
	return math.Min(2.0, multiplier)
}

// buildContributors builds risk contributors from findings and threats
func (e *Engine) buildContributors(findings []models.Finding, threats []models.ThreatEvent) []models.RiskContributor {
	contributors := make([]models.RiskContributor, 0)
	
	// Add finding contributors
	for _, finding := range findings {
		contributors = append(contributors, models.RiskContributor{
			Type:        "finding",
			ID:          finding.ID,
			Name:        finding.PolicyID,
			Impact:      finding.Severity,
			Description: finding.Description,
		})
	}
	
	// Add threat contributors
	for _, threat := range threats {
		contributors = append(contributors, models.RiskContributor{
			Type:        "threat",
			ID:          threat.ThreatID,
			Name:        threat.ThreatType,
			Impact:      threat.Confidence * 10, // Scale confidence to 0-10
			Description: threat.Description,
		})
	}
	
	return contributors
}

// updateRiskDistribution updates risk distribution metrics
func (e *Engine) updateRiskDistribution(score float64) {
	if !e.config.EnableMetrics {
		return
	}
	
	e.metrics.mu.Lock()
	defer e.metrics.mu.Unlock()
	
	level := models.GetRiskLevel(score)
	e.metrics.RiskDistribution[level]++
}

// updateMetrics updates engine metrics
func (e *Engine) updateMetrics(duration time.Duration, err error) {
	if !e.config.EnableMetrics {
		return
	}
	
	e.metrics.mu.Lock()
	defer e.metrics.mu.Unlock()
	
	e.metrics.CalculationsPerformed++
	e.metrics.LastCalculation = time.Now()
	
	if err != nil {
		e.metrics.CalculationsFailed++
		e.metrics.CalculationErrors[err.Error()]++
	}
	
	// Update average calculation time
	if e.metrics.AverageCalculationTime == 0 {
		e.metrics.AverageCalculationTime = duration
	} else {
		e.metrics.AverageCalculationTime = (e.metrics.AverageCalculationTime + duration) / 2
	}
}

// GetMetrics returns current engine metrics
func (e *Engine) GetMetrics() EngineMetrics {
	e.metrics.mu.RLock()
	defer e.metrics.mu.RUnlock()
	return *e.metrics
}

// GetRiskSummary returns risk summary for all assets
func (e *Engine) GetRiskSummary(ctx context.Context) (*models.RiskSummary, error) {
	// Get all assets
	assets, err := e.graphStore.ListAssets(ctx, models.AssetFilter{})
	if err != nil {
		return nil, fmt.Errorf("failed to list assets: %w", err)
	}
	
	summary := &models.RiskSummary{
		TotalAssets:      len(assets),
		AssetsByType:     make(map[models.AssetType]int),
		AssetsByEnv:      make(map[models.Environment]int),
		RiskDistribution: make(map[models.RiskLevel]int),
		HighRiskAssets:   make([]string, 0),
		LastUpdated:      time.Now(),
	}
	
	var totalRisk float64
	var criticalFindings int
	
	for _, asset := range assets {
		// Count by type and environment
		summary.AssetsByType[asset.GetType()]++
		summary.AssetsByEnv[asset.GetEnvironment()]++
		
		// Get risk score
		risk, err := e.graphStore.GetAssetRisk(ctx, asset.GetID())
		if err != nil {
			log.Printf("Failed to get risk for asset %s: %v", asset.GetID(), err)
			continue
		}
		
		totalRisk += risk.Score
		
		// Count by risk level
		level := models.GetRiskLevel(risk.Score)
		summary.RiskDistribution[level]++
		
		// Track high-risk assets
		if level == models.RiskLevelHigh || level == models.RiskLevelCritical {
			summary.HighRiskAssets = append(summary.HighRiskAssets, asset.GetID())
		}
		
		// Count critical findings
		if level == models.RiskLevelCritical {
			criticalFindings++
		}
	}
	
	// Calculate average risk
	if len(assets) > 0 {
		summary.AverageRisk = totalRisk / float64(len(assets))
	}
	
	summary.CriticalFindings = criticalFindings
	
	return summary, nil
}

// ClearCache clears the risk cache
func (e *Engine) ClearCache() {
	if e.cache != nil {
		e.cache.mu.Lock()
		defer e.cache.mu.Unlock()
		e.cache.entries = make(map[string]*CacheEntry)
	}
}

// GetCacheStats returns cache statistics
func (e *Engine) GetCacheStats() map[string]interface{} {
	if e.cache == nil {
		return map[string]interface{}{
			"enabled": false,
		}
	}
	
	e.cache.mu.RLock()
	defer e.cache.mu.RUnlock()
	
	return map[string]interface{}{
		"enabled":    true,
		"size":       len(e.cache.entries),
		"max_size":   e.cache.maxSize,
		"ttl":        e.cache.ttl.String(),
		"hit_rate":   float64(e.metrics.CacheHits) / float64(e.metrics.CacheHits+e.metrics.CacheMisses),
		"hits":       e.metrics.CacheHits,
		"misses":     e.metrics.CacheMisses,
	}
}
