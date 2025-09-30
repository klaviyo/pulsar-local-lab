package pulsar

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/pulsar-local-lab/perf-test/internal/config"
)

// mockProducer implements pulsar.Producer interface for testing
type mockProducer struct {
	sendFunc      func(context.Context, *pulsar.ProducerMessage) (pulsar.MessageID, error)
	sendAsyncFunc func(context.Context, *pulsar.ProducerMessage, func(pulsar.MessageID, *pulsar.ProducerMessage, error))
	flushFunc     func() error
	closeFunc     func()
	sendCount     uint64
}

func (m *mockProducer) Send(ctx context.Context, msg *pulsar.ProducerMessage) (pulsar.MessageID, error) {
	if m.sendFunc != nil {
		return m.sendFunc(ctx, msg)
	}
	atomic.AddUint64(&m.sendCount, 1)
	return &mockMessageID{id: 1}, nil
}

func (m *mockProducer) SendAsync(ctx context.Context, msg *pulsar.ProducerMessage, callback func(pulsar.MessageID, *pulsar.ProducerMessage, error)) {
	if m.sendAsyncFunc != nil {
		m.sendAsyncFunc(ctx, msg, callback)
		return
	}
	atomic.AddUint64(&m.sendCount, 1)
	go callback(&mockMessageID{id: 1}, msg, nil)
}

func (m *mockProducer) Flush() error {
	if m.flushFunc != nil {
		return m.flushFunc()
	}
	return nil
}

func (m *mockProducer) FlushWithCtx(ctx context.Context) error {
	if m.flushFunc != nil {
		return m.flushFunc()
	}
	return nil
}

func (m *mockProducer) Close() {
	if m.closeFunc != nil {
		m.closeFunc()
	}
}

func (m *mockProducer) Topic() string                          { return "test-topic" }
func (m *mockProducer) Name() string                           { return "test-producer" }
func (m *mockProducer) LastSequenceID() int64                  { return 0 }
func (m *mockProducer) Schema() pulsar.Schema                  { return nil }

func TestGetCompressionType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected pulsar.CompressionType
	}{
		{"LZ4", "LZ4", pulsar.LZ4},
		{"ZLIB", "ZLIB", pulsar.ZLib},
		{"ZSTD", "ZSTD", pulsar.ZSTD},
		{"NONE", "NONE", pulsar.NoCompression},
		{"Invalid", "INVALID", pulsar.NoCompression},
		{"Empty", "", pulsar.NoCompression},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getCompressionType(tt.input)
			if result != tt.expected {
				t.Errorf("getCompressionType(%s) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNewProducer_ValidationErrors(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		pulsarCfg   *config.PulsarConfig
		producerCfg *config.ProducerConfig
		wantErr     bool
		errContains string
	}{
		{
			name:        "nil pulsar config",
			pulsarCfg:   nil,
			producerCfg: &config.ProducerConfig{},
			wantErr:     true,
			errContains: "pulsar config cannot be nil",
		},
		{
			name:        "nil producer config",
			pulsarCfg:   &config.PulsarConfig{ServiceURL: "pulsar://localhost:6650", Topic: "test"},
			producerCfg: nil,
			wantErr:     true,
			errContains: "producer config cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewProducer(ctx, tt.pulsarCfg, tt.producerCfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewProducer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errContains != "" {
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("NewProducer() error = %v, want error containing %s", err, tt.errContains)
				}
			}
		})
	}
}

