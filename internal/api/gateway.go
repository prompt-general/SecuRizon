package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"github.com/securizon/pkg/models"
)

// Gateway represents the API gateway
type Gateway struct {
	server          *http.Server
	router          *mux.Router
	graphStore      GraphStore
	riskEngine      RiskEngine
	eventBus        EventBus
	config          GatewayConfig
	middleware      []Middleware
	metrics         *GatewayMetrics
}

// GraphStore interface for graph operations
type GraphStore interface {
	CreateAsset(ctx context.Context, asset models.Asset) error
	GetAsset(ctx context.Context, id string) (models.Asset, error)
	UpdateAsset(ctx context.Context, asset models.Asset) error
	DeleteAsset(ctx context.Context, id string) error
	ListAssets(ctx context.Context, filter models.AssetFilter) ([]models.Asset, error)
	SearchAssets(ctx context.Context, query models.AssetQuery) ([]models.Asset, error)
	CreateRelationship(ctx context.Context, rel models.Relationship) error
	GetRelationship(ctx context.Context, id string) (models.Relationship, error)
	UpdateRelationship(ctx context.Context, rel models.Relationship) error
	DeleteRelationship(ctx context.Context, id string) error
	ListRelationships(ctx context.Context, filter models.RelationshipFilter) ([]models.Relationship, error)
	SearchRelationships(ctx context.Context, query models.RelationshipQuery) ([]models.Relationship, error)
	GetNeighbors(ctx context.Context, assetID string, direction string, maxDepth int) ([]models.Asset, []models.Relationship, error)
	FindPath(ctx context.Context, fromAssetID, toAssetID string, maxDepth int) (*models.GraphPath, error)
	FindAttackPaths(ctx context.Context, entryPoints []string, targets []string, maxDepth int) ([]models.GraphPath, error)
	GetAssetRisk(ctx context.Context, assetID string) (models.RiskScore, error)
	UpdateAssetRisk(ctx context.Context, risk models.RiskScore) error
	GetAssetFindings(ctx context.Context, assetID string) ([]models.Finding, error)
	CreateFinding(ctx context.Context, finding models.Finding) error
	UpdateFinding(ctx context.Context, finding models.Finding) error
	GetRiskSummary(ctx context.Context, filter models.AssetFilter) (*models.RiskSummary, error)
	GetRiskTrends(ctx context.Context, assetID string, timeRange models.TimeRange) (*models.RiskTrend, error)
}

// RiskEngine interface for risk operations
type RiskEngine interface {
	CalculateRisk(ctx context.Context, asset models.Asset, findings []models.Finding, threats []models.ThreatEvent) (models.RiskScore, error)
	RecalculateRisk(ctx context.Context, assetID string) (models.RiskScore, error)
	UpdateRiskScore(ctx context.Context, assetID string, score models.RiskScore) error
	BatchRecalculateRisk(ctx context.Context, assetIDs []string) ([]models.RiskScore, error)
	GetMetrics() interface{}
	GetRiskSummary(ctx context.Context) (*models.RiskSummary, error)
}

// EventBus interface for event operations
type EventBus interface {
	PublishEvent(ctx context.Context, topic string, event models.BaseEvent) error
	PublishBatch(ctx context.Context, topic string, batch models.EventBatch) error
	Ping(ctx context.Context) error
}

// GatewayConfig represents gateway configuration
type GatewayConfig struct {
	Host              string        `json:"host"`
	Port              int           `json:"port"`
	ReadTimeout       time.Duration `json:"read_timeout"`
	WriteTimeout      time.Duration `json:"write_timeout"`
	IdleTimeout       time.Duration `json:"idle_timeout"`
	EnableCORS        bool          `json:"enable_cors"`
	AllowedOrigins    []string      `json:"allowed_origins"`
	AllowedMethods    []string      `json:"allowed_methods"`
	AllowedHeaders    []string      `json:"allowed_headers"`
	EnableAuth        bool          `json:"enable_auth"`
	AuthType          string        `json:"auth_type"` // jwt, oauth2, apikey
	JWTSecret         string        `json:"jwt_secret"`
	EnableMetrics     bool          `json:"enable_metrics"`
	EnablePprof       bool          `json:"enable_pprof"`
	EnableSwagger     bool          `json:"enable_swagger"`
	RateLimitEnabled  bool          `json:"rate_limit_enabled"`
	RateLimitRPS      int           `json:"rate_limit_rps"`
	RequestTimeout    time.Duration `json:"request_timeout"`
	MaxRequestSize    int64         `json:"max_request_size"`
}

