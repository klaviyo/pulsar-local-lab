package pulsar

// mockMessageID implements pulsar.MessageID interface for testing
type mockMessageID struct {
	id int64
}

func (m *mockMessageID) Serialize() []byte   { return []byte{} }
func (m *mockMessageID) LedgerID() int64     { return 0 }
func (m *mockMessageID) EntryID() int64      { return m.id }
func (m *mockMessageID) BatchIdx() int32     { return 0 }
func (m *mockMessageID) PartitionIdx() int32 { return 0 }
func (m *mockMessageID) BatchSize() int32    { return 1 }
func (m *mockMessageID) String() string      { return "mock-message-id" }