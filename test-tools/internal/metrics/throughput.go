package metrics

import (
	"sync"
	"time"
)

// ThroughputTracker tracks message throughput over time
type ThroughputTracker struct {
	mu sync.RWMutex

	sendTimestamps    []time.Time
	receiveTimestamps []time.Time

	// Rolling window for rate calculation
	windowDuration time.Duration
}

// NewThroughputTracker creates a new throughput tracker
func NewThroughputTracker() *ThroughputTracker {
	return &ThroughputTracker{
		sendTimestamps:    make([]time.Time, 0, 10000),
		receiveTimestamps: make([]time.Time, 0, 10000),
		windowDuration:    10 * time.Second, // 10-second rolling window
	}
}

// RecordSend records a send event
func (t *ThroughputTracker) RecordSend() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.sendTimestamps = append(t.sendTimestamps, time.Now())
}

// RecordReceive records a receive event
func (t *ThroughputTracker) RecordReceive() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.receiveTimestamps = append(t.receiveTimestamps, time.Now())
}

// GetStats returns throughput statistics
func (t *ThroughputTracker) GetStats() ThroughputStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	now := time.Now()
	windowStart := now.Add(-t.windowDuration)

	// Count messages in window
	sendCount := countInWindow(t.sendTimestamps, windowStart, now)
	receiveCount := countInWindow(t.receiveTimestamps, windowStart, now)

	// Calculate rates
	windowSeconds := t.windowDuration.Seconds()

	return ThroughputStats{
		SendRate:    float64(sendCount) / windowSeconds,
		ReceiveRate: float64(receiveCount) / windowSeconds,
		Window:      t.windowDuration,
	}
}

// Reset clears all throughput data
func (t *ThroughputTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.sendTimestamps = t.sendTimestamps[:0]
	t.receiveTimestamps = t.receiveTimestamps[:0]
}

// ThroughputStats contains throughput statistics
type ThroughputStats struct {
	SendRate    float64       // messages per second
	ReceiveRate float64       // messages per second
	Window      time.Duration // window duration
}

// countInWindow counts timestamps within a time window
func countInWindow(timestamps []time.Time, start, end time.Time) int {
	count := 0
	for i := len(timestamps) - 1; i >= 0; i-- {
		if timestamps[i].After(start) && timestamps[i].Before(end) {
			count++
		} else if timestamps[i].Before(start) {
			break // Timestamps are ordered, so we can stop
		}
	}
	return count
}