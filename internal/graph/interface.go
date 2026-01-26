package graph

import (
	"context"
	"time"
	"github.com/securizon/pkg/models"
)

// GraphStore interface defines operations for graph database
type GraphStore interface {
	// Asset operations
	CreateAsset(ctx context.Context, asset models.Asset) error
	GetAsset(ctx context.Context, id string) (models.Asset, error)
	UpdateAsset(ctx context.Context, asset models.Asset) error
	DeleteAsset(ctx context.Context, id string) error
	ListAssets(ctx context.Context, filter models.AssetFilter) ([]models.Asset, error)
	SearchAssets(ctx context.Context, query models.AssetQuery) ([]models.Asset, error)
	
	// Relationship operations
	CreateRelationship(ctx context.Context, rel models.Relationship) error
	GetRelationship(ctx context.Context, id string) (models.Relationship, error)
	UpdateRelationship(ctx context.Context, rel models.Relationship) error
	DeleteRelationship(ctx context.Context, id string) error
	ListRelationships(ctx context.Context, filter models.RelationshipFilter) ([]models.Relationship, error)
	SearchRelationships(ctx context.Context, query models.RelationshipQuery) ([]models.Relationship, error)
	
	// Graph traversal operations
	GetNeighbors(ctx context.Context, assetID string, direction string, maxDepth int) ([]models.Asset, []models.Relationship, error)
	FindPath(ctx context.Context, fromAssetID, toAssetID string, maxDepth int) (*models.GraphPath, error)
	FindAttackPaths(ctx context.Context, entryPoints []string, targets []string, maxDepth int) ([]models.GraphPath, error)
	GetConnectedComponents(ctx context.Context, assetIDs []string) ([][]string, error)
	
	// Risk and finding operations
	GetAssetRisk(ctx context.Context, assetID string) (models.RiskScore, error)
	UpdateAssetRisk(ctx context.Context, risk models.RiskScore) error
	GetAssetFindings(ctx context.Context, assetID string) ([]models.Finding, error)
	CreateFinding(ctx context.Context, finding models.Finding) error
	UpdateFinding(ctx context.Context, finding models.Finding) error
	
	// Analytics and aggregation
	GetRiskSummary(ctx context.Context, filter models.AssetFilter) (*models.RiskSummary, error)
	GetRiskTrends(ctx context.Context, assetID string, timeRange models.TimeRange) (*models.RiskTrend, error)
	GetAssetStatistics(ctx context.Context) (map[string]interface{}, error)
	
	// Bulk operations
	BulkCreateAssets(ctx context.Context, assets []models.Asset) error
	BulkUpdateAssets(ctx context.Context, assets []models.Asset) error
	BulkCreateRelationships(ctx context.Context, relationships []models.Relationship) error
	BulkDeleteAssets(ctx context.Context, assetIDs []string) error
	
	// Health and maintenance
	Ping(ctx context.Context) error
	Close() error
}

// GraphConfig represents graph database configuration
type GraphConfig struct {
	URI          string        `json:"uri"`
	Database     string        `json:"database"`
	Username     string        `json:"username"`
	Password     string        `json:"password"`
	MaxPoolSize  int           `json:"max_pool_size"`
	MaxIdleConns int           `json:"max_idle_conns"`
	ConnTimeout  time.Duration `json:"conn_timeout"`
	ReadTimeout  time.Duration `json:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout"`
}

// DefaultGraphConfig returns default graph configuration
func DefaultGraphConfig() GraphConfig {
	return GraphConfig{
		URI:          "bolt://localhost:7687",
		Database:     "neo4j",
		MaxPoolSize:  50,
		MaxIdleConns: 10,
		ConnTimeout:  30 * time.Second,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
}

// GraphQuery represents a generic graph query
type GraphQuery struct {
	Cypher    string                 `json:"cypher"`
	Params    map[string]interface{} `json:"params"`
	Limit     int                    `json:"limit"`
	Skip      int                    `json:"skip"`
	Timestamp time.Time              `json:"timestamp"`
}

// GraphResult represents the result of a graph query
type GraphResult struct {
	Records []map[string]interface{} `json:"records"`
	Count   int                      `json:"count"`
	Time    time.Duration            `json:"time"`
	Error   error                    `json:"error,omitempty"`
}

// AttackPathQuery represents a query for attack paths
type AttackPathQuery struct {
	EntryPoints    []string                 `json:"entry_points"`
	Targets        []string                 `json:"targets"`
	MaxDepth       int                      `json:"max_depth"`
	MinRiskScore   float64                  `json:"min_risk_score"`
	AllowedEdges   []models.RelationshipType `json:"allowed_edges"`
	ForbiddenEdges []models.RelationshipType `json:"forbidden_edges"`
	NodeFilters    map[string]interface{}   `json:"node_filters"`
	EdgeFilters    map[string]interface{}   `json:"edge_filters"`
}

// GraphSchema represents the graph schema definition
type GraphSchema struct {
	NodeLabels    []NodeLabel    `json:"node_labels"`
	EdgeTypes     []EdgeType     `json:"edge_types"`
	Constraints   []Constraint   `json:"constraints"`
	Indexes       []Index        `json:"indexes"`
	Procedures    []Procedure    `json:"procedures"`
}

// NodeLabel represents a node label definition
type NodeLabel struct {
	Name        string            `json:"name"`
	Properties  []Property        `json:"properties"`
	Description string            `json:"description"`
}

// EdgeType represents an edge type definition
type EdgeType struct {
	Name        string            `json:"name"`
	FromLabel   string            `json:"from_label"`
	ToLabel     string            `json:"to_label"`
	Properties  []Property        `json:"properties"`
	Description string            `json:"description"`
}

// Property represents a property definition
type Property struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Required    bool        `json:"required"`
	Indexed     bool        `json:"indexed"`
	Unique      bool        `json:"unique"`
	Description string      `json:"description"`
	DefaultValue interface{} `json:"default_value,omitempty"`
}

// Constraint represents a database constraint
type Constraint struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Label       string   `json:"label"`
	Properties  []string `json:"properties"`
	Description string   `json:"description"`
}

// Index represents a database index
type Index struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Properties  []string `json:"properties"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
}

// Procedure represents a stored procedure
type Procedure struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Inputs      []Property `json:"inputs"`
	Outputs     []Property `json:"outputs"`
}

// GraphMetrics represents graph database metrics
type GraphMetrics struct {
	NodeCount       int64             `json:"node_count"`
	EdgeCount       int64             `json:"edge_count"`
	AvgDegree       float64           `json:"avg_degree"`
	ConnectedComponents int64          `json:"connected_components"`
	LargestComponentSize int64        `json:"largest_component_size"`
	NodesByType     map[string]int64  `json:"nodes_by_type"`
	EdgesByType     map[string]int64  `json:"edges_by_type"`
	RiskDistribution map[string]int64 `json:"risk_distribution"`
	LastUpdated     time.Time         `json:"last_updated"`
}