// DefaultGatewayConfig returns default gateway configuration
func DefaultGatewayConfig() GatewayConfig {
	return GatewayConfig{
		Host:             "0.0.0.0",
		Port:             8080,
		ReadTimeout:      30 * time.Second,
		WriteTimeout:     30 * time.Second,
		IdleTimeout:      120 * time.Second,
		EnableCORS:       true,
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		EnableAuth:       false,
		AuthType:         "jwt",
		EnableMetrics:    true,
		EnablePprof:      false,
		EnableSwagger:    true,
		RateLimitEnabled: false,
		RateLimitRPS:     100,
		RequestTimeout:   30 * time.Second,
		MaxRequestSize:   10 << 20, // 10MB
	}
}

// Middleware represents HTTP middleware
type Middleware func(http.Handler) http.Handler

// GatewayMetrics represents gateway metrics
type GatewayMetrics struct {
	RequestsTotal    int64                    `json:"requests_total"`
	RequestsActive   int64                    `json:"requests_active"`
	RequestsFailed   int64                    `json:"requests_failed"`
	AverageLatency   time.Duration            `json:"average_latency"`
	RequestsByPath   map[string]int64          `json:"requests_by_path"`
	RequestsByMethod map[string]int64         `json:"requests_by_method"`
	RequestsByStatus map[int]int64             `json:"requests_by_status"`
	LastRequest      time.Time                 `json:"last_request"`
}

// NewGateway creates a new API gateway
func NewGateway(config GatewayConfig, graphStore GraphStore, riskEngine RiskEngine, eventBus EventBus) *Gateway {
	router := mux.NewRouter()
	
	gateway := &Gateway{
		router:     router,
		graphStore: graphStore,
		riskEngine: riskEngine,
		eventBus:   eventBus,
		config:     config,
		middleware: make([]Middleware, 0),
		metrics: &GatewayMetrics{
			RequestsByPath:   make(map[string]int64),
			RequestsByMethod: make(map[string]int64),
			RequestsByStatus: make(map[string]int64),
		},
	}
	
	// Setup routes
	gateway.setupRoutes()
	
	// Setup middleware
	gateway.setupMiddleware()
	
	// Create HTTP server
	gateway.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", config.Host, config.Port),
		Handler:      router,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		IdleTimeout:  config.IdleTimeout,
	}
	
	return gateway
}

// setupRoutes configures all API routes
func (g *Gateway) setupRoutes() {
	api := g.router.PathPrefix("/api/v1").Subrouter()
	
	// Asset routes
	assets := api.PathPrefix("/assets").Subrouter()
	assets.HandleFunc("", g.handleListAssets).Methods("GET")
	assets.HandleFunc("", g.handleCreateAsset).Methods("POST")
	assets.HandleFunc("/{id}", g.handleGetAsset).Methods("GET")
	assets.HandleFunc("/{id}", g.handleUpdateAsset).Methods("PUT")
	assets.HandleFunc("/{id}", g.handleDeleteAsset).Methods("DELETE")
	assets.HandleFunc("/search", g.handleSearchAssets).Methods("POST")
	assets.HandleFunc("/{id}/neighbors", g.handleGetNeighbors).Methods("GET")
	assets.HandleFunc("/{id}/risk", g.handleGetAssetRisk).Methods("GET")
	assets.HandleFunc("/{id}/findings", g.handleGetAssetFindings).Methods("GET")
	
	// Relationship routes
	relationships := api.PathPrefix("/relationships").Subrouter()
	relationships.HandleFunc("", g.handleListRelationships).Methods("GET")
	relationships.HandleFunc("", g.handleCreateRelationship).Methods("POST")
	relationships.HandleFunc("/{id}", g.handleGetRelationship).Methods("GET")
	relationships.HandleFunc("/{id}", g.handleUpdateRelationship).Methods("PUT")
	relationships.HandleFunc("/{id}", g.handleDeleteRelationship).Methods("DELETE")
	relationships.HandleFunc("/search", g.handleSearchRelationships).Methods("POST")
	
	// Finding routes
	findings := api.PathPrefix("/findings").Subrouter()
	findings.HandleFunc("", g.handleListFindings).Methods("GET")
	findings.HandleFunc("", g.handleCreateFinding).Methods("POST")
	findings.HandleFunc("/{id}", g.handleGetFinding).Methods("GET")
	findings.HandleFunc("/{id}", g.handleUpdateFinding).Methods("PUT")
	findings.HandleFunc("/{id}/resolve", g.handleResolveFinding).Methods("POST")
	
	// Risk routes
	risk := api.PathPrefix("/risk").Subrouter()
	risk.HandleFunc("/summary", g.handleGetRiskSummary).Methods("GET")
	risk.HandleFunc("/trends/{assetId}", g.handleGetRiskTrends).Methods("GET")
	risk.HandleFunc("/recalculate", g.handleRecalculateRisk).Methods("POST")
	risk.HandleFunc("/batch-recalculate", g.handleBatchRecalculateRisk).Methods("POST")
	
	// Attack path routes
	attackPaths := api.PathPrefix("/attack-paths").Subrouter()
	attackPaths.HandleFunc("/find", g.handleFindAttackPaths).Methods("POST")
	attackPaths.HandleFunc("/path", g.handleFindPath).Methods("POST")
	
	// Health and metrics
	api.HandleFunc("/health", g.handleHealth).Methods("GET")
	api.HandleFunc("/metrics", g.handleMetrics).Methods("GET")
	
	// Admin routes
	admin := api.PathPrefix("/admin").Subrouter()
	admin.HandleFunc("/cache/clear", g.handleClearCache).Methods("POST")
	admin.HandleFunc("/cache/stats", g.handleCacheStats).Methods("GET")
}

