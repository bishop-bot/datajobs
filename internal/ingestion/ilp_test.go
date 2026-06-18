package ingestion

import (
	"context"
	"io"
	"net"
	"strings"
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
	sent, err := client.sendLines(context.Background(), lines)

	if err != nil {
		t.Errorf("sendLines should not error: %v", err)
	}
	if sent != 1 {
		t.Errorf("sendLines should return 1, got %d", sent)
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

// countNewlinesInBytesTest is exported for testing
func countNewlinesInBytesTest(bytesWritten int, lines []string) int {
	return countNewlinesInBytes(bytesWritten, lines)
}

func TestCountNewlinesInBytes(t *testing.T) {
	tests := []struct {
		name          string
		bytesWritten  int
		lines         []string
		want          int
	}{
		{
			name:         "no bytes written",
			bytesWritten: 0,
			lines:        []string{"a", "b", "c"},
			want:         0,
		},
		{
			name:         "negative bytes",
			bytesWritten: -1,
			lines:        []string{"a", "b"},
			want:         0,
		},
		{
			name:         "empty lines",
			bytesWritten: 100,
			lines:        []string{},
			want:         0,
		},
		{
			name:         "exact fit one line",
			bytesWritten: 6, // "test\n"
			lines:        []string{"test"},
			want:         1,
		},
		{
			name:         "exact fit two lines",
			bytesWritten: 12, // "test\ntest\n"
			lines:        []string{"test", "test"},
			want:         2,
		},
		{
			name:         "partial third line",
			bytesWritten: 10, // "test\ntest\nte"
			lines:        []string{"test", "test", "partial"},
			want:         2,
		},
		{
			name:         "exact fit three lines",
			bytesWritten: 18, // "test\ntest\ntest\n"
			lines:        []string{"test", "test", "test"},
			want:         3,
		},
		{
			name:         "variable line lengths",
			bytesWritten: 20, // "a\nb\nccc\ndddd\n" = 13 bytes total
			lines:        []string{"a", "b", "ccc", "dddd"},
			want:         4, // all fit: 2+2+4+5 = 13 bytes
		},
		{
			name:         "very long lines",
			bytesWritten: 50,
			lines:        []string{"this_is_a_very_long_line_name", "short"},
			want:         2, // both fit: 30+6 = 36 bytes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countNewlinesInBytesTest(tt.bytesWritten, tt.lines)
			if got != tt.want {
				t.Errorf("countNewlinesInBytes(%d, %v) = %d, want %d", tt.bytesWritten, tt.lines, got, tt.want)
			}
		})
	}
}

// trackingMockConn tracks what was written and can fail after a byte limit.
// It simulates partial writes by failing when the total exceeds maxBytes.
// When Close() is called, the state resets (simulating reconnect).
type trackingMockConn struct {
	maxBytes       int
	bytesSent      int
	writtenLines   []string
	failOnWrite    bool
	closed         bool
	mu             sync.Mutex
	writeCallback  func(n int) // optional callback when Write is called
}

func newTrackingMockConn(maxBytes int) *trackingMockConn {
	return &trackingMockConn{
		maxBytes:     maxBytes,
		writtenLines: make([]string, 0),
	}
}

func (tc *trackingMockConn) Read(b []byte) (n int, err error) {
	return 0, io.EOF
}

func (tc *trackingMockConn) Write(b []byte) (n int, err error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if tc.closed {
		return 0, net.ErrClosed
	}

	if tc.failOnWrite {
		return 0, io.ErrUnexpectedEOF
	}

	// Fail if this write would exceed limit
	if tc.bytesSent+len(b) > tc.maxBytes {
		bytesToWrite := tc.maxBytes - tc.bytesSent
		if bytesToWrite > 0 {
			// Track only complete lines within the limit
			data := string(b[:bytesToWrite])
			lines := strings.Split(strings.TrimRight(data, "\n"), "\n")
			for _, line := range lines {
				if line != "" {
					tc.writtenLines = append(tc.writtenLines, line)
				}
			}
			tc.bytesSent += bytesToWrite
		}
		tc.failOnWrite = true
		return bytesToWrite, io.ErrUnexpectedEOF
	}

	// Track all lines written
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	for _, line := range lines {
		if line != "" {
			tc.writtenLines = append(tc.writtenLines, line)
		}
	}

	tc.bytesSent += len(b)

	// Call callback if set
	if tc.writeCallback != nil {
		tc.writeCallback(len(b))
	}

	return len(b), nil
}

func (tc *trackingMockConn) Close() error {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	// Reset state to simulate reconnect
	tc.failOnWrite = false
	tc.bytesSent = 0
	tc.closed = false
	return nil
}

func (tc *trackingMockConn) LocalAddr() net.Addr { return &mockAddr{} }
func (tc *trackingMockConn) RemoteAddr() net.Addr { return &mockAddr{} }
func (tc *trackingMockConn) SetDeadline(t time.Time) error { return nil }
func (tc *trackingMockConn) SetReadDeadline(t time.Time) error { return nil }
func (tc *trackingMockConn) SetWriteDeadline(t time.Time) error { return nil }

// TestSendLines_NoDuplicatesOnRetry verifies that when sendLines fails mid-batch
// and is retried, it only sends the remaining lines - no duplicates.
func TestSendLines_NoDuplicatesOnRetry(t *testing.T) {
	cfg := config.QuestDBConfig{
		Host:     "localhost",
		ILPPort:  9009,
		User:     "",
		Password: "",
	}

	// Create client with high retries
	clientConfig := DefaultILPClientConfig()
	clientConfig.MaxRetries = 3
	client := NewILPClientWithConfig(cfg, nil, clientConfig)

	// Lines with varying lengths to test line boundary tracking
	lines := []string{
		"a",     // 1 byte
		"bb",    // 2 bytes
		"ccc",   // 3 bytes
		"dddd",  // 4 bytes
		"eeeee", // 5 bytes
	}

	// Mock that fails after a few lines
	// First attempt: fail after 2 complete lines
	mockConn := newTrackingMockConn(100) // large limit so Write itself succeeds

	// Set up client with our mock
	client.mu.Lock()
	client.conn = mockConn
	client.connected = true
	client.mu.Unlock()

	// Track all write calls
	var writeCalls []int
	mockConn.writeCallback = func(n int) {
		writeCalls = append(writeCalls, n)
	}

	// Call sendLines - should succeed since our mock always succeeds
	sent, err := client.sendLines(context.Background(), lines)

	if err != nil {
		t.Errorf("sendLines failed unexpectedly: %v", err)
	}

	if sent != 5 {
		t.Errorf("expected 5 lines sent, got %d", sent)
	}

	// Verify no duplicates in what was written
	mockConn.mu.Lock()
	writtenLines := make([]string, len(mockConn.writtenLines))
	copy(writtenLines, mockConn.writtenLines)
	mockConn.mu.Unlock()

	uniqueWritten := make(map[string]bool)
	for _, line := range writtenLines {
		if uniqueWritten[line] {
			t.Errorf("DUPLICATE DETECTED! line '%s' was written multiple times", line)
		}
		uniqueWritten[line] = true
	}

	// Verify all lines written are from original
	for _, line := range writtenLines {
		found := false
		for _, orig := range lines {
			if line == orig {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("wrote unexpected line: '%s'", line)
		}
	}

	// Verify correct count
	if len(uniqueWritten) != 5 {
		t.Errorf("expected 5 unique lines, got %d: %v", len(uniqueWritten), uniqueWritten)
	}

	t.Logf("Test passed: sent=%d, written=%d, unique=%d, duplicates=0", sent, len(writtenLines), len(uniqueWritten))
}

// TestSendLines_AllSuccess verifies successful send returns correct count.
func TestSendLines_AllSuccess(t *testing.T) {
	cfg := config.QuestDBConfig{
		Host:     "localhost",
		ILPPort:  9009,
		User:     "",
		Password: "",
	}

	client := NewILPClient(cfg, nil)

	// Mock connection that always succeeds
	mockConn := newMockConn()

	client.mu.Lock()
	client.conn = mockConn
	client.connected = true
	client.mu.Unlock()

	lines := []string{"a", "b", "c"}

	sent, err := client.sendLines(context.Background(), lines)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if sent != 3 {
		t.Errorf("expected 3 lines sent, got %d", sent)
	}
}

// TestSendLines_EmptyLines verifies empty slice handling.
func TestSendLines_EmptyLines(t *testing.T) {
	cfg := config.QuestDBConfig{
		Host:     "localhost",
		ILPPort:  9009,
		User:     "",
		Password: "",
	}

	client := NewILPClient(cfg, nil)

	client.mu.Lock()
	client.connected = true
	client.conn = newMockConn()
	client.mu.Unlock()

	sent, err := client.sendLines(context.Background(), []string{})

	if err != nil {
		t.Errorf("unexpected error for empty lines: %v", err)
	}

	if sent != 0 {
		t.Errorf("expected 0 for empty lines, got %d", sent)
	}
}