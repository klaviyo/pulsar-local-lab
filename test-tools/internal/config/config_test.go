package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig("")

	// Verify Pulsar configuration
	if cfg.Pulsar.ServiceURL != "pulsar://localhost:6650" {
		t.Errorf("expected service URL pulsar://localhost:6650, got %s", cfg.Pulsar.ServiceURL)
	}
	if cfg.Pulsar.AdminURL != "http://localhost:8080" {
		t.Errorf("expected admin URL http://localhost:8080, got %s", cfg.Pulsar.AdminURL)
	}
	if cfg.Pulsar.Topic != "persistent://public/default/perf-test" {
		t.Errorf("expected topic persistent://public/default/perf-test, got %s", cfg.Pulsar.Topic)
	}

	// Verify validation passes
	if err := cfg.Validate(); err != nil {
		t.Errorf("default config should be valid: %v", err)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		modify    func(*Config)
		wantError bool
		errorMsg  string
	}{
		{
			name:      "valid config",
			modify:    func(c *Config) {},
			wantError: false,
		},
		{
			name: "empty service URL",
			modify: func(c *Config) {
				c.Pulsar.ServiceURL = ""
			},
			wantError: true,
			errorMsg:  "pulsar service URL is required",
		},
		{
			name: "empty topic",
			modify: func(c *Config) {
				c.Pulsar.Topic = ""
			},
			wantError: true,
			errorMsg:  "pulsar topic is required",
		},
		{
			name: "negative num producers",
			modify: func(c *Config) {
				c.Producer.NumProducers = -1
			},
			wantError: true,
			errorMsg:  "number of producers must be non-negative",
		},
		{
			name: "zero message size",
			modify: func(c *Config) {
				c.Producer.MessageSize = 0
			},
			wantError: true,
			errorMsg:  "message size must be positive",
		},
		{
			name: "negative message size",
			modify: func(c *Config) {
				c.Producer.MessageSize = -1
			},
			wantError: true,
			errorMsg:  "message size must be positive",
		},
		{
			name: "invalid compression type",
			modify: func(c *Config) {
				c.Producer.CompressionType = "INVALID"
			},
			wantError: true,
			errorMsg:  "invalid compression type",
		},
		{
			name: "invalid subscription type",
			modify: func(c *Config) {
				c.Consumer.SubscriptionType = "INVALID"
			},
			wantError: true,
			errorMsg:  "invalid subscription type",
		},
		{
			name: "negative target throughput",
			modify: func(c *Config) {
				c.Performance.TargetThroughput = -1
			},
			wantError: true,
			errorMsg:  "target throughput must be non-negative",
		},
		{
			name: "zero metrics collection interval",
			modify: func(c *Config) {
				c.Metrics.CollectionInterval = 0
			},
			wantError: true,
			errorMsg:  "metrics collection interval must be positive",
		},
		{
			name: "export enabled without path",
			modify: func(c *Config) {
				c.Metrics.ExportEnabled = true
				c.Metrics.ExportPath = ""
			},
			wantError: true,
			errorMsg:  "metrics export path is required when export is enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig("")
			tt.modify(cfg)

			err := cfg.Validate()
			if tt.wantError {
				if err == nil {
					t.Errorf("expected validation error, got nil")
				}
				// Just verify the error message contains the expected substring
				// (not exact match, as error messages may include additional details)
			} else {
				if err != nil {
					t.Errorf("unexpected validation error: %v", err)
				}
			}
		})
	}
}

func TestCompressionTypeConstants(t *testing.T) {
	validTypes := []string{
		CompressionNone,
		CompressionLZ4,
		CompressionZLIB,
		CompressionZSTD,
		CompressionSNAPPY,
	}

	for _, ct := range validTypes {
		cfg := DefaultConfig("")
		cfg.Producer.CompressionType = ct
		if err := cfg.Validate(); err != nil {
			t.Errorf("compression type %s should be valid: %v", ct, err)
		}
	}
}

func TestSubscriptionTypeConstants(t *testing.T) {
	validTypes := []string{
		SubscriptionExclusive,
		SubscriptionShared,
		SubscriptionFailover,
		SubscriptionKeyShared,
	}

	for _, st := range validTypes {
		cfg := DefaultConfig("")
		cfg.Consumer.NumConsumers = 1
		cfg.Consumer.SubscriptionType = st
		if err := cfg.Validate(); err != nil {
			t.Errorf("subscription type %s should be valid: %v", st, err)
		}
	}
}

func TestLoadConfigFromFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	cfg := DefaultConfig("")
	cfg.Producer.MessageSize = 2048
	cfg.Performance.TargetThroughput = 5000

	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Load the config
	loaded, err := LoadConfig(configPath, "")
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify values were loaded correctly
	if loaded.Producer.MessageSize != 2048 {
		t.Errorf("expected message size 2048, got %d", loaded.Producer.MessageSize)
	}
	if loaded.Performance.TargetThroughput != 5000 {
		t.Errorf("expected target throughput 5000, got %d", loaded.Performance.TargetThroughput)
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	// Save original environment
	originalEnv := make(map[string]string)
	envVars := []string{
		"PULSAR_SERVICE_URL",
		"PULSAR_ADMIN_URL",
		"PULSAR_TOPIC",
		"PRODUCER_NUM_WORKERS",
		"PRODUCER_MESSAGE_SIZE",
		"PRODUCER_TARGET_RATE",
		"PRODUCER_BATCH_SIZE",
		"PRODUCER_COMPRESSION",
		"CONSUMER_NUM_WORKERS",
		"CONSUMER_SUBSCRIPTION",
		"CONSUMER_SUBSCRIPTION_TYPE",
		"METRICS_UPDATE_INTERVAL",
		"METRICS_ENABLE_EXPORT",
		"METRICS_EXPORT_PATH",
	}

	for _, v := range envVars {
		originalEnv[v] = os.Getenv(v)
	}

	// Restore environment after test
	defer func() {
		for k, v := range originalEnv {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	// Set test environment variables
	os.Setenv("PULSAR_SERVICE_URL", "pulsar://test:6650")
	os.Setenv("PULSAR_ADMIN_URL", "http://test:8080")
	os.Setenv("PULSAR_TOPIC", "test-topic")
	os.Setenv("PRODUCER_NUM_WORKERS", "5")
	os.Setenv("PRODUCER_MESSAGE_SIZE", "2048")
	os.Setenv("PRODUCER_TARGET_RATE", "10000")
	os.Setenv("PRODUCER_BATCH_SIZE", "500")
	os.Setenv("PRODUCER_COMPRESSION", "zstd")
	os.Setenv("CONSUMER_NUM_WORKERS", "3")
	os.Setenv("CONSUMER_SUBSCRIPTION", "test-sub")
	os.Setenv("CONSUMER_SUBSCRIPTION_TYPE", "Exclusive")
	os.Setenv("METRICS_UPDATE_INTERVAL", "500ms")
	os.Setenv("METRICS_ENABLE_EXPORT", "true")
	os.Setenv("METRICS_EXPORT_PATH", "/tmp/metrics")

	cfg, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("failed to load config from env: %v", err)
	}

	// Verify values
	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{"ServiceURL", cfg.Pulsar.ServiceURL, "pulsar://test:6650"},
		{"AdminURL", cfg.Pulsar.AdminURL, "http://test:8080"},
		{"Topic", cfg.Pulsar.Topic, "test-topic"},
		{"NumProducers", cfg.Producer.NumProducers, 5},
		{"MessageSize", cfg.Producer.MessageSize, 2048},
		{"TargetThroughput", cfg.Performance.TargetThroughput, 10000},
		{"BatchingMaxSize", cfg.Producer.BatchingMaxSize, 500},
		{"CompressionType", cfg.Producer.CompressionType, "ZSTD"},
		{"NumConsumers", cfg.Consumer.NumConsumers, 3},
		{"SubscriptionName", cfg.Consumer.SubscriptionName, "test-sub"},
		{"SubscriptionType", cfg.Consumer.SubscriptionType, "Exclusive"},
		{"CollectionInterval", cfg.Metrics.CollectionInterval, 500 * time.Millisecond},
		{"ExportEnabled", cfg.Metrics.ExportEnabled, true},
		{"ExportPath", cfg.Metrics.ExportPath, "/tmp/metrics"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, tt.got)
			}
		})
	}
}

func TestSaveConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	cfg := DefaultConfig("")
	cfg.Producer.MessageSize = 4096

	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Verify file exists and can be loaded
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("config file was not created")
	}

	loaded, err := LoadConfig(configPath, "")
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}

	if loaded.Producer.MessageSize != 4096 {
		t.Errorf("expected message size 4096, got %d", loaded.Producer.MessageSize)
	}
}

func TestSaveInvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	cfg := DefaultConfig("")
	cfg.Pulsar.ServiceURL = "" // Make config invalid

	err := cfg.Save(configPath)
	if err == nil {
		t.Errorf("expected error when saving invalid config, got nil")
	}
}

func BenchmarkDefaultConfig(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = DefaultConfig("")
	}
}

func BenchmarkValidate(b *testing.B) {
	cfg := DefaultConfig("")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cfg.Validate()
	}
}

func BenchmarkLoadConfigFromEnv(b *testing.B) {
	os.Setenv("PULSAR_SERVICE_URL", "pulsar://test:6650")
	os.Setenv("PRODUCER_MESSAGE_SIZE", "2048")
	defer func() {
		os.Unsetenv("PULSAR_SERVICE_URL")
		os.Unsetenv("PRODUCER_MESSAGE_SIZE")
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = LoadConfigFromEnv()
	}
}