// setupMiddleware configures HTTP middleware
func (g *Gateway) setupMiddleware() {
	// Apply middleware in reverse order
	for i := len(g.middleware) - 1; i >= 0; i-- {
		g.router.Use(g.middleware[i])
	}
	
	// Add built-in middleware
	if g.config.EnableCORS {
		g.setupCORS()
	}
	
	if g.config.EnableAuth {
		g.setupAuth()
	}
	
	if g.config.RateLimitEnabled {
		g.setupRateLimit()
	}
	
	// Metrics middleware (always last to capture all requests)
	g.router.Use(g.metricsMiddleware)
}

// setupCORS configures CORS
func (g *Gateway) setupCORS() {
	c := cors.New(cors.Options{
		AllowedOrigins:   g.config.AllowedOrigins,
		AllowedMethods:   g.config.AllowedMethods,
		AllowedHeaders:   g.config.AllowedHeaders,
		AllowCredentials: true,
	})
	
	g.router.Use(c.Handler)
}

// setupAuth configures authentication
func (g *Gateway) setupAuth() {
	// Implementation depends on auth type
	switch g.config.AuthType {
	case "jwt":
		g.router.Use(g.jwtAuthMiddleware)
	case "oauth2":
		g.router.Use(g.oauth2AuthMiddleware)
	case "apikey":
		g.router.Use(g.apiKeyAuthMiddleware)
	}
}

// setupRateLimit configures rate limiting
func (g *Gateway) setupRateLimit() {
	g.router.Use(g.rateLimitMiddleware)
}

// Start starts the API gateway
func (g *Gateway) Start() error {
	log.Printf("Starting API gateway on %s", g.server.Addr)
	return g.server.ListenAndServe()
}

// Stop stops the API gateway
func (g *Gateway) Stop(ctx context.Context) error {
	log.Printf("Stopping API gateway")
	return g.server.Shutdown(ctx)
}

// AddMiddleware adds middleware to the gateway
func (g *Gateway) AddMiddleware(middleware Middleware) {
	g.middleware = append(g.middleware, middleware)
}

// Request/Response types

type ListAssetsRequest struct {
	Types         []models.AssetType  `json:"types,omitempty"`
	Providers     []models.Provider   `json:"providers,omitempty"`
	Environments  []models.Environment `json:"environments,omitempty"`
	MinRiskScore  float64             `json:"min_risk_score,omitempty"`
	MaxRiskScore  float64             `json:"max_risk_score,omitempty"`
	Limit         int                 `json:"limit,omitempty"`
	Offset        int                 `json:"offset,omitempty"`
}

type SearchAssetsRequest struct {
	Query        string              `json:"query"`
	Types        []models.AssetType  `json:"types,omitempty"`
	Providers    []models.Provider   `json:"providers,omitempty"`
	Environments []models.Environment `json:"environments,omitempty"`
	Limit        int                 `json:"limit,omitempty"`
}

type CreateAssetRequest struct {
	Asset models.Asset `json:"asset"`
}

