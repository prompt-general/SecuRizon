package attackpath

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/securazion/event-processor/internal/graph"
	"github.com/securazion/event-processor/internal/risk"
	"github.com/securazion/event-processor/pkg/models"
)

// AttackPathEngine discovers and analyzes attack paths through the asset graph
type AttackPathEngine struct {
	graphClient  *graph.Client
	riskEngine   *risk.Engine
	pathCache    map[string]*models.AttackPath
	cacheTTL     time.Duration
	cacheExpiry  map[string]time.Time
	mu           sync.RWMutex
	maxPathDepth int
}

// NewAttackPathEngine creates a new attack path engine
func NewAttackPathEngine(graphClient *graph.Client, riskEngine *risk.Engine) *AttackPathEngine {
	return &AttackPathEngine{
		graphClient:  graphClient,
		riskEngine:   riskEngine,
		pathCache:    make(map[string]*models.AttackPath),
		cacheExpiry:  make(map[string]time.Time),
		cacheTTL:     5 * time.Minute,
		maxPathDepth: 10,
	}
}

// DiscoverPaths finds attack paths between source and target assets
func (ape *AttackPathEngine) DiscoverPaths(ctx context.Context, sourceID, targetID string, maxHops int) ([]models.AttackPath, error) {
	cacheKey := fmt.Sprintf("%s->%s:%d", sourceID, targetID, maxHops)
	
	// Check cache
	if path, ok := ape.getFromCache(cacheKey); ok {
		return []models.AttackPath{*path}, nil
	}
	
	// Find paths using BFS (Breadth-First Search)
	paths := ape.findPathsBFS(ctx, sourceID, targetID, maxHops)
	
	if len(paths) == 0 {
		return []models.AttackPath{}, nil
	}
	
	// Enrich paths with vulnerability information
	for i := range paths {
		ape.enrichPathWithVulnerabilities(ctx, &paths[i])
		paths[i].CumulativeRisk = ape.calculatePathRisk(&paths[i])
	}
	
	// Sort by risk (highest first)
	for i := 0; i < len(paths); i++ {
		for j := i + 1; j < len(paths); j++ {
			if paths[j].CumulativeRisk > paths[i].CumulativeRisk {
				paths[i], paths[j] = paths[j], paths[i]
			}
		}
	}
	
	// Cache the primary path
	if len(paths) > 0 {
		ape.cacheResult(cacheKey, &paths[0])
	}
	
	return paths, nil
}

// findPathsBFS performs breadth-first search to find shortest paths
func (ape *AttackPathEngine) findPathsBFS(ctx context.Context, sourceID, targetID string, maxHops int) []models.AttackPath {
	var paths []models.AttackPath
	
	// Initialize BFS
	queue := [][]string{{sourceID}}
	visited := make(map[string]bool)
	visited[sourceID] = true
	
	for len(queue) > 0 && len(queue[0]) <= maxHops {
		currentPath := queue[0]
		queue = queue[1:]
		
		currentAssetID := currentPath[len(currentPath)-1]
		
		// Get neighbors of current asset
		neighbors, err := ape.graphClient.GetAssetNeighbors(ctx, currentAssetID)
		if err != nil {
			continue
		}
		
		for _, neighbor := range neighbors {
			if neighbor.ID == targetID {
				// Found a path to target
				fullPath := append(currentPath, neighbor.ID)
				paths = append(paths, ape.constructAttackPath(fullPath))
				continue
			}
			
			// Continue BFS if not visited and within depth limit
			if !visited[neighbor.ID] && len(currentPath) < maxHops {
				visited[neighbor.ID] = true
				newPath := append(append([]string{}, currentPath...), neighbor.ID)
				queue = append(queue, newPath)
			}
		}
		
		// Early exit if we found enough paths
		if len(paths) >= 5 {
			break
		}
	}
	
	return paths
}

// enrichPathWithVulnerabilities adds finding information to path nodes
func (ape *AttackPathEngine) enrichPathWithVulnerabilities(ctx context.Context, path *models.AttackPath) {
	for _, node := range path.Path {
		findings, err := ape.graphClient.GetAssetFindings(ctx, node.ID)
		if err != nil {
			continue
		}
		
		// Add findings as vulnerabilities to the path
		for _, finding := range findings {
			vuln := models.AttackPathVulnerability{
				FindingID:       finding.ID,
				Severity:        finding.Severity,
				Description:     finding.Title,
				ExploitedInPath: ape.isVulnerabilityExploitedInPath(finding, path),
			}
			path.Vulnerabilities = append(path.Vulnerabilities, vuln)
		}
	}
}