func TestProducerClient_SendOperations(t *testing.T) {
	// Create producer client with mock
	pc := &ProducerClient{
		pulsarCfg: &config.PulsarConfig{
			ServiceURL: "pulsar://localhost:6650",
			Topic:      "test-topic",
		},
		producerCfg: &config.ProducerConfig{},
		producer:    &mockProducer{},
		connected:   true,
		closed:      false,
	}

	ctx := context.Background()

	t.Run("Send success", func(t *testing.T) {
		payload := []byte("test message")
		msgID, err := pc.Send(ctx, payload)
		if err != nil {
			t.Errorf("Send() error = %v, want nil", err)
		}
		if msgID == nil {
			t.Error("Send() returned nil message ID")
		}

		stats := pc.Stats()
		if stats.MessagesSent != 1 {
			t.Errorf("Stats.MessagesSent = %d, want 1", stats.MessagesSent)
		}
		if stats.BytesSent != uint64(len(payload)) {
			t.Errorf("Stats.BytesSent = %d, want %d", stats.BytesSent, len(payload))
		}
	})

	t.Run("Send with properties", func(t *testing.T) {
		payload := []byte("test message")
		properties := map[string]string{"key": "value"}

		msgID, err := pc.SendWithProperties(ctx, payload, properties)
		if err != nil {
			t.Errorf("SendWithProperties() error = %v, want nil", err)
		}
		if msgID == nil {
			t.Error("SendWithProperties() returned nil message ID")
		}
	})

	t.Run("Send when not connected", func(t *testing.T) {
		pc.connected = false
		_, err := pc.Send(ctx, []byte("test"))
		if err == nil {
			t.Error("Send() error = nil, want error when not connected")
		}
		pc.connected = true
	})

	t.Run("Send when closed", func(t *testing.T) {
		pc.closed = true
		_, err := pc.Send(ctx, []byte("test"))
		if err == nil {
			t.Error("Send() error = nil, want error when closed")
		}
		pc.closed = false
	})
}

func TestProducerClient_SendAsync(t *testing.T) {
	// Create producer client with mock
	pc := &ProducerClient{
		pulsarCfg: &config.PulsarConfig{
			ServiceURL: "pulsar://localhost:6650",
			Topic:      "test-topic",
		},
		producerCfg: &config.ProducerConfig{},
		producer:    &mockProducer{},
		connected:   true,
		closed:      false,
	}

	ctx := context.Background()

	t.Run("SendAsync success", func(t *testing.T) {
		callbackCalled := make(chan bool, 1)
		pc.SendAsync(ctx, []byte("test"), func(msgID pulsar.MessageID, msg *pulsar.ProducerMessage, err error) {
			if err != nil {
				t.Errorf("SendAsync callback error = %v, want nil", err)
			}
			if msgID == nil {
				t.Error("SendAsync callback received nil message ID")
			}
			callbackCalled <- true
		})

		select {
		case <-callbackCalled:
			// Success
		case <-time.After(1 * time.Second):
			t.Error("SendAsync callback was not called within timeout")
		}
	})

	t.Run("SendAsync when not connected", func(t *testing.T) {
		pc.connected = false
		callbackCalled := make(chan bool, 1)
		pc.SendAsync(ctx, []byte("test"), func(msgID pulsar.MessageID, msg *pulsar.ProducerMessage, err error) {
			if err == nil {
				t.Error("SendAsync callback error = nil, want error when not connected")
			}
			callbackCalled <- true
		})

		select {
		case <-callbackCalled:
			// Success
		case <-time.After(1 * time.Second):
			t.Error("SendAsync callback was not called within timeout")
		}
		pc.connected = true
	})

	t.Run("SendAsync with nil callback", func(t *testing.T) {
		// Should not panic with nil callback
		pc.SendAsync(ctx, []byte("test"), nil)
		time.Sleep(100 * time.Millisecond) // Give time for async operation
	})
}

