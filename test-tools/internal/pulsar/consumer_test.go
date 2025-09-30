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

// mockConsumer implements pulsar.Consumer interface for testing
type mockConsumer struct {
	receiveFunc     func(context.Context) (pulsar.Message, error)
	ackFunc         func(pulsar.Message)
	ackIDFunc       func(pulsar.MessageID) error
	nackFunc        func(pulsar.Message)
	nackIDFunc      func(pulsar.MessageID)
	seekFunc        func(pulsar.MessageID) error
	seekByTimeFunc  func(time.Time) error
	unsubscribeFunc func() error
	closeFunc       func()
	chanFunc        func() <-chan pulsar.ConsumerMessage
	receiveCount    uint64
	ackCount        uint64
	nackCount       uint64
}

func (m *mockConsumer) Subscription() string { return "test-subscription" }
func (m *mockConsumer) Topic() string        { return "test-topic" }
func (m *mockConsumer) Unsubscribe() error {
	if m.unsubscribeFunc != nil {
		return m.unsubscribeFunc()
	}
	return nil
}

func (m *mockConsumer) Receive(ctx context.Context) (pulsar.Message, error) {
	if m.receiveFunc != nil {
		return m.receiveFunc(ctx)
	}
	atomic.AddUint64(&m.receiveCount, 1)
	return &mockMessage{payload: []byte("test message")}, nil
}

func (m *mockConsumer) Ack(msg pulsar.Message) error {
	if m.ackFunc != nil {
		m.ackFunc(msg)
		return nil
	}
	atomic.AddUint64(&m.ackCount, 1)
	return nil
}

func (m *mockConsumer) AckID(msgID pulsar.MessageID) error {
	if m.ackIDFunc != nil {
		return m.ackIDFunc(msgID)
	}
	atomic.AddUint64(&m.ackCount, 1)
	return nil
}

func (m *mockConsumer) AckCumulative(msg pulsar.Message) error {
	atomic.AddUint64(&m.ackCount, 1)
	return nil
}

func (m *mockConsumer) AckIDCumulative(msgID pulsar.MessageID) error {
	atomic.AddUint64(&m.ackCount, 1)
	return nil
}

func (m *mockConsumer) AckWithResponse(msg pulsar.Message) error {
	atomic.AddUint64(&m.ackCount, 1)
	return nil
}

func (m *mockConsumer) AckIDWithResponse(msgID pulsar.MessageID) error {
	atomic.AddUint64(&m.ackCount, 1)
	return nil
}

func (m *mockConsumer) AckCumulativeWithResponse(msg pulsar.Message) error {
	atomic.AddUint64(&m.ackCount, 1)
	return nil
}

func (m *mockConsumer) AckIDCumulativeWithResponse(msgID pulsar.MessageID) error {
	atomic.AddUint64(&m.ackCount, 1)
	return nil
}

func (m *mockConsumer) AckWithTxn(msg pulsar.Message, txn pulsar.Transaction) error {
	atomic.AddUint64(&m.ackCount, 1)
	return nil
}

func (m *mockConsumer) AckIDWithTxn(msgID pulsar.MessageID, txn pulsar.Transaction) error {
	atomic.AddUint64(&m.ackCount, 1)
	return nil
}

func (m *mockConsumer) AckCumulativeWithTxn(msg pulsar.Message, txn pulsar.Transaction) error {
	atomic.AddUint64(&m.ackCount, 1)
	return nil
}

func (m *mockConsumer) AckIDCumulativeWithTxn(msgID pulsar.MessageID, txn pulsar.Transaction) error {
	atomic.AddUint64(&m.ackCount, 1)
	return nil
}

func (m *mockConsumer) Nack(msg pulsar.Message) {
	if m.nackFunc != nil {
		m.nackFunc(msg)
		return
	}
	atomic.AddUint64(&m.nackCount, 1)
}

func (m *mockConsumer) NackID(msgID pulsar.MessageID) {
	if m.nackIDFunc != nil {
		m.nackIDFunc(msgID)
		return
	}
	atomic.AddUint64(&m.nackCount, 1)
}

