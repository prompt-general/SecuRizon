package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/securizon/pkg/models"
)

// Asset handlers

func (g *Gateway) handleListAssets(w http.ResponseWriter, r *http.Request) {
	var req ListAssetsRequest
	
	// Parse query parameters
	if types := r.URL.Query()["type"]; len(types) > 0 {
		req.Types = make([]models.AssetType, len(types))
		for i, t := range types {
			req.Types[i] = models.AssetType(t)
		}
	}
	
	if providers := r.URL.Query()["provider"]; len(providers) > 0 {
		req.Providers = make([]models.Provider, len(providers))
		for i, p := range providers {
			req.Providers[i] = models.Provider(p)
		}
	}
	
	if environments := r.URL.Query()["environment"]; len(environments) > 0 {
		req.Environments = make([]models.Environment, len(environments))
		for i, e := range environments {
			req.Environments[i] = models.Environment(e)
		}
	}
	
	if minRisk := r.URL.Query().Get("min_risk_score"); minRisk != "" {
		if score, err := strconv.ParseFloat(minRisk, 64); err == nil {
			req.MinRiskScore = score
		}
	}
	
	if maxRisk := r.URL.Query().Get("max_risk_score"); maxRisk != "" {
		if score, err := strconv.ParseFloat(maxRisk, 64); err == nil {
			req.MaxRiskScore = score
		}
	}
	
	if limit := r.URL.Query().Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			req.Limit = l
		}
	}
	
	if offset := r.URL.Query().Get("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil {
			req.Offset = o
		}
	}
	
	// Create filter
	filter := models.AssetFilter{
		Types:        req.Types,
		Providers:    req.Providers,
		Environments: req.Environments,
		MinRiskScore: req.MinRiskScore,
		MaxRiskScore: req.MaxRiskScore,
		Limit:        req.Limit,
		Offset:       req.Offset,
	}
	
	// Get assets
	assets, err := g.graphStore.ListAssets(r.Context(), filter)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list assets", err.Error())
		return
	}
	
	// Prepare response meta
	meta := &APIMeta{
		Total:  len(assets),
		Limit:  req.Limit,
		Offset: req.Offset,
	}
	
	if req.Limit > 0 && len(assets) == req.Limit {
		meta.HasMore = true
	}
	
	writeSuccessResponse(w, assets, meta)
}

func (g *Gateway) handleCreateAsset(w http.ResponseWriter, r *http.Request) {
	var req CreateAssetRequest
	if err := parseRequestBody(r, &req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "INVALID_REQUEST", "Failed to parse request body", err.Error())
		return
	}
	
	// Create asset
	if err := g.graphStore.CreateAsset(r.Context(), req.Asset); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create asset", err.Error())
		return
	}
	
	writeSuccessResponse(w, req.Asset, nil)
}

func (g *Gateway) handleGetAsset(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	assetID := vars["id"]
	
	asset, err := g.graphStore.GetAsset(r.Context(), assetID)
	if err != nil {
		writeErrorResponse(w, http.StatusNotFound, "NOT_FOUND", "Asset not found", err.Error())
		return
	}
	
	writeSuccessResponse(w, asset, nil)
}

func (g *Gateway) handleUpdateAsset(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	assetID := vars["id"]
	
	var req UpdateAssetRequest
	if err := parseRequestBody(r, &req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "INVALID_REQUEST", "Failed to parse request body", err.Error())
		return
	}
	
	// Verify asset ID matches
	if req.Asset.GetID() != assetID {
		writeErrorResponse(w, http.StatusBadRequest, "INVALID_REQUEST", "Asset ID mismatch", "")
		return
	}
	
	// Update asset
	if err := g.graphStore.UpdateAsset(r.Context(), req.Asset); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update asset", err.Error())
		return
	}
	
	writeSuccessResponse(w, req.Asset, nil)
}

func (g *Gateway) handleDeleteAsset(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	assetID := vars["id"]
	
	if err := g.graphStore.DeleteAsset(r.Context(), assetID); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete asset", err.Error())
		return
	}
	
	writeSuccessResponse(w, map[string]string{"id": assetID}, nil)
}

func (g *Gateway) handleSearchAssets(w http.ResponseWriter, r *http.Request) {
	var req SearchAssetsRequest
	if err := parseRequestBody(r, &req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "INVALID_REQUEST", "Failed to parse request body", err.Error())
		return
	}
	
	// Create query
	query := models.AssetQuery{
		AssetFilter: models.AssetFilter{
			Types:        req.Types,
			Providers:    req.Providers,
			Environments: req.Environments,
			Limit:        req.Limit,
		},
		TextSearch: req.Query,
	}
	
	// Search assets
	assets, err := g.graphStore.SearchAssets(r.Context(), query)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to search assets", err.Error())
		return
	}
	
	writeSuccessResponse(w, assets, nil)
}

