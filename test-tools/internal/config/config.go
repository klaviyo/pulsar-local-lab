package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Compression type constants
const (
	CompressionNone   = "NONE"
	CompressionLZ4    = "LZ4"
	CompressionZLIB   = "ZLIB"
	CompressionZSTD   = "ZSTD"
	CompressionSNAPPY = "SNAPPY"
)

// Subscription type constants
const (
	SubscriptionExclusive = "Exclusive"
	SubscriptionShared    = "Shared"
	SubscriptionFailover  = "Failover"
	SubscriptionKeyShared = "KeyShared"
)

// Config represents the main configuration for performance testing.
//
// Example JSON configuration:
//
//	{
//	  "pulsar": {
//	    "service_url": "pulsar://localhost:6650",
//	    "admin_url": "http://localhost:8080",
//	    "topic": "persistent://public/default/perf-test"
//	  },
//	  "producer": {
//	    "num_producers": 5,
//	    "message_size": 1024,
//	    "batching_enabled": true,
//	    "batching_max_size": 1000,
//	    "compression_type": "LZ4",
//	    "send_timeout": "30s",
//	    "max_pending_messages": 1000
//	  },
//	  "consumer": {
//	    "num_consumers": 5,
//	    "subscription_name": "perf-test-sub",
//	    "subscription_type": "Shared",
//	    "receiver_queue_size": 1000,
//	    "ack_timeout": "30s"
//	  },
//	  "performance": {
//	    "target_throughput": 10000,
//	    "duration": "5m",
//	    "warmup": "5s",
//	    "rate_limit_enabled": true
//	  },
//	  "metrics": {
//	    "collection_interval": "1s",
//	    "histogram_buckets": [1, 5, 10, 25, 50, 100, 250, 500, 1000],
//	    "export_enabled": true,
//	    "export_path": "./metrics"
//	  }
//	}
type Config struct {
	// Pulsar connection settings
	Pulsar PulsarConfig `json:"pulsar"`

	// Producer settings
	Producer ProducerConfig `json:"producer"`

	// Consumer settings
	Consumer ConsumerConfig `json:"consumer"`

	// Performance settings
	Performance PerformanceConfig `json:"performance"`

	// Metrics settings
	Metrics MetricsConfig `json:"metrics"`
}

// PulsarConfig contains Pulsar connection parameters.
type PulsarConfig struct {
	// ServiceURL is the Pulsar broker service URL (e.g., pulsar://localhost:6650)
	ServiceURL string `json:"service_url"`

	// AdminURL is the Pulsar admin API URL (e.g., http://localhost:8080)
	AdminURL string `json:"admin_url"`

	// Topic is the Pulsar topic name (e.g., persistent://public/default/perf-test)
	Topic string `json:"topic"`
}

// ProducerConfig contains producer-specific settings.
type ProducerConfig struct {
	// NumProducers is the number of concurrent producer workers
	NumProducers int `json:"num_producers"`

	// MessageSize is the size of each message in bytes
	MessageSize int `json:"message_size"`

	// BatchingEnabled enables message batching for better throughput
	BatchingEnabled bool `json:"batching_enabled"`

	// BatchingMaxSize is the maximum number of messages in a batch
	BatchingMaxSize int `json:"batching_max_size"`

	// CompressionType specifies the compression algorithm (NONE, LZ4, ZLIB, ZSTD, SNAPPY)
	CompressionType string `json:"compression_type"`

	// SendTimeout is the timeout for send operations
	SendTimeout time.Duration `json:"send_timeout"`

	// MaxPendingMsg is the maximum number of pending messages
	MaxPendingMsg int `json:"max_pending_messages"`
}

// ConsumerConfig contains consumer-specific settings.
type ConsumerConfig struct {
	// NumConsumers is the number of concurrent consumer workers
	NumConsumers int `json:"num_consumers"`

	// SubscriptionName is the name of the subscription
	SubscriptionName string `json:"subscription_name"`

	// SubscriptionType specifies the subscription type (Exclusive, Shared, Failover, KeyShared)
	SubscriptionType string `json:"subscription_type"`

	// ReceiverQueueSize is the size of the consumer receive queue
	ReceiverQueueSize int `json:"receiver_queue_size"`

	// AckTimeout is the timeout for acknowledgment operations
	AckTimeout time.Duration `json:"ack_timeout"`
}

// PerformanceConfig contains performance tuning parameters.
type PerformanceConfig struct {
	// TargetThroughput is the target messages per second (0 = unlimited)
	TargetThroughput int `json:"target_throughput"`

	// Duration is the test duration (0 = unlimited)
	Duration time.Duration `json:"duration"`

	// Warmup is the warmup period before measurements begin
	Warmup time.Duration `json:"warmup"`

	// RateLimitEnabled enables rate limiting to achieve target throughput
	RateLimitEnabled bool `json:"rate_limit_enabled"`
}