func (m *mockConsumer) Seek(msgID pulsar.MessageID) error {
	if m.seekFunc != nil {
		return m.seekFunc(msgID)
	}
	return nil
}

func (m *mockConsumer) SeekByTime(t time.Time) error {
	if m.seekByTimeFunc != nil {
		return m.seekByTimeFunc(t)
	}
	return nil
}

func (m *mockConsumer) Close() {
	if m.closeFunc != nil {
		m.closeFunc()
	}
}

func (m *mockConsumer) Chan() <-chan pulsar.ConsumerMessage {
	if m.chanFunc != nil {
		return m.chanFunc()
	}
	ch := make(chan pulsar.ConsumerMessage)
	close(ch)
	return ch
}

func (m *mockConsumer) ReconsumeLater(msg pulsar.Message, delay time.Duration) {}
func (m *mockConsumer) ReconsumeLaterWithCustomProperties(msg pulsar.Message, customProperties map[string]string, delay time.Duration) {
}
func (m *mockConsumer) Name() string { return "test-consumer" }

// mockMessage implements pulsar.Message interface for testing
type mockMessage struct {
	payload    []byte
	properties map[string]string
	key        string
	topic      string
	msgID      pulsar.MessageID
}

func (m *mockMessage) Topic() string                             { return m.topic }
func (m *mockMessage) Properties() map[string]string             { return m.properties }
func (m *mockMessage) Payload() []byte                           { return m.payload }
func (m *mockMessage) ID() pulsar.MessageID                      { return m.msgID }
func (m *mockMessage) PublishTime() time.Time                    { return time.Now() }
func (m *mockMessage) EventTime() time.Time                      { return time.Now() }
func (m *mockMessage) Key() string                               { return m.key }
func (m *mockMessage) OrderingKey() string                       { return "" }
func (m *mockMessage) RedeliveryCount() uint32                   { return 0 }
func (m *mockMessage) IsReplicated() bool                        { return false }
func (m *mockMessage) GetReplicatedFrom() string                 { return "" }
func (m *mockMessage) GetSchemaValue(v interface{}) error        { return nil }
func (m *mockMessage) SchemaVersion() []byte                     { return nil }
func (m *mockMessage) ProducerName() string                      { return "test-producer" }
func (m *mockMessage) GetEncryptionContext() *pulsar.EncryptionContext { return nil }
func (m *mockMessage) Index() *uint64                            { return nil }
func (m *mockMessage) BrokerPublishTime() *time.Time            { return nil }

func TestGetSubscriptionType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected pulsar.SubscriptionType
	}{
		{"Exclusive", "Exclusive", pulsar.Exclusive},
		{"Shared", "Shared", pulsar.Shared},
		{"Failover", "Failover", pulsar.Failover},
		{"KeyShared", "KeyShared", pulsar.KeyShared},
		{"Invalid", "Invalid", pulsar.Shared},
		{"Empty", "", pulsar.Shared},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getSubscriptionType(tt.input)
			if result != tt.expected {
				t.Errorf("getSubscriptionType(%s) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNewConsumer_ValidationErrors(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		pulsarCfg   *config.PulsarConfig
		consumerCfg *config.ConsumerConfig
		consumerID  string
		wantErr     bool
		errContains string
	}{
		{
			name:        "nil pulsar config",
			pulsarCfg:   nil,
			consumerCfg: &config.ConsumerConfig{},
			consumerID:  "test",
			wantErr:     true,
			errContains: "pulsar config cannot be nil",
		},
		{
			name:        "nil consumer config",
			pulsarCfg:   &config.PulsarConfig{ServiceURL: "pulsar://localhost:6650", Topic: "test"},
			consumerCfg: nil,
			consumerID:  "test",
			wantErr:     true,
			errContains: "consumer config cannot be nil",
		},
		{
			name:        "empty consumer ID",
			pulsarCfg:   &config.PulsarConfig{ServiceURL: "pulsar://localhost:6650", Topic: "test"},
			consumerCfg: &config.ConsumerConfig{},
			consumerID:  "",
			wantErr:     true,
			errContains: "consumer ID cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewConsumer(ctx, tt.pulsarCfg, tt.consumerCfg, tt.consumerID)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewConsumer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errContains != "" {
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("NewConsumer() error = %v, want error containing %s", err, tt.errContains)
				}
			}
		})
	}
}