func (g *Gateway) handleGetNeighbors(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	assetID := vars["id"]
	
	// Parse query parameters
	direction := r.URL.Query().Get("direction")
	if direction == "" {
		direction = "both"
	}
	
	maxDepth := 1
	if depth := r.URL.Query().Get("max_depth"); depth != "" {
		if d, err := strconv.Atoi(depth); err == nil {
			maxDepth = d
		}
	}
	
	// Get neighbors
	assets, relationships, err := g.graphStore.GetNeighbors(r.Context(), assetID, direction, maxDepth)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get neighbors", err.Error())
		return
	}
	
	response := map[string]interface{}{
		"assets":        assets,
		"relationships": relationships,
	}
	
	writeSuccessResponse(w, response, nil)
}

func (g *Gateway) handleGetAssetRisk(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	assetID := vars["id"]
	
	risk, err := g.graphStore.GetAssetRisk(r.Context(), assetID)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get asset risk", err.Error())
		return
	}
	
	writeSuccessResponse(w, risk, nil)
}

func (g *Gateway) handleGetAssetFindings(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	assetID := vars["id"]
	
	findings, err := g.graphStore.GetAssetFindings(r.Context(), assetID)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get asset findings", err.Error())
		return
	}
	
	writeSuccessResponse(w, findings, nil)
}

// Relationship handlers

func (g *Gateway) handleListRelationships(w http.ResponseWriter, r *http.Request) {
	var req SearchRelationshipsRequest
	
	// Parse query parameters
	if assetIDs := r.URL.Query()["asset_id"]; len(assetIDs) > 0 {
		req.AssetIDs = assetIDs
	}
	
	if types := r.URL.Query()["type"]; len(types) > 0 {
		req.Types = make([]models.RelationshipType, len(types))
		for i, t := range types {
			req.Types[i] = models.RelationshipType(t)
		}
	}
	
	if minStrength := r.URL.Query().Get("min_strength"); minStrength != "" {
		if strength, err := strconv.ParseFloat(minStrength, 64); err == nil {
			req.MinStrength = strength
		}
	}
	
	if maxStrength := r.URL.Query().Get("max_strength"); maxStrength != "" {
		if strength, err := strconv.ParseFloat(maxStrength, 64); err == nil {
			req.MaxStrength = strength
		}
	}
	
	if limit := r.URL.Query().Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			req.Limit = l
		}
	}
	
	// Create filter
	filter := models.RelationshipFilter{
		AssetIDs:    req.AssetIDs,
		Types:       req.Types,
		MinStrength: req.MinStrength,
		MaxStrength: req.MaxStrength,
		ActiveOnly:  true,
	}
	
	// Get relationships
	relationships, err := g.graphStore.ListRelationships(r.Context(), filter)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list relationships", err.Error())
		return
	}
	
	writeSuccessResponse(w, relationships, nil)
}

func (g *Gateway) handleCreateRelationship(w http.ResponseWriter, r *http.Request) {
	var req CreateRelationshipRequest
	if err := parseRequestBody(r, &req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "INVALID_REQUEST", "Failed to parse request body", err.Error())
		return
	}
	
	// Create relationship
	if err := g.graphStore.CreateRelationship(r.Context(), req.Relationship); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create relationship", err.Error())
		return
	}
	
	writeSuccessResponse(w, req.Relationship, nil)
}

func (g *Gateway) handleGetRelationship(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	relationshipID := vars["id"]
	
	relationship, err := g.graphStore.GetRelationship(r.Context(), relationshipID)
	if err != nil {
		writeErrorResponse(w, http.StatusNotFound, "NOT_FOUND", "Relationship not found", err.Error())
		return
	}
	
	writeSuccessResponse(w, relationship, nil)
}

func (g *Gateway) handleUpdateRelationship(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	relationshipID := vars["id"]
	
	var req UpdateRelationshipRequest
	if err := parseRequestBody(r, &req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "INVALID_REQUEST", "Failed to parse request body", err.Error())
		return
	}
	
	// Verify relationship ID matches
	if req.Relationship.ID != relationshipID {
		writeErrorResponse(w, http.StatusBadRequest, "INVALID_REQUEST", "Relationship ID mismatch", "")
		return
	}
	
	// Update relationship
	if err := g.graphStore.UpdateRelationship(r.Context(), req.Relationship); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update relationship", err.Error())
		return
	}
	
	writeSuccessResponse(w, req.Relationship, nil)
}

func (g *Gateway) handleDeleteRelationship(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	relationshipID := vars["id"]
	
	if err := g.graphStore.DeleteRelationship(r.Context(), relationshipID); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete relationship", err.Error())
		return
	}
	
	writeSuccessResponse(w, map[string]string{"id": relationshipID}, nil)
}

