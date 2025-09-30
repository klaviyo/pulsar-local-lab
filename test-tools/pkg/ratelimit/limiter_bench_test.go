package ratelimit

import (
	"context"
	"testing"
)

func BenchmarkLimiterAllow(b *testing.B) {
	limiter := NewLimiter(1000000) // High rate to avoid blocking
	defer limiter.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.Allow()
	}
}

func BenchmarkLimiterAllowParallel(b *testing.B) {
	limiter := NewLimiter(1000000) // High rate to avoid blocking
	defer limiter.Stop()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Allow()
		}
	})
}

func BenchmarkLimiterWait(b *testing.B) {
	limiter := NewLimiter(1000000) // High rate to avoid blocking
	defer limiter.Stop()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.Wait(ctx)
	}
}

func BenchmarkLimiterWaitParallel(b *testing.B) {
	limiter := NewLimiter(1000000) // High rate to avoid blocking
	defer limiter.Stop()

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Wait(ctx)
		}
	})
}

func BenchmarkLimiterGetRate(b *testing.B) {
	limiter := NewLimiter(1000)
	defer limiter.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = limiter.GetRate()
	}
}

func BenchmarkLimiterGetRateParallel(b *testing.B) {
	limiter := NewLimiter(1000)
	defer limiter.Stop()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = limiter.GetRate()
		}
	})
}

func BenchmarkLimiterGetAvailable(b *testing.B) {
	limiter := NewLimiter(1000)
	defer limiter.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = limiter.GetAvailable()
	}
}

func BenchmarkLimiterGetAvailableParallel(b *testing.B) {
	limiter := NewLimiter(1000)
	defer limiter.Stop()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = limiter.GetAvailable()
		}
	})
}

func BenchmarkLimiterSetRate(b *testing.B) {
	limiter := NewLimiter(1000)
	defer limiter.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.SetRate(1000 + i%1000)
	}
}

func BenchmarkLimiterSetRateParallel(b *testing.B) {
	limiter := NewLimiter(1000)
	defer limiter.Stop()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			limiter.SetRate(1000 + i%1000)
			i++
		}
	})
}

func BenchmarkLimiterTryAcquire(b *testing.B) {
	limiter := NewLimiter(1000000) // High rate to avoid blocking
	defer limiter.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.tryAcquire()
	}
}

func BenchmarkLimiterTryAcquireParallel(b *testing.B) {
	limiter := NewLimiter(1000000) // High rate to avoid blocking
	defer limiter.Stop()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.tryAcquire()
		}
	})
}

func BenchmarkLimiterMixedOperations(b *testing.B) {
	limiter := NewLimiter(1000000) // High rate to avoid blocking
	defer limiter.Stop()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		switch i % 4 {
		case 0:
			limiter.Allow()
		case 1:
			limiter.Wait(ctx)
		case 2:
			_ = limiter.GetRate()
		case 3:
			_ = limiter.GetAvailable()
		}
	}
}

func BenchmarkLimiterMixedOperationsParallel(b *testing.B) {
	limiter := NewLimiter(1000000) // High rate to avoid blocking
	defer limiter.Stop()

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			switch i % 4 {
			case 0:
				limiter.Allow()
			case 1:
				limiter.Wait(ctx)
			case 2:
				_ = limiter.GetRate()
			case 3:
				_ = limiter.GetAvailable()
			}
			i++
		}
	})
}

func BenchmarkLimiterContentionLow(b *testing.B) {
	limiter := NewLimiter(100000)
	defer limiter.Stop()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Allow()
		}
	})
}

func BenchmarkLimiterContentionHigh(b *testing.B) {
	limiter := NewLimiter(10) // Low rate creates high contention
	defer limiter.Stop()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Allow()
		}
	})
}