type UpdateAssetRequest struct {
	Asset models.Asset `json:"asset"`
}

type GetNeighborsRequest struct {
	Direction string `json:"direction"` // incoming, outgoing, both
	MaxDepth  int    `json:"max_depth"`
}

type CreateRelationshipRequest struct {
	Relationship models.Relationship `json:"relationship"`
}

type UpdateRelationshipRequest struct {
	Relationship models.Relationship `json:"relationship"`
}

type SearchRelationshipsRequest struct {
	FromAssetID   string                     `json:"from_asset_id,omitempty"`
	ToAssetID     string                     `json:"to_asset_id,omitempty"`
	Types         []models.RelationshipType  `json:"types,omitempty"`
	MinStrength   float64                    `json:"min_strength,omitempty"`
	MaxStrength   float64                    `json:"max_strength,omitempty"`
	Limit         int                        `json:"limit,omitempty"`
}

type CreateFindingRequest struct {
	Finding models.Finding `json:"finding"`
}

type UpdateFindingRequest struct {
	Finding models.Finding `json:"finding"`
}

type FindAttackPathsRequest struct {
	EntryPoints    []string                   `json:"entry_points"`
	Targets        []string                   `json:"targets"`
	MaxDepth       int                        `json:"max_depth"`
	MinRiskScore   float64                    `json:"min_risk_score,omitempty"`
	AllowedEdges   []models.RelationshipType  `json:"allowed_edges,omitempty"`
	ForbiddenEdges []models.RelationshipType  `json:"forbidden_edges,omitempty"`
}

type FindPathRequest struct {
	FromAssetID string `json:"from_asset_id"`
	ToAssetID   string `json:"to_asset_id"`
	MaxDepth    int    `json:"max_depth"`
}

type RecalculateRiskRequest struct {
	AssetIDs []string `json:"asset_ids,omitempty"` // If empty, recalculate all
}

type BatchRecalculateRiskRequest struct {
	AssetIDs []string `json:"asset_ids"`
}

// Response types

type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
	Meta    *APIMeta    `json:"meta,omitempty"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

type APIMeta struct {
	Total   int `json:"total,omitempty"`
	Limit   int `json:"limit,omitempty"`
	Offset  int `json:"offset,omitempty"`
	HasMore bool `json:"has_more,omitempty"`
}

// Helper functions

func writeJSONResponse(w http.ResponseWriter, status int, response APIResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

func writeErrorResponse(w http.ResponseWriter, status int, code, message, details string) {
	response := APIResponse{
		Success: false,
		Error: &APIError{
			Code:    code,
			Message: message,
			Details: details,
		},
	}
	writeJSONResponse(w, status, response)
}

func writeSuccessResponse(w http.ResponseWriter, data interface{}, meta *APIMeta) {
	response := APIResponse{
		Success: true,
		Data:    data,
		Meta:    meta,
	}
	writeJSONResponse(w, http.StatusOK, response)
}

func parseRequestBody(r *http.Request, target interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(target)
}

// Middleware implementations

func (g *Gateway) metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Wrap response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		
		// Process request
		next.ServeHTTP(wrapped, r)
		
		// Update metrics
		duration := time.Since(start)
		g.updateMetrics(r, wrapped.statusCode, duration)
	})
}

func (g *Gateway) updateMetrics(r *http.Request, statusCode int, duration time.Duration) {
	g.metrics.mu.Lock()
	defer g.metrics.mu.Unlock()
	
	g.metrics.RequestsTotal++
	g.metrics.RequestsByPath[r.URL.Path]++
	g.metrics.RequestsByMethod[r.Method]++
	g.metrics.RequestsByStatus[statusCode]++
	g.metrics.LastRequest = time.Now()
	
	// Update average latency
	if g.metrics.AverageLatency == 0 {
		g.metrics.AverageLatency = duration
	} else {
		g.metrics.AverageLatency = (g.metrics.AverageLatency + duration) / 2
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Placeholder middleware implementations
func (g *Gateway) jwtAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// JWT authentication implementation
		next.ServeHTTP(w, r)
	})
}

func (g *Gateway) oauth2AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// OAuth2 authentication implementation
		next.ServeHTTP(w, r)
	})
}

func (g *Gateway) apiKeyAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// API key authentication implementation
		next.ServeHTTP(w, r)
	})
}

func (g *Gateway) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Rate limiting implementation
		next.ServeHTTP(w, r)
	})
}
