package metrics

import (
	"math"
	"sort"
	"sync"
)

// Histogram tracks latency distribution
type Histogram struct {
	mu      sync.RWMutex
	buckets []float64
	counts  []uint64
	samples []float64 // Store all samples for percentile calculation
	sum     float64
	count   uint64
	min     float64
	max     float64
}

// NewHistogram creates a new histogram with specified bucket boundaries
func NewHistogram(buckets []float64) *Histogram {
	// Sort buckets
	sorted := make([]float64, len(buckets))
	copy(sorted, buckets)
	sort.Float64s(sorted)

	return &Histogram{
		buckets: sorted,
		counts:  make([]uint64, len(sorted)+1),
		samples: make([]float64, 0, 10000),
		min:     math.MaxFloat64,
		max:     0,
	}
}

// Observe records a new observation
func (h *Histogram) Observe(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.sum += value
	h.count++
	h.samples = append(h.samples, value)

	if value < h.min {
		h.min = value
	}
	if value > h.max {
		h.max = value
	}

	// Find bucket
	bucket := sort.SearchFloat64s(h.buckets, value)
	h.counts[bucket]++
}

// GetStats returns latency statistics
func (h *Histogram) GetStats() LatencyStats {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.count == 0 {
		return LatencyStats{}
	}

	// Calculate percentiles
	sortedSamples := make([]float64, len(h.samples))
	copy(sortedSamples, h.samples)
	sort.Float64s(sortedSamples)

	return LatencyStats{
		Min:    h.min,
		Max:    h.max,
		Mean:   h.sum / float64(h.count),
		P50:    percentile(sortedSamples, 0.50),
		P95:    percentile(sortedSamples, 0.95),
		P99:    percentile(sortedSamples, 0.99),
		P999:   percentile(sortedSamples, 0.999),
		Count:  h.count,
	}
}

// Reset clears all histogram data
func (h *Histogram) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for i := range h.counts {
		h.counts[i] = 0
	}
	h.samples = h.samples[:0]
	h.sum = 0
	h.count = 0
	h.min = math.MaxFloat64
	h.max = 0
}

// LatencyStats contains latency statistics
type LatencyStats struct {
	Min   float64
	Max   float64
	Mean  float64
	P50   float64
	P95   float64
	P99   float64
	P999  float64
	Count uint64
}

// percentile calculates the percentile from sorted samples
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}

	index := p * float64(len(sorted)-1)
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))

	if lower == upper {
		return sorted[lower]
	}

	// Linear interpolation
	weight := index - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}