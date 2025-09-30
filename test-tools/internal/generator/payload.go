package generator

import (
	"crypto/rand"
	"encoding/binary"
	"sync"
)

// PayloadPool manages reusable byte buffers to minimize allocations.
// It uses sync.Pool for thread-safe buffer pooling, significantly reducing
// GC pressure in high-throughput scenarios.
type PayloadPool struct {
	size int
	pool *sync.Pool
}

// NewPayloadPool creates a new buffer pool for payloads of the specified size.
// The capacity parameter is currently unused but reserved for future optimizations.
//
// Performance characteristics:
//   - Thread-safe for concurrent access
//   - Reduces allocations by reusing buffers
//   - Ideal for high-throughput scenarios (>100K msgs/sec)
//
// Example:
//
//	pool := NewPayloadPool(1024, 100)
//	buf := pool.Get()
//	// ... use buffer ...
//	pool.Put(buf)
func NewPayloadPool(size, capacity int) *PayloadPool {
	return &PayloadPool{
		size: size,
		pool: &sync.Pool{
			New: func() interface{} {
				return make([]byte, size)
			},
		},
	}
}

// Get retrieves a buffer from the pool. The returned buffer has length equal
// to the pool's configured size. The buffer may contain data from previous use
// and should be overwritten as needed.
//
// The buffer must be returned to the pool via Put() when no longer needed.
func (p *PayloadPool) Get() []byte {
	return p.pool.Get().([]byte)
}

// Put returns a buffer to the pool for reuse. The buffer should not be used
// after calling Put(). Only buffers obtained from Get() should be returned.
//
// Buffers of incorrect size will still be accepted but may reduce pool efficiency.
func (p *PayloadPool) Put(buf []byte) {
	p.pool.Put(buf)
}

// GenerateRandomPayload generates a payload of the specified size filled with
// cryptographically secure random bytes using crypto/rand.
//
// Performance characteristics:
//   - Allocates new buffer for each call
//   - ~1-2 µs for 1KB payload on modern hardware
//   - Suitable for >100K payloads/sec
//
// For high-throughput scenarios, consider using PayloadPool to reuse buffers.
//
// Example:
//
//	payload := GenerateRandomPayload(1024) // 1KB random payload
func GenerateRandomPayload(size int) []byte {
	buf := make([]byte, size)
	// crypto/rand.Read always returns len(buf), nil on success
	// or panics on system entropy exhaustion (extremely rare)
	rand.Read(buf)
	return buf
}

// GenerateSequentialPayload generates a payload with an embedded sequence number
// followed by random data. The sequence number is encoded as a uint64 in big-endian
// format at the start of the payload (first 8 bytes).
//
// This is useful for:
//   - Message ordering verification
//   - Duplicate detection
//   - Loss detection in testing scenarios
//
// The minimum size is 8 bytes to accommodate the sequence number.
// If size < 8, the payload will still be 8 bytes.
//
// Performance characteristics:
//   - Slightly slower than GenerateRandomPayload due to encoding overhead
//   - ~1.5-2.5 µs for 1KB payload
//
// Example:
//
//	for i := uint64(0); i < 1000; i++ {
//	    payload := GenerateSequentialPayload(1024, i)
//	    // First 8 bytes contain sequence number
//	}
func GenerateSequentialPayload(size int, seqNum uint64) []byte {
	// Ensure minimum size for sequence number
	if size < 8 {
		size = 8
	}

	buf := make([]byte, size)

	// Encode sequence number in first 8 bytes (big-endian)
	binary.BigEndian.PutUint64(buf[0:8], seqNum)

	// Fill remaining bytes with random data
	if size > 8 {
		rand.Read(buf[8:])
	}

	return buf
}

// ExtractSequenceNumber extracts the sequence number from a payload generated
// by GenerateSequentialPayload. Returns the sequence number and true if the
// payload is valid (at least 8 bytes), or 0 and false otherwise.
//
// Example:
//
//	payload := GenerateSequentialPayload(1024, 42)
//	seqNum, ok := ExtractSequenceNumber(payload)
//	if ok {
//	    fmt.Printf("Sequence: %d\n", seqNum) // Output: Sequence: 42
//	}
func ExtractSequenceNumber(payload []byte) (uint64, bool) {
	if len(payload) < 8 {
		return 0, false
	}
	return binary.BigEndian.Uint64(payload[0:8]), true
}