// isVulnerabilityExploitedInPath checks if a vulnerability can be exploited to traverse the path
func (ape *AttackPathEngine) isVulnerabilityExploitedInPath(finding *models.Finding, path *models.AttackPath) bool {
	if len(path.Path) < 2 {
		return false
	}
	
	// Check if finding relates to a relationship in the path
	for i := 0; i < len(path.Path)-1; i++ {
		sourceNode := path.Path[i]
		targetNode := path.Path[i+1]
		
		// Determine if this vulnerability enables the transition
		if ape.canVulnerabilityEnableTransition(finding, sourceNode, targetNode) {
			return true
		}
	}
	
	return false
}

// canVulnerabilityEnableTransition checks if a vulnerability enables moving between two nodes
func (ape *AttackPathEngine) canVulnerabilityEnableTransition(finding *models.Finding, sourceNodeID, targetNodeID string) bool {
	// Check if the finding's category relates to a privilege escalation or access path
	categories := []string{"privilege_escalation", "lateral_movement", "access_control", "authentication"}
	
	for _, cat := range categories {
		if finding.Category == cat {
			return true
		}
	}
	
	return false
}

// calculatePathRisk computes cumulative risk for an attack path
func (ape *AttackPathEngine) calculatePathRisk(path *models.AttackPath) float64 {
	if len(path.Path) == 0 {
		return 0
	}
	
	// Calculate risk as weighted combination of:
	// 1. Node risks along the path
	// 2. Exploitability of vulnerabilities
	// 3. Likelihood of success
	
	var totalRisk float64
	
	// Base risk from path nodes
	for _, node := range path.Path {
		totalRisk += node.RiskScore
	}
	
	// Vulnerability exploitation risk
	for _, vuln := range path.Vulnerabilities {
		if vuln.ExploitedInPath {
			totalRisk += vuln.Severity * 10 // Amplify if exploited
		}
	}
	
	// Normalize by path length (shorter paths = higher concentration of risk)
	normalizedRisk := totalRisk / float64(len(path.Path))
	
	// Cap at 100
	if normalizedRisk > 100 {
		normalizedRisk = 100
	}
	
	return normalizedRisk
}

// constructAttackPath creates an AttackPath model from an asset ID chain
func (ape *AttackPathEngine) constructAttackPath(assetIDs []string) models.AttackPath {
	path := models.AttackPath{
		ID:                 generateUUID(),
		SourceID:           assetIDs[0],
		TargetID:           assetIDs[len(assetIDs)-1],
		Hops:               len(assetIDs) - 1,
		Path:               make([]models.PathNode, 0),
		Vulnerabilities:    make([]models.AttackPathVulnerability, 0),
	}
	
	// Create path nodes
	for i, assetID := range assetIDs {
		node := models.PathNode{
			ID: assetID,
		}
		
		if i == 0 {
			node.Role = "entry_point"
		} else if i == len(assetIDs)-1 {
			node.Role = "target"
		} else {
			node.Role = "pivot_point"
		}
		
		path.Path = append(path.Path, node)
	}
	
	return path
}

// FindPathsFromInternet discovers attack paths from the internet to internal assets
func (ape *AttackPathEngine) FindPathsFromInternet(ctx context.Context, targetAssetID string, maxHops int) ([]models.AttackPath, error) {
	// Get all internet-exposed assets (entry points)
	entryPoints, err := ape.graphClient.FindInternetExposedAssets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find entry points: %v", err)
	}
	
	var allPaths []models.AttackPath
	
	// Find paths from each entry point to the target
	for _, entryPoint := range entryPoints {
		paths, err := ape.DiscoverPaths(ctx, entryPoint.ID, targetAssetID, maxHops)
		if err != nil {
			continue
		}
		allPaths = append(allPaths, paths...)
	}
	
	return allPaths, nil
}

// FindCriticalPaths identifies the most dangerous attack paths
func (ape *AttackPathEngine) FindCriticalPaths(ctx context.Context, minRiskScore float64) ([]models.AttackPath, error) {
	paths, err := ape.graphClient.GetAllAttackPaths(ctx)
	if err != nil {
		return nil, err
	}
	
	var criticalPaths []models.AttackPath
	
	for _, path := range paths {
		if path.CumulativeRisk >= minRiskScore {
			criticalPaths = append(criticalPaths, path)
		}
	}
	
	// Sort by risk
	for i := 0; i < len(criticalPaths); i++ {
		for j := i + 1; j < len(criticalPaths); j++ {
			if criticalPaths[j].CumulativeRisk > criticalPaths[i].CumulativeRisk {
				criticalPaths[i], criticalPaths[j] = criticalPaths[j], criticalPaths[i]
			}
		}
	}
	
	return criticalPaths, nil
}

