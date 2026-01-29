package customersuccess

import (
	"time"
)

type CustomerHealth struct {
	TenantID        string           `json:"tenant_id"`
	Score           float64          `json:"score"` // 0-100
	Level           HealthLevel      `json:"level"` // critical, warning, healthy
	Factors         []HealthFactor   `json:"factors"`
	Recommendations []Recommendation `json:"recommendations"`
	LastUpdated     time.Time        `json:"last_updated"`
}

type HealthLevel string

const (
	HealthCritical HealthLevel = "critical"
	HealthWarning  HealthLevel = "warning"
	HealthHealthy  HealthLevel = "healthy"
)

type HealthFactor struct {
	Name        string  `json:"name"`
	Score       float64 `json:"score"`
	Weight      float64 `json:"weight"`
	Description string  `json:"description"`
	Trend       string  `json:"trend"` // improving, stable, declining
}

type Recommendation struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"` // high, medium, low
	ActionURL   string `json:"action_url"`
	Category    string `json:"category"`
}

type QuarterlyBusinessReview struct {
	TenantID         string            `json:"tenant_id"`
	Quarter          string            `json:"quarter"`
	Period           string            `json:"period"`
	ExecutiveSummary string            `json:"executive_summary"`
	UsageAnalytics   map[string]int64  `json:"usage_analytics"`
	HealthScore      *CustomerHealth   `json:"health_score"`
	SupportActivity  interface{}       `json:"support_activity"`
	KeyAchievements  []string          `json:"key_achievements"`
	Recommendations  []Recommendation  `json:"recommendations"`
	NextQuarterGoals []string          `json:"next_quarter_goals"`
	PDFReport        []byte            `json:"pdf_report"`
}

type CSConfig struct {
	HealthCheckInterval time.Duration
	RiskThresholds      map[HealthLevel]float64
}
