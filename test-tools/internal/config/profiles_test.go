package config

import (
	"testing"
	"time"
)

func TestGetProfile(t *testing.T) {
	tests := []struct {
		name        string
		profileName string
		wantError   bool
	}{
		{"default profile", "default", false},
		{"empty profile", "", false},
		{"low-latency profile", "low-latency", false},
		{"high-throughput profile", "high-throughput", false},
		{"burst profile", "burst", false},
		{"sustained profile", "sustained", false},
		{"invalid profile", "invalid-profile", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := GetProfile(tt.profileName)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error for profile %q, got nil", tt.profileName)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for profile %q: %v", tt.profileName, err)
				}
				if cfg == nil {
					t.Errorf("expected config for profile %q, got nil", tt.profileName)
				}
				// Verify the config is valid
				if err := cfg.Validate(); err != nil {
					t.Errorf("profile %q produced invalid config: %v", tt.profileName, err)
				}
			}
		})
	}
}

func TestListProfiles(t *testing.T) {
	profiles := ListProfiles()

	expectedProfiles := []string{
		"default",
		"low-latency",
		"high-throughput",
		"burst",
		"sustained",
	}

	if len(profiles) != len(expectedProfiles) {
		t.Errorf("expected %d profiles, got %d", len(expectedProfiles), len(profiles))
	}

	profileMap := make(map[string]bool)
	for _, p := range profiles {
		profileMap[p] = true
	}

	for _, expected := range expectedProfiles {
		if !profileMap[expected] {
			t.Errorf("expected profile %q not found in list", expected)
		}
	}
}

func TestDefaultProfile(t *testing.T) {
	cfg := DefaultProfile()

	// Verify it's a valid config
	if err := cfg.Validate(); err != nil {
		t.Errorf("default profile should be valid: %v", err)
	}

	// Verify some default characteristics
	if cfg.Producer.NumProducers != 1 {
		t.Errorf("expected 1 producer, got %d", cfg.Producer.NumProducers)
	}
	if cfg.Consumer.NumConsumers != 1 {
		t.Errorf("expected 1 consumer, got %d", cfg.Consumer.NumConsumers)
	}
}

func TestLowLatencyProfile(t *testing.T) {
	cfg := LowLatencyProfile()

	// Verify it's a valid config
	if err := cfg.Validate(); err != nil {
		t.Errorf("low-latency profile should be valid: %v", err)
	}

	// Verify low-latency characteristics
	if cfg.Producer.BatchingEnabled {
		t.Error("batching should be disabled for low latency")
	}
	if cfg.Producer.CompressionType != CompressionNone {
		t.Errorf("expected no compression, got %s", cfg.Producer.CompressionType)
	}
	if cfg.Producer.MessageSize != 512 {
		t.Errorf("expected 512 byte messages, got %d", cfg.Producer.MessageSize)
	}
	if cfg.Consumer.ReceiverQueueSize != 10 {
		t.Errorf("expected small queue size of 10, got %d", cfg.Consumer.ReceiverQueueSize)
	}
	if cfg.Performance.TargetThroughput != 1000 {
		t.Errorf("expected target throughput 1000, got %d", cfg.Performance.TargetThroughput)
	}
	if !cfg.Performance.RateLimitEnabled {
		t.Error("rate limiting should be enabled for low latency")
	}
	if cfg.Metrics.CollectionInterval != 100*time.Millisecond {
		t.Errorf("expected 100ms collection interval, got %v", cfg.Metrics.CollectionInterval)
	}
}

func TestHighThroughputProfile(t *testing.T) {
	cfg := HighThroughputProfile()

	// Verify it's a valid config
	if err := cfg.Validate(); err != nil {
		t.Errorf("high-throughput profile should be valid: %v", err)
	}

	// Verify high-throughput characteristics
	if !cfg.Producer.BatchingEnabled {
		t.Error("batching should be enabled for high throughput")
	}
	if cfg.Producer.BatchingMaxSize != 10000 {
		t.Errorf("expected large batch size 10000, got %d", cfg.Producer.BatchingMaxSize)
	}
	if cfg.Producer.NumProducers != 10 {
		t.Errorf("expected 10 producers, got %d", cfg.Producer.NumProducers)
	}
	if cfg.Consumer.NumConsumers != 10 {
		t.Errorf("expected 10 consumers, got %d", cfg.Consumer.NumConsumers)
	}
	if cfg.Consumer.SubscriptionType != SubscriptionShared {
		t.Errorf("expected Shared subscription, got %s", cfg.Consumer.SubscriptionType)
	}
	if cfg.Performance.TargetThroughput != 0 {
		t.Errorf("expected unlimited throughput, got %d", cfg.Performance.TargetThroughput)
	}
	if cfg.Performance.RateLimitEnabled {
		t.Error("rate limiting should be disabled for high throughput")
	}
}

