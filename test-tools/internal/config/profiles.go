package config

import (
	"fmt"
	"time"
)

// GetProfile returns a configuration for the specified profile name.
// Available profiles: default, low-latency, high-throughput, burst, sustained.
// Returns an error if the profile name is not recognized.
func GetProfile(name string) (*Config, error) {
	switch name {
	case "default", "":
		return DefaultProfile(), nil
	case "low-latency":
		return LowLatencyProfile(), nil
	case "high-throughput":
		return HighThroughputProfile(), nil
	case "burst":
		return BurstProfile(), nil
	case "sustained":
		return SustainedProfile(), nil
	default:
		return nil, fmt.Errorf("unknown profile: %s (available profiles: %v)", name, ListProfiles())
	}
}

// ListProfiles returns a list of all available profile names.
func ListProfiles() []string {
	return GetAvailableProfiles()
}

// ApplyProfile applies a predefined performance profile to the configuration.
// This modifies the configuration in place. If the profile name is not recognized,
// the configuration remains unchanged.
func ApplyProfile(cfg *Config, profile string) {
	switch profile {
	case "low-latency":
		applyLowLatencyProfile(cfg)
	case "high-throughput":
		applyHighThroughputProfile(cfg)
	case "burst":
		applyBurstProfile(cfg)
	case "sustained":
		applySustainedProfile(cfg)
	default:
		// Keep default/current config
	}
}

// DefaultProfile returns a balanced configuration suitable for general testing.
// This is equivalent to calling DefaultConfig("") or DefaultConfig("default").
func DefaultProfile() *Config {
	return DefaultConfig("")
}

// LowLatencyProfile returns a configuration optimized for minimal message latency.
// Characteristics:
//   - Disabled batching for immediate sends
//   - No compression to reduce CPU overhead
//   - Small queue sizes to minimize queueing delays
//   - Single producer/consumer for simplicity
//   - Rate limited to 1000 msg/s for controlled load
//   - Fine-grained metrics collection (100ms intervals)
func LowLatencyProfile() *Config {
	cfg := DefaultConfig("")
	applyLowLatencyProfile(cfg)
	return cfg
}

// HighThroughputProfile returns a configuration optimized for maximum message throughput.
// Characteristics:
//   - Large batches (10000 messages) for efficiency
//   - LZ4 compression for bandwidth optimization
//   - Multiple workers (10) for parallelism
//   - Large queue sizes (10000) for buffering
//   - No rate limiting for maximum speed
//   - Shared subscription for parallel consumption
func HighThroughputProfile() *Config {
	cfg := DefaultConfig("")
	applyHighThroughputProfile(cfg)
	return cfg
}

// BurstProfile returns a configuration optimized for bursty traffic patterns.
// Characteristics:
//   - Medium batch sizes (5000) for balance
//   - ZSTD compression for better compression ratio
//   - Multiple workers (5) for parallelism
//   - Rate limited to 10000 msg/s with bursts
//   - Time-limited test (5 minutes)
//   - Medium-frequency metrics (500ms intervals)
func BurstProfile() *Config {
	cfg := DefaultConfig("")
	applyBurstProfile(cfg)
	return cfg
}

// SustainedProfile returns a configuration for long-running sustained load tests.
// Characteristics:
//   - Balanced settings for stability
//   - LZ4 compression for efficiency
//   - Multiple workers (5) for parallelism
//   - Rate limited to 5000 msg/s for sustainability
//   - Unlimited duration for long-running tests
//   - Metrics export enabled for analysis
func SustainedProfile() *Config {
	cfg := DefaultConfig("")
	applySustainedProfile(cfg)
	return cfg
}

