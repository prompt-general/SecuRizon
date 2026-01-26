package config

import (
	"log"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the overall application configuration
type Config struct {
	Graph   GraphConfig   `yaml:"graph"`
	Events  EventsConfig  `yaml:"events"`
	Risk    RiskConfig    `yaml:"risk"`
	API     APIConfig     `yaml:"api"`
	Logging LoggingConfig `yaml:"logging"`
	Metrics MetricsConfig `yaml:"metrics"`
	Tracing TracingConfig `yaml:"tracing"`
	Kafka   KafkaConfig   `yaml:"kafka"`
}

// GraphConfig represents Neo4j database configuration
type GraphConfig struct {
	URI            string        `yaml:"uri"`
	Database       string        `yaml:"database"`
	Username       string        `yaml:"username"`
	Password       string        `yaml:"password"`
	MaxPoolSize    int           `yaml:"max_pool_size"`
	MaxIdleConns   int           `yaml:"max_idle_conns"`
	ConnTimeout    time.Duration `yaml:"conn_timeout"`
	ReadTimeout    time.Duration `yaml:"read_timeout"`
	WriteTimeout   time.Duration `yaml:"write_timeout"`
}

// EventsConfig represents Kafka event bus configuration
type EventsConfig struct {
	Brokers              []string      `yaml:"brokers"`
	ClientID             string        `yaml:"client_id"`
	ConsumerGroup        string        `yaml:"consumer_group"`
	BatchSize            int           `yaml:"batch_size"`
	BatchTimeout         time.Duration `yaml:"batch_timeout"`
	CommitInterval       time.Duration `yaml:"commit_interval"`
	HeartbeatInterval    time.Duration `yaml:"heartbeat_interval"`
	SessionTimeout       time.Duration `yaml:"session_timeout"`
	RebalanceTimeout     time.Duration `yaml:"rebalance_timeout"`
	StartOffset          int64         `yaml:"start_offset"`
	MinBytes             int           `yaml:"min_bytes"`
	MaxBytes             int           `yaml:"max_bytes"`
	MaxWait              time.Duration `yaml:"max_wait"`
	CompressionType      string        `yaml:"compression_type"`
	SecurityProtocol     string        `yaml:"security_protocol"`
}

// RiskConfig represents risk engine configuration
type RiskConfig struct {
	BaseSeverityWeight  float64       `yaml:"base_severity_weight"`
	ExposureWeight      float64       `yaml:"exposure_weight"`
	EnvironmentWeight   float64       `yaml:"environment_weight"`
	ThreatIntelWeight   float64       `yaml:"threat_intel_weight"`
	CriticalThreshold   float64       `yaml:"critical_threshold"`
	HighThreshold       float64       `yaml:"high_threshold"`
	MediumThreshold     float64       `yaml:"medium_threshold"`
	CacheEnabled        bool          `yaml:"cache_enabled"`
	CacheTTL            time.Duration `yaml:"cache_ttl"`
	CacheSize           int           `yaml:"cache_size"`
	EnablePropagation   bool          `yaml:"enable_propagation"`
	PropagationDepth    int           `yaml:"propagation_depth"`
	DecayFactor         float64       `yaml:"decay_factor"`
	BatchSize           int           `yaml:"batch_size"`
	CalculationTimeout  time.Duration `yaml:"calculation_timeout"`
	EnableMetrics       bool          `yaml:"enable_metrics"`
	MetricsInterval     time.Duration `yaml:"metrics_interval"`
}

// APIConfig represents API gateway configuration
type APIConfig struct {
	Host              string        `yaml:"host"`
	Port              int           `yaml:"port"`
	ReadTimeout       time.Duration `yaml:"read_timeout"`
	WriteTimeout      time.Duration `yaml:"write_timeout"`
	IdleTimeout       time.Duration `yaml:"idle_timeout"`
	EnableCORS        bool          `yaml:"enable_cors"`
	AllowedOrigins    []string      `yaml:"allowed_origins"`
	AllowedMethods    []string      `yaml:"allowed_methods"`
	AllowedHeaders    []string      `yaml:"allowed_headers"`
	EnableAuth        bool          `yaml:"enable_auth"`
	AuthType          string        `yaml:"auth_type"`
	JWTSecret         string        `yaml:"jwt_secret"`
	EnableMetrics     bool          `yaml:"enable_metrics"`
	EnablePprof       bool          `yaml:"enable_pprof"`
	EnableSwagger     bool          `yaml:"enable_swagger"`
	RateLimitEnabled  bool          `yaml:"rate_limit_enabled"`
	RateLimitRPS      int           `yaml:"rate_limit_rps"`
	RequestTimeout    time.Duration `yaml:"request_timeout"`
	MaxRequestSize    int64         `yaml:"max_request_size"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level       string `yaml:"level"`
	Format      string `yaml:"format"`
	Output      string `yaml:"output"`
	File        string `yaml:"file"`
	MaxSize     int    `yaml:"max_size"`
	MaxBackups  int    `yaml:"max_backups"`
	MaxAge      int    `yaml:"max_age"`
	Compress    bool   `yaml:"compress"`
}

// MetricsConfig represents metrics configuration
type MetricsConfig struct {
	Enabled    bool          `yaml:"enabled"`
	Interval   time.Duration `yaml:"interval"`
	Endpoint   string        `yaml:"endpoint"`
	Prometheus PrometheusConfig `yaml:"prometheus"`
}

// PrometheusConfig represents Prometheus configuration
type PrometheusConfig struct {
	Enabled bool `yaml:"enabled"`
	Port    int  `yaml:"port"`
}

// TracingConfig represents tracing configuration
type TracingConfig struct {
	Enabled bool         `yaml:"enabled"`
	Jaeger  JaegerConfig `yaml:"jaeger"`
}

// JaegerConfig represents Jaeger tracing configuration
type JaegerConfig struct {
	Endpoint    string `yaml:"endpoint"`
	ServiceName string `yaml:"service_name"`
}

// KafkaConfig represents Kafka producer configuration
type KafkaConfig struct {
	Brokers []string      `yaml:"brokers"`
	Topic   string        `yaml:"topic"`
	Timeout time.Duration `yaml:"timeout"`
}

// Load loads configuration from file
func Load() *Config {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config/config.yaml"
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatalf("Failed to read config file: %v", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		log.Fatalf("Failed to parse config file: %v", err)
	}

	// Set defaults for Kafka if not specified
	if len(cfg.Kafka.Brokers) == 0 {
		cfg.Kafka.Brokers = cfg.Events.Brokers
	}
	if cfg.Kafka.Topic == "" {
		cfg.Kafka.Topic = "securizon-events"
	}
	if cfg.Kafka.Timeout == 0 {
		cfg.Kafka.Timeout = 10 * time.Second
	}

	return cfg
}