func TestConsumerClient_Receive(t *testing.T) {
	t.Run("Receive success", func(t *testing.T) {
		cc := &ConsumerClient{
			pulsarCfg: &config.PulsarConfig{
				ServiceURL: "pulsar://localhost:6650",
				Topic:      "test-topic",
			},
			consumerCfg: &config.ConsumerConfig{
				SubscriptionName: "test-sub",
			},
			consumerID: "test-consumer",
			consumer:   &mockConsumer{},
			connected:  true,
			closed:     false,
		}

		ctx := context.Background()
		msg, err := cc.Receive(ctx)
		if err != nil {
			t.Errorf("Receive() error = %v, want nil", err)
		}
		if msg == nil {
			t.Error("Receive() returned nil message")
		}

		stats := cc.Stats()
		if stats.MessagesReceived != 1 {
			t.Errorf("Stats.MessagesReceived = %d, want 1", stats.MessagesReceived)
		}
	})

	t.Run("Receive when not connected", func(t *testing.T) {
		cc := &ConsumerClient{
			connected: false,
		}

		ctx := context.Background()
		_, err := cc.Receive(ctx)
		if err == nil {
			t.Error("Receive() error = nil, want error when not connected")
		}
	})

	t.Run("Receive when closed", func(t *testing.T) {
		cc := &ConsumerClient{
			closed: true,
		}

		ctx := context.Background()
		_, err := cc.Receive(ctx)
		if err == nil {
			t.Error("Receive() error = nil, want error when closed")
		}
	})

	t.Run("Receive error", func(t *testing.T) {
		expectedErr := errors.New("receive failed")
		mock := &mockConsumer{
			receiveFunc: func(ctx context.Context) (pulsar.Message, error) {
				return nil, expectedErr
			},
		}

		cc := &ConsumerClient{
			pulsarCfg: &config.PulsarConfig{
				ServiceURL: "pulsar://localhost:6650",
				Topic:      "test-topic",
			},
			consumerCfg: &config.ConsumerConfig{
				SubscriptionName: "test-sub",
			},
			consumerID: "test-consumer",
			consumer:   mock,
			connected:  true,
			closed:     false,
		}

		ctx := context.Background()
		_, err := cc.Receive(ctx)
		if err == nil {
			t.Error("Receive() error = nil, want error")
		}

		stats := cc.Stats()
		if stats.ReceiveErrors != 1 {
			t.Errorf("Stats.ReceiveErrors = %d, want 1", stats.ReceiveErrors)
		}
	})
}

func TestConsumerClient_Ack(t *testing.T) {
	t.Run("Ack success", func(t *testing.T) {
		mock := &mockConsumer{}
		cc := &ConsumerClient{
			pulsarCfg: &config.PulsarConfig{
				ServiceURL: "pulsar://localhost:6650",
				Topic:      "test-topic",
			},
			consumerCfg: &config.ConsumerConfig{
				SubscriptionName: "test-sub",
			},
			consumerID: "test-consumer",
			consumer:   mock,
			connected:  true,
			closed:     false,
		}

		msg := &mockMessage{payload: []byte("test")}
		err := cc.Ack(msg)
		if err != nil {
			t.Errorf("Ack() error = %v, want nil", err)
		}

		if atomic.LoadUint64(&mock.ackCount) != 1 {
			t.Errorf("Mock ackCount = %d, want 1", mock.ackCount)
		}

		stats := cc.Stats()
		if stats.MessagesAcked != 1 {
			t.Errorf("Stats.MessagesAcked = %d, want 1", stats.MessagesAcked)
		}
	})

	t.Run("Ack when not connected", func(t *testing.T) {
		cc := &ConsumerClient{
			connected: false,
		}

		msg := &mockMessage{}
		err := cc.Ack(msg)
		if err == nil {
			t.Error("Ack() error = nil, want error when not connected")
		}
	})
}

