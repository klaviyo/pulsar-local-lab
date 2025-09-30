package metrics

import (
	"testing"
	"time"
)

func BenchmarkCollectorRecordSend(b *testing.B) {
	collector := NewCollector([]float64{1, 10, 100, 1000})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.RecordSend(1024, 10*time.Millisecond)
	}
}

func BenchmarkCollectorRecordSendParallel(b *testing.B) {
	collector := NewCollector([]float64{1, 10, 100, 1000})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			collector.RecordSend(1024, 10*time.Millisecond)
		}
	})
}

func BenchmarkCollectorRecordReceive(b *testing.B) {
	collector := NewCollector([]float64{1, 10, 100, 1000})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.RecordReceive(1024)
	}
}

func BenchmarkCollectorRecordReceiveParallel(b *testing.B) {
	collector := NewCollector([]float64{1, 10, 100, 1000})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			collector.RecordReceive(1024)
		}
	})
}

func BenchmarkCollectorRecordAck(b *testing.B) {
	collector := NewCollector([]float64{1, 10, 100, 1000})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.RecordAck()
	}
}

func BenchmarkCollectorRecordAckParallel(b *testing.B) {
	collector := NewCollector([]float64{1, 10, 100, 1000})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			collector.RecordAck()
		}
	})
}

func BenchmarkCollectorRecordFailure(b *testing.B) {
	collector := NewCollector([]float64{1, 10, 100, 1000})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.RecordFailure()
	}
}

func BenchmarkCollectorRecordFailureParallel(b *testing.B) {
	collector := NewCollector([]float64{1, 10, 100, 1000})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			collector.RecordFailure()
		}
	})
}

func BenchmarkCollectorGetSnapshot(b *testing.B) {
	collector := NewCollector([]float64{1, 10, 100, 1000})

	// Pre-populate with some data
	for i := 0; i < 1000; i++ {
		collector.RecordSend(1024, time.Millisecond)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = collector.GetSnapshot()
	}
}

func BenchmarkCollectorGetSnapshotParallel(b *testing.B) {
	collector := NewCollector([]float64{1, 10, 100, 1000})

	// Pre-populate with some data
	for i := 0; i < 1000; i++ {
		collector.RecordSend(1024, time.Millisecond)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = collector.GetSnapshot()
		}
	})
}

func BenchmarkCollectorMixedOperations(b *testing.B) {
	collector := NewCollector([]float64{1, 10, 100, 1000})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		switch i % 5 {
		case 0:
			collector.RecordSend(1024, time.Millisecond)
		case 1:
			collector.RecordReceive(512)
		case 2:
			collector.RecordAck()
		case 3:
			collector.RecordFailure()
		case 4:
			_ = collector.GetSnapshot()
		}
	}
}

func BenchmarkCollectorMixedOperationsParallel(b *testing.B) {
	collector := NewCollector([]float64{1, 10, 100, 1000})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			switch i % 5 {
			case 0:
				collector.RecordSend(1024, time.Millisecond)
			case 1:
				collector.RecordReceive(512)
			case 2:
				collector.RecordAck()
			case 3:
				collector.RecordFailure()
			case 4:
				_ = collector.GetSnapshot()
			}
			i++
		}
	})
}

func BenchmarkHistogramObserve(b *testing.B) {
	hist := NewHistogram([]float64{1, 10, 100, 1000})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hist.Observe(float64(i % 1000))
	}
}

func BenchmarkHistogramObserveParallel(b *testing.B) {
	hist := NewHistogram([]float64{1, 10, 100, 1000})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			hist.Observe(float64(i % 1000))
			i++
		}
	})
}

func BenchmarkHistogramGetStats(b *testing.B) {
	hist := NewHistogram([]float64{1, 10, 100, 1000})

	// Pre-populate with data
	for i := 0; i < 10000; i++ {
		hist.Observe(float64(i % 1000))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = hist.GetStats()
	}
}

func BenchmarkThroughputTrackerRecordSend(b *testing.B) {
	tracker := NewThroughputTracker()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tracker.RecordSend()
	}
}

func BenchmarkThroughputTrackerRecordSendParallel(b *testing.B) {
	tracker := NewThroughputTracker()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			tracker.RecordSend()
		}
	})
}

func BenchmarkThroughputTrackerGetStats(b *testing.B) {
	tracker := NewThroughputTracker()

	// Pre-populate with data
	for i := 0; i < 1000; i++ {
		tracker.RecordSend()
		tracker.RecordReceive()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tracker.GetStats()
	}
}