package graph

import (
	"context"
	"fmt"
	"sync"
)

// GraphAlgorithms provides various graph traversal and analysis algorithms
type GraphAlgorithms struct {
	mu sync.RWMutex
}

// PathFinder contains state for path finding operations
type PathFinder struct {
	visited map[string]bool
	paths   [][]string
	maxPath int
}

// NewPathFinder creates a new path finder
func NewPathFinder(maxPath int) *PathFinder {
	return &PathFinder{
		visited: make(map[string]bool),
		paths:   make([][]string, 0),
		maxPath: maxPath,
	}
}

// BFSPaths finds shortest paths between two nodes using Breadth-First Search
func (ga *GraphAlgorithms) BFSPaths(ctx context.Context, gc *Client, sourceID, targetID string, maxDepth int) ([][]string, error) {
	paths := make([][]string, 0)
	
	// Check context
	select {
	case <-ctx.Done():
		return paths, ctx.Err()
	default:
	}
	
	queue := [][]string{{sourceID}}
	visited := make(map[string]bool)
	visited[sourceID] = true
	
	for len(queue) > 0 && len(queue[0]) <= maxDepth {
		currentPath := queue[0]
		queue = queue[1:]
		
		currentNode := currentPath[len(currentPath)-1]
		
		// Get neighbors
		neighbors, err := gc.GetAssetNeighbors(ctx, currentNode)
		if err != nil {
			continue
		}
		
		for _, neighbor := range neighbors {
			// Check if target reached
			if neighbor.ID == targetID {
				fullPath := append(currentPath, neighbor.ID)
				paths = append(paths, fullPath)
				continue
			}
			
			// Continue BFS if not visited
			if !visited[neighbor.ID] && len(currentPath) < maxDepth {
				visited[neighbor.ID] = true
				newPath := make([]string, len(currentPath))
				copy(newPath, currentPath)
				newPath = append(newPath, neighbor.ID)
				queue = append(queue, newPath)
			}
		}
	}
	
	return paths, nil
}

// DFSPaths finds all paths between two nodes using Depth-First Search
func (ga *GraphAlgorithms) DFSPaths(ctx context.Context, gc *Client, sourceID, targetID string, maxDepth int) ([][]string, error) {
	paths := make([][]string, 0)
	visited := make(map[string]bool)
	currentPath := []string{sourceID}
	
	ga.dfSearchHelper(ctx, gc, sourceID, targetID, &currentPath, visited, maxDepth, &paths)
	
	return paths, nil
}

// dfSearchHelper is the recursive helper for DFS
func (ga *GraphAlgorithms) dfSearchHelper(ctx context.Context, gc *Client, currentID, targetID string, 
	currentPath *[]string, visited map[string]bool, maxDepth int, paths *[][]string) {
	
	// Check context
	select {
	case <-ctx.Done():
		return
	default:
	}
	
	// Check depth limit
	if len(*currentPath) > maxDepth {
		return
	}
	
	// Check if target reached
	if currentID == targetID {
		pathCopy := make([]string, len(*currentPath))
		copy(pathCopy, *currentPath)
		*paths = append(*paths, pathCopy)
		return
	}
	
	// Mark as visited
	visited[currentID] = true
	
	// Get neighbors
	neighbors, err := gc.GetAssetNeighbors(ctx, currentID)
	if err != nil {
		visited[currentID] = false
		return
	}
	
	// Explore neighbors
	for _, neighbor := range neighbors {
		if !visited[neighbor.ID] {
			*currentPath = append(*currentPath, neighbor.ID)
			ga.dfSearchHelper(ctx, gc, neighbor.ID, targetID, currentPath, visited, maxDepth, paths)
			*currentPath = (*currentPath)[:len(*currentPath)-1]
		}
	}
	
	// Unmark for backtracking
	visited[currentID] = false
}