// SimulateAttack simulates an attack from source to target with recommendations
func (ape *AttackPathEngine) SimulateAttack(ctx context.Context, sourceID, targetID string) (*models.AttackSimulation, error) {
	paths, err := ape.DiscoverPaths(ctx, sourceID, targetID, ape.maxPathDepth)
	if err != nil {
		return nil, fmt.Errorf("failed to discover paths: %v", err)
	}
	
	simulation := &models.AttackSimulation{
		Paths:           paths,
		HighestRisk:     0,
		AverageRisk:     0,
		CriticalPaths:   0,
		Recommendations: []string{},
	}
	
	if len(paths) == 0 {
		simulation.Recommendations = append(simulation.Recommendations, "No active attack paths found")
		return simulation, nil
	}
	
	// Calculate statistics
	var totalRisk float64
	for _, path := range paths {
		totalRisk += path.CumulativeRisk
		if path.CumulativeRisk >= 80 {
			simulation.CriticalPaths++
		}
		if path.CumulativeRisk > simulation.HighestRisk {
			simulation.HighestRisk = path.CumulativeRisk
		}
	}
	
	simulation.AverageRisk = totalRisk / float64(len(paths))
	
	// Generate recommendations
	simulation.Recommendations = ape.generateRecommendations(paths)
	
	return simulation, nil
}

// generateRecommendations creates remediation recommendations based on paths
func (ape *AttackPathEngine) generateRecommendations(paths []models.AttackPath) []string {
	recommendations := []string{}
	vulnerabilityCount := make(map[string]int)
	
	// Count vulnerabilities across all paths
	for _, path := range paths {
		for _, vuln := range path.Vulnerabilities {
			vulnerabilityCount[vuln.FindingID]++
		}
	}
	
	// Find most impactful vulnerabilities
	type vulnRank struct {
		id    string
		count int
	}
	
	var ranked []vulnRank
	for id, count := range vulnerabilityCount {
		ranked = append(ranked, vulnRank{id, count})
	}
	
	// Sort by impact
	for i := 0; i < len(ranked); i++ {
		for j := i + 1; j < len(ranked); j++ {
			if ranked[j].count > ranked[i].count {
				ranked[i], ranked[j] = ranked[j], ranked[i]
			}
		}
	}
	
	// Generate recommendations for top vulnerabilities
	for _, r := range ranked {
		if len(recommendations) >= 5 {
			break
		}
		recommendation := fmt.Sprintf("Remediate vulnerability %s (affects %d attack paths)", r.id, r.count)
		recommendations = append(recommendations, recommendation)
	}
	
	// Add general recommendations
	if len(paths) > 0 && paths[0].CumulativeRisk > 80 {
		recommendations = append(recommendations, "Implement network segmentation to break attack paths")
		recommendations = append(recommendations, "Enable privileged access management for sensitive assets")
		recommendations = append(recommendations, "Implement multi-factor authentication on critical systems")
	}
	
	return recommendations
}

// cacheResult stores a path in cache
func (ape *AttackPathEngine) cacheResult(key string, path *models.AttackPath) {
	ape.mu.Lock()
	defer ape.mu.Unlock()
	
	ape.pathCache[key] = path
	ape.cacheExpiry[key] = time.Now().Add(ape.cacheTTL)
}

// getFromCache retrieves a cached path if not expired
func (ape *AttackPathEngine) getFromCache(key string) (*models.AttackPath, bool) {
	ape.mu.RLock()
	defer ape.mu.RUnlock()
	
	path, exists := ape.pathCache[key]
	if !exists {
		return nil, false
	}
	
	expiry, hasExpiry := ape.cacheExpiry[key]
	if hasExpiry && time.Now().After(expiry) {
		return nil, false
	}
	
	return path, true
}

// ClearCache removes all cached paths
func (ape *AttackPathEngine) ClearCache() {
	ape.mu.Lock()
	defer ape.mu.Unlock()
	
	ape.pathCache = make(map[string]*models.AttackPath)
	ape.cacheExpiry = make(map[string]time.Time)
}