func TestProducerClient_SendAsyncErrorHandling(t *testing.T) {
	expectedErr := errors.New("send failed")

	mock := &mockProducer{
		sendAsyncFunc: func(ctx context.Context, msg *pulsar.ProducerMessage, callback func(pulsar.MessageID, *pulsar.ProducerMessage, error)) {
			go callback(nil, msg, expectedErr)
		},
	}

	pc := &ProducerClient{
		pulsarCfg: &config.PulsarConfig{
			ServiceURL: "pulsar://localhost:6650",
			Topic:      "test-topic",
		},
		producerCfg: &config.ProducerConfig{},
		producer:    mock,
		connected:   true,
		closed:      false,
	}

	ctx := context.Background()
	callbackCalled := make(chan bool, 1)

	pc.SendAsync(ctx, []byte("test"), func(msgID pulsar.MessageID, msg *pulsar.ProducerMessage, err error) {
		if err == nil {
			t.Error("SendAsync callback error = nil, want error")
		}
		if err != expectedErr {
			t.Errorf("SendAsync callback error = %v, want %v", err, expectedErr)
		}
		callbackCalled <- true
	})

	select {
	case <-callbackCalled:
		// Verify stats were updated
		stats := pc.Stats()
		if stats.MessageFailures != 1 {
			t.Errorf("Stats.MessageFailures = %d, want 1", stats.MessageFailures)
		}
	case <-time.After(1 * time.Second):
		t.Error("SendAsync callback was not called within timeout")
	}
}

func TestProducerClient_Flush(t *testing.T) {
	t.Run("Flush success", func(t *testing.T) {
		pc := &ProducerClient{
			pulsarCfg: &config.PulsarConfig{
				ServiceURL: "pulsar://localhost:6650",
				Topic:      "test-topic",
			},
			producerCfg: &config.ProducerConfig{},
			producer:    &mockProducer{},
			connected:   true,
			closed:      false,
		}

		if err := pc.Flush(); err != nil {
			t.Errorf("Flush() error = %v, want nil", err)
		}
	})

	t.Run("Flush when not connected", func(t *testing.T) {
		pc := &ProducerClient{
			connected: false,
		}

		if err := pc.Flush(); err == nil {
			t.Error("Flush() error = nil, want error when not connected")
		}
	})

	t.Run("Flush error", func(t *testing.T) {
		expectedErr := errors.New("flush failed")
		mock := &mockProducer{
			flushFunc: func() error {
				return expectedErr
			},
		}

		pc := &ProducerClient{
			pulsarCfg: &config.PulsarConfig{
				ServiceURL: "pulsar://localhost:6650",
				Topic:      "test-topic",
			},
			producerCfg: &config.ProducerConfig{},
			producer:    mock,
			connected:   true,
			closed:      false,
		}

		err := pc.Flush()
		if err == nil {
			t.Error("Flush() error = nil, want error")
		}
	})
}

func TestProducerClient_Close(t *testing.T) {
	t.Run("Close success", func(t *testing.T) {
		closeCalled := false
		mock := &mockProducer{
			closeFunc: func() {
				closeCalled = true
			},
		}

		pc := &ProducerClient{
			pulsarCfg: &config.PulsarConfig{
				ServiceURL: "pulsar://localhost:6650",
				Topic:      "test-topic",
			},
			producerCfg: &config.ProducerConfig{},
			producer:    mock,
			connected:   true,
			closed:      false,
		}

		if err := pc.Close(); err != nil {
			t.Errorf("Close() error = %v, want nil", err)
		}

		if !closeCalled {
			t.Error("Close() did not call producer.Close()")
		}

		if pc.IsConnected() {
			t.Error("IsConnected() = true after Close(), want false")
		}
	})

	t.Run("Close idempotent", func(t *testing.T) {
		pc := &ProducerClient{
			pulsarCfg: &config.PulsarConfig{
				ServiceURL: "pulsar://localhost:6650",
				Topic:      "test-topic",
			},
			producerCfg: &config.ProducerConfig{},
			producer:    &mockProducer{},
			connected:   true,
			closed:      false,
		}

		if err := pc.Close(); err != nil {
			t.Errorf("First Close() error = %v, want nil", err)
		}

		// Second close should not error
		if err := pc.Close(); err != nil {
			t.Errorf("Second Close() error = %v, want nil", err)
		}
	})
}

