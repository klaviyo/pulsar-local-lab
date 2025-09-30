package generator

import (
	"bytes"
	"sync"
	"testing"
)

func TestGenerateRandomPayload(t *testing.T) {
	tests := []struct {
		name string
		size int
	}{
		{"small payload", 10},
		{"1KB payload", 1024},
		{"4KB payload", 4096},
		{"empty payload", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := GenerateRandomPayload(tt.size)

			if len(payload) != tt.size {
				t.Errorf("expected payload size %d, got %d", tt.size, len(payload))
			}

			// Verify randomness by generating two payloads and comparing
			if tt.size > 0 {
				payload2 := GenerateRandomPayload(tt.size)
				if bytes.Equal(payload, payload2) {
					t.Error("two random payloads should not be identical")
				}
			}
		})
	}
}

func TestGenerateSequentialPayload(t *testing.T) {
	tests := []struct {
		name   string
		size   int
		seqNum uint64
	}{
		{"small payload with seq 0", 8, 0},
		{"small payload with seq 1", 8, 1},
		{"1KB payload with seq 42", 1024, 42},
		{"large seq number", 1024, 18446744073709551615}, // max uint64
		{"size smaller than 8 bytes", 4, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := GenerateSequentialPayload(tt.size, tt.seqNum)

			// Verify minimum size
			if len(payload) < 8 {
				t.Errorf("payload should be at least 8 bytes, got %d", len(payload))
			}

			// Extract and verify sequence number
			extractedSeq, ok := ExtractSequenceNumber(payload)
			if !ok {
				t.Fatal("failed to extract sequence number")
			}

			if extractedSeq != tt.seqNum {
				t.Errorf("expected sequence number %d, got %d", tt.seqNum, extractedSeq)
			}

			// Verify remaining bytes are filled (if size > 8)
			if tt.size > 8 && len(payload) > 8 {
				// Check that not all remaining bytes are zero
				allZero := true
				for i := 8; i < len(payload); i++ {
					if payload[i] != 0 {
						allZero = false
						break
					}
				}
				if allZero {
					t.Error("expected random data after sequence number, got all zeros")
				}
			}
		})
	}
}

func TestExtractSequenceNumber(t *testing.T) {
	tests := []struct {
		name      string
		payload   []byte
		expectSeq uint64
		expectOk  bool
	}{
		{
			name:      "valid payload with seq 42",
			payload:   GenerateSequentialPayload(1024, 42),
			expectSeq: 42,
			expectOk:  true,
		},
		{
			name:      "valid payload with seq 0",
			payload:   GenerateSequentialPayload(8, 0),
			expectSeq: 0,
			expectOk:  true,
		},
		{
			name:      "payload too small",
			payload:   []byte{1, 2, 3},
			expectSeq: 0,
			expectOk:  false,
		},
		{
			name:      "empty payload",
			payload:   []byte{},
			expectSeq: 0,
			expectOk:  false,
		},
		{
			name:      "nil payload",
			payload:   nil,
			expectSeq: 0,
			expectOk:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq, ok := ExtractSequenceNumber(tt.payload)

			if ok != tt.expectOk {
				t.Errorf("expected ok=%v, got ok=%v", tt.expectOk, ok)
			}

			if ok && seq != tt.expectSeq {
				t.Errorf("expected sequence %d, got %d", tt.expectSeq, seq)
			}
		})
	}
}

func TestGeneratePatternPayload(t *testing.T) {
	tests := []struct {
		name            string
		size            int
		pattern         string
		expectedPrefix  string
		expectedLen     int
		checkFullBuffer bool
	}{
		{
			name:           "simple pattern",
			size:           12,
			pattern:        "ABC",
			expectedPrefix: "ABCABCABCABC",
			expectedLen:    12,
		},
		{
			name:           "pattern longer than size",
			size:           5,
			pattern:        "ABCDEFGH",
			expectedPrefix: "ABCDE",
			expectedLen:    5,
		},
		{
			name:           "single character pattern",
			size:           10,
			pattern:        "X",
			expectedPrefix: "XXXXXXXXXX",
			expectedLen:    10,
		},
		{
			name:        "empty pattern",
			size:        10,
			pattern:     "",
			expectedLen: 10,
		},
		{
			name:        "zero size",
			size:        0,
			pattern:     "ABC",
			expectedLen: 0,
		},
		{
			name:           "pattern doesn't divide evenly",
			size:           10,
			pattern:        "ABC",
			expectedPrefix: "ABCABCABCA",
			expectedLen:    10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := GeneratePatternPayload(tt.size, tt.pattern)

			if len(payload) != tt.expectedLen {
				t.Errorf("expected length %d, got %d", tt.expectedLen, len(payload))
			}

			if tt.expectedPrefix != "" {
				actual := string(payload)
				if actual != tt.expectedPrefix {
					t.Errorf("expected prefix %q, got %q", tt.expectedPrefix, actual)
				}
			}

			// For empty pattern, verify all zeros
			if tt.pattern == "" && tt.size > 0 {
				for i, b := range payload {
					if b != 0 {
						t.Errorf("expected zero byte at position %d, got %d", i, b)
						break
					}
				}
			}
		})
	}
}