func TestConsumerClient_AckID(t *testing.T) {
	t.Run("AckID success", func(t *testing.T) {
		mock := &mockConsumer{}
		cc := &ConsumerClient{
			pulsarCfg: &config.PulsarConfig{
				ServiceURL: "pulsar://localhost:6650",
				Topic:      "test-topic",
			},
			consumerCfg: &config.ConsumerConfig{
				SubscriptionName: "test-sub",
			},
			consumerID: "test-consumer",
			consumer:   mock,
			connected:  true,
			closed:     false,
		}

		msgID := &mockMessageID{id: 1}
		err := cc.AckID(msgID)
		if err != nil {
			t.Errorf("AckID() error = %v, want nil", err)
		}

		stats := cc.Stats()
		if stats.MessagesAcked != 1 {
			t.Errorf("Stats.MessagesAcked = %d, want 1", stats.MessagesAcked)
		}
	})

	t.Run("AckID error", func(t *testing.T) {
		expectedErr := errors.New("ack failed")
		mock := &mockConsumer{
			ackIDFunc: func(msgID pulsar.MessageID) error {
				return expectedErr
			},
		}

		cc := &ConsumerClient{
			pulsarCfg: &config.PulsarConfig{
				ServiceURL: "pulsar://localhost:6650",
				Topic:      "test-topic",
			},
			consumerCfg: &config.ConsumerConfig{
				SubscriptionName: "test-sub",
			},
			consumerID: "test-consumer",
			consumer:   mock,
			connected:  true,
			closed:     false,
		}

		msgID := &mockMessageID{id: 1}
		err := cc.AckID(msgID)
		if err == nil {
			t.Error("AckID() error = nil, want error")
		}
	})
}

func TestConsumerClient_Nack(t *testing.T) {
	t.Run("Nack success", func(t *testing.T) {
		mock := &mockConsumer{}
		cc := &ConsumerClient{
			pulsarCfg: &config.PulsarConfig{
				ServiceURL: "pulsar://localhost:6650",
				Topic:      "test-topic",
			},
			consumerCfg: &config.ConsumerConfig{
				SubscriptionName: "test-sub",
			},
			consumerID: "test-consumer",
			consumer:   mock,
			connected:  true,
			closed:     false,
		}

		msg := &mockMessage{payload: []byte("test")}
		err := cc.Nack(msg)
		if err != nil {
			t.Errorf("Nack() error = %v, want nil", err)
		}

		if atomic.LoadUint64(&mock.nackCount) != 1 {
			t.Errorf("Mock nackCount = %d, want 1", mock.nackCount)
		}

		stats := cc.Stats()
		if stats.MessagesNacked != 1 {
			t.Errorf("Stats.MessagesNacked = %d, want 1", stats.MessagesNacked)
		}
	})

	t.Run("Nack when not connected", func(t *testing.T) {
		cc := &ConsumerClient{
			connected: false,
		}

		msg := &mockMessage{}
		err := cc.Nack(msg)
		if err == nil {
			t.Error("Nack() error = nil, want error when not connected")
		}
	})
}

func TestConsumerClient_NackID(t *testing.T) {
	mock := &mockConsumer{}
	cc := &ConsumerClient{
		pulsarCfg: &config.PulsarConfig{
			ServiceURL: "pulsar://localhost:6650",
			Topic:      "test-topic",
		},
		consumerCfg: &config.ConsumerConfig{
			SubscriptionName: "test-sub",
		},
		consumerID: "test-consumer",
		consumer:   mock,
		connected:  true,
		closed:     false,
	}

	msgID := &mockMessageID{id: 1}
	err := cc.NackID(msgID)
	if err != nil {
		t.Errorf("NackID() error = %v, want nil", err)
	}

	stats := cc.Stats()
	if stats.MessagesNacked != 1 {
		t.Errorf("Stats.MessagesNacked = %d, want 1", stats.MessagesNacked)
	}
}