// GeneratePatternPayload generates a payload by repeating the specified pattern
// until the desired size is reached. If the pattern is longer than size, it will
// be truncated. If the pattern is shorter, it will be repeated.
//
// This is useful for:
//   - Testing compression algorithms (repeated patterns compress well)
//   - Debugging with recognizable data patterns
//   - Testing edge cases with specific byte sequences
//
// Performance characteristics:
//   - Very fast for short patterns (mostly memory copies)
//   - ~0.5-1 µs for 1KB payload with 4-byte pattern
//
// Example:
//
//	// Create 1KB payload with repeating "TEST" pattern
//	payload := GeneratePatternPayload(1024, "TEST")
//	// Result: TESTTESTTESTTEST...
func GeneratePatternPayload(size int, pattern string) []byte {
	if size <= 0 {
		return []byte{}
	}

	if len(pattern) == 0 {
		// Return zero-filled buffer if no pattern
		return make([]byte, size)
	}

	buf := make([]byte, size)
	patternBytes := []byte(pattern)
	patternLen := len(patternBytes)

	// Copy pattern repeatedly
	for i := 0; i < size; i += patternLen {
		n := copy(buf[i:], patternBytes)
		if n < patternLen {
			// Partial copy at the end
			break
		}
	}

	return buf
}

// GenerateRandomPayloadTo fills the provided buffer with random bytes.
// This is a zero-allocation version of GenerateRandomPayload when used with
// pooled buffers.
//
// The entire buffer will be filled with random data. Returns the same buffer
// for convenience in chaining operations.
//
// Example with PayloadPool:
//
//	pool := NewPayloadPool(1024, 100)
//	buf := pool.Get()
//	GenerateRandomPayloadTo(buf)
//	// ... use buffer ...
//	pool.Put(buf)
func GenerateRandomPayloadTo(buf []byte) []byte {
	rand.Read(buf)
	return buf
}

// GenerateSequentialPayloadTo fills the provided buffer with a sequence number
// and random data. This is a zero-allocation version of GenerateSequentialPayload
// when used with pooled buffers.
//
// The buffer must be at least 8 bytes. If smaller, this function will panic.
// Returns the same buffer for convenience in chaining operations.
//
// Example with PayloadPool:
//
//	pool := NewPayloadPool(1024, 100)
//	for i := uint64(0); i < 1000; i++ {
//	    buf := pool.Get()
//	    GenerateSequentialPayloadTo(buf, i)
//	    // ... use buffer ...
//	    pool.Put(buf)
//	}
func GenerateSequentialPayloadTo(buf []byte, seqNum uint64) []byte {
	if len(buf) < 8 {
		panic("buffer must be at least 8 bytes for sequence number")
	}

	// Encode sequence number in first 8 bytes
	binary.BigEndian.PutUint64(buf[0:8], seqNum)

	// Fill remaining bytes with random data
	if len(buf) > 8 {
		rand.Read(buf[8:])
	}

	return buf
}

// GeneratePatternPayloadTo fills the provided buffer with a repeating pattern.
// This is a zero-allocation version of GeneratePatternPayload when used with
// pooled buffers.
//
// Returns the same buffer for convenience in chaining operations.
//
// Example with PayloadPool:
//
//	pool := NewPayloadPool(1024, 100)
//	buf := pool.Get()
//	GeneratePatternPayloadTo(buf, "TEST")
//	// ... use buffer ...
//	pool.Put(buf)
func GeneratePatternPayloadTo(buf []byte, pattern string) []byte {
	if len(pattern) == 0 || len(buf) == 0 {
		return buf
	}

	patternBytes := []byte(pattern)
	patternLen := len(patternBytes)

	// Copy pattern repeatedly
	for i := 0; i < len(buf); i += patternLen {
		copy(buf[i:], patternBytes)
	}

	return buf
}
