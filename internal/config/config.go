package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the complete Securizon configuration
type Config struct {
	Version      string               `yaml:"version"`
	Collector    CollectorConfig      `yaml:"collector"`
	AWS          AWSConfig            `yaml:"aws"`
	Azure        AzureConfig          `yaml:"azure"`
	GCP          GCPConfig            `yaml:"gcp"`
	SaaS         SaaSConfig           `yaml:"saas"`
	Kafka        KafkaConfig          `yaml:"kafka"`
	Neo4j        Neo4jConfig          `yaml:"neo4j"`
	Risk         RiskConfig           `yaml:"risk"`
	Policies     PoliciesConfig       `yaml:"policies"`
	API          APIConfig            `yaml:"api"`
	Logging      LoggingConfig        `yaml:"logging"`
	Metrics      MetricsConfig        `yaml:"metrics"`
	Health       HealthConfig         `yaml:"health"`
	Tracing      TracingConfig        `yaml:"tracing"`
	FeatureFlags FeatureFlagsConfig   `yaml:"feature_flags"`
}

type CollectorConfig struct {
	ID                       string           `yaml:"id"`
	Mode                     string           `yaml:"mode"`
	FullSyncInterval         string           `yaml:"full_sync_interval"`
	EventPollInterval        string           `yaml:"event_poll_interval"`
	MaxConcurrentCollections int              `yaml:"max_concurrent_collections"`
	RateLimit                RateLimitConfig  `yaml:"rate_limit"`
}

type AWSConfig struct {
	Enabled  bool               `yaml:"enabled"`
	Regions  []string           `yaml:"regions"`
	Accounts []AWSAccountConfig `yaml:"accounts"`
	Features map[string]bool    `yaml:"features"`
}

type AWSAccountConfig struct {
	ID         string `yaml:"id"`
	RoleARN    string `yaml:"role_arn"`
	ExternalID string `yaml:"external_id"`
}

type AzureConfig struct {
	Enabled bool `yaml:"enabled"`
}

type GCPConfig struct {
	Enabled bool `yaml:"enabled"`
}

type SaaSConfig struct {
	Enabled bool         `yaml:"enabled"`
	GitHub  GitHubConfig `yaml:"github"`
}

type GitHubConfig struct {
	Enabled       bool     `yaml:"enabled"`
	Organizations []string `yaml:"organizations"`
	Scopes        []string `yaml:"scopes"`
}

type KafkaConfig struct {
	BootstrapServers []string            `yaml:"bootstrap_servers"`
	ClientID         string              `yaml:"client_id"`
	CompressionType  string              `yaml:"compression_type"`
	Security         KafkaSecurityConfig `yaml:"security"`
}

type KafkaSecurityConfig struct {
	SASLMechanism string `yaml:"sasl_mechanism"`
	SASLUsername  string `yaml:"sasl_username"`
	SASLPassword  string `yaml:"sasl_password"`
	SSLEnabled    bool   `yaml:"ssl_enabled"`
	SSLCACertPath string `yaml:"ssl_ca_cert_path"`
}

type Neo4jConfig struct {
	URI                   string `yaml:"uri"`
	Username              string `yaml:"username"`
	Password              string `yaml:"password"`
	MaxConnectionPoolSize int    `yaml:"max_connection_pool_size"`
	Encryption            bool   `yaml:"encryption"`
	TrustStrategy         string `yaml:"trust_strategy"`
}

type RiskConfig struct {
	Weights               RiskWeights    `yaml:"weights"`
	Thresholds            RiskThresholds `yaml:"thresholds"`
	RecalculationInterval string         `yaml:"recalculation_interval"`
}

type RiskWeights struct {
	Exposure        float64 `yaml:"exposure"`
	Environment     float64 `yaml:"environment"`
	ThreatIntel     float64 `yaml:"threat_intel"`
	DataSensitivity float64 `yaml:"data_sensitivity"`
}

type RiskThresholds struct {
	Critical int `yaml:"critical"`
	High     int `yaml:"high"`
	Medium   int `yaml:"medium"`
	Low      int `yaml:"low"`
}

type PoliciesConfig struct {
	Paths             []string `yaml:"paths"`
	AutoRemediate     bool     `yaml:"auto_remediate"`
	RequireApproval   bool     `yaml:"require_approval"`
	EvaluationTimeout string   `yaml:"evaluation_timeout"`
}

type APIConfig struct {
	Port      int             `yaml:"port"`
	Host      string          `yaml:"host"`
	TLS       TLSConfig       `yaml:"tls"`
	Auth      AuthConfig      `yaml:"auth"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	CORS      CORSConfig      `yaml:"cors"`
}

type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

type AuthConfig struct {
	Enabled  bool   `yaml:"enabled"`
	JWKURL   string `yaml:"jwk_url"`
	Audience string `yaml:"audience"`
	TokenTTL string `yaml:"token_ttl"`
}

type RateLimitConfig struct {
	RequestsPerSecond int `yaml:"requests_per_second"`
	RequestsPerMinute int `yaml:"requests_per_minute"`
	BurstSize         int `yaml:"burst_size"`
}

type CORSConfig struct {
	Enabled        bool     `yaml:"enabled"`
	AllowedOrigins []string `yaml:"allowed_origins"`
}

type LoggingConfig struct {
	Level  string        `yaml:"level"`
	Format string        `yaml:"format"`
	Output string        `yaml:"output"`
	File   FileLogConfig `yaml:"file"`
}

type FileLogConfig struct {
	Path       string `yaml:"path"`
	MaxSize    string `yaml:"max_size"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAge     string `yaml:"max_age"`
}

type MetricsConfig struct {
	Enabled      bool   `yaml:"enabled"`
	Port         int    `yaml:"port"`
	Path         string `yaml:"path"`
	PushEnabled  bool   `yaml:"push_enabled"`
	PushGateway  string `yaml:"push_gateway"`
	PushInterval string `yaml:"push_interval"`
}

type HealthConfig struct {
	Enabled       bool   `yaml:"enabled"`
	Port          int    `yaml:"port"`
	Path          string `yaml:"path"`
	ReadinessPath string `yaml:"readiness_path"`
	LivenessPath  string `yaml:"liveness_path"`
}

type TracingConfig struct {
	Enabled bool         `yaml:"enabled"`
	Jaeger  JaegerConfig `yaml:"jaeger"`
}

type JaegerConfig struct {
	Endpoint string        `yaml:"endpoint"`
	Sampler  SamplerConfig `yaml:"sampler"`
}

type SamplerConfig struct {
	Type  string  `yaml:"type"`
	Param float64 `yaml:"param"`
}

type FeatureFlagsConfig struct {
	AttackPathAnalysis            bool `yaml:"attack_path_analysis"`
	RealTimeCorrelation           bool `yaml:"real_time_correlation"`
	AutomatedRemediation          bool `yaml:"automated_remediation"`
	ThreatIntelligenceIntegration bool `yaml:"threat_intelligence_integration"`
}

// Load reads and parses the configuration file
func Load(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	// Validate and expand environment variables
	expandEnv(cfg)

	return cfg, nil
}

// expandEnv replaces ${VAR} placeholders with environment variables
func expandEnv(cfg *Config) {
	cfg.Kafka.Security.SASLPassword = os.ExpandEnv(cfg.Kafka.Security.SASLPassword)
	cfg.Neo4j.Password = os.ExpandEnv(cfg.Neo4j.Password)
}

// GetDuration parses a duration string
func GetDuration(s string) (time.Duration, error) {
	return time.ParseDuration(s)
}