// FindStrongestPaths finds paths with highest cumulative risk scores
func (ga *GraphAlgorithms) FindStrongestPaths(ctx context.Context, gc *Client, paths [][]string) ([][]string, error) {
	type pathScore struct {
		path  []string
		score float64
	}
	
	scoredPaths := make([]pathScore, 0)
	
	// Calculate risk score for each path
	for _, path := range paths {
		score := float64(0)
		
		for _, assetID := range path {
			asset, err := gc.GetAsset(ctx, assetID)
			if err != nil {
				continue
			}
			score += asset.RiskScore
		}
		
		scoredPaths = append(scoredPaths, pathScore{path, score})
	}
	
	// Sort by score (highest first)
	for i := 0; i < len(scoredPaths); i++ {
		for j := i + 1; j < len(scoredPaths); j++ {
			if scoredPaths[j].score > scoredPaths[i].score {
				scoredPaths[i], scoredPaths[j] = scoredPaths[j], scoredPaths[i]
			}
		}
	}
	
	// Extract top paths
	maxPaths := 10
	if len(scoredPaths) < maxPaths {
		maxPaths = len(scoredPaths)
	}
	
	topPaths := make([][]string, maxPaths)
	for i := 0; i < maxPaths; i++ {
		topPaths[i] = scoredPaths[i].path
	}
	
	return topPaths, nil
}

// FindBottlenecks identifies critical nodes that appear in many attack paths
func (ga *GraphAlgorithms) FindBottlenecks(ctx context.Context, paths [][]string) map[string]int {
	bottlenecks := make(map[string]int)
	
	// Count appearances of each node
	for _, path := range paths {
		for _, nodeID := range path {
			bottlenecks[nodeID]++
		}
	}
	
	return bottlenecks
}

// FindSegmentationOpportunities identifies where network segmentation would break the most paths
func (ga *GraphAlgorithms) FindSegmentationOpportunities(ctx context.Context, paths [][]string) []SegmentationMetric {
	opportunities := make([]SegmentationMetric, 0)
	
	// For each node, calculate how many paths it blocks
	nodeMetrics := make(map[string]*SegmentationMetric)
	
	for _, path := range paths {
		// Skip entry and target points
		for i := 1; i < len(path)-1; i++ {
			nodeID := path[i]
			
			if _, exists := nodeMetrics[nodeID]; !exists {
				nodeMetrics[nodeID] = &SegmentationMetric{
					NodeID:         nodeID,
					PathsBlocked:   0,
					PositionCounts: make(map[int]int),
				}
			}
			
			nodeMetrics[nodeID].PathsBlocked++
			nodeMetrics[nodeID].PositionCounts[i]++
		}
	}
	
	// Convert to slice and sort
	for _, metric := range nodeMetrics {
		opportunities = append(opportunities, *metric)
	}
	
	// Sort by paths blocked (descending)
	for i := 0; i < len(opportunities); i++ {
		for j := i + 1; j < len(opportunities); j++ {
			if opportunities[j].PathsBlocked > opportunities[i].PathsBlocked {
				opportunities[i], opportunities[j] = opportunities[j], opportunities[i]
			}
		}
	}
	
	return opportunities
}

// SegmentationMetric represents the impact of segmenting a node
type SegmentationMetric struct {
	NodeID         string
	PathsBlocked   int
	PositionCounts map[int]int // Position in path -> count
	ImpactScore    float64
}

// DetectLateralMovementChains identifies sequential relationships that enable lateral movement
func (ga *GraphAlgorithms) DetectLateralMovementChains(ctx context.Context, gc *Client, paths [][]string) ([]LateralMovementChain, error) {
	chains := make([]LateralMovementChain, 0)
	
	for _, path := range paths {
		// Analyze consecutive pairs for privilege escalation or movement
		for i := 0; i < len(path)-1; i++ {
			sourceID := path[i]
			targetID := path[i+1]
			
			// Get relationship details
			rel, err := gc.GetRelationship(ctx, sourceID, targetID)
			if err != nil {
				continue
			}
			
			chain := LateralMovementChain{
				Source:         sourceID,
				Target:         targetID,
				RelationType:   rel.Type,
				Position:       i,
				PathLength:     len(path),
				EnablesEscalation: ga.isEscalationMove(rel.Type),
			}
			
			chains = append(chains, chain)
		}
	}
	
	return chains, nil
}