func (g *Gateway) handleSearchRelationships(w http.ResponseWriter, r *http.Request) {
	var req SearchRelationshipsRequest
	if err := parseRequestBody(r, &req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "INVALID_REQUEST", "Failed to parse request body", err.Error())
		return
	}
	
	// Create query
	query := models.RelationshipQuery{
		RelationshipFilter: models.RelationshipFilter{
			AssetIDs:    req.AssetIDs,
			Types:       req.Types,
			MinStrength: req.MinStrength,
			MaxStrength: req.MaxStrength,
		},
		Limit: req.Limit,
	}
	
	// Search relationships
	relationships, err := g.graphStore.SearchRelationships(r.Context(), query)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to search relationships", err.Error())
		return
	}
	
	writeSuccessResponse(w, relationships, nil)
}

// Finding handlers

func (g *Gateway) handleListFindings(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	var filter models.FindingFilter
	
	if statuses := r.URL.Query()["status"]; len(statuses) > 0 {
		filter.Statuses = statuses
	}
	
	if severities := r.URL.Query()["severity"]; len(severities) > 0 {
		filter.Severities = make([]float64, len(severities))
		for i, s := range severities {
			if severity, err := strconv.ParseFloat(s, 64); err == nil {
				filter.Severities[i] = severity
			}
		}
	}
	
	if assetIDs := r.URL.Query()["asset_id"]; len(assetIDs) > 0 {
		filter.AssetIDs = assetIDs
	}
	
	if limit := r.URL.Query().Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			filter.Limit = l
		}
	}
	
	// Get findings (this would need to be implemented in the graph store)
	findings := []models.Finding{} // Placeholder
	
	writeSuccessResponse(w, findings, nil)
}

func (g *Gateway) handleCreateFinding(w http.ResponseWriter, r *http.Request) {
	var req CreateFindingRequest
	if err := parseRequestBody(r, &req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "INVALID_REQUEST", "Failed to parse request body", err.Error())
		return
	}
	
	// Create finding
	if err := g.graphStore.CreateFinding(r.Context(), req.Finding); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create finding", err.Error())
		return
	}
	
	writeSuccessResponse(w, req.Finding, nil)
}

func (g *Gateway) handleGetFinding(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	findingID := vars["id"]
	
	// Get finding (this would need to be implemented in the graph store)
	finding := models.Finding{ID: findingID} // Placeholder
	
	writeSuccessResponse(w, finding, nil)
}

func (g *Gateway) handleUpdateFinding(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	findingID := vars["id"]
	
	var req UpdateFindingRequest
	if err := parseRequestBody(r, &req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "INVALID_REQUEST", "Failed to parse request body", err.Error())
		return
	}
	
	// Verify finding ID matches
	if req.Finding.ID != findingID {
		writeErrorResponse(w, http.StatusBadRequest, "INVALID_REQUEST", "Finding ID mismatch", "")
		return
	}
	
	// Update finding
	if err := g.graphStore.UpdateFinding(r.Context(), req.Finding); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update finding", err.Error())
		return
	}
	
	writeSuccessResponse(w, req.Finding, nil)
}

func (g *Gateway) handleResolveFinding(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	findingID := vars["id"]
	
	// Get finding
	finding, err := g.graphStore.GetAssetFindings(r.Context(), findingID)
	if err != nil {
		writeErrorResponse(w, http.StatusNotFound, "NOT_FOUND", "Finding not found", err.Error())
		return
	}
	
	// Update status to resolved
	if len(finding) > 0 {
		finding[0].Status = "resolved"
		if err := g.graphStore.UpdateFinding(r.Context(), finding[0]); err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to resolve finding", err.Error())
			return
		}
	}
	
	writeSuccessResponse(w, map[string]string{"id": findingID, "status": "resolved"}, nil)
}

// Risk handlers

func (g *Gateway) handleGetRiskSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := g.riskEngine.GetRiskSummary(r.Context())
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get risk summary", err.Error())
		return
	}
	
	writeSuccessResponse(w, summary, nil)
}

func (g *Gateway) handleGetRiskTrends(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	assetID := vars["id"]
	
	// Parse time range
	startTime := time.Now().AddDate(0, -1, 0) // Default: last month
	endTime := time.Now()
	
	if start := r.URL.Query().Get("start_time"); start != "" {
		if t, err := time.Parse(time.RFC3339, start); err == nil {
			startTime = t
		}
	}
	
	if end := r.URL.Query().Get("end_time"); end != "" {
		if t, err := time.Parse(time.RFC3339, end); err == nil {
			endTime = t
		}
	}
	
	timeRange := models.TimeRange{
		Start: startTime,
		End:   endTime,
	}
	
	trends, err := g.graphStore.GetRiskTrends(r.Context(), assetID, timeRange)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get risk trends", err.Error())
		return
	}
	
	writeSuccessResponse(w, trends, nil)
}

