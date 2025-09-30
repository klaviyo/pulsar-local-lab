package metrics

import (
	"math"
	"sync"
	"testing"
)

func TestNewHistogram(t *testing.T) {
	buckets := []float64{1, 5, 10, 50, 100}
	hist := NewHistogram(buckets)

	if hist == nil {
		t.Fatal("NewHistogram returned nil")
	}

	if len(hist.buckets) != len(buckets) {
		t.Errorf("Expected %d buckets, got %d", len(buckets), len(hist.buckets))
	}

	// Verify buckets are sorted
	for i := 1; i < len(hist.buckets); i++ {
		if hist.buckets[i] < hist.buckets[i-1] {
			t.Error("Buckets are not sorted")
			break
		}
	}
}

func TestHistogramObserve(t *testing.T) {
	hist := NewHistogram([]float64{10, 50, 100})

	tests := []struct {
		name  string
		value float64
	}{
		{"small value", 5.0},
		{"medium value", 25.0},
		{"large value", 75.0},
		{"very large value", 150.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hist.Observe(tt.value)

			stats := hist.GetStats()
			if stats.Count == 0 {
				t.Error("Count should be greater than 0 after observation")
			}
		})
	}
}

func TestHistogramStats(t *testing.T) {
	hist := NewHistogram([]float64{10, 50, 100})

	// Record specific values
	values := []float64{1, 5, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120}
	for _, v := range values {
		hist.Observe(v)
	}

	stats := hist.GetStats()

	// Test count
	if stats.Count != uint64(len(values)) {
		t.Errorf("Expected count %d, got %d", len(values), stats.Count)
	}

	// Test min
	if stats.Min != 1.0 {
		t.Errorf("Expected min 1.0, got %f", stats.Min)
	}

	// Test max
	if stats.Max != 120.0 {
		t.Errorf("Expected max 120.0, got %f", stats.Max)
	}

	// Test mean (sum of values 1+5+10+...+120 = 785, divided by 14)
	expectedMean := 785.0 / 14.0 // 56.07
	if math.Abs(stats.Mean-expectedMean) > 1.0 {
		t.Errorf("Expected mean approximately %f, got %f", expectedMean, stats.Mean)
	}

	// Test P50 (median)
	if stats.P50 < 50 || stats.P50 > 60 {
		t.Errorf("Expected P50 between 50 and 60, got %f", stats.P50)
	}

	// Test P99
	if stats.P99 < 110 {
		t.Errorf("Expected P99 >= 110, got %f", stats.P99)
	}
}

func TestHistogramPercentiles(t *testing.T) {
	hist := NewHistogram([]float64{10, 50, 100})

	// Add 100 observations from 1 to 100
	for i := 1; i <= 100; i++ {
		hist.Observe(float64(i))
	}

	stats := hist.GetStats()

	// Test specific percentiles
	tests := []struct {
		name     string
		percentile float64
		expected   float64
		tolerance  float64
	}{
		{"P50", stats.P50, 50.0, 5.0},
		{"P95", stats.P95, 95.0, 5.0},
		{"P99", stats.P99, 99.0, 2.0},
		{"P999", stats.P999, 100.0, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if math.Abs(tt.percentile-tt.expected) > tt.tolerance {
				t.Errorf("Expected %s near %f (±%f), got %f",
					tt.name, tt.expected, tt.tolerance, tt.percentile)
			}
		})
	}
}

func TestHistogramReset(t *testing.T) {
	hist := NewHistogram([]float64{10, 50, 100})

	// Add observations
	hist.Observe(10)
	hist.Observe(20)
	hist.Observe(30)

	// Verify data exists
	stats := hist.GetStats()
	if stats.Count == 0 {
		t.Error("Expected count > 0 before reset")
	}

	// Reset
	hist.Reset()

	// Verify all data cleared
	stats = hist.GetStats()
	if stats.Count != 0 {
		t.Errorf("Expected count 0 after reset, got %d", stats.Count)
	}
	if stats.Min != 0 {
		t.Errorf("Expected min 0 after reset, got %f", stats.Min)
	}
	if stats.Max != 0 {
		t.Errorf("Expected max 0 after reset, got %f", stats.Max)
	}
	if stats.Mean != 0 {
		t.Errorf("Expected mean 0 after reset, got %f", stats.Mean)
	}
}

func TestHistogramEmptyStats(t *testing.T) {
	hist := NewHistogram([]float64{10, 50, 100})

	stats := hist.GetStats()

	if stats.Count != 0 {
		t.Error("Empty histogram should have count 0")
	}
	if stats.Min != 0 {
		t.Error("Empty histogram should have min 0")
	}
	if stats.Max != 0 {
		t.Error("Empty histogram should have max 0")
	}
	if stats.Mean != 0 {
		t.Error("Empty histogram should have mean 0")
	}
}

func TestHistogramConcurrentAccess(t *testing.T) {
	hist := NewHistogram([]float64{10, 50, 100})

	const numGoroutines = 50
	const observationsPerGoroutine = 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Concurrent observations
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < observationsPerGoroutine; j++ {
				hist.Observe(float64(id*10 + j%100))
			}
		}(i)
	}

	wg.Wait()

	// Verify count
	stats := hist.GetStats()
	expected := uint64(numGoroutines * observationsPerGoroutine)
	if stats.Count != expected {
		t.Errorf("Expected count %d, got %d", expected, stats.Count)
	}
}

func TestPercentileFunction(t *testing.T) {
	tests := []struct {
		name       string
		sorted     []float64
		percentile float64
		want       float64
	}{
		{"empty slice", []float64{}, 0.5, 0},
		{"single element", []float64{42}, 0.5, 42},
		{"two elements p50", []float64{10, 20}, 0.5, 15},
		{"two elements p100", []float64{10, 20}, 1.0, 20},
		{"sorted values p50", []float64{1, 2, 3, 4, 5}, 0.5, 3},
		{"sorted values p0", []float64{1, 2, 3, 4, 5}, 0.0, 1},
		{"sorted values p100", []float64{1, 2, 3, 4, 5}, 1.0, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := percentile(tt.sorted, tt.percentile)
			if math.Abs(got-tt.want) > 0.01 {
				t.Errorf("percentile(%v, %f) = %f, want %f",
					tt.sorted, tt.percentile, got, tt.want)
			}
		})
	}
}