func TestProducerClient_IsConnected(t *testing.T) {
	tests := []struct {
		name      string
		connected bool
		closed    bool
		want      bool
	}{
		{"Connected and open", true, false, true},
		{"Not connected", false, false, false},
		{"Connected but closed", true, true, false},
		{"Not connected and closed", false, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := &ProducerClient{
				connected: tt.connected,
				closed:    tt.closed,
			}

			if got := pc.IsConnected(); got != tt.want {
				t.Errorf("IsConnected() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProducerClient_Stats(t *testing.T) {
	pc := &ProducerClient{
		pulsarCfg: &config.PulsarConfig{
			ServiceURL: "pulsar://localhost:6650",
			Topic:      "test-topic",
		},
		producerCfg: &config.ProducerConfig{},
		producer:    &mockProducer{},
		connected:   true,
		closed:      false,
	}

	ctx := context.Background()

	// Send multiple messages
	for i := 0; i < 5; i++ {
		_, err := pc.Send(ctx, []byte("test message"))
		if err != nil {
			t.Errorf("Send() error = %v, want nil", err)
		}
	}

	stats := pc.Stats()

	if stats.MessagesSent != 5 {
		t.Errorf("Stats.MessagesSent = %d, want 5", stats.MessagesSent)
	}

	if stats.BytesSent != 5*12 { // 5 messages * 12 bytes
		t.Errorf("Stats.BytesSent = %d, want %d", stats.BytesSent, 5*12)
	}

	if stats.MessageFailures != 0 {
		t.Errorf("Stats.MessageFailures = %d, want 0", stats.MessageFailures)
	}
}

func TestProducerClient_LastError(t *testing.T) {
	expectedErr := errors.New("test error")

	mock := &mockProducer{
		sendFunc: func(ctx context.Context, msg *pulsar.ProducerMessage) (pulsar.MessageID, error) {
			return nil, expectedErr
		},
	}

	pc := &ProducerClient{
		pulsarCfg: &config.PulsarConfig{
			ServiceURL: "pulsar://localhost:6650",
			Topic:      "test-topic",
		},
		producerCfg: &config.ProducerConfig{},
		producer:    mock,
		connected:   true,
		closed:      false,
	}

	ctx := context.Background()
	_, err := pc.Send(ctx, []byte("test"))
	if err == nil {
		t.Error("Send() error = nil, want error")
	}

	lastErr := pc.LastError()
	if lastErr != expectedErr {
		t.Errorf("LastError() = %v, want %v", lastErr, expectedErr)
	}
}

func TestProducerClient_ConcurrentSends(t *testing.T) {
	pc := &ProducerClient{
		pulsarCfg: &config.PulsarConfig{
			ServiceURL: "pulsar://localhost:6650",
			Topic:      "test-topic",
		},
		producerCfg: &config.ProducerConfig{},
		producer:    &mockProducer{},
		connected:   true,
		closed:      false,
	}

	ctx := context.Background()
	numGoroutines := 100
	numMessages := 10

	done := make(chan bool, numGoroutines)

	// Launch multiple goroutines sending messages concurrently
	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < numMessages; j++ {
				_, err := pc.Send(ctx, []byte("test message"))
				if err != nil {
					t.Errorf("Send() error = %v, want nil", err)
				}
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		select {
		case <-done:
			// Success
		case <-time.After(5 * time.Second):
			t.Fatal("Concurrent sends timed out")
		}
	}

	stats := pc.Stats()
	expectedCount := uint64(numGoroutines * numMessages)
	if stats.MessagesSent != expectedCount {
		t.Errorf("Stats.MessagesSent = %d, want %d", stats.MessagesSent, expectedCount)
	}
}

func TestNewProducerClient_LegacyWrapper(t *testing.T) {
	// Test the legacy wrapper function for backward compatibility
	// This will fail to connect without a real Pulsar cluster, which is expected
	cfg := config.DefaultConfig("")
	cfg.Pulsar.ServiceURL = "pulsar://invalid-host:6650"

	_, err := NewProducerClient(cfg)
	if err == nil {
		t.Error("NewProducerClient() with invalid host error = nil, want error")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && hasSubstring(s, substr)))
}

func hasSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}