func TestPayloadPool(t *testing.T) {
	t.Run("basic get and put", func(t *testing.T) {
		pool := NewPayloadPool(1024, 10)

		buf := pool.Get()
		if len(buf) != 1024 {
			t.Errorf("expected buffer size 1024, got %d", len(buf))
		}

		pool.Put(buf)
	})

	t.Run("reuse buffer", func(t *testing.T) {
		pool := NewPayloadPool(100, 10)

		// Get buffer and mark it
		buf1 := pool.Get()
		buf1[0] = 0xFF
		addr1 := &buf1[0]
		pool.Put(buf1)

		// Get buffer again - might be the same one
		buf2 := pool.Get()
		addr2 := &buf2[0]

		// If addresses match, the buffer was reused
		// This is not guaranteed but likely in this simple test
		if addr1 == addr2 {
			t.Log("Buffer was reused (expected behavior)")
		}

		pool.Put(buf2)
	})

	t.Run("concurrent access", func(t *testing.T) {
		pool := NewPayloadPool(1024, 100)
		var wg sync.WaitGroup
		concurrency := 10
		iterations := 100

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					buf := pool.Get()
					if len(buf) != 1024 {
						t.Errorf("expected buffer size 1024, got %d", len(buf))
					}
					// Simulate some work
					buf[0] = byte(j)
					pool.Put(buf)
				}
			}()
		}

		wg.Wait()
	})
}

func TestGenerateRandomPayloadTo(t *testing.T) {
	t.Run("fills entire buffer", func(t *testing.T) {
		buf := make([]byte, 1024)
		result := GenerateRandomPayloadTo(buf)

		if len(result) != 1024 {
			t.Errorf("expected result length 1024, got %d", len(result))
		}

		// Verify not all zeros
		allZero := true
		for _, b := range result {
			if b != 0 {
				allZero = false
				break
			}
		}
		if allZero {
			t.Error("expected random data, got all zeros")
		}
	})

	t.Run("works with pooled buffer", func(t *testing.T) {
		pool := NewPayloadPool(1024, 10)
		buf := pool.Get()

		GenerateRandomPayloadTo(buf)

		// Verify buffer is filled
		allZero := true
		for _, b := range buf {
			if b != 0 {
				allZero = false
				break
			}
		}
		if allZero {
			t.Error("expected random data, got all zeros")
		}

		pool.Put(buf)
	})
}

func TestGenerateSequentialPayloadTo(t *testing.T) {
	t.Run("fills buffer with sequence", func(t *testing.T) {
		buf := make([]byte, 1024)
		seqNum := uint64(12345)

		result := GenerateSequentialPayloadTo(buf, seqNum)

		// Verify sequence number
		extracted, ok := ExtractSequenceNumber(result)
		if !ok {
			t.Fatal("failed to extract sequence number")
		}
		if extracted != seqNum {
			t.Errorf("expected sequence %d, got %d", seqNum, extracted)
		}
	})

	t.Run("panics on small buffer", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for buffer < 8 bytes")
			}
		}()

		buf := make([]byte, 4)
		GenerateSequentialPayloadTo(buf, 123)
	})

	t.Run("works with pooled buffer", func(t *testing.T) {
		pool := NewPayloadPool(1024, 10)
		buf := pool.Get()

		GenerateSequentialPayloadTo(buf, 999)

		extracted, ok := ExtractSequenceNumber(buf)
		if !ok || extracted != 999 {
			t.Errorf("expected sequence 999, got %d (ok=%v)", extracted, ok)
		}

		pool.Put(buf)
	})
}

