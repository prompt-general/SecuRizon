package models

import "time"

// AttackPath represents a chain of assets that can be exploited to reach a target
type AttackPath struct {
	ID                 string                      `json:"id"`
	SourceID           string                      `json:"source_id"`
	TargetID           string                      `json:"target_id"`
	Hops               int                         `json:"hops"`
	CumulativeRisk     float64                     `json:"cumulative_risk"`
	Path               []PathNode                  `json:"path"`
	Vulnerabilities    []AttackPathVulnerability   `json:"vulnerabilities"`
	CreatedAt          time.Time                   `json:"created_at"`
	UpdatedAt          time.Time                   `json:"updated_at"`
	Recommendations    []string                    `json:"recommendations,omitempty"`
}

// PathNode represents an asset in an attack path
type PathNode struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	RiskScore float64               `json:"risk_score"`
	Role     string                 `json:"role"` // entry_point, pivot_point, target
	Details  map[string]interface{} `json:"details,omitempty"`
}

// AttackPathVulnerability represents a finding that enables exploitation in an attack path
type AttackPathVulnerability struct {
	FindingID       string  `json:"finding_id"`
	Severity        float64 `json:"severity"`
	Description     string  `json:"description"`
	ExploitedInPath bool    `json:"exploited_in_path"`
}

// AttackSimulation represents the result of simulating an attack
type AttackSimulation struct {
	Paths              []AttackPath `json:"paths"`
	HighestRisk        float64      `json:"highest_risk"`
	AverageRisk        float64      `json:"average_risk"`
	CriticalPaths      int          `json:"critical_paths"`
	Recommendations    []string     `json:"recommendations"`
	SimulatedAt        time.Time    `json:"simulated_at"`
}

// PathDiscoveryRequest represents parameters for discovering attack paths
type PathDiscoveryRequest struct {
	SourceID        string `json:"source_id,omitempty"`
	TargetID        string `json:"target_id,omitempty"`
	SourceType      string `json:"source_type,omitempty"` // internet, public, compromised
	TargetType      string `json:"target_type,omitempty"` // data, admin, sensitive
	MaxHops         int    `json:"max_hops,omitempty"`
	MinRiskScore    float64 `json:"min_risk_score,omitempty"`
	IncludePaths    []string `json:"include_paths,omitempty"` // relationship types to follow
	ExcludePaths    []string `json:"exclude_paths,omitempty"` // relationship types to skip
}

// PathVisualization provides graph-format data for frontend visualization
type PathVisualization struct {
	Nodes []VisualizationNode `json:"nodes"`
	Edges []VisualizationEdge `json:"edges"`
}

// VisualizationNode represents a node in path visualization
type VisualizationNode struct {
	ID        string  `json:"id"`
	Label     string  `json:"label"`
	AssetType string  `json:"asset_type"`
	RiskScore float64 `json:"risk_score"`
	Role      string  `json:"role"`
	Color     string  `json:"color"` // Based on risk severity
}

// VisualizationEdge represents a relationship edge in path visualization
type VisualizationEdge struct {
	Source       string  `json:"source"`
	Target       string  `json:"target"`
	RelationType string  `json:"relation_type"`
	RiskScore    float64 `json:"risk_score"`
	Vulnerabilities int  `json:"vulnerabilities"`
}

// PathAnalysisReport provides detailed analysis of attack paths
type PathAnalysisReport struct {
	TotalPaths           int                    `json:"total_paths"`
	CriticalPaths        int                    `json:"critical_paths"`
	HighRiskPaths        int                    `json:"high_risk_paths"`
	AveragePathLength    float64                `json:"average_path_length"`
	MostCommonVulnerabilities []VulnerabilityImpact `json:"most_common_vulnerabilities"`
	TopTargets           []TargetRisk           `json:"top_targets"`
	Recommendations      []RecommendationAction `json:"recommendations"`
	GeneratedAt          time.Time              `json:"generated_at"`
}

// VulnerabilityImpact tracks how often a vulnerability appears in critical paths
type VulnerabilityImpact struct {
	FindingID     string  `json:"finding_id"`
	Title         string  `json:"title"`
	AffectedAssets int    `json:"affected_assets"`
	PathsAffected  int    `json:"paths_affected"`
	AvgRiskContribution float64 `json:"avg_risk_contribution"`
}

// TargetRisk represents assets that are frequently targeted in attack paths
type TargetRisk struct {
	AssetID           string  `json:"asset_id"`
	AssetType         string  `json:"asset_type"`
	TimesBeTarged     int     `json:"times_targeted"`
	HighestPathRisk   float64 `json:"highest_path_risk"`
	UniqueAttackPaths int     `json:"unique_attack_paths"`
	DataSensitivity   string  `json:"data_sensitivity"`
}

// RecommendationAction represents a specific action to break attack paths
type RecommendationAction struct {
	ID              string    `json:"id"`
	Priority        string    `json:"priority"` // critical, high, medium, low
	Type            string    `json:"type"`     // network_segmentation, access_control, remediation, detection
	Description     string    `json:"description"`
	TargetAssets    []string  `json:"target_assets"`
	PathsAffected   int       `json:"paths_affected"`
	EstimatedRiskReduction float64 `json:"estimated_risk_reduction"`
	EffortRequired  string    `json:"effort_required"`
}

// PathTrendAnalysis tracks changes in attack paths over time
type PathTrendAnalysis struct {
	TimeperiodStart  time.Time `json:"timeperiod_start"`
	TimeperiodEnd    time.Time `json:"timeperiod_end"`
	PathsDiscovered  int       `json:"paths_discovered"`
	PathsResolved    int       `json:"paths_resolved"`
	NewCriticalPaths int       `json:"new_critical_paths"`
	RiskTrend        string    `json:"risk_trend"` // increasing, decreasing, stable
	TopNewRisks      []AttackPath `json:"top_new_risks"`
}

// ConfinementZone represents network segments to contain lateral movement
type ConfinementZone struct {
	ID            string              `json:"id"`
	Name          string              `json:"name"`
	Assets        []string            `json:"assets"`
	Boundaries    []ConfinementRule   `json:"boundaries"`
	CreatedAt     time.Time           `json:"created_at"`
	LastUpdatedAt time.Time           `json:"last_updated_at"`
}

// ConfinementRule defines access controls for a confinement zone
type ConfinementRule struct {
	ID            string       `json:"id"`
	SourceZone    string       `json:"source_zone"`
	TargetZone    string       `json:"target_zone"`
	AllowedPorts  []int        `json:"allowed_ports"`
	AllowedProtocols []string `json:"allowed_protocols"`
	Enabled       bool         `json:"enabled"`
}

// PathExploitability represents how easily an attack path can be exploited
type PathExploitability struct {
	PathID           string  `json:"path_id"`
	ExploitabilityScore float64 `json:"exploitability_score"` // 0-100
	ToolsAvailable   []string `json:"tools_available"`
	SkillRequired    string  `json:"skill_required"` // novice, intermediate, advanced
	TimeToExploit    string  `json:"time_to_exploit"`
	DetectionGaps    []string `json:"detection_gaps"`
	MitigationStatus string  `json:"mitigation_status"`
}
