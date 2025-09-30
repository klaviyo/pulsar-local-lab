package metrics

import (
	"sync"
	"testing"
	"time"
)

func TestNewCollector(t *testing.T) {
	buckets := []float64{1, 5, 10, 50, 100, 500, 1000}
	collector := NewCollector(buckets)

	if collector == nil {
		t.Fatal("NewCollector returned nil")
	}

	if collector.latencies == nil {
		t.Error("latencies histogram not initialized")
	}

	if collector.throughput == nil {
		t.Error("throughput tracker not initialized")
	}
}

func TestCollectorRecordSend(t *testing.T) {
	collector := NewCollector([]float64{1, 10, 100, 1000})

	tests := []struct {
		name    string
		bytes   int
		latency time.Duration
	}{
		{"small message", 100, 5 * time.Millisecond},
		{"medium message", 1024, 10 * time.Millisecond},
		{"large message", 10240, 50 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector.RecordSend(tt.bytes, tt.latency)

			snapshot := collector.GetSnapshot()
			if snapshot.MessagesSent == 0 {
				t.Error("MessagesSent should be greater than 0")
			}

			if snapshot.BytesSent == 0 {
				t.Error("BytesSent should be greater than 0")
			}
		})
	}
}

func TestCollectorRecordReceive(t *testing.T) {
	collector := NewCollector([]float64{1, 10, 100, 1000})

	collector.RecordReceive(1024)
	collector.RecordReceive(2048)

	snapshot := collector.GetSnapshot()

	if snapshot.MessagesReceived != 2 {
		t.Errorf("Expected 2 messages received, got %d", snapshot.MessagesReceived)
	}

	expectedBytes := uint64(1024 + 2048)
	if snapshot.BytesReceived != expectedBytes {
		t.Errorf("Expected %d bytes received, got %d", expectedBytes, snapshot.BytesReceived)
	}
}

func TestCollectorRecordAck(t *testing.T) {
	collector := NewCollector([]float64{1, 10, 100, 1000})

	collector.RecordAck()
	collector.RecordAck()
	collector.RecordAck()

	snapshot := collector.GetSnapshot()

	if snapshot.MessagesAcked != 3 {
		t.Errorf("Expected 3 messages acked, got %d", snapshot.MessagesAcked)
	}
}

func TestCollectorRecordFailure(t *testing.T) {
	collector := NewCollector([]float64{1, 10, 100, 1000})

	collector.RecordFailure()
	collector.RecordFailure()

	snapshot := collector.GetSnapshot()

	if snapshot.MessagesFailed != 2 {
		t.Errorf("Expected 2 messages failed, got %d", snapshot.MessagesFailed)
	}
}

func TestCollectorReset(t *testing.T) {
	collector := NewCollector([]float64{1, 10, 100, 1000})

	// Record some data
	collector.RecordSend(1024, 10*time.Millisecond)
	collector.RecordReceive(512)
	collector.RecordAck()
	collector.RecordFailure()

	// Verify data exists
	snapshot := collector.GetSnapshot()
	if snapshot.MessagesSent == 0 {
		t.Error("Expected some messages sent before reset")
	}

	// Reset
	collector.Reset()

	// Verify all counters are zero
	snapshot = collector.GetSnapshot()
	if snapshot.MessagesSent != 0 {
		t.Errorf("MessagesSent should be 0 after reset, got %d", snapshot.MessagesSent)
	}
	if snapshot.MessagesReceived != 0 {
		t.Errorf("MessagesReceived should be 0 after reset, got %d", snapshot.MessagesReceived)
	}
	if snapshot.MessagesAcked != 0 {
		t.Errorf("MessagesAcked should be 0 after reset, got %d", snapshot.MessagesAcked)
	}
	if snapshot.MessagesFailed != 0 {
		t.Errorf("MessagesFailed should be 0 after reset, got %d", snapshot.MessagesFailed)
	}
	if snapshot.BytesSent != 0 {
		t.Errorf("BytesSent should be 0 after reset, got %d", snapshot.BytesSent)
	}
	if snapshot.BytesReceived != 0 {
		t.Errorf("BytesReceived should be 0 after reset, got %d", snapshot.BytesReceived)
	}
}

func TestCollectorConcurrentAccess(t *testing.T) {
	collector := NewCollector([]float64{1, 10, 100, 1000})

	const numGoroutines = 100
	const operationsPerGoroutine = 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 4) // 4 types of operations

	// Concurrent sends
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				collector.RecordSend(100, time.Millisecond)
			}
		}()
	}

	// Concurrent receives
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				collector.RecordReceive(100)
			}
		}()
	}

	// Concurrent acks
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				collector.RecordAck()
			}
		}()
	}

	// Concurrent failures
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				collector.RecordFailure()
			}
		}()
	}

	wg.Wait()

	// Verify counts
	snapshot := collector.GetSnapshot()
	expected := uint64(numGoroutines * operationsPerGoroutine)

	if snapshot.MessagesSent != expected {
		t.Errorf("Expected %d messages sent, got %d", expected, snapshot.MessagesSent)
	}
	if snapshot.MessagesReceived != expected {
		t.Errorf("Expected %d messages received, got %d", expected, snapshot.MessagesReceived)
	}
	if snapshot.MessagesAcked != expected {
		t.Errorf("Expected %d messages acked, got %d", expected, snapshot.MessagesAcked)
	}
	if snapshot.MessagesFailed != expected {
		t.Errorf("Expected %d messages failed, got %d", expected, snapshot.MessagesFailed)
	}
}

func TestSnapshotMessageRate(t *testing.T) {
	snapshot := Snapshot{
		MessagesSent: 1000,
		Elapsed:      time.Second,
	}

	rate := snapshot.MessageRate()
	if rate != 1000.0 {
		t.Errorf("Expected message rate 1000.0, got %f", rate)
	}

	// Test with zero elapsed time
	snapshot.Elapsed = 0
	rate = snapshot.MessageRate()
	if rate != 0 {
		t.Errorf("Expected message rate 0 for zero elapsed time, got %f", rate)
	}
}

func TestSnapshotThroughputMBps(t *testing.T) {
	snapshot := Snapshot{
		BytesSent: 1024 * 1024, // 1 MB
		Elapsed:   time.Second,
	}

	throughput := snapshot.ThroughputMBps()
	if throughput != 1.0 {
		t.Errorf("Expected throughput 1.0 MB/s, got %f", throughput)
	}

	// Test with zero elapsed time
	snapshot.Elapsed = 0
	throughput = snapshot.ThroughputMBps()
	if throughput != 0 {
		t.Errorf("Expected throughput 0 for zero elapsed time, got %f", throughput)
	}
}