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
	sendBytes         []int // bytes per send event
	receiveBytes      []int // bytes per receive event

	// Rolling window for rate calculation
	windowDuration time.Duration
}

// NewThroughputTracker creates a new throughput tracker
func NewThroughputTracker() *ThroughputTracker {
	return &ThroughputTracker{
		sendTimestamps:    make([]time.Time, 0, 10000),
		receiveTimestamps: make([]time.Time, 0, 10000),
		sendBytes:         make([]int, 0, 10000),
		receiveBytes:      make([]int, 0, 10000),
		windowDuration:    10 * time.Second, // 10-second rolling window
	}
}

// RecordSend records a send event with byte count
func (t *ThroughputTracker) RecordSend(bytes int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.sendTimestamps = append(t.sendTimestamps, time.Now())
	t.sendBytes = append(t.sendBytes, bytes)
}

// RecordReceive records a receive event with byte count
func (t *ThroughputTracker) RecordReceive(bytes int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.receiveTimestamps = append(t.receiveTimestamps, time.Now())
	t.receiveBytes = append(t.receiveBytes, bytes)
}

// GetStats returns throughput statistics
func (t *ThroughputTracker) GetStats() ThroughputStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	now := time.Now()
	windowStart := now.Add(-t.windowDuration)

	// Count messages and bytes in window
	sendCount, sendBytesTotal := countAndSumInWindow(t.sendTimestamps, t.sendBytes, windowStart, now)
	receiveCount, receiveBytesTotal := countAndSumInWindow(t.receiveTimestamps, t.receiveBytes, windowStart, now)

	// Calculate rates
	windowSeconds := t.windowDuration.Seconds()

	return ThroughputStats{
		SendRate:        float64(sendCount) / windowSeconds,
		ReceiveRate:     float64(receiveCount) / windowSeconds,
		SendBandwidth:   float64(sendBytesTotal) / windowSeconds,       // bytes per second
		ReceiveBandwidth: float64(receiveBytesTotal) / windowSeconds, // bytes per second
		Window:          t.windowDuration,
	}
}

// Reset clears all throughput data
func (t *ThroughputTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.sendTimestamps = t.sendTimestamps[:0]
	t.receiveTimestamps = t.receiveTimestamps[:0]
	t.sendBytes = t.sendBytes[:0]
	t.receiveBytes = t.receiveBytes[:0]
}

// ThroughputStats contains throughput statistics
type ThroughputStats struct {
	SendRate         float64       // messages per second
	ReceiveRate      float64       // messages per second
	SendBandwidth    float64       // bytes per second
	ReceiveBandwidth float64       // bytes per second
	Window           time.Duration // window duration
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

// countAndSumInWindow counts timestamps and sums corresponding values within a time window
func countAndSumInWindow(timestamps []time.Time, values []int, start, end time.Time) (int, int) {
	count := 0
	sum := 0
	for i := len(timestamps) - 1; i >= 0; i-- {
		if timestamps[i].After(start) && timestamps[i].Before(end) {
			count++
			if i < len(values) {
				sum += values[i]
			}
		} else if timestamps[i].Before(start) {
			break // Timestamps are ordered, so we can stop
		}
	}
	return count, sum
}