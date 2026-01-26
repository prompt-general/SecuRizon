package models

import (
	"time"
	"math"
)

// RiskScore represents a risk score with its components
type RiskScore struct {
	AssetID        string    `json:"asset_id"`
	Score          float64   `json:"score"`           // 0-100
	BaseSeverity   float64   `json:"base_severity"`   // 0-10
	ExposureMult   float64   `json:"exposure_mult"`   // 1-2
	EnvironmentMult float64  `json:"environment_mult"` // 1-1.5
	ThreatIntelMult float64  `json:"threat_intel_mult"` // 1-2
	LastCalculated time.Time `json:"last_calculated"`
	Contributors   []RiskContributor `json:"contributors,omitempty"`
}

// RiskLevel represents risk levels
type RiskLevel string

const (
	RiskLevelCritical RiskLevel = "critical"
	RiskLevelHigh     RiskLevel = "high"
	RiskLevelMedium   RiskLevel = "medium"
	RiskLevelLow      RiskLevel = "low"
	RiskLevelInfo     RiskLevel = "info"
)

// GetRiskLevel returns the risk level based on score
func GetRiskLevel(score float64) RiskLevel {
	switch {
	case score >= 80:
		return RiskLevelCritical
	case score >= 60:
		return RiskLevelHigh
	case score >= 40:
		return RiskLevelMedium
	case score >= 20:
		return RiskLevelLow
	default:
		return RiskLevelInfo
	}
}

// RiskEngine interface for risk calculation
type RiskEngine interface {
	CalculateRisk(asset Asset, findings []Finding, threats []ThreatEvent) RiskScore
	RecalculateRisk(assetID string) (RiskScore, error)
	UpdateRiskScore(assetID string, score RiskScore) error
}

// DefaultRiskEngine implements the default risk calculation algorithm
type DefaultRiskEngine struct {
	exposureWeights map[AssetType]float64
	environmentWeights map[Environment]float64
}

// NewDefaultRiskEngine creates a new default risk engine
func NewDefaultRiskEngine() *DefaultRiskEngine {
	return &DefaultRiskEngine{
		exposureWeights: map[AssetType]float64{
			AssetTypeCompute: 1.5,
			AssetTypeNetwork: 1.3,
			AssetTypeData:    2.0,
			AssetTypeIdentity: 1.8,
			AssetTypeSaaS:    1.4,
		},
		environmentWeights: map[Environment]float64{
			EnvironmentProduction:  1.5,
			EnvironmentStaging:     1.2,
			EnvironmentDevelopment: 1.0,
			EnvironmentTesting:     1.1,
		},
	}
}

// CalculateRisk calculates risk score for an asset
func (r *DefaultRiskEngine) CalculateRisk(asset Asset, findings []Finding, threats []ThreatEvent) RiskScore {
	now := time.Now()
	
	// Calculate base severity from findings
	baseSeverity := r.calculateBaseSeverity(findings)
	
	// Calculate exposure multiplier
	exposureMult := r.calculateExposureMultiplier(asset)
	
	// Calculate environment multiplier
	environmentMult := r.calculateEnvironmentMultiplier(asset)
	
	// Calculate threat intel multiplier
	threatIntelMult := r.calculateThreatIntelMultiplier(threats)
	
	// Calculate final risk score
	score := baseSeverity * exposureMult * environmentMult * threatIntelMult
	
	// Ensure score is within bounds
	score = math.Min(100, math.Max(0, score))
	
	return RiskScore{
		AssetID:         asset.GetID(),
		Score:           score,
		BaseSeverity:    baseSeverity,
		ExposureMult:    exposureMult,
		EnvironmentMult: environmentMult,
		ThreatIntelMult: threatIntelMult,
		LastCalculated:  now,
		Contributors:    r.buildContributors(findings, threats),
	}
}