func TestConsumerClient_Seek(t *testing.T) {
	t.Run("Seek success", func(t *testing.T) {
		cc := &ConsumerClient{
			pulsarCfg: &config.PulsarConfig{
				ServiceURL: "pulsar://localhost:6650",
				Topic:      "test-topic",
			},
			consumerCfg: &config.ConsumerConfig{
				SubscriptionName: "test-sub",
			},
			consumerID: "test-consumer",
			consumer:   &mockConsumer{},
			connected:  true,
			closed:     false,
		}

		msgID := &mockMessageID{id: 1}
		err := cc.Seek(msgID)
		if err != nil {
			t.Errorf("Seek() error = %v, want nil", err)
		}
	})

	t.Run("Seek error", func(t *testing.T) {
		expectedErr := errors.New("seek failed")
		mock := &mockConsumer{
			seekFunc: func(msgID pulsar.MessageID) error {
				return expectedErr
			},
		}

		cc := &ConsumerClient{
			pulsarCfg: &config.PulsarConfig{
				ServiceURL: "pulsar://localhost:6650",
				Topic:      "test-topic",
			},
			consumerCfg: &config.ConsumerConfig{
				SubscriptionName: "test-sub",
			},
			consumerID: "test-consumer",
			consumer:   mock,
			connected:  true,
			closed:     false,
		}

		msgID := &mockMessageID{id: 1}
		err := cc.Seek(msgID)
		if err == nil {
			t.Error("Seek() error = nil, want error")
		}
	})

	t.Run("Seek when not connected", func(t *testing.T) {
		cc := &ConsumerClient{
			connected: false,
		}

		msgID := &mockMessageID{id: 1}
		err := cc.Seek(msgID)
		if err == nil {
			t.Error("Seek() error = nil, want error when not connected")
		}
	})
}

func TestConsumerClient_SeekByTime(t *testing.T) {
	t.Run("SeekByTime success", func(t *testing.T) {
		cc := &ConsumerClient{
			pulsarCfg: &config.PulsarConfig{
				ServiceURL: "pulsar://localhost:6650",
				Topic:      "test-topic",
			},
			consumerCfg: &config.ConsumerConfig{
				SubscriptionName: "test-sub",
			},
			consumerID: "test-consumer",
			consumer:   &mockConsumer{},
			connected:  true,
			closed:     false,
		}

		timestamp := time.Now().Add(-1 * time.Hour)
		err := cc.SeekByTime(timestamp)
		if err != nil {
			t.Errorf("SeekByTime() error = %v, want nil", err)
		}
	})

	t.Run("SeekByTime error", func(t *testing.T) {
		expectedErr := errors.New("seek by time failed")
		mock := &mockConsumer{
			seekByTimeFunc: func(t time.Time) error {
				return expectedErr
			},
		}

		cc := &ConsumerClient{
			pulsarCfg: &config.PulsarConfig{
				ServiceURL: "pulsar://localhost:6650",
				Topic:      "test-topic",
			},
			consumerCfg: &config.ConsumerConfig{
				SubscriptionName: "test-sub",
			},
			consumerID: "test-consumer",
			consumer:   mock,
			connected:  true,
			closed:     false,
		}

		timestamp := time.Now()
		err := cc.SeekByTime(timestamp)
		if err == nil {
			t.Error("SeekByTime() error = nil, want error")
		}
	})
}

func TestConsumerClient_Chan(t *testing.T) {
	t.Run("Chan when connected", func(t *testing.T) {
		cc := &ConsumerClient{
			pulsarCfg: &config.PulsarConfig{
				ServiceURL: "pulsar://localhost:6650",
				Topic:      "test-topic",
			},
			consumerCfg: &config.ConsumerConfig{
				SubscriptionName: "test-sub",
			},
			consumerID: "test-consumer",
			consumer:   &mockConsumer{},
			connected:  true,
			closed:     false,
		}

		ch := cc.Chan()
		if ch == nil {
			t.Error("Chan() returned nil")
		}
	})

	t.Run("Chan when not connected", func(t *testing.T) {
		cc := &ConsumerClient{
			connected: false,
		}

		ch := cc.Chan()
		if ch == nil {
			t.Error("Chan() returned nil, want closed channel")
		}

		// Check that channel is closed
		select {
		case _, ok := <-ch:
			if ok {
				t.Error("Chan() returned open channel when not connected")
			}
		default:
			t.Error("Chan() channel was not closed when not connected")
		}
	})
}

