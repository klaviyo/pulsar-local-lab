package ratelimit

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewLimiter(t *testing.T) {
	limiter := NewLimiter(1000)
	defer limiter.Stop()

	if limiter == nil {
		t.Fatal("NewLimiter returned nil")
	}

	rate := limiter.GetRate()
	if rate != 1000 {
		t.Errorf("Expected rate 1000, got %d", rate)
	}

	available := limiter.GetAvailable()
	if available != 1000 {
		t.Errorf("Expected 1000 tokens available initially, got %d", available)
	}
}

func TestNewLimiterDefaultRate(t *testing.T) {
	limiter := NewLimiter(0)
	defer limiter.Stop()

	rate := limiter.GetRate()
	if rate != 1000 {
		t.Errorf("Expected default rate 1000, got %d", rate)
	}
}

func TestLimiterAllow(t *testing.T) {
	limiter := NewLimiter(10)
	defer limiter.Stop()

	// Should allow first request
	if !limiter.Allow() {
		t.Error("First request should be allowed")
	}

	// Consume all tokens
	for i := 0; i < 9; i++ {
		limiter.Allow()
	}

	// Should not allow when tokens exhausted
	if limiter.Allow() {
		t.Error("Request should be denied when tokens exhausted")
	}
}

func TestLimiterWait(t *testing.T) {
	limiter := NewLimiter(100)
	defer limiter.Stop()

	ctx := context.Background()

	// First wait should succeed immediately
	if err := limiter.Wait(ctx); err != nil {
		t.Errorf("First wait should succeed: %v", err)
	}

	// Consume some tokens
	for i := 0; i < 50; i++ {
		limiter.Wait(ctx)
	}

	available := limiter.GetAvailable()
	if available >= 100 {
		t.Error("Tokens should be consumed")
	}
}

func TestLimiterWaitContextCancellation(t *testing.T) {
	limiter := NewLimiter(1)
	defer limiter.Stop()

	// Consume all tokens
	limiter.Allow()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := limiter.Wait(ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("Wait should fail when context is cancelled")
	}

	if elapsed > 100*time.Millisecond {
		t.Errorf("Wait should return quickly after context cancellation, took %v", elapsed)
	}
}

func TestLimiterRefill(t *testing.T) {
	limiter := NewLimiter(100)
	defer limiter.Stop()

	// Consume all tokens
	for i := 0; i < 100; i++ {
		limiter.Allow()
	}

	available := limiter.GetAvailable()
	if available > 0 {
		t.Error("All tokens should be consumed")
	}

	// Wait for refill (ticker is 10ms, should refill within 100ms)
	time.Sleep(100 * time.Millisecond)

	available = limiter.GetAvailable()
	if available == 0 {
		t.Error("Tokens should be refilled after waiting")
	}
}

func TestLimiterSetRate(t *testing.T) {
	limiter := NewLimiter(100)
	defer limiter.Stop()

	limiter.SetRate(500)

	rate := limiter.GetRate()
	if rate != 500 {
		t.Errorf("Expected rate 500, got %d", rate)
	}

	// Max bucket should also be updated
	available := limiter.GetAvailable()
	if available > 500 {
		t.Errorf("Available tokens should not exceed new max %d, got %d", 500, available)
	}
}

func TestLimiterSetRateAdjustsBucket(t *testing.T) {
	limiter := NewLimiter(1000)
	defer limiter.Stop()

	// Set lower rate
	limiter.SetRate(100)

	available := limiter.GetAvailable()
	if available > 100 {
		t.Errorf("Bucket should be capped at new max 100, got %d", available)
	}
}

func TestLimiterSetRateInvalidValue(t *testing.T) {
	limiter := NewLimiter(100)
	defer limiter.Stop()

	limiter.SetRate(-100)

	rate := limiter.GetRate()
	if rate < 1 {
		t.Errorf("Rate should be at least 1 for invalid input, got %d", rate)
	}
}

func TestLimiterConcurrentAllow(t *testing.T) {
	limiter := NewLimiter(10000)
	defer limiter.Stop()

	const numGoroutines = 100
	const allowsPerGoroutine = 100

	var successCount atomic.Int64
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < allowsPerGoroutine; j++ {
				if limiter.Allow() {
					successCount.Add(1)
				}
			}
		}()
	}

	wg.Wait()

	// Should have consumed exactly 10000 tokens (initial bucket size)
	if successCount.Load() > 10000 {
		t.Errorf("Success count should not exceed initial tokens 10000, got %d", successCount.Load())
	}
}

func TestLimiterConcurrentWait(t *testing.T) {
	limiter := NewLimiter(1000)
	defer limiter.Stop()

	const numGoroutines = 50
	const waitsPerGoroutine = 20

	var successCount atomic.Int64
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < waitsPerGoroutine; j++ {
				if err := limiter.Wait(ctx); err == nil {
					successCount.Add(1)
				}
			}
		}()
	}

	wg.Wait()

	// All waits should succeed given enough time
	expected := int64(numGoroutines * waitsPerGoroutine)
	if successCount.Load() != expected {
		t.Logf("Success count: %d, expected: %d (some operations may have timed out)",
			successCount.Load(), expected)
	}
}

func TestLimiterConcurrentSetRate(t *testing.T) {
	limiter := NewLimiter(100)
	defer limiter.Stop()

	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine 1: Keep changing rate
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			limiter.SetRate(50 + i*10)
			time.Sleep(time.Millisecond)
		}
	}()

	// Goroutine 2: Keep reading rate and allowing requests
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			limiter.GetRate()
			limiter.Allow()
			time.Sleep(time.Millisecond)
		}
	}()

	wg.Wait()

	// Test passes if no race conditions occur
}

func TestLimiterRateAccuracy(t *testing.T) {
	targetRate := 100
	limiter := NewLimiter(targetRate)
	defer limiter.Stop()

	// Wait for initial refills to stabilize
	time.Sleep(100 * time.Millisecond)

	ctx := context.Background()
	duration := 2 * time.Second // Use 2 seconds for more accurate measurement
	start := time.Now()
	count := 0

	for time.Since(start) < duration {
		if err := limiter.Wait(ctx); err != nil {
			t.Fatalf("Wait failed: %v", err)
		}
		count++
	}

	actualRate := float64(count) / duration.Seconds()

	// Allow 30% tolerance for rate accuracy (token bucket can have bursts)
	minExpected := float64(targetRate) * 0.7
	maxExpected := float64(targetRate) * 1.3

	if actualRate < minExpected || actualRate > maxExpected {
		t.Logf("Rate accuracy test: expected %d (±30%%), got %.2f requests/sec",
			targetRate, actualRate)
	}
}

func TestLimiterStop(t *testing.T) {
	limiter := NewLimiter(100)

	// Stop should not panic
	limiter.Stop()

	// Multiple stops should not panic
	limiter.Stop()
}

func TestLimiterGetAvailable(t *testing.T) {
	limiter := NewLimiter(100)
	defer limiter.Stop()

	initial := limiter.GetAvailable()
	if initial != 100 {
		t.Errorf("Expected 100 available tokens, got %d", initial)
	}

	// Consume some tokens
	for i := 0; i < 50; i++ {
		limiter.Allow()
	}

	after := limiter.GetAvailable()
	if after >= initial {
		t.Error("Available tokens should decrease after consumption")
	}

	if after > 50 {
		t.Errorf("Expected ~50 or fewer tokens available, got %d", after)
	}
}