// MetricsConfig contains metrics collection settings.
type MetricsConfig struct {
	// CollectionInterval is the interval for collecting metrics snapshots
	CollectionInterval time.Duration `json:"collection_interval"`

	// HistogramBuckets defines the latency histogram bucket boundaries in milliseconds
	HistogramBuckets []float64 `json:"histogram_buckets"`

	// ExportEnabled enables exporting metrics to files
	ExportEnabled bool `json:"export_enabled"`

	// ExportPath is the directory path for exported metrics
	ExportPath string `json:"export_path"`
}

// LoadConfig loads configuration from a file or returns defaults.
// If path is empty, returns the default configuration with the specified profile applied.
// If a file path is provided, loads the configuration from the file and validates it.
func LoadConfig(path string, profile string) (*Config, error) {
	if path == "" {
		return DefaultConfig(profile), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate the loaded configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// LoadConfigFromEnv loads configuration from environment variables.
// Environment variables take precedence over default values.
// Supported environment variables:
//   - PULSAR_SERVICE_URL: Pulsar broker service URL
//   - PULSAR_ADMIN_URL: Pulsar admin API URL
//   - PULSAR_TOPIC: Pulsar topic name
//   - PRODUCER_NUM_WORKERS: Number of producer workers
//   - PRODUCER_MESSAGE_SIZE: Message size in bytes
//   - PRODUCER_TARGET_RATE: Target message rate per second
//   - PRODUCER_BATCH_SIZE: Batch size for producers
//   - PRODUCER_COMPRESSION: Compression type (NONE, LZ4, ZLIB, ZSTD, SNAPPY)
//   - CONSUMER_NUM_WORKERS: Number of consumer workers
//   - CONSUMER_SUBSCRIPTION: Consumer subscription name
//   - CONSUMER_SUBSCRIPTION_TYPE: Subscription type (Exclusive, Shared, Failover, KeyShared)
//   - METRICS_UPDATE_INTERVAL: Metrics collection interval (e.g., "1s", "100ms")
//   - METRICS_ENABLE_EXPORT: Enable metrics export (true/false)
//   - METRICS_EXPORT_PATH: Path for exported metrics
func LoadConfigFromEnv() (*Config, error) {
	cfg := DefaultConfig("")

	// Pulsar configuration
	if v := os.Getenv("PULSAR_SERVICE_URL"); v != "" {
		cfg.Pulsar.ServiceURL = v
	}
	if v := os.Getenv("PULSAR_ADMIN_URL"); v != "" {
		cfg.Pulsar.AdminURL = v
	}
	if v := os.Getenv("PULSAR_TOPIC"); v != "" {
		cfg.Pulsar.Topic = v
	}

	// Producer configuration
	if v := os.Getenv("PRODUCER_NUM_WORKERS"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			cfg.Producer.NumProducers = val
		}
	}
	if v := os.Getenv("PRODUCER_MESSAGE_SIZE"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			cfg.Producer.MessageSize = val
		}
	}
	if v := os.Getenv("PRODUCER_TARGET_RATE"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			cfg.Performance.TargetThroughput = val
		}
	}
	if v := os.Getenv("PRODUCER_BATCH_SIZE"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			cfg.Producer.BatchingMaxSize = val
		}
	}
	if v := os.Getenv("PRODUCER_COMPRESSION"); v != "" {
		cfg.Producer.CompressionType = strings.ToUpper(v)
	}

	// Consumer configuration
	if v := os.Getenv("CONSUMER_NUM_WORKERS"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			cfg.Consumer.NumConsumers = val
		}
	}
	if v := os.Getenv("CONSUMER_SUBSCRIPTION"); v != "" {
		cfg.Consumer.SubscriptionName = v
	}
	if v := os.Getenv("CONSUMER_SUBSCRIPTION_TYPE"); v != "" {
		cfg.Consumer.SubscriptionType = v
	}

	// Metrics configuration
	if v := os.Getenv("METRICS_UPDATE_INTERVAL"); v != "" {
		if val, err := time.ParseDuration(v); err == nil {
			cfg.Metrics.CollectionInterval = val
		}
	}
	if v := os.Getenv("METRICS_ENABLE_EXPORT"); v != "" {
		if val, err := strconv.ParseBool(v); err == nil {
			cfg.Metrics.ExportEnabled = val
		}
	}
	if v := os.Getenv("METRICS_EXPORT_PATH"); v != "" {
		cfg.Metrics.ExportPath = v
	}

	// Validate the configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration from environment: %w", err)
	}

	return cfg, nil
}