func TestConsumerClient_Close(t *testing.T) {
	t.Run("Close success", func(t *testing.T) {
		closeCalled := false
		mock := &mockConsumer{
			closeFunc: func() {
				closeCalled = true
			},
		}

		cc := &ConsumerClient{
			pulsarCfg: &config.PulsarConfig{
				ServiceURL: "pulsar://localhost:6650",
				Topic:      "test-topic",
			},
			consumerCfg: &config.ConsumerConfig{
				SubscriptionName: "test-sub",
			},
			consumerID: "test-consumer",
			consumer:   mock,
			connected:  true,
			closed:     false,
		}

		err := cc.Close()
		if err != nil {
			t.Errorf("Close() error = %v, want nil", err)
		}

		if !closeCalled {
			t.Error("Close() did not call consumer.Close()")
		}

		if cc.IsConnected() {
			t.Error("IsConnected() = true after Close(), want false")
		}
	})

	t.Run("Close idempotent", func(t *testing.T) {
		cc := &ConsumerClient{
			pulsarCfg: &config.PulsarConfig{
				ServiceURL: "pulsar://localhost:6650",
				Topic:      "test-topic",
			},
			consumerCfg: &config.ConsumerConfig{
				SubscriptionName: "test-sub",
			},
			consumerID: "test-consumer",
			consumer:   &mockConsumer{},
			connected:  true,
			closed:     false,
		}

		err := cc.Close()
		if err != nil {
			t.Errorf("First Close() error = %v, want nil", err)
		}

		// Second close should not error
		err = cc.Close()
		if err != nil {
			t.Errorf("Second Close() error = %v, want nil", err)
		}
	})
}

func TestConsumerClient_Unsubscribe(t *testing.T) {
	t.Run("Unsubscribe success", func(t *testing.T) {
		cc := &ConsumerClient{
			pulsarCfg: &config.PulsarConfig{
				ServiceURL: "pulsar://localhost:6650",
				Topic:      "test-topic",
			},
			consumerCfg: &config.ConsumerConfig{
				SubscriptionName: "test-sub",
			},
			consumerID: "test-consumer",
			consumer:   &mockConsumer{},
			connected:  true,
			closed:     false,
		}

		err := cc.Unsubscribe()
		if err != nil {
			t.Errorf("Unsubscribe() error = %v, want nil", err)
		}
	})

	t.Run("Unsubscribe error", func(t *testing.T) {
		expectedErr := errors.New("unsubscribe failed")
		mock := &mockConsumer{
			unsubscribeFunc: func() error {
				return expectedErr
			},
		}

		cc := &ConsumerClient{
			pulsarCfg: &config.PulsarConfig{
				ServiceURL: "pulsar://localhost:6650",
				Topic:      "test-topic",
			},
			consumerCfg: &config.ConsumerConfig{
				SubscriptionName: "test-sub",
			},
			consumerID: "test-consumer",
			consumer:   mock,
			connected:  true,
			closed:     false,
		}

		err := cc.Unsubscribe()
		if err == nil {
			t.Error("Unsubscribe() error = nil, want error")
		}
	})

	t.Run("Unsubscribe when not connected", func(t *testing.T) {
		cc := &ConsumerClient{
			connected: false,
		}

		err := cc.Unsubscribe()
		if err == nil {
			t.Error("Unsubscribe() error = nil, want error when not connected")
		}
	})
}