func TestBurstProfile(t *testing.T) {
	cfg := BurstProfile()

	// Verify it's a valid config
	if err := cfg.Validate(); err != nil {
		t.Errorf("burst profile should be valid: %v", err)
	}

	// Verify burst characteristics
	if cfg.Producer.CompressionType != CompressionZSTD {
		t.Errorf("expected ZSTD compression, got %s", cfg.Producer.CompressionType)
	}
	if cfg.Producer.NumProducers != 5 {
		t.Errorf("expected 5 producers, got %d", cfg.Producer.NumProducers)
	}
	if cfg.Performance.TargetThroughput != 10000 {
		t.Errorf("expected target throughput 10000, got %d", cfg.Performance.TargetThroughput)
	}
	if !cfg.Performance.RateLimitEnabled {
		t.Error("rate limiting should be enabled for burst")
	}
	if cfg.Performance.Duration != 5*time.Minute {
		t.Errorf("expected 5 minute duration, got %v", cfg.Performance.Duration)
	}
	if cfg.Metrics.CollectionInterval != 500*time.Millisecond {
		t.Errorf("expected 500ms collection interval, got %v", cfg.Metrics.CollectionInterval)
	}
}

func TestSustainedProfile(t *testing.T) {
	cfg := SustainedProfile()

	// Verify it's a valid config
	if err := cfg.Validate(); err != nil {
		t.Errorf("sustained profile should be valid: %v", err)
	}

	// Verify sustained characteristics
	if cfg.Producer.NumProducers != 5 {
		t.Errorf("expected 5 producers, got %d", cfg.Producer.NumProducers)
	}
	if cfg.Performance.TargetThroughput != 5000 {
		t.Errorf("expected target throughput 5000, got %d", cfg.Performance.TargetThroughput)
	}
	if !cfg.Performance.RateLimitEnabled {
		t.Error("rate limiting should be enabled for sustained")
	}
	if cfg.Performance.Duration != 0 {
		t.Errorf("expected unlimited duration, got %v", cfg.Performance.Duration)
	}
	if !cfg.Metrics.ExportEnabled {
		t.Error("metrics export should be enabled for sustained")
	}
	if cfg.Producer.CompressionType != CompressionLZ4 {
		t.Errorf("expected LZ4 compression, got %s", cfg.Producer.CompressionType)
	}
}

func TestApplyProfile(t *testing.T) {
	tests := []struct {
		name    string
		profile string
		verify  func(*testing.T, *Config)
	}{
		{
			name:    "apply default profile",
			profile: "default",
			verify: func(t *testing.T, cfg *Config) {
				// Should not modify the config
				if cfg.Producer.NumProducers != 1 {
					t.Errorf("default profile should not change producer count")
				}
			},
		},
		{
			name:    "apply low-latency profile",
			profile: "low-latency",
			verify: func(t *testing.T, cfg *Config) {
				if cfg.Producer.BatchingEnabled {
					t.Error("batching should be disabled")
				}
			},
		},
		{
			name:    "apply unknown profile",
			profile: "unknown",
			verify: func(t *testing.T, cfg *Config) {
				// Should not modify the config
				if cfg.Producer.NumProducers != 1 {
					t.Errorf("unknown profile should not change config")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig("")
			ApplyProfile(cfg, tt.profile)
			tt.verify(t, cfg)
		})
	}
}

func TestGetAvailableProfiles(t *testing.T) {
	profiles := GetAvailableProfiles()

	if len(profiles) == 0 {
		t.Error("expected at least one profile, got none")
	}

	// Verify all profiles are valid
	for _, profile := range profiles {
		if profile == "" {
			t.Error("profile name should not be empty")
		}

		// Try to get each profile
		cfg, err := GetProfile(profile)
		if err != nil {
			t.Errorf("failed to get profile %q: %v", profile, err)
		}
		if cfg == nil {
			t.Errorf("profile %q returned nil config", profile)
		}
	}
}

func TestGetProfileDescription(t *testing.T) {
	profiles := GetAvailableProfiles()

	for _, profile := range profiles {
		desc := GetProfileDescription(profile)
		if desc == "" {
			t.Errorf("profile %q should have a description", profile)
		}
	}

	// Test unknown profile
	desc := GetProfileDescription("unknown-profile")
	if desc != "" {
		t.Errorf("unknown profile should return empty description, got %q", desc)
	}
}

func TestProfilesAreDistinct(t *testing.T) {
	// Get all profiles
	lowLatency := LowLatencyProfile()
	highThroughput := HighThroughputProfile()
	burst := BurstProfile()
	sustained := SustainedProfile()

	// Verify they have different characteristics
	if lowLatency.Producer.BatchingEnabled == highThroughput.Producer.BatchingEnabled &&
		lowLatency.Producer.NumProducers == highThroughput.Producer.NumProducers {
		t.Error("low-latency and high-throughput profiles should be distinct")
	}

	if burst.Performance.Duration == sustained.Performance.Duration {
		t.Error("burst and sustained profiles should have different durations")
	}

	if highThroughput.Performance.RateLimitEnabled == lowLatency.Performance.RateLimitEnabled {
		t.Error("high-throughput and low-latency profiles should have different rate limiting")
	}
}

func BenchmarkGetProfile(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = GetProfile("low-latency")
	}
}

func BenchmarkLowLatencyProfile(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = LowLatencyProfile()
	}
}

func BenchmarkHighThroughputProfile(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = HighThroughputProfile()
	}
}

func BenchmarkApplyProfile(b *testing.B) {
	cfg := DefaultConfig("")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ApplyProfile(cfg, "low-latency")
	}
}
