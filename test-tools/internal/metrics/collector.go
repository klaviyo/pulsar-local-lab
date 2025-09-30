package metrics

import (
	"sync/atomic"
	"time"
)

// Collector collects and aggregates performance metrics using atomic operations for thread-safe counters
type Collector struct {
	// Message counters (atomic operations)
	messagesSent     atomic.Uint64
	messagesReceived atomic.Uint64
	messagesAcked    atomic.Uint64
	messagesFailed   atomic.Uint64

	// Byte counters (atomic operations)
	bytesSent     atomic.Uint64
	bytesReceived atomic.Uint64

	// Latency tracking
	latencies *Histogram

	// Throughput tracking
	throughput *ThroughputTracker

	// Timestamps
	startTime time.Time
	lastReset atomic.Value // stores time.Time
}

// NewCollector creates a new metrics collector
func NewCollector(histogramBuckets []float64) *Collector {
	now := time.Now()
	c := &Collector{
		latencies:  NewHistogram(histogramBuckets),
		throughput: NewThroughputTracker(),
		startTime:  now,
	}
	c.lastReset.Store(now)
	return c
}

// RecordSend records a sent message with atomic operations for thread safety
func (c *Collector) RecordSend(bytes int, latency time.Duration) {
	c.messagesSent.Add(1)
	c.bytesSent.Add(uint64(bytes))
	c.latencies.Observe(float64(latency.Milliseconds()))
	c.throughput.RecordSend()
}

// RecordReceive records a received message with atomic operations for thread safety
func (c *Collector) RecordReceive(bytes int) {
	c.messagesReceived.Add(1)
	c.bytesReceived.Add(uint64(bytes))
	c.throughput.RecordReceive()
}

// RecordAck records a message acknowledgment with atomic operations for thread safety
func (c *Collector) RecordAck() {
	c.messagesAcked.Add(1)
}

// RecordFailure records a failed operation with atomic operations for thread safety
func (c *Collector) RecordFailure() {
	c.messagesFailed.Add(1)
}

// GetSnapshot returns a snapshot of current metrics using atomic loads for thread safety
func (c *Collector) GetSnapshot() Snapshot {
	elapsed := time.Since(c.startTime)
	lastReset := c.lastReset.Load().(time.Time)
	sinceReset := time.Since(lastReset)

	return Snapshot{
		MessagesSent:     c.messagesSent.Load(),
		MessagesReceived: c.messagesReceived.Load(),
		MessagesAcked:    c.messagesAcked.Load(),
		MessagesFailed:   c.messagesFailed.Load(),
		BytesSent:        c.bytesSent.Load(),
		BytesReceived:    c.bytesReceived.Load(),
		LatencyStats:     c.latencies.GetStats(),
		Throughput:       c.throughput.GetStats(),
		Elapsed:          elapsed,
		SinceReset:       sinceReset,
	}
}

// Reset resets the metrics collector using atomic operations for thread safety
func (c *Collector) Reset() {
	c.messagesSent.Store(0)
	c.messagesReceived.Store(0)
	c.messagesAcked.Store(0)
	c.messagesFailed.Store(0)
	c.bytesSent.Store(0)
	c.bytesReceived.Store(0)
	c.latencies.Reset()
	c.throughput.Reset()
	c.lastReset.Store(time.Now())
}

// Snapshot represents a point-in-time snapshot of metrics
type Snapshot struct {
	MessagesSent     uint64
	MessagesReceived uint64
	MessagesAcked    uint64
	MessagesFailed   uint64
	BytesSent        uint64
	BytesReceived    uint64
	LatencyStats     LatencyStats
	Throughput       ThroughputStats
	Elapsed          time.Duration
	SinceReset       time.Duration
}

// MessageRate returns messages per second since start
func (s Snapshot) MessageRate() float64 {
	seconds := s.Elapsed.Seconds()
	if seconds == 0 {
		return 0
	}
	return float64(s.MessagesSent) / seconds
}

// ThroughputMBps returns throughput in MB/s since start
func (s Snapshot) ThroughputMBps() float64 {
	seconds := s.Elapsed.Seconds()
	if seconds == 0 {
		return 0
	}
	return float64(s.BytesSent) / seconds / 1024 / 1024
}