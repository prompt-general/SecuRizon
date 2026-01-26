package models

import (
	"time"
	"github.com/google/uuid"
)

// RelationshipType represents the type of relationship between assets
type RelationshipType string

const (
	// Identity relationships
	RelationshipAssumesRole RelationshipType = "ASSUMES_ROLE"
	RelationshipHasAccessTo RelationshipType = "HAS_ACCESS_TO"
	
	// Network relationships
	RelationshipConnectedTo RelationshipType = "CONNECTED_TO"
	
	// Compute relationships
	RelationshipRunsOn RelationshipType = "RUNS_ON"
	
	// Data relationships
	RelationshipStores RelationshipType = "STORES"
	
	// Finding relationships
	RelationshipGenerates RelationshipType = "GENERATES"
	
	// General relationships
	RelationshipContains RelationshipType = "CONTAINS"
	RelationshipDependsOn RelationshipType = "DEPENDS_ON"
	RelationshipManages RelationshipType = "MANAGES"
	RelationshipOwns RelationshipType = "OWNS"
)

// Relationship represents a relationship between two assets
type Relationship struct {
	ID           string           `json:"id"`
	FromAssetID  string           `json:"from_asset_id"`
	ToAssetID    string           `json:"to_asset_id"`
	Type         RelationshipType `json:"type"`
	Properties   map[string]interface{} `json:"properties,omitempty"`
	ValidFrom    time.Time        `json:"valid_from"`
	ValidTo      *time.Time       `json:"valid_to,omitempty"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
	Strength     float64          `json:"strength"` // 0.0-1.0, relationship strength/confidence
	Description  string           `json:"description,omitempty"`
}

// NewRelationship creates a new relationship between two assets
func NewRelationship(fromAssetID, toAssetID string, relType RelationshipType) Relationship {
	now := time.Now()
	return Relationship{
		ID:          uuid.New().String(),
		FromAssetID: fromAssetID,
		ToAssetID:   toAssetID,
		Type:        relType,
		Properties:  make(map[string]interface{}),
		ValidFrom:   now,
		CreatedAt:   now,
		UpdatedAt:   now,
		Strength:    1.0,
	}
}

// WithProperties adds properties to the relationship
func (r *Relationship) WithProperties(props map[string]interface{}) *Relationship {
	if r.Properties == nil {
		r.Properties = make(map[string]interface{})
	}
	for k, v := range props {
		r.Properties[k] = v
	}
	r.UpdatedAt = time.Now()
	return r
}

// WithStrength sets the relationship strength
func (r *Relationship) WithStrength(strength float64) *Relationship {
	if strength < 0 {
		strength = 0
	}
	if strength > 1 {
		strength = 1
	}
	r.Strength = strength
	r.UpdatedAt = time.Now()
	return r
}

// WithValidTo sets the validity end time
func (r *Relationship) WithValidTo(validTo time.Time) *Relationship {
	r.ValidTo = &validTo
	r.UpdatedAt = time.Now()
	return r
}

// IsValid checks if the relationship is currently valid
func (r *Relationship) IsValid(at time.Time) bool {
	if at.Before(r.ValidFrom) {
		return false
	}
	if r.ValidTo != nil && at.After(*r.ValidTo) {
		return false
	}
	return true
}

// IsActive checks if the relationship is currently active
func (r *Relationship) IsActive() bool {
	now := time.Now()
	return r.IsValid(now)
}

// Invalidate marks the relationship as invalid from now
func (r *Relationship) Invalidate() {
	now := time.Now()
	r.ValidTo = &now
	r.UpdatedAt = now
}

// RelationshipQuery represents a query for relationships
type RelationshipQuery struct {
	FromAssetID   string            `json:"from_asset_id,omitempty"`
	ToAssetID     string            `json:"to_asset_id,omitempty"`
	Types         []RelationshipType `json:"types,omitempty"`
	ValidAt       *time.Time        `json:"valid_at,omitempty"`
	MinStrength   *float64          `json:"min_strength,omitempty"`
	MaxStrength   *float64          `json:"max_strength,omitempty"`
	Properties    map[string]interface{} `json:"properties,omitempty"`
	Limit         int               `json:"limit,omitempty"`
	Offset        int               `json:"offset,omitempty"`
}

// RelationshipFilter represents a filter for relationships
type RelationshipFilter struct {
	AssetIDs      []string          `json:"asset_ids,omitempty"`
	Types         []RelationshipType `json:"types,omitempty"`
	ActiveOnly    bool              `json:"active_only,omitempty"`
	ValidAt       time.Time         `json:"valid_at,omitempty"`
	MinStrength   float64           `json:"min_strength,omitempty"`
	MaxStrength   float64           `json:"max_strength,omitempty"`
}

// RelationshipEdge represents an edge in the graph with additional metadata
type RelationshipEdge struct {
	Relationship Relationship `json:"relationship"`
	FromAsset    Asset        `json:"from_asset"`
	ToAsset      Asset        `json:"to_asset"`
	PathWeight   float64      `json:"path_weight"` // Used for path calculations
}

// GraphPath represents a path through the graph
type GraphPath struct {
	Edges []RelationshipEdge `json:"edges"`
	Nodes []Asset           `json:"nodes"`
	TotalWeight float64     `json:"total_weight"`
	Length int              `json:"length"`
}

// AddEdge adds an edge to the path
func (p *GraphPath) AddEdge(edge RelationshipEdge) {
	p.Edges = append(p.Edges, edge)
	if len(p.Nodes) == 0 {
		p.Nodes = append(p.Nodes, edge.FromAsset)
	}
	p.Nodes = append(p.Nodes, edge.ToAsset)
	p.TotalWeight += edge.PathWeight
	p.Length = len(p.Edges)
}

// GetAssetIDs returns all asset IDs in the path
func (p *GraphPath) GetAssetIDs() []string {
	ids := make([]string, len(p.Nodes))
	for i, node := range p.Nodes {
		ids[i] = node.GetID()
	}
	return ids
}

// GetRelationshipTypes returns all relationship types in the path
func (p *GraphPath) GetRelationshipTypes() []RelationshipType {
	types := make([]RelationshipType, len(p.Edges))
	for i, edge := range p.Edges {
		types[i] = edge.Relationship.Type
	}
	return types
}