// applyLowLatencyProfile optimizes for minimal latency.
func applyLowLatencyProfile(cfg *Config) {
	// Producer settings for low latency
	cfg.Producer.NumProducers = 1
	cfg.Producer.MessageSize = 512
	cfg.Producer.BatchingEnabled = false // Disable batching for lowest latency
	cfg.Producer.CompressionType = CompressionNone
	cfg.Producer.MaxPendingMsg = 100

	// Consumer settings
	cfg.Consumer.NumConsumers = 1
	cfg.Consumer.ReceiverQueueSize = 10

	// Performance settings
	cfg.Performance.TargetThroughput = 1000 // messages per second
	cfg.Performance.RateLimitEnabled = true

	// Metrics
	cfg.Metrics.CollectionInterval = 100 * time.Millisecond
	cfg.Metrics.HistogramBuckets = []float64{0.1, 0.5, 1, 2, 5, 10, 25, 50, 100}
}

// applyHighThroughputProfile optimizes for maximum throughput.
func applyHighThroughputProfile(cfg *Config) {
	// Producer settings for high throughput
	cfg.Producer.NumProducers = 10
	cfg.Producer.MessageSize = 4096
	cfg.Producer.BatchingEnabled = true
	cfg.Producer.BatchingMaxSize = 10000
	cfg.Producer.CompressionType = CompressionLZ4
	cfg.Producer.MaxPendingMsg = 10000

	// Consumer settings
	cfg.Consumer.NumConsumers = 10
	cfg.Consumer.ReceiverQueueSize = 10000
	cfg.Consumer.SubscriptionType = SubscriptionShared // For parallel consumption

	// Performance settings
	cfg.Performance.TargetThroughput = 0 // unlimited
	cfg.Performance.RateLimitEnabled = false

	// Metrics
	cfg.Metrics.CollectionInterval = 1 * time.Second
}

// applyBurstProfile simulates bursty traffic patterns.
func applyBurstProfile(cfg *Config) {
	// Producer settings for burst mode
	cfg.Producer.NumProducers = 5
	cfg.Producer.MessageSize = 2048
	cfg.Producer.BatchingEnabled = true
	cfg.Producer.BatchingMaxSize = 5000
	cfg.Producer.CompressionType = CompressionZSTD
	cfg.Producer.MaxPendingMsg = 5000

	// Consumer settings
	cfg.Consumer.NumConsumers = 5
	cfg.Consumer.ReceiverQueueSize = 5000

	// Performance settings
	cfg.Performance.TargetThroughput = 10000
	cfg.Performance.RateLimitEnabled = true
	cfg.Performance.Duration = 5 * time.Minute

	// Metrics
	cfg.Metrics.CollectionInterval = 500 * time.Millisecond
}

// applySustainedProfile for long-running sustained load.
func applySustainedProfile(cfg *Config) {
	// Producer settings for sustained load
	cfg.Producer.NumProducers = 5
	cfg.Producer.MessageSize = 1024
	cfg.Producer.BatchingEnabled = true
	cfg.Producer.BatchingMaxSize = 1000
	cfg.Producer.CompressionType = CompressionLZ4
	cfg.Producer.MaxPendingMsg = 2000

	// Consumer settings
	cfg.Consumer.NumConsumers = 5
	cfg.Consumer.ReceiverQueueSize = 2000

	// Performance settings
	cfg.Performance.TargetThroughput = 5000
	cfg.Performance.RateLimitEnabled = true
	cfg.Performance.Duration = 0 // unlimited

	// Metrics
	cfg.Metrics.CollectionInterval = 1 * time.Second
	cfg.Metrics.ExportEnabled = true
}

// GetAvailableProfiles returns a list of available profile names
func GetAvailableProfiles() []string {
	return []string{
		"default",
		"low-latency",
		"high-throughput",
		"burst",
		"sustained",
	}
}

// GetProfileDescription returns a description for a profile
func GetProfileDescription(profile string) string {
	descriptions := map[string]string{
		"default":         "Balanced configuration suitable for general testing",
		"low-latency":     "Optimized for minimal message latency (disabled batching, small queues)",
		"high-throughput": "Optimized for maximum message throughput (large batches, many workers)",
		"burst":           "Simulates bursty traffic with rate limiting",
		"sustained":       "Long-running sustained load with metrics export enabled",
	}
	return descriptions[profile]
}