func TestConsumerClient_IsConnected(t *testing.T) {
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
			cc := &ConsumerClient{
				connected: tt.connected,
				closed:    tt.closed,
			}

			if got := cc.IsConnected(); got != tt.want {
				t.Errorf("IsConnected() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConsumerClient_Stats(t *testing.T) {
	cc := &ConsumerClient{
		pulsarCfg: &config.PulsarConfig{
			ServiceURL: "pulsar://localhost:6650",
			Topic:      "test-topic",
		},
		consumerCfg: &config.ConsumerConfig{
			SubscriptionName: "test-sub",
		},
		consumerID: "test-consumer",
		consumer:   &mockConsumer{},
		connected:  true,
		closed:     false,
	}

	ctx := context.Background()

	// Receive multiple messages
	for i := 0; i < 5; i++ {
		msg, err := cc.Receive(ctx)
		if err != nil {
			t.Errorf("Receive() error = %v, want nil", err)
		}
		if i%2 == 0 {
			cc.Ack(msg)
		} else {
			cc.Nack(msg)
		}
	}

	stats := cc.Stats()

	if stats.MessagesReceived != 5 {
		t.Errorf("Stats.MessagesReceived = %d, want 5", stats.MessagesReceived)
	}

	if stats.MessagesAcked != 3 {
		t.Errorf("Stats.MessagesAcked = %d, want 3", stats.MessagesAcked)
	}

	if stats.MessagesNacked != 2 {
		t.Errorf("Stats.MessagesNacked = %d, want 2", stats.MessagesNacked)
	}

	expectedBytes := uint64(5 * len("test message"))
	if stats.BytesReceived != expectedBytes {
		t.Errorf("Stats.BytesReceived = %d, want %d", stats.BytesReceived, expectedBytes)
	}

	if stats.ReceiveErrors != 0 {
		t.Errorf("Stats.ReceiveErrors = %d, want 0", stats.ReceiveErrors)
	}
}

func TestConsumerClient_LastError(t *testing.T) {
	expectedErr := errors.New("test error")

	mock := &mockConsumer{
		receiveFunc: func(ctx context.Context) (pulsar.Message, error) {
			return nil, expectedErr
		},
	}

	cc := &ConsumerClient{
		pulsarCfg: &config.PulsarConfig{
			ServiceURL: "pulsar://localhost:6650",
			Topic:      "test-topic",
		},
		consumerCfg: &config.ConsumerConfig{
			SubscriptionName: "test-sub",
		},
		consumerID: "test-consumer",
		consumer:   mock,
		connected:  true,
		closed:     false,
	}

	ctx := context.Background()
	_, err := cc.Receive(ctx)
	if err == nil {
		t.Error("Receive() error = nil, want error")
	}

	lastErr := cc.LastError()
	if lastErr != expectedErr {
		t.Errorf("LastError() = %v, want %v", lastErr, expectedErr)
	}
}

func TestConsumerClient_ConcurrentOperations(t *testing.T) {
	cc := &ConsumerClient{
		pulsarCfg: &config.PulsarConfig{
			ServiceURL: "pulsar://localhost:6650",
			Topic:      "test-topic",
		},
		consumerCfg: &config.ConsumerConfig{
			SubscriptionName: "test-sub",
		},
		consumerID: "test-consumer",
		consumer:   &mockConsumer{},
		connected:  true,
		closed:     false,
	}

	ctx := context.Background()
	numGoroutines := 50
	numMessages := 10

	done := make(chan bool, numGoroutines)

	// Launch multiple goroutines performing concurrent operations
	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < numMessages; j++ {
				msg, err := cc.Receive(ctx)
				if err != nil {
					t.Errorf("Receive() error = %v, want nil", err)
					continue
				}
				if j%2 == 0 {
					cc.Ack(msg)
				} else {
					cc.Nack(msg)
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
			t.Fatal("Concurrent operations timed out")
		}
	}

	stats := cc.Stats()
	expectedCount := uint64(numGoroutines * numMessages)
	if stats.MessagesReceived != expectedCount {
		t.Errorf("Stats.MessagesReceived = %d, want %d", stats.MessagesReceived, expectedCount)
	}
}

func TestNewConsumerClient_LegacyWrapper(t *testing.T) {
	// Test the legacy wrapper function for backward compatibility
	// This will fail to connect without a real Pulsar cluster, which is expected
	cfg := config.DefaultConfig("")
	cfg.Pulsar.ServiceURL = "pulsar://invalid-host:6650"

	_, err := NewConsumerClient(cfg, 1)
	if err == nil {
		t.Error("NewConsumerClient() with invalid host error = nil, want error")
	}
}