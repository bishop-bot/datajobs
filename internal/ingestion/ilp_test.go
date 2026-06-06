package ingestion

import (
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/bishop-bot/datajobs/internal/config"
)

// mockAddr implements net.Addr for testing.
type mockAddr struct{}

func (m *mockAddr) Network() string { return "tcp" }
func (m *mockAddr) String() string  { return "localhost:9000" }

// mockConn implements net.Conn for testing.
type mockConn struct {
	readBuf  []byte
	writeBuf []byte
	closed   bool
	mu       sync.RWMutex
}

func newMockConn() *mockConn {
	return &mockConn{
		readBuf:  make([]byte, 0),
		writeBuf: make([]byte, 0),
	}
}

func (mc *mockConn) Read(b []byte) (n int, err error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	if mc.closed {
		return 0, net.ErrClosed
	}
	if len(mc.readBuf) == 0 {
		return 0, io.EOF
	}
	n = copy(b, mc.readBuf)
	mc.readBuf = mc.readBuf[n:]
	return n, nil
}

func (mc *mockConn) Write(b []byte) (n int, err error) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	if mc.closed {
		return 0, net.ErrClosed
	}
	mc.writeBuf = append(mc.writeBuf, b...)
	return len(b), nil
}

func (mc *mockConn) Close() error {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.closed = true
	return nil
}

func (mc *mockConn) LocalAddr() net.Addr  { return &mockAddr{} }
func (mc *mockConn) RemoteAddr() net.Addr { return &mockAddr{} }
func (mc *mockConn) SetDeadline(t time.Time) error {
	return nil
}
func (mc *mockConn) SetReadDeadline(t time.Time) error {
	return nil
}
func (mc *mockConn) SetWriteDeadline(t time.Time) error {
	return nil
}

// TestILPClient_NoDeadlockOnSendLines verifies that sendLines does not deadlock
// when calling Connect under the lock. The fix uses ensureConnected which calls
// connectLocked (assumes lock is held) instead of Connect.
func TestILPClient_NoDeadlockOnSendLines(t *testing.T) {
	cfg := config.QuestDBConfig{
		Host:     "localhost",
		ILPPort:  9009,
		User:     "",
		Password: "",
	}
	client := NewILPClient(cfg, nil)

	// Manually set a mock connection to simulate connected state
	client.mu.Lock()
	client.connected = true
	mockConn := newMockConn()
	client.conn = mockConn
	client.mu.Unlock()

	// Now call sendLines - it should NOT try to call Connect (which would deadlock)
	lines := []string{"test_table col1=val1 1234567890000000000"}
	err := client.sendLines(context.Background(), lines)

	if err != nil {
		t.Errorf("sendLines should not error: %v", err)
	}

	// Verify data was written
	mockConn.mu.RLock()
	if len(mockConn.writeBuf) == 0 {
		t.Error("data should have been written to connection")
	}
	mockConn.mu.RUnlock()
}

func TestILPClient_ConnectLocksCorrectly(t *testing.T) {
	cfg := config.QuestDBConfig{
		Host:     "localhost",
		ILPPort:  65535, // Unlikely to have anything on this port
		User:     "",
		Password: "",
	}
	client := NewILPClient(cfg, nil)

	// Test that Connect can be called multiple times safely
	ctx := context.Background()

	// First call should fail (no server) but not deadlock
	err1 := client.Connect(ctx)

	// Second call should also not deadlock
	err2 := client.Connect(ctx)

	// Both should return errors (no server running), not deadlock
	if err1 == nil || err2 == nil {
		t.Error("expected connection errors (no server)")
	}
}

func TestILPClient_CloseIsThreadSafe(t *testing.T) {
	cfg := config.QuestDBConfig{
		Host:     "localhost",
		ILPPort:  9009,
		User:     "",
		Password: "",
	}
	client := NewILPClient(cfg, nil)

	// Set up a mock connected state
	client.mu.Lock()
	client.connected = true
	client.conn = newMockConn()
	client.mu.Unlock()

	// Close should be thread-safe
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client.Close()
		}()
	}
	wg.Wait()
}

func TestBufferedILPClient_ThreadSafety(t *testing.T) {
	cfg := config.QuestDBConfig{
		Host:     "localhost",
		ILPPort:  9009,
		User:     "",
		Password: "",
	}
	client := NewILPClient(cfg, nil)

	// Set up mock connection
	client.mu.Lock()
	client.connected = true
	client.conn = newMockConn()
	client.mu.Unlock()

	bc := NewBufferedILPClient(client, 10)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				bc.Write("test,tag=val field=value 1234567890000000000")
			}
		}(i)
	}
	wg.Wait()

	// Flush should complete without deadlock
	bc.Flush()
}

func TestILPClient_NewILPClient(t *testing.T) {
	cfg := config.QuestDBConfig{
		Host:     "localhost",
		ILPPort:  9000,
		User:     "admin",
		Password: "quest",
	}
	client := NewILPClient(cfg, nil)

	if client == nil {
		t.Fatal("NewILPClient should not return nil")
	}
	if client.addr != "localhost:9000" {
		t.Errorf("expected addr 'localhost:9000', got '%s'", client.addr)
	}
	if client.user != "admin" {
		t.Errorf("expected user 'admin', got '%s'", client.user)
	}
	if client.timeout != 30*time.Second {
		t.Errorf("expected timeout 30s, got %v", client.timeout)
	}
}

func TestILPClient_ensureConnected(t *testing.T) {
	cfg := config.QuestDBConfig{
		Host:     "localhost",
		ILPPort:  65535, // Unlikely to have anything on this port
		User:     "",
		Password: "",
	}
	client := NewILPClient(cfg, nil)

	// Test when not connected - should fail (no server) but not deadlock
	client.mu.Lock()
	err := client.ensureConnected(context.Background())
	client.mu.Unlock()

	if err == nil {
		t.Error("expected error when no server available")
	}
}

func TestIngestResult_Duration(t *testing.T) {
	start := time.Now()
	result := &IngestResult{
		StartTime: start,
		EndTime:   start.Add(5 * time.Second),
	}

	duration := result.Duration()
	if duration != 5*time.Second {
		t.Errorf("expected duration 5s, got %v", duration)
	}
}

func TestIngestResult_InitialState(t *testing.T) {
	result := &IngestResult{}
	
	if result.RowsIngested != 0 {
		t.Errorf("expected 0 rows, got %d", result.RowsIngested)
	}
	if result.ErrorCount != 0 {
		t.Errorf("expected 0 errors, got %d", result.ErrorCount)
	}
}

func TestCSVOptions_Defaults(t *testing.T) {
	opts := CSVOptions{}
	
	if opts.BatchSize != 0 {
		t.Errorf("expected default batch size 0, got %d", opts.BatchSize)
	}
	if opts.SkipRows != 0 {
		t.Errorf("expected default skip rows 0, got %d", opts.SkipRows)
	}
}

func TestNewBufferedILPClient(t *testing.T) {
	cfg := config.QuestDBConfig{Host: "localhost", ILPPort: 9000}
	client := NewILPClient(cfg, nil)
	
	bc := NewBufferedILPClient(client, 100)
	if bc == nil {
		t.Fatal("NewBufferedILPClient should not return nil")
	}
	if bc.flushSize != 100 {
		t.Errorf("expected flush size 100, got %d", bc.flushSize)
	}
	if cap(bc.buffer) != 100 {
		t.Errorf("expected buffer capacity 100, got %d", cap(bc.buffer))
	}
}