func TestGeneratePatternPayloadTo(t *testing.T) {
	t.Run("fills buffer with pattern", func(t *testing.T) {
		buf := make([]byte, 12)
		result := GeneratePatternPayloadTo(buf, "ABC")

		expected := "ABCABCABCABC"
		if string(result) != expected {
			t.Errorf("expected %q, got %q", expected, string(result))
		}
	})

	t.Run("empty pattern returns unchanged buffer", func(t *testing.T) {
		buf := make([]byte, 10)
		copy(buf, []byte("ORIGINAL"))

		result := GeneratePatternPayloadTo(buf, "")

		// Buffer should be unchanged
		if string(result[:8]) != "ORIGINAL" {
			t.Error("expected buffer to remain unchanged with empty pattern")
		}
	})

	t.Run("works with pooled buffer", func(t *testing.T) {
		pool := NewPayloadPool(100, 10)
		buf := pool.Get()

		GeneratePatternPayloadTo(buf, "TEST")

		// Verify pattern repetition
		if string(buf[:4]) != "TEST" {
			t.Errorf("expected pattern to start with TEST, got %q", string(buf[:4]))
		}

		pool.Put(buf)
	})
}

// Benchmark tests

func BenchmarkGenerateRandomPayload(b *testing.B) {
	sizes := []int{64, 256, 1024, 4096}

	for _, size := range sizes {
		b.Run(string(rune(size)), func(b *testing.B) {
			b.SetBytes(int64(size))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = GenerateRandomPayload(size)
			}
		})
	}
}

func BenchmarkGenerateSequentialPayload(b *testing.B) {
	sizes := []int{64, 256, 1024, 4096}

	for _, size := range sizes {
		b.Run(string(rune(size)), func(b *testing.B) {
			b.SetBytes(int64(size))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = GenerateSequentialPayload(size, uint64(i))
			}
		})
	}
}

func BenchmarkGeneratePatternPayload(b *testing.B) {
	sizes := []int{64, 256, 1024, 4096}
	pattern := "TESTDATA"

	for _, size := range sizes {
		b.Run(string(rune(size)), func(b *testing.B) {
			b.SetBytes(int64(size))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = GeneratePatternPayload(size, pattern)
			}
		})
	}
}

func BenchmarkPayloadPoolGetPut(b *testing.B) {
	pool := NewPayloadPool(1024, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := pool.Get()
		pool.Put(buf)
	}
}

func BenchmarkGenerateRandomPayloadTo(b *testing.B) {
	sizes := []int{64, 256, 1024, 4096}

	for _, size := range sizes {
		b.Run(string(rune(size)), func(b *testing.B) {
			pool := NewPayloadPool(size, 100)
			b.SetBytes(int64(size))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				buf := pool.Get()
				GenerateRandomPayloadTo(buf)
				pool.Put(buf)
			}
		})
	}
}

func BenchmarkGenerateSequentialPayloadTo(b *testing.B) {
	sizes := []int{64, 256, 1024, 4096}

	for _, size := range sizes {
		b.Run(string(rune(size)), func(b *testing.B) {
			pool := NewPayloadPool(size, 100)
			b.SetBytes(int64(size))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				buf := pool.Get()
				GenerateSequentialPayloadTo(buf, uint64(i))
				pool.Put(buf)
			}
		})
	}
}

func BenchmarkGeneratePatternPayloadTo(b *testing.B) {
	sizes := []int{64, 256, 1024, 4096}
	pattern := "TESTDATA"

	for _, size := range sizes {
		b.Run(string(rune(size)), func(b *testing.B) {
			pool := NewPayloadPool(size, 100)
			b.SetBytes(int64(size))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				buf := pool.Get()
				GeneratePatternPayloadTo(buf, pattern)
				pool.Put(buf)
			}
		})
	}
}

func BenchmarkPayloadPoolParallel(b *testing.B) {
	pool := NewPayloadPool(1024, 100)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := pool.Get()
			GenerateRandomPayloadTo(buf)
			pool.Put(buf)
		}
	})
}

func BenchmarkExtractSequenceNumber(b *testing.B) {
	payload := GenerateSequentialPayload(1024, 12345)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ExtractSequenceNumber(payload)
	}
}