// LateralMovementChain represents a single step in lateral movement
type LateralMovementChain struct {
	Source              string
	Target              string
	RelationType        string
	Position            int
	PathLength          int
	EnablesEscalation   bool
}

// isEscalationMove checks if a relationship type enables privilege escalation
func (ga *GraphAlgorithms) isEscalationMove(relType string) bool {
	escalationTypes := map[string]bool{
		"ASSUMES_ROLE":        true,
		"HAS_ADMIN_ACCESS":    true,
		"PRIVILEGE_ESCALATION": true,
		"LATERAL_MOVEMENT":    true,
	}
	return escalationTypes[relType]
}

// CalculatePathChordality measures how many shortcuts exist between path nodes
func (ga *GraphAlgorithms) CalculatePathChordality(ctx context.Context, gc *Client, path []string) (float64, error) {
	if len(path) < 3 {
		return 0, nil
	}
	
	// Chordality = shortcuts / possible shortcuts
	shortcuts := 0
	possibleShortcuts := 0
	
	// Check for all non-adjacent pairs
	for i := 0; i < len(path); i++ {
		for j := i + 2; j < len(path); j++ {
			possibleShortcuts++
			
			// Check if direct connection exists
			_, err := gc.GetRelationship(ctx, path[i], path[j])
			if err == nil {
				shortcuts++
			}
		}
	}
	
	if possibleShortcuts == 0 {
		return 0, nil
	}
	
	chordality := float64(shortcuts) / float64(possibleShortcuts)
	return chordality, nil
}

// FindPeerAssets finds similar assets that could be parallel attack vectors
func (ga *GraphAlgorithms) FindPeerAssets(ctx context.Context, gc *Client, assetID string) ([]PeerAsset, error) {
	peers := make([]PeerAsset, 0)
	
	asset, err := gc.GetAsset(ctx, assetID)
	if err != nil {
		return peers, err
	}
	
	// Find assets with similar type and risk profile
	similarAssets, err := gc.FindSimilarAssets(ctx, asset.Type, asset.Provider)
	if err != nil {
		return peers, err
	}
	
	for _, similar := range similarAssets {
		if similar.ID != assetID {
			peers = append(peers, PeerAsset{
				ID:              similar.ID,
				Type:            similar.Type,
				RiskScore:       similar.RiskScore,
				Similarity:      ga.calculateSimilarity(asset, similar),
				CommonVulnerabilities: ga.countCommonVulnerabilities(ctx, gc, assetID, similar.ID),
			})
		}
	}
	
	return peers, nil
}

// PeerAsset represents a similar asset that could be exploited via similar paths
type PeerAsset struct {
	ID                       string
	Type                     string
	RiskScore                float64
	Similarity               float64
	CommonVulnerabilities    int
}

// calculateSimilarity computes similarity between two assets
func (ga *GraphAlgorithms) calculateSimilarity(asset1, asset2 *Asset) float64 {
	similarity := 0.0
	
	// Type match
	if asset1.Type == asset2.Type {
		similarity += 0.5
	}
	
	// Provider match
	if asset1.Provider == asset2.Provider {
		similarity += 0.3
	}
	
	// Environment match
	if asset1.Environment == asset2.Environment {
		similarity += 0.2
	}
	
	return similarity
}

// countCommonVulnerabilities counts shared vulnerabilities between assets
func (ga *GraphAlgorithms) countCommonVulnerabilities(ctx context.Context, gc *Client, assetID1, assetID2 string) int {
	findings1, err := gc.GetAssetFindings(ctx, assetID1)
	if err != nil {
		return 0
	}
	
	findings2, err := gc.GetAssetFindings(ctx, assetID2)
	if err != nil {
		return 0
	}
	
	// Count matching findings by policy
	policyMatches := make(map[string]int)
	for _, f1 := range findings1 {
		for _, f2 := range findings2 {
			if f1.PolicyID == f2.PolicyID {
				policyMatches[f1.PolicyID]++
			}
		}
	}
	
	return len(policyMatches)
}