// DefaultConfig returns a default configuration with the specified profile applied.
// If profile is empty or "default", returns base defaults without profile modifications.
func DefaultConfig(profile string) *Config {
	// Base defaults
	cfg := &Config{
		Pulsar: PulsarConfig{
			ServiceURL: "pulsar://localhost:6650",
			AdminURL:   "http://localhost:8080",
			Topic:      "persistent://public/default/perf-test",
		},
		Producer: ProducerConfig{
			NumProducers:    1,
			MessageSize:     1024,
			BatchingEnabled: true,
			BatchingMaxSize: 1000,
			CompressionType: CompressionLZ4,
			SendTimeout:     30 * time.Second,
			MaxPendingMsg:   1000,
		},
		Consumer: ConsumerConfig{
			NumConsumers:      1,
			SubscriptionName:  "perf-test-sub",
			SubscriptionType:  SubscriptionShared,
			ReceiverQueueSize: 1000,
			AckTimeout:        30 * time.Second,
		},
		Performance: PerformanceConfig{
			TargetThroughput: 0, // unlimited
			Duration:         0, // unlimited
			Warmup:           5 * time.Second,
			RateLimitEnabled: false,
		},
		Metrics: MetricsConfig{
			CollectionInterval: 1 * time.Second,
			HistogramBuckets:   []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000},
			ExportEnabled:      false,
			ExportPath:         "./metrics",
		},
	}

	// Apply profile-specific settings
	if profile != "" && profile != "default" {
		ApplyProfile(cfg, profile)
	}

	return cfg
}

// Validate validates the configuration and returns an error if any values are invalid.
// This ensures that the configuration is safe to use before starting the performance test.
func (c *Config) Validate() error {
	// Validate Pulsar configuration
	if c.Pulsar.ServiceURL == "" {
		return fmt.Errorf("pulsar service URL is required")
	}
	if c.Pulsar.Topic == "" {
		return fmt.Errorf("pulsar topic is required")
	}

	// Validate producer configuration
	if c.Producer.NumProducers < 0 {
		return fmt.Errorf("number of producers must be non-negative, got %d", c.Producer.NumProducers)
	}
	if c.Producer.MessageSize <= 0 {
		return fmt.Errorf("message size must be positive, got %d", c.Producer.MessageSize)
	}
	if c.Producer.BatchingMaxSize < 0 {
		return fmt.Errorf("batching max size must be non-negative, got %d", c.Producer.BatchingMaxSize)
	}
	if c.Producer.MaxPendingMsg < 0 {
		return fmt.Errorf("max pending messages must be non-negative, got %d", c.Producer.MaxPendingMsg)
	}
	if c.Producer.SendTimeout < 0 {
		return fmt.Errorf("send timeout must be non-negative, got %v", c.Producer.SendTimeout)
	}

	// Validate compression type
	validCompressionTypes := map[string]bool{
		CompressionNone:   true,
		CompressionLZ4:    true,
		CompressionZLIB:   true,
		CompressionZSTD:   true,
		CompressionSNAPPY: true,
	}
	if !validCompressionTypes[c.Producer.CompressionType] {
		return fmt.Errorf("invalid compression type: %s (must be one of: NONE, LZ4, ZLIB, ZSTD, SNAPPY)", c.Producer.CompressionType)
	}

	// Validate consumer configuration
	if c.Consumer.NumConsumers < 0 {
		return fmt.Errorf("number of consumers must be non-negative, got %d", c.Consumer.NumConsumers)
	}
	if c.Consumer.SubscriptionName == "" && c.Consumer.NumConsumers > 0 {
		return fmt.Errorf("consumer subscription name is required when consumers are enabled")
	}
	if c.Consumer.ReceiverQueueSize < 0 {
		return fmt.Errorf("receiver queue size must be non-negative, got %d", c.Consumer.ReceiverQueueSize)
	}
	if c.Consumer.AckTimeout < 0 {
		return fmt.Errorf("ack timeout must be non-negative, got %v", c.Consumer.AckTimeout)
	}

	// Validate subscription type
	validSubscriptionTypes := map[string]bool{
		SubscriptionExclusive: true,
		SubscriptionShared:    true,
		SubscriptionFailover:  true,
		SubscriptionKeyShared: true,
	}
	if c.Consumer.SubscriptionType != "" && !validSubscriptionTypes[c.Consumer.SubscriptionType] {
		return fmt.Errorf("invalid subscription type: %s (must be one of: Exclusive, Shared, Failover, KeyShared)", c.Consumer.SubscriptionType)
	}

	// Validate performance configuration
	if c.Performance.TargetThroughput < 0 {
		return fmt.Errorf("target throughput must be non-negative, got %d", c.Performance.TargetThroughput)
	}
	if c.Performance.Duration < 0 {
		return fmt.Errorf("duration must be non-negative, got %v", c.Performance.Duration)
	}
	if c.Performance.Warmup < 0 {
		return fmt.Errorf("warmup period must be non-negative, got %v", c.Performance.Warmup)
	}

	// Validate metrics configuration
	if c.Metrics.CollectionInterval <= 0 {
		return fmt.Errorf("metrics collection interval must be positive, got %v", c.Metrics.CollectionInterval)
	}
	if c.Metrics.ExportEnabled && c.Metrics.ExportPath == "" {
		return fmt.Errorf("metrics export path is required when export is enabled")
	}

	return nil
}

// Save saves the configuration to a JSON file at the specified path.
// The file is created with 0644 permissions and formatted with indentation for readability.
func (c *Config) Save(path string) error {
	// Validate before saving
	if err := c.Validate(); err != nil {
		return fmt.Errorf("cannot save invalid configuration: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