// calculateBaseSeverity calculates base severity from findings
func (r *DefaultRiskEngine) calculateBaseSeverity(findings []Finding) float64 {
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
func (r *DefaultRiskEngine) calculateExposureMultiplier(asset Asset) float64 {
	baseMultiplier := 1.0
	
	// Asset type specific exposure
	if weight, exists := r.exposureWeights[asset.GetType()]; exists {
		baseMultiplier *= weight
	}
	
	// Check for internet exposure
	switch a := asset.(type) {
	case *Compute:
		if a.InternetExposed {
			baseMultiplier *= 2.0
		}
		if len(a.ExposedPorts) > 0 {
			baseMultiplier *= 1.2
		}
	case *Data:
		if a.ExternalSharing {
			baseMultiplier *= 1.8
		}
		if a.DataSensitivity == DataSensitivityPublic {
			baseMultiplier *= 1.3
		}
	case *SaaS:
		if a.ExternalSharing {
			baseMultiplier *= 1.6
		}
		if a.Public {
			baseMultiplier *= 1.4
		}
	case *Identity:
		if a.PrivilegeLevel == PrivilegeLevelAdmin {
			baseMultiplier *= 1.8
		}
	}
	
	return math.Min(2.0, baseMultiplier)
}

// calculateEnvironmentMultiplier calculates environment-based risk
func (r *DefaultRiskEngine) calculateEnvironmentMultiplier(asset Asset) float64 {
	if weight, exists := r.environmentWeights[asset.GetEnvironment()]; exists {
		return weight
	}
	return 1.0
}

// calculateThreatIntelMultiplier calculates threat intelligence multiplier
func (r *DefaultRiskEngine) calculateThreatIntelMultiplier(threats []ThreatEvent) float64 {
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
func (r *DefaultRiskEngine) buildContributors(findings []Finding, threats []ThreatEvent) []RiskContributor {
	contributors := make([]RiskContributor, 0)
	
	// Add finding contributors
	for _, finding := range findings {
		contributors = append(contributors, RiskContributor{
			Type:        "finding",
			ID:          finding.ID,
			Name:        finding.PolicyID,
			Impact:      finding.Severity,
			Description: finding.Description,
		})
	}
	
	// Add threat contributors
	for _, threat := range threats {
		contributors = append(contributors, RiskContributor{
			Type:        "threat",
			ID:          threat.ThreatID,
			Name:        threat.ThreatType,
			Impact:      threat.Confidence * 10, // Scale confidence to 0-10
			Description: threat.Description,
		})
	}
	
	return contributors
}

// RiskTrend represents risk score trends over time
type RiskTrend struct {
	AssetID    string    `json:"asset_id"`
	Scores     []RiskScorePoint `json:"scores"`
	TimeRange  TimeRange `json:"time_range"`
	Trend      string    `json:"trend"` // improving, worsening, stable
	ChangeRate float64   `json:"change_rate"` // Score change per day
}

// RiskScorePoint represents a risk score at a point in time
type RiskScorePoint struct {
	Timestamp time.Time `json:"timestamp"`
	Score     float64   `json:"score"`
	Level     RiskLevel `json:"level"`
}

// TimeRange represents a time range
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// RiskSummary represents a summary of risk across assets
type RiskSummary struct {
	TotalAssets      int                `json:"total_assets"`
	AssetsByType     map[AssetType]int  `json:"assets_by_type"`
	AssetsByEnv      map[Environment]int `json:"assets_by_environment"`
	RiskDistribution map[RiskLevel]int  `json:"risk_distribution"`
	AverageRisk      float64            `json:"average_risk"`
	HighRiskAssets   []string           `json:"high_risk_assets"` // Asset IDs
	CriticalFindings int                `json:"critical_findings"`
	LastUpdated      time.Time          `json:"last_updated"`
}

// RiskThreshold represents risk thresholds for alerting
type RiskThreshold struct {
	Level      RiskLevel `json:"level"`
	MinScore   float64   `json:"min_score"`
	MaxScore   float64   `json:"max_score"`
	Actions    []string  `json:"actions"` // alert, ticket, remediate
	Enabled    bool      `json:"enabled"`
}

// DefaultRiskThresholds returns default risk thresholds
func DefaultRiskThresholds() []RiskThreshold {
	return []RiskThreshold{
		{
			Level:    RiskLevelCritical,
			MinScore: 80,
			MaxScore: 100,
			Actions:  []string{"alert", "ticket", "remediate"},
			Enabled:  true,
		},
		{
			Level:    RiskLevelHigh,
			MinScore: 60,
			MaxScore: 79.9,
			Actions:  []string{"alert", "ticket"},
			Enabled:  true,
		},
		{
			Level:    RiskLevelMedium,
			MinScore: 40,
			MaxScore: 59.9,
			Actions:  []string{"alert"},
			Enabled:  true,
		},
	}
}
