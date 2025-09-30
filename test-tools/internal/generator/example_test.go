package generator_test

import (
	"fmt"

	"github.com/pulsar-local-lab/perf-test/internal/generator"
)

// Example demonstrates basic usage of GenerateRandomPayload
func ExampleGenerateRandomPayload() {
	payload := generator.GenerateRandomPayload(16)
	fmt.Printf("Generated random payload of %d bytes\n", len(payload))
	// Output: Generated random payload of 16 bytes
}

// Example demonstrates sequential payload generation with sequence numbers
func ExampleGenerateSequentialPayload() {
	payload := generator.GenerateSequentialPayload(1024, 42)
	seqNum, ok := generator.ExtractSequenceNumber(payload)
	if ok {
		fmt.Printf("Sequence number: %d\n", seqNum)
	}
	// Output: Sequence number: 42
}

// Example demonstrates pattern-based payload generation
func ExampleGeneratePatternPayload() {
	payload := generator.GeneratePatternPayload(16, "TEST")
	fmt.Printf("Pattern payload: %s\n", string(payload))
	// Output: Pattern payload: TESTTESTTESTTEST
}

// Example demonstrates high-performance payload generation with buffer pooling
func ExamplePayloadPool() {
	// Create a pool for 1KB payloads
	pool := generator.NewPayloadPool(1024, 100)

	// Get a buffer from the pool
	buf := pool.Get()

	// Use the buffer (e.g., fill with random data)
	generator.GenerateRandomPayloadTo(buf)

	// Return the buffer to the pool when done
	pool.Put(buf)

	fmt.Printf("Buffer size: %d bytes\n", len(buf))
	// Output: Buffer size: 1024 bytes
}

// Example demonstrates zero-allocation payload generation with pooled buffers
func ExamplePayloadPool_zeroAllocation() {
	pool := generator.NewPayloadPool(1024, 100)

	// Generate multiple sequential payloads without additional allocations
	for i := uint64(0); i < 3; i++ {
		buf := pool.Get()
		generator.GenerateSequentialPayloadTo(buf, i)

		seqNum, _ := generator.ExtractSequenceNumber(buf)
		fmt.Printf("Sequence: %d\n", seqNum)

		pool.Put(buf)
	}
	// Output:
	// Sequence: 0
	// Sequence: 1
	// Sequence: 2
}