func (g *Gateway) handleRecalculateRisk(w http.ResponseWriter, r *http.Request) {
	var req RecalculateRiskRequest
	if err := parseRequestBody(r, &req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "INVALID_REQUEST", "Failed to parse request body", err.Error())
		return
	}
	
	// If no asset IDs specified, recalculate all
	if len(req.AssetIDs) == 0 {
		// Get all assets and recalculate
		assets, err := g.graphStore.ListAssets(r.Context(), models.AssetFilter{})
		if err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list assets", err.Error())
			return
		}
		
		for _, asset := range assets {
			if _, err := g.riskEngine.RecalculateRisk(r.Context(), asset.GetID()); err != nil {
				writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to recalculate risk", err.Error())
				return
			}
		}
	} else {
		// Recalculate specified assets
		for _, assetID := range req.AssetIDs {
			if _, err := g.riskEngine.RecalculateRisk(r.Context(), assetID); err != nil {
				writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to recalculate risk", err.Error())
				return
			}
		}
	}
	
	writeSuccessResponse(w, map[string]string{"message": "Risk recalculation started"}, nil)
}

func (g *Gateway) handleBatchRecalculateRisk(w http.ResponseWriter, r *http.Request) {
	var req BatchRecalculateRiskRequest
	if err := parseRequestBody(r, &req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "INVALID_REQUEST", "Failed to parse request body", err.Error())
		return
	}
	
	results, err := g.riskEngine.BatchRecalculateRisk(r.Context(), req.AssetIDs)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to batch recalculate risk", err.Error())
		return
	}
	
	writeSuccessResponse(w, results, nil)
}

// Attack path handlers

func (g *Gateway) handleFindAttackPaths(w http.ResponseWriter, r *http.Request) {
	var req FindAttackPathsRequest
	if err := parseRequestBody(r, &req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "INVALID_REQUEST", "Failed to parse request body", err.Error())
		return
	}
	
	// Find attack paths
	paths, err := g.graphStore.FindAttackPaths(r.Context(), req.EntryPoints, req.Targets, req.MaxDepth)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to find attack paths", err.Error())
		return
	}
	
	writeSuccessResponse(w, paths, nil)
}

func (g *Gateway) handleFindPath(w http.ResponseWriter, r *http.Request) {
	var req FindPathRequest
	if err := parseRequestBody(r, &req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "INVALID_REQUEST", "Failed to parse request body", err.Error())
		return
	}
	
	// Find path
	path, err := g.graphStore.FindPath(r.Context(), req.FromAssetID, req.ToAssetID, req.MaxDepth)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to find path", err.Error())
		return
	}
	
	writeSuccessResponse(w, path, nil)
}

// Health and metrics handlers

func (g *Gateway) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	
	health := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now(),
		"version":   "1.0.0", // This should come from build info
	}
	
	// Check graph store
	if err := g.graphStore.Ping(ctx); err != nil {
		health["graph_store"] = map[string]string{
			"status": "error",
			"error":  err.Error(),
		}
	} else {
		health["graph_store"] = map[string]string{"status": "ok"}
	}
	
	// Check event bus
	if err := g.eventBus.Ping(ctx); err != nil {
		health["event_bus"] = map[string]string{
			"status": "error",
			"error":  err.Error(),
		}
	} else {
		health["event_bus"] = map[string]string{"status": "ok"}
	}
	
	writeSuccessResponse(w, health, nil)
}

func (g *Gateway) handleMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := map[string]interface{}{
		"gateway": g.GetMetrics(),
		"risk":    g.riskEngine.GetMetrics(),
	}
	
	writeSuccessResponse(w, metrics, nil)
}

// Admin handlers

func (g *Gateway) handleClearCache(w http.ResponseWriter, r *http.Request) {
	// Clear risk engine cache
	if riskEngine, ok := g.riskEngine.(interface{ ClearCache() }); ok {
		riskEngine.ClearCache()
	}
	
	writeSuccessResponse(w, map[string]string{"message": "Cache cleared"}, nil)
}

func (g *Gateway) handleCacheStats(w http.ResponseWriter, r *http.Request) {
	var stats map[string]interface{}
	
	if riskEngine, ok := g.riskEngine.(interface{ GetCacheStats() map[string]interface{} }); ok {
		stats = riskEngine.GetCacheStats()
	} else {
		stats = map[string]interface{}{"enabled": false}
	}
	
	writeSuccessResponse(w, stats, nil)
}

// GetMetrics returns gateway metrics
func (g *Gateway) GetMetrics() GatewayMetrics {
	g.metrics.mu.RLock()
	defer g.metrics.mu.RUnlock()
	return *g.metrics
}
