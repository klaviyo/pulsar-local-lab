package metrics

import (
	"sync"
	"testing"
	"time"
)

func TestNewThroughputTracker(t *testing.T) {
	tracker := NewThroughputTracker()

	if tracker == nil {
		t.Fatal("NewThroughputTracker returned nil")
	}

	if tracker.windowDuration != 10*time.Second {
		t.Errorf("Expected window duration 10s, got %v", tracker.windowDuration)
	}
}

func TestThroughputTrackerRecordSend(t *testing.T) {
	tracker := NewThroughputTracker()

	tracker.RecordSend()
	tracker.RecordSend()
	tracker.RecordSend()

	stats := tracker.GetStats()

	if stats.SendRate == 0 {
		t.Error("Send rate should be greater than 0")
	}
}

func TestThroughputTrackerRecordReceive(t *testing.T) {
	tracker := NewThroughputTracker()

	tracker.RecordReceive()
	tracker.RecordReceive()

	stats := tracker.GetStats()

	if stats.ReceiveRate == 0 {
		t.Error("Receive rate should be greater than 0")
	}
}

func TestThroughputTrackerStats(t *testing.T) {
	tracker := NewThroughputTracker()

	// Record some sends
	for i := 0; i < 100; i++ {
		tracker.RecordSend()
	}

	// Record some receives
	for i := 0; i < 50; i++ {
		tracker.RecordReceive()
	}

	stats := tracker.GetStats()

	if stats.SendRate <= 0 {
		t.Error("SendRate should be positive")
	}

	if stats.ReceiveRate <= 0 {
		t.Error("ReceiveRate should be positive")
	}

	if stats.Window != tracker.windowDuration {
		t.Errorf("Expected window %v, got %v", tracker.windowDuration, stats.Window)
	}

	// SendRate should be higher than ReceiveRate since we sent more
	if stats.SendRate < stats.ReceiveRate {
		t.Error("SendRate should be greater than ReceiveRate")
	}
}

func TestThroughputTrackerReset(t *testing.T) {
	tracker := NewThroughputTracker()

	// Record some data
	for i := 0; i < 10; i++ {
		tracker.RecordSend()
		tracker.RecordReceive()
	}

	// Verify data exists
	statsBefore := tracker.GetStats()
	if statsBefore.SendRate == 0 {
		t.Error("Expected send rate > 0 before reset")
	}

	// Reset
	tracker.Reset()

	// Verify data cleared
	statsAfter := tracker.GetStats()
	if statsAfter.SendRate != 0 {
		t.Errorf("Expected send rate 0 after reset, got %f", statsAfter.SendRate)
	}
	if statsAfter.ReceiveRate != 0 {
		t.Errorf("Expected receive rate 0 after reset, got %f", statsAfter.ReceiveRate)
	}
}

func TestThroughputTrackerWindow(t *testing.T) {
	tracker := NewThroughputTracker()

	// Record events
	for i := 0; i < 50; i++ {
		tracker.RecordSend()
	}

	// Wait briefly
	time.Sleep(100 * time.Millisecond)

	// Record more events
	for i := 0; i < 50; i++ {
		tracker.RecordSend()
	}

	stats := tracker.GetStats()

	// All 100 events should be in the window
	expectedMinRate := 100.0 / tracker.windowDuration.Seconds()
	if stats.SendRate < expectedMinRate {
		t.Errorf("Expected send rate >= %f, got %f", expectedMinRate, stats.SendRate)
	}
}

func TestThroughputTrackerOldEventsExcluded(t *testing.T) {
	tracker := NewThroughputTracker()
	// Use shorter window for testing
	tracker.windowDuration = 500 * time.Millisecond

	// Record old events
	for i := 0; i < 100; i++ {
		tracker.RecordSend()
	}

	// Wait for window to expire
	time.Sleep(600 * time.Millisecond)

	stats := tracker.GetStats()

	// Old events should be excluded, rate should be very low or zero
	if stats.SendRate > 1.0 {
		t.Errorf("Expected low send rate after window expiry, got %f", stats.SendRate)
	}
}

func TestThroughputTrackerConcurrentAccess(t *testing.T) {
	tracker := NewThroughputTracker()

	const numGoroutines = 50
	const operationsPerGoroutine = 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2)

	// Concurrent sends
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				tracker.RecordSend()
			}
		}()
	}

	// Concurrent receives
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				tracker.RecordReceive()
			}
		}()
	}

	wg.Wait()

	stats := tracker.GetStats()

	// Verify we got some rate
	if stats.SendRate == 0 {
		t.Error("Expected send rate > 0")
	}
	if stats.ReceiveRate == 0 {
		t.Error("Expected receive rate > 0")
	}

	// Rates should be approximately equal since we did equal operations
	ratio := stats.SendRate / stats.ReceiveRate
	if ratio < 0.5 || ratio > 2.0 {
		t.Errorf("Expected send/receive rate ratio near 1.0, got %f", ratio)
	}
}

func TestCountInWindow(t *testing.T) {
	now := time.Now()
	start := now.Add(-5 * time.Second)

	tests := []struct {
		name       string
		timestamps []time.Time
		start      time.Time
		end        time.Time
		want       int
	}{
		{
			name:       "empty timestamps",
			timestamps: []time.Time{},
			start:      start,
			end:        now,
			want:       0,
		},
		{
			name: "all in window",
			timestamps: []time.Time{
				now.Add(-4 * time.Second),
				now.Add(-3 * time.Second),
				now.Add(-2 * time.Second),
			},
			start: start,
			end:   now,
			want:  3,
		},
		{
			name: "some before window",
			timestamps: []time.Time{
				now.Add(-10 * time.Second),
				now.Add(-3 * time.Second),
				now.Add(-2 * time.Second),
			},
			start: start,
			end:   now,
			want:  2,
		},
		{
			name: "all before window",
			timestamps: []time.Time{
				now.Add(-10 * time.Second),
				now.Add(-9 * time.Second),
			},
			start: start,
			end:   now,
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countInWindow(tt.timestamps, tt.start, tt.end)
			if got != tt.want {
				t.Errorf("countInWindow() = %d, want %d", got, tt.want)
			}
		})
	}
}