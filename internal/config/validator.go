package config

import (
	"fmt"
	"net/url"
	"strings"
)

// Validate performs comprehensive validation of the configuration
func (c *Config) Validate() error {
	if c.Version == "" {
		return fmt.Errorf("version is required")
	}

	// Validate Kafka configuration
	if err := c.validateKafka(); err != nil {
		return fmt.Errorf("kafka config error: %v", err)
	}

	// Validate Neo4j configuration
	if err := c.validateNeo4j(); err != nil {
		return fmt.Errorf("neo4j config error: %v", err)
	}

	// Validate API configuration
	if err := c.validateAPI(); err != nil {
		return fmt.Errorf("api config error: %v", err)
	}

	// Validate Risk configuration
	if err := c.validateRisk(); err != nil {
		return fmt.Errorf("risk config error: %v", err)
	}

	// Validate Logging configuration
	if err := c.validateLogging(); err != nil {
		return fmt.Errorf("logging config error: %v", err)
	}

	return nil
}

func (c *Config) validateKafka() error {
	if len(c.Kafka.BootstrapServers) == 0 {
		return fmt.Errorf("bootstrap_servers is required")
	}

	for _, server := range c.Kafka.BootstrapServers {
		if !strings.Contains(server, ":") {
			return fmt.Errorf("invalid bootstrap server format: %s (expected host:port)", server)
		}
	}

	if c.Kafka.ClientID == "" {
		return fmt.Errorf("client_id is required")
	}

	if c.Kafka.Security.SASLMechanism != "" && c.Kafka.Security.SASLMechanism != "PLAIN" && 
	   c.Kafka.Security.SASLMechanism != "SCRAM-SHA-256" && c.Kafka.Security.SASLMechanism != "SCRAM-SHA-512" {
		return fmt.Errorf("invalid sasl_mechanism: %s", c.Kafka.Security.SASLMechanism)
	}

	return nil
}

func (c *Config) validateNeo4j() error {
	if c.Neo4j.URI == "" {
		return fmt.Errorf("uri is required")
	}

	// Validate URI format
	if _, err := url.Parse(c.Neo4j.URI); err != nil {
		return fmt.Errorf("invalid uri format: %v", err)
	}

	if c.Neo4j.Username == "" {
		return fmt.Errorf("username is required")
	}

	if c.Neo4j.MaxConnectionPoolSize <= 0 {
		return fmt.Errorf("max_connection_pool_size must be greater than 0")
	}

	return nil
}

func (c *Config) validateAPI() error {
	if c.API.Port <= 0 || c.API.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}

	if c.API.Auth.Enabled && c.API.Auth.JWKURL == "" {
		return fmt.Errorf("jwk_url is required when auth is enabled")
	}

	if c.API.Auth.Enabled && c.API.Auth.Audience == "" {
		return fmt.Errorf("audience is required when auth is enabled")
	}

	if c.API.CORS.Enabled && len(c.API.CORS.AllowedOrigins) == 0 {
		return fmt.Errorf("allowed_origins is required when CORS is enabled")
	}

	return nil
}

func (c *Config) validateRisk() error {
	if c.Risk.Thresholds.Critical <= 0 || c.Risk.Thresholds.Critical > 100 {
		return fmt.Errorf("critical threshold must be between 1 and 100")
	}

	if c.Risk.Thresholds.High <= 0 || c.Risk.Thresholds.High > 100 {
		return fmt.Errorf("high threshold must be between 1 and 100")
	}

	if c.Risk.Thresholds.Medium <= 0 || c.Risk.Thresholds.Medium > 100 {
		return fmt.Errorf("medium threshold must be between 1 and 100")
	}

	if c.Risk.Thresholds.Low <= 0 || c.Risk.Thresholds.Low > 100 {
		return fmt.Errorf("low threshold must be between 1 and 100")
	}

	// Ensure thresholds are in descending order
	if c.Risk.Thresholds.Critical <= c.Risk.Thresholds.High {
		return fmt.Errorf("critical threshold must be greater than high threshold")
	}

	if c.Risk.Thresholds.High <= c.Risk.Thresholds.Medium {
		return fmt.Errorf("high threshold must be greater than medium threshold")
	}

	if c.Risk.Thresholds.Medium <= c.Risk.Thresholds.Low {
		return fmt.Errorf("medium threshold must be greater than low threshold")
	}

	return nil
}

func (c *Config) validateLogging() error {
	level := strings.ToLower(c.Logging.Level)
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}

	if !validLevels[level] {
		return fmt.Errorf("invalid log level: %s (must be debug, info, warn, or error)", level)
	}

	format := strings.ToLower(c.Logging.Format)
	validFormats := map[string]bool{"json": true, "text": true}

	if !validFormats[format] {
		return fmt.Errorf("invalid log format: %s (must be json or text)", format)
	}

	output := strings.ToLower(c.Logging.Output)
	validOutputs := map[string]bool{"stdout": true, "file": true, "both": true}

	if !validOutputs[output] {
		return fmt.Errorf("invalid log output: %s (must be stdout, file, or both)", output)
	}

	if (output == "file" || output == "both") && c.Logging.File.Path == "" {
		return fmt.Errorf("file path is required when output is file or both")
	}

	return nil
}

// GetRiskLevel returns the risk level name based on a score
func (c *Config) GetRiskLevel(score float64) string {
	switch {
	case score >= float64(c.Risk.Thresholds.Critical):
		return "critical"
	case score >= float64(c.Risk.Thresholds.High):
		return "high"
	case score >= float64(c.Risk.Thresholds.Medium):
		return "medium"
	case score >= float64(c.Risk.Thresholds.Low):
		return "low"
	default:
		return "minimal"
	}
}

// IsFeatureEnabled checks if a feature flag is enabled
func (c *Config) IsFeatureEnabled(feature string) bool {
	switch feature {
	case "attack_path_analysis":
		return c.FeatureFlags.AttackPathAnalysis
	case "real_time_correlation":
		return c.FeatureFlags.RealTimeCorrelation
	case "automated_remediation":
		return c.FeatureFlags.AutomatedRemediation
	case "threat_intelligence_integration":
		return c.FeatureFlags.ThreatIntelligenceIntegration
	default:
		return false
	}
}
