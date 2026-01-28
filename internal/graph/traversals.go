package graph

import (
    "context"
    "fmt"
    "log"
    "math"
    "sync"
    "time"

    "github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

type AttackPathEngine struct {
    driver neo4j.Driver
    config AttackPathConfig
}

type AttackPathConfig struct {
    MaxHops                int
    MaxPathsPerQuery       int
    RiskThreshold          float64
    CacheTTL               time.Duration
    EnableParallelTraversal bool
}

type AttackPath struct {
    ID             string               `json:"id"`
    SourceID       string               `json:"source_id"`
    TargetID       string               `json:"target_id"`
    Hops           int                  `json:"hops"`
    CumulativeRisk float64              `json:"cumulative_risk"`
    Path           []PathNode           `json:"path"`
    Vulnerabilities []PathVulnerability `json:"vulnerabilities"`
    Exploitable    bool                 `json:"exploitable"`
}

type PathNode struct {
    ID        string  `json:"id"`
    Type      string  `json:"type"`
    Name      string  `json:"name"`
    RiskScore float64 `json:"risk_score"`
    Role      string  `json:"role"` // entry_point, pivot_point, target
}

type PathVulnerability struct {
    FindingID      string  `json:"finding_id"`
    Title          string  `json:"title"`
    Severity       float64 `json:"severity"`
    Exploited      bool    `json:"exploited_in_path"`
    RemediationURL string  `json:"remediation_url,omitempty"`
}

func NewAttackPathEngine(driver neo4j.Driver) *AttackPathEngine {
    return &AttackPathEngine{
        driver: driver,
        config: AttackPathConfig{
            MaxHops:          5,
            MaxPathsPerQuery: 50,
            RiskThreshold:    50.0,
            CacheTTL:         5 * time.Minute,
        },
    }
}

// FindPathsFromInternet finds all attack paths from internet-facing assets
func (ape *AttackPathEngine) FindPathsFromInternet(ctx context.Context, maxHops int) ([]AttackPath, error) {
    session := ape.driver.NewSession(neo4j.SessionConfig{})
    defer session.Close()

    query := `
        // Find internet-facing assets as entry points
        MATCH (entry:Asset {internet_exposed: true})
        WHERE entry.risk_score >= $risk_threshold
        
        // Find potential targets (sensitive data, admin roles)
        MATCH (target:Asset)
        WHERE (target:Data AND target.data_sensitivity IN ['confidential', 'restricted'])
           OR (target:Identity AND target.privilege_level = 'admin')
        
        // Find all simple paths between entry and target
        MATCH path = shortestPath((entry)-[:HAS_ACCESS_TO|CONNECTED_TO|RUNS_ON|ASSUMES_ROLE*1..$max_hops]-(target))
        WHERE ALL(r IN relationships(path) WHERE r.valid_to = 0)
        
        WITH entry, target, path,
                nodes(path) as pathNodes,
                relationships(path) as pathRels
                
        // Calculate cumulative risk (max of node risks + weighted sum)
        WITH entry, target, path, pathNodes,
             reduce(maxRisk = 0.0, n IN pathNodes | 
                CASE WHEN n.risk_score > maxRisk THEN n.risk_score ELSE maxRisk END
                ) as maxNodeRisk,
             reduce(relRisk = 0.0, r IN pathRels | 
                relRisk + COALESCE(r.trust_score, 1.0) * 10
                ) as relationshipRisk
             
        // Combine risks with weights
        WITH entry, target, path, pathNodes,
             (maxNodeRisk * 0.7 + relationshipRisk * 0.3) as cumulativeRisk
             
        WHERE cumulativeRisk >= $risk_threshold
        RETURN entry.id as source_id,
               target.id as target_id,
               path,
               cumulativeRisk,
               [n IN pathNodes | n.id] as node_ids,
               length(path) as hop_count
        ORDER BY cumulativeRisk DESC
        LIMIT $max_paths`

    params := map[string]interface{}{
        "max_hops":       maxHops,
        "risk_threshold": ape.config.RiskThreshold,
        "max_paths":      ape.config.MaxPathsPerQuery,
    }

    result, err := session.Run(ctx, query, params)
    if err != nil {
        return nil, fmt.Errorf("failed to execute path query: %v", err)
    }

    var paths []AttackPath
    for result.Next(ctx) {
        record := result.Record()
        path, err := ape.recordToAttackPath(record)
        if err != nil {
            log.Printf("Failed to convert record to attack path: %v", err)
            continue
        }
        paths = append(paths, path)
    }

    return paths, nil
}

// FindPathsBetween finds attack paths between specific assets
func (ape *AttackPathEngine) FindPathsBetween(ctx context.Context, sourceID, targetID string, maxHops int) ([]AttackPath, error) {
    session := ape.driver.NewSession(neo4j.SessionConfig{})
    defer session.Close()

    query := `
        MATCH (source:Asset {id: $source_id})
        MATCH (target:Asset {id: $target_id})
        
        // Find all paths up to maxHops
        CALL apoc.path.expandConfig(source, {
            relationshipFilter: 'HAS_ACCESS_TO|CONNECTED_TO|RUNS_ON|ASSUMES_ROLE>',
            labelFilter: '>Asset',
            minLevel: 1,
            maxLevel: $max_hops,
            terminatorNodes: [target],
            uniqueness: 'NODE_GLOBAL'
        }) YIELD path
        
        WITH path,
             nodes(path) as pathNodes,
             relationships(path) as pathRels
             
        // Filter for valid relationships (not expired)
        WHERE ALL(r IN pathRels WHERE r.valid_to = 0)
        
        // Calculate path metrics
        WITH path, pathNodes,
             [n IN pathNodes | n.risk_score] as nodeRisks,
             [n IN pathNodes | n.id] as nodeIds,
             length(path) as hopCount,
             
             // Find vulnerabilities along the path
             [n IN pathNodes WHERE EXISTS((n)-[:GENERATES]->(:Finding)) | 
                {node_id: n.id, findings: [(n)-[:GENERATES]->(f:Finding) | f]}
             ] as nodeFindings
        
        // Calculate cumulative risk
        WITH path, nodeIds, hopCount, nodeFindings,
             reduce(maxRisk = 0.0, r IN nodeRisks | 
                CASE WHEN r > maxRisk THEN r ELSE maxRisk END
                ) as maxRisk,
             
             // Count critical vulnerabilities
             reduce(criticalCount = 0, nf IN nodeFindings |
                criticalCount + size([f IN nf.findings WHERE f.severity >= 8.5 | 1])
                ) as criticalVulns
        
        // Enhanced risk calculation
        WITH *, 
             (maxRisk * 0.6 + 
              (criticalVulns * 15) + 
              (hopCount * 2)) as cumulativeRisk
              
        WHERE cumulativeRisk >= $risk_threshold
        RETURN nodeIds,
               cumulativeRisk,
               hopCount,
               nodeFindings
        ORDER BY cumulativeRisk DESC`

    params := map[string]interface{}{
        "source_id":      sourceID,
        "target_id":      targetID,
        "max_hops":       maxHops,
        "risk_threshold": ape.config.RiskThreshold,
    }

    result, err := session.Run(ctx, query, params)
    if err != nil {
        return nil, fmt.Errorf("failed to execute path query: %v", err)
    }

    return ape.processPathResults(ctx, result)
}

// SimulateAttack simulates an attack from a starting point
func (ape *AttackPathEngine) SimulateAttack(ctx context.Context, startAssetID string, maxHops int) (*AttackSimulation, error) {
    session := ape.driver.NewSession(neo4j.SessionConfig{})
    defer session.Close()

    query := `
        MATCH (start:Asset {id: $start_id})
        
        // Use APOC's Dijkstra algorithm with risk scores as weights
        CALL apoc.algo.dijkstra(
            start, 
            null,
            'HAS_ACCESS_TO|CONNECTED_TO|RUNS_ON|ASSUMES_ROLE',
            'risk_score',
            1,
            $max_hops
        ) YIELD path, weight
        
        WITH path, weight,
             nodes(path) as pathNodes,
             relationships(path) as pathRels
             
        WHERE ALL(r IN pathRels WHERE r.valid_to = 0)
        
        // Group by target
        WITH last(pathNodes) as target,
             collect({
                path: path,
                weight: weight,
                hops: length(path)
             }) as pathsToTarget
        
        // Find the shortest (lowest weight) path to each target
        WITH target, 
             apoc.coll.sortMulti(pathsToTarget, ['weight', 'hops'])[0] as bestPath
        
        WHERE target.risk_score >= 30
        RETURN target.id as target_id,
               target.type as target_type,
               target.risk_score as target_risk,
               bestPath.weight as path_risk,
               bestPath.hops as hop_count,
               [n IN nodes(bestPath.path) | {id: n.id, type: n.type}] as path_nodes
        ORDER BY target_risk DESC
        LIMIT 20`

    params := map[string]interface{}{
        "start_id": startAssetID,
        "max_hops": maxHops,
    }

    result, err := session.Run(ctx, query, params)
    if err != nil {
        return nil, fmt.Errorf("failed to simulate attack: %v", err)
    }

    return ape.processSimulationResults(ctx, result)
}

// GetCriticalPaths returns the most critical attack paths across the environment
func (ape *AttackPathEngine) GetCriticalPaths(ctx context.Context, limit int) ([]CriticalPath, error) {
    session := ape.driver.NewSession(neo4j.SessionConfig{})
    defer session.Close()

    // This query uses Neo4j's Graph Data Science library for more advanced analysis
    query := `
        // Create in-memory graph
        CALL gds.graph.project(
            'attack-graph',
            'Asset',
            {
                HAS_ACCESS_TO: {orientation: 'NATURAL'},
                CONNECTED_TO: {orientation: 'UNDIRECTED'},
                ASSUMES_ROLE: {orientation: 'NATURAL'}
            },
            {
                nodeProperties: ['risk_score', 'type', 'internet_exposed'],
                relationshipProperties: ['trust_score']
            }
        )
        
        // Find betweenness centrality to identify critical nodes
        CALL gds.betweenness.stream('attack-graph', {samplingSize: 1000})
        YIELD nodeId, score
        WITH gds.util.asNode(nodeId) as node, score
        WHERE score > 0.1 AND node.risk_score > 40
        ORDER BY score DESC
        LIMIT 10
        
        // For each critical node, find paths from internet
        MATCH (internet:Asset {type: 'internet'})
        MATCH path = shortestPath((internet)-[*1..5]-(node))
        WHERE ALL(r IN relationships(path) WHERE r.valid_to = 0)
        
        RETURN node.id as critical_node_id,
               node.type as node_type,
               node.risk_score as node_risk,
               collect({
                path: [n IN nodes(path) | n.id],
                length: length(path),
                exposure: size([n IN nodes(path) WHERE n.internet_exposed | 1])
               }) as exposure_paths
        ORDER BY node_risk DESC`

    result, err := session.Run(ctx, query, nil)
    if err != nil {
        // Fallback to simpler query if GDS is not available
        return ape.getCriticalPathsFallback(ctx, limit)
    }

    return ape.processCriticalPaths(ctx, result)
}

// Optimized path finding for real-time updates
func (ape *AttackPathEngine) FindPathsAffectedByAsset(ctx context.Context, assetID string) ([]AffectedPath, error) {
    session := ape.driver.NewSession(neo4j.SessionConfig{})
    defer session.Close()

    // Find all paths that include this asset and recalculate their risk
    query := `
        MATCH (asset:Asset {id: $asset_id})
        
        // Find all incoming and outgoing relationships
        MATCH (asset)-[r]-(neighbor:Asset)
        WHERE r.valid_to = 0
        
        // Find all shortest paths that go through this asset
        WITH collect(DISTINCT neighbor) as neighbors, asset
        
        UNWIND neighbors as neighbor
        MATCH path = (n1)-[*1..3]-(asset)-[*1..3]-(n2)
        WHERE n1 <> n2 
          AND n1.internet_exposed = true
          AND (n2:Data OR n2.privilege_level = 'admin')
          AND ALL(r IN relationships(path) WHERE r.valid_to = 0)
        
        RETURN DISTINCT path,
               [n IN nodes(path) | n.id] as node_ids,
               reduce(maxRisk = 0.0, n IN nodes(path) |
                CASE WHEN n.risk_score > maxRisk THEN n.risk_score ELSE maxRisk END
               ) as path_risk
        ORDER BY path_risk DESC
        LIMIT 25`

    params := map[string]interface{}{
        "asset_id": assetID,
    }

    result, err := session.Run(ctx, query, params)
    if err != nil {
        return nil, fmt.Errorf("failed to find affected paths: %v", err)
    }

    return ape.processAffectedPaths(ctx, result)
}

// Helper function to process path results
func (ape *AttackPathEngine) processPathResults(ctx context.Context, result neo4j.Result) ([]AttackPath, error) {
    var paths []AttackPath
    
    for result.Next(ctx) {
        record := result.Record()
        
        nodeIDs, _ := record.Get("nodeIds")
        cumulativeRisk, _ := record.Get("cumulativeRisk")
        hopCount, _ := record.Get("hopCount")
        
        path := AttackPath{
            ID:             generateUUID(),
            Hops:           int(hopCount.(int64)),
            CumulativeRisk: cumulativeRisk.(float64),
        }
        
        // Convert node IDs to full node objects
        if nodeIDsSlice, ok := nodeIDs.([]interface{}); ok {
            for i, nodeID := range nodeIDsSlice {
                node, err := ape.getNodeByID(ctx, nodeID.(string))
                if err != nil {
                    continue
                }
                
                pathNode := PathNode{
                    ID:        node.ID,
                    Type:      node.Type,
                    Name:      node.Name,
                    RiskScore: node.RiskScore,
                }
                
                // Determine role
                if i == 0 {
                    pathNode.Role = "entry_point"
                    path.SourceID = node.ID
                } else if i == len(nodeIDsSlice)-1 {
                    pathNode.Role = "target"
                    path.TargetID = node.ID
                } else {
                    pathNode.Role = "pivot_point"
                }
                
                path.Path = append(path.Path, pathNode)
            }
        }
        
        // Find vulnerabilities along the path
        vulns, err := ape.findPathVulnerabilities(ctx, path.Path)
        if err == nil {
            path.Vulnerabilities = vulns
            path.Exploitable = ape.isPathExploitable(vulns)
        }
        
        paths = append(paths, path)
    }
    
    return paths, nil
}

// Calculate if a path is exploitable based on vulnerabilities
func (ape *AttackPathEngine) isPathExploitable(vulns []PathVulnerability) bool {
    // A path is considered exploitable if it has at least one high-severity vulnerability
    // or multiple medium-severity vulnerabilities in sequence
    
    highSeverityCount := 0
    consecutiveMedium := 0
    
    for _, vuln := range vulns {
        if vuln.Severity >= 8.0 {
            highSeverityCount++
        }
        
        if vuln.Severity >= 5.0 && vuln.Severity < 8.0 {
            consecutiveMedium++
            if consecutiveMedium >= 2 {
                return true
            }
        } else {
            consecutiveMedium = 0
        }
    }
    
    return highSeverityCount > 0
}

// Batch processing for better performance
func (ape *AttackPathEngine) BatchFindPaths(ctx context.Context, assetIDs []string) (map[string][]AttackPath, error) {
    results := make(map[string][]AttackPath)
    
    // Process in batches to avoid overwhelming Neo4j
    batchSize := 10
    for i := 0; i < len(assetIDs); i += batchSize {
        end := i + batchSize
        if end > len(assetIDs) {
            end = len(assetIDs)
        }
        
        batch := assetIDs[i:end]
        batchResults, err := ape.processBatch(ctx, batch)
        if err != nil {
            log.Printf("Error processing batch %d-%d: %v", i, end, err)
            continue
        }
        
        // Merge results
        for assetID, paths := range batchResults {
            results[assetID] = append(results[assetID], paths...)
        }
    }
    
    return results, nil
}

// Cache layer for frequently queried paths
type PathCache struct {
    mu    sync.RWMutex
    cache map[string]CachedPaths
}

type CachedPaths struct {
    Paths     []AttackPath
    Timestamp time.Time
    TTL       time.Duration
}

func (pc *PathCache) Get(key string) ([]AttackPath, bool) {
    pc.mu.RLock()
    defer pc.mu.RUnlock()
    
    if cached, exists := pc.cache[key]; exists {
        if time.Since(cached.Timestamp) < cached.TTL {
            return cached.Paths, true
        }
        // Expired, remove from cache
        delete(pc.cache, key)
    }
    return nil, false
}

func (pc *PathCache) Set(key string, paths []AttackPath, ttl time.Duration) {
    pc.mu.Lock()
    defer pc.mu.Unlock()
    
    pc.cache[key] = CachedPaths{
        Paths:     paths,
        Timestamp: time.Now(),
        TTL:       ttl,
    }
}
