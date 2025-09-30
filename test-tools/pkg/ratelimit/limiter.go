package ratelimit

import (
	"context"
	"sync/atomic"
	"time"
)

// Limiter implements a thread-safe token bucket rate limiter using atomic operations
type Limiter struct {
	rate       atomic.Int64  // tokens per second
	bucket     atomic.Int64  // current tokens available
	maxBucket  atomic.Int64  // maximum bucket size
	lastRefill atomic.Int64  // last refill time (Unix nanoseconds)
	ticker     *time.Ticker  // ticker for refilling
	done       chan struct{} // signal to stop refilling
	stopped    atomic.Bool   // flag to prevent double close
}

// NewLimiter creates a new thread-safe rate limiter with token bucket algorithm
func NewLimiter(ratePerSecond int) *Limiter {
	if ratePerSecond <= 0 {
		ratePerSecond = 1000 // default
	}

	l := &Limiter{
		ticker: time.NewTicker(time.Millisecond * 10), // 10ms granularity for smoother rate limiting
		done:   make(chan struct{}),
	}

	now := time.Now().UnixNano()
	l.rate.Store(int64(ratePerSecond))
	l.bucket.Store(int64(ratePerSecond))
	l.maxBucket.Store(int64(ratePerSecond))
	l.lastRefill.Store(now)

	// Start refill goroutine
	go l.refillLoop()

	return l
}

// Wait blocks until a token is available, respecting context cancellation
func (l *Limiter) Wait(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if l.tryAcquire() {
			return nil
		}

		// Sleep for a short duration before retrying
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Millisecond):
		}
	}
}

// Allow returns true if a token is available, false otherwise (non-blocking)
func (l *Limiter) Allow() bool {
	return l.tryAcquire()
}

// tryAcquire attempts to acquire a token using atomic compare-and-swap
func (l *Limiter) tryAcquire() bool {
	for {
		current := l.bucket.Load()
		if current <= 0 {
			return false
		}
		// Try to atomically decrement the bucket
		if l.bucket.CompareAndSwap(current, current-1) {
			return true
		}
		// CAS failed, retry
	}
}

// refillLoop periodically refills the token bucket using atomic operations
func (l *Limiter) refillLoop() {
	for {
		select {
		case <-l.done:
			l.ticker.Stop()
			return
		case <-l.ticker.C:
			l.refill()
		}
	}
}

// refill adds tokens to the bucket based on elapsed time using atomic operations
func (l *Limiter) refill() {
	now := time.Now().UnixNano()
	lastRefill := l.lastRefill.Load()
	elapsed := time.Duration(now - lastRefill)

	rate := l.rate.Load()
	tokensToAdd := int64(float64(rate) * elapsed.Seconds())

	if tokensToAdd > 0 {
		maxBucket := l.maxBucket.Load()
		for {
			current := l.bucket.Load()
			newBucket := current + tokensToAdd
			if newBucket > maxBucket {
				newBucket = maxBucket
			}
			if l.bucket.CompareAndSwap(current, newBucket) {
				l.lastRefill.Store(now)
				break
			}
		}
	}
}

// Stop stops the rate limiter gracefully, safe to call multiple times
func (l *Limiter) Stop() {
	if l.stopped.CompareAndSwap(false, true) {
		close(l.done)
	}
}

// SetRate updates the rate limit using atomic operations for thread safety
func (l *Limiter) SetRate(ratePerSecond int) {
	if ratePerSecond <= 0 {
		ratePerSecond = 1
	}
	rate := int64(ratePerSecond)
	l.rate.Store(rate)
	l.maxBucket.Store(rate)

	// Adjust current bucket if it exceeds new max
	for {
		current := l.bucket.Load()
		if current <= rate {
			break
		}
		if l.bucket.CompareAndSwap(current, rate) {
			break
		}
	}
}

// GetRate returns the current rate limit using atomic load
func (l *Limiter) GetRate() int {
	return int(l.rate.Load())
}

// GetAvailable returns the number of available tokens using atomic load
func (l *Limiter) GetAvailable() int {
	return int(l.bucket.Load())
}