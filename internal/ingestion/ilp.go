package ingestion

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/bishop-bot/datajobs/internal/config"
	"github.com/bishop-bot/datajobs/internal/logging"
	"github.com/bishop-bot/datajobs/internal/metrics"
)

// ILPClient sends data to QuestDB via the InfluxDB Line Protocol over TCP.
// It includes connection health checking and automatic retry with exponential backoff.
type ILPClient struct {
	addr      string
	user      string
	password  string
	conn      net.Conn
	mu        sync.Mutex
	connected bool
	timeout   time.Duration
	metrics   *metrics.Metrics

	// Retry configuration
	maxRetries     int
	retryBaseDelay time.Duration
	retryMaxDelay  time.Duration
}

// ILPClientConfig contains configuration options for ILPClient.
type ILPClientConfig struct {
	// Timeout for network operations (default: 30s)
	Timeout time.Duration
	// MaxRetries for transient failures (default: 3)
	MaxRetries int
	// RetryBaseDelay for exponential backoff (default: 100ms)
	RetryBaseDelay time.Duration
	// RetryMaxDelay caps the backoff (default: 5s)
	RetryMaxDelay time.Duration
}

// DefaultILPClientConfig returns the default configuration.
func DefaultILPClientConfig() ILPClientConfig {
	return ILPClientConfig{
		Timeout:        30 * time.Second,
		MaxRetries:     3,
		RetryBaseDelay: 100 * time.Millisecond,
		RetryMaxDelay:  5 * time.Second,
	}
}

// NewILPClient creates a new ILP client with default configuration.
func NewILPClient(cfg config.QuestDBConfig, m *metrics.Metrics) *ILPClient {
	return NewILPClientWithConfig(cfg, m, DefaultILPClientConfig())
}

// NewILPClientWithConfig creates a new ILP client with custom configuration.
func NewILPClientWithConfig(cfg config.QuestDBConfig, m *metrics.Metrics, config ILPClientConfig) *ILPClient {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryBaseDelay == 0 {
		config.RetryBaseDelay = 100 * time.Millisecond
	}
	if config.RetryMaxDelay == 0 {
		config.RetryMaxDelay = 5 * time.Second
	}

	return &ILPClient{
		addr:           fmt.Sprintf("%s:%d", cfg.Host, cfg.ILPPort),
		user:           cfg.User,
		password:       cfg.Password,
		timeout:        config.Timeout,
		metrics:        m,
		maxRetries:     config.MaxRetries,
		retryBaseDelay: config.RetryBaseDelay,
		retryMaxDelay:  config.RetryMaxDelay,
	}
}

// Connect establishes a connection to QuestDB ILP.
// Thread-safe: uses internal mutex for connection state.
func (c *ILPClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.connectLocked(ctx)
}

// connectLocked establishes connection assuming caller holds mu.
// Use Connect() for external callers.
func (c *ILPClient) connectLocked(ctx context.Context) error {
	// Close existing connection if any
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}

	// Use dialer with context for timeout
	dialer := &net.Dialer{
		Timeout: c.timeout,
	}

	conn, err := dialer.DialContext(ctx, "tcp", c.addr)
	if err != nil {
		c.connected = false
		return fmt.Errorf("failed to connect to ILP: %w", err)
	}

	c.conn = conn
	c.connected = true

	// Send authentication if needed
	if c.user != "" {
		authLine := fmt.Sprintf("auth\t%s:%s\n", c.user, c.password)
		if _, err := conn.Write([]byte(authLine)); err != nil {
			conn.Close()
			c.conn = nil
			c.connected = false
			return fmt.Errorf("failed to send auth: %w", err)
		}
	}

	logging.Info("connected to QuestDB ILP", "addr", c.addr)
	return nil
}

// Close closes the ILP connection.
func (c *ILPClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.connected = false
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

// ping sends a ping to test the connection.
// Returns true if the connection is healthy, false otherwise.
func (c *ILPClient) ping() bool {
	if c.conn == nil {
		return false
	}

	// Set a short deadline for the ping
	c.conn.SetReadDeadline(time.Now().Add(1 * time.Second))

	// Send a newline (no-op in ILP)
	if _, err := c.conn.Write([]byte("\n")); err != nil {
		return false
	}

	return true
}

// testAndReconnect tests the connection and reconnects if unhealthy.
// Must be called with mu held.
func (c *ILPClient) testAndReconnect(ctx context.Context) error {
	if c.conn == nil {
		return c.connectLocked(ctx)
	}

	// Test with a ping
	if c.ping() {
		return nil
	}

	// Connection unhealthy, reconnect
	logging.Debug("ILP connection unhealthy, reconnecting")
	c.connected = false
	c.conn.Close()
	c.conn = nil

	return c.connectLocked(ctx)
}

// ensureConnected attempts to connect if not connected.
// Must be called with mu held.
func (c *ILPClient) ensureConnected(ctx context.Context) error {
	if c.connected && c.conn != nil {
		return nil
	}
	return c.connectLocked(ctx)
}

// IngestCSV ingests a CSV file using ILP.
func (c *ILPClient) IngestCSV(ctx context.Context, table string, csvPath string, opts CSVOptions) (*IngestResult, error) {
	logger := logging.FromContext(ctx)

	if err := c.Connect(ctx); err != nil {
		return nil, err
	}

	result := &IngestResult{
		Table:     table,
		CSVPath:   csvPath,
		StartTime: time.Now(),
	}

	file, err := os.Open(csvPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // Allow variable fields
	reader.TrimLeadingSpace = true
	reader.LazyQuotes = true

	// Read and validate header
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	// Skip rows if specified
	for i := 0; i < opts.SkipRows; i++ {
		if _, err := reader.Read(); err != nil {
			return nil, fmt.Errorf("failed to skip rows: %w", err)
		}
	}

	// Find timestamp column
	timestampIdx := findColumn(header, opts.TimestampColumn)
	if timestampIdx < 0 {
		return nil, fmt.Errorf("timestamp column '%s' not found in header", opts.TimestampColumn)
	}

	logger.Info("starting CSV ingestion",
		"table", table,
		"csv_path", csvPath,
		"rows", opts.MaxRows,
		"batch_size", opts.BatchSize,
	)

	batchCount := 0
	totalRows := 0
	errors := 0

	for {
		select {
		case <-ctx.Done():
			result.Errors = append(result.Errors, ctx.Err().Error())
			break
		default:
		}

		batch, err := c.readBatch(reader, opts.BatchSize)
		if err != nil {
			errors++
			result.Errors = append(result.Errors, err.Error())
			continue
		}

		if len(batch) == 0 {
			break
		}

		// Convert batch to ILP format and send
		lines, skippedRows, err := batchToILP(batch, header, timestampIdx, table, opts)
		if err != nil {
			errors++
			result.Errors = append(result.Errors, err.Error())
			continue
		}
		if skippedRows > 0 {
			logging.Warn("dropped malformed rows during batch conversion", "count", skippedRows)
			result.Errors = append(result.Errors, fmt.Sprintf("dropped %d malformed rows", skippedRows))
		}

		// Send with idempotent retry - no duplicates
		sent, err := c.sendLines(ctx, lines)
		if err != nil {
			if sent > 0 {
				logger.Error("partial batch sent before failure",
					"sent", sent,
					"total", len(lines),
					"error", err,
				)
			}
			errors++
			result.Errors = append(result.Errors, fmt.Sprintf("batch %d: sent %d/%d: %v", batchCount, sent, len(lines), err))
			// Continue to next batch even on error
		}

		batchCount++
		totalRows += len(batch)

		// Update metrics
		if c.metrics != nil {
			c.metrics.RecordJobEnd(ctx, "csv_ingest", "ingesting")
		}

		// Check max rows limit
		if opts.MaxRows > 0 && totalRows >= opts.MaxRows {
			break
		}

		// Progress logging
		if batchCount%100 == 0 {
			logger.Info("ingestion progress",
				"batches", batchCount,
				"rows", totalRows,
				"errors", errors,
			)
		}
	}

	result.EndTime = time.Now()
	result.RowsIngested = totalRows
	result.BatchCount = batchCount
	result.ErrorCount = errors

	logger.Info("CSV ingestion complete",
		"rows", totalRows,
		"batches", batchCount,
		"errors", errors,
		"duration", result.Duration(),
	)

	return result, nil
}

// CSVOptions contains options for CSV ingestion.
type CSVOptions struct {
	TimestampColumn string
	SymbolColumns   []string
	SkipRows        int
	MaxRows         int
	BatchSize       int
	Delimiter       rune
}

// IngestResult contains the result of an ingestion operation.
type IngestResult struct {
	Table        string
	CSVPath      string
	StartTime    time.Time
	EndTime      time.Time
	RowsIngested int
	BatchCount   int
	ErrorCount   int
	Errors       []string
}

// Duration returns the ingestion duration.
func (r *IngestResult) Duration() time.Duration {
	return r.EndTime.Sub(r.StartTime)
}

func (c *ILPClient) readBatch(reader *csv.Reader, batchSize int) ([][]string, error) {
	var batch [][]string
	for i := 0; i < batchSize; i++ {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return batch, err
		}
		batch = append(batch, record)
	}
	return batch, nil
}

func batchToILP(batch [][]string, header []string, timestampIdx int, table string, opts CSVOptions) ([]string, int, error) {
	lines := make([]string, 0, len(batch))
	skippedRows := 0

	for _, row := range batch {
		if len(row) != len(header) {
			skippedRows++
			logging.Warn("skipping malformed row", "expected_cols", len(header), "actual_cols", len(row))
			continue
		}

		// Build ILP line: table,tag fields,field values,timestamp
		var tags, fields []string

		// Add symbol columns as tags
		for _, col := range opts.SymbolColumns {
			idx := findColumn(header, col)
			if idx >= 0 && idx < len(row) && row[idx] != "" {
				tags = append(tags, fmt.Sprintf("%s=%s", col, row[idx]))
			}
		}

		// Add non-timestamp columns as fields
		for i, col := range header {
			if i == timestampIdx {
				continue // Skip timestamp column
			}
			if isSymbolColumn(col, opts.SymbolColumns) {
				continue // Already added as tag
			}
			value := row[i]
			if value != "" {
				fields = append(fields, fmt.Sprintf("%s=%s", col, value))
			}
		}

		// Build the ILP line
		var line string
		if len(tags) > 0 {
			line = fmt.Sprintf("%s,%s %s %s", table, joinStrings(tags, ","), joinStrings(fields, ","), row[timestampIdx])
		} else {
			line = fmt.Sprintf("%s %s %s", table, joinStrings(fields, ","), row[timestampIdx])
		}

		lines = append(lines, line)
	}

	return lines, skippedRows, nil
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

// countNewlinesInBytes calculates how many complete lines were sent in a partial write.
// Each line ends with '\n', so we count how many lines fit in bytesWritten.
func countNewlinesInBytes(bytesWritten int, lines []string) int {
	if bytesWritten <= 0 || len(lines) == 0 {
		return 0
	}

	totalBytes := 0
	linesSent := 0

	for _, line := range lines {
		lineBytes := len(line) + 1 // +1 for '\n'
		if totalBytes+lineBytes <= bytesWritten {
			linesSent++
			totalBytes += lineBytes
		} else {
			break
		}
	}

	return linesSent
}

// sendLines sends ILP lines to QuestDB with idempotent retry.
// Returns (linesSent, error). On failure, linesSent indicates how many were
// successfully sent before the failure - caller can retry from that point
// to avoid duplicates.
func (c *ILPClient) sendLines(ctx context.Context, lines []string) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check context before network operations
	if ctx.Err() != nil {
		return 0, fmt.Errorf("context cancelled: %w", ctx.Err())
	}

	// Nothing to send
	if len(lines) == 0 {
		return 0, nil
	}

	var linesSent int
	currentLines := lines

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		// Test connection health and reconnect if needed
		if err := c.testAndReconnect(ctx); err != nil {
			return linesSent, fmt.Errorf("connection failed: %w", err)
		}

		// Build buffer for current lines (remaining from previous attempts)
		var buf bytes.Buffer
		for _, line := range currentLines {
			buf.WriteString(line)
			buf.WriteByte('\n')
		}

		c.conn.SetWriteDeadline(time.Now().Add(c.timeout))

		// Attempt single write for current batch
		written, err := c.conn.Write(buf.Bytes())

		if err != nil {
			c.connected = false

			// Calculate how many lines were successfully sent
			recentlySent := countNewlinesInBytes(written, currentLines)
			linesSent += recentlySent

			logging.Warn("ILP batch write failed, tracking progress",
				"attempt", attempt+1,
				"max_retries", c.maxRetries,
				"recentlySent", recentlySent,
				"totalSentSoFar", linesSent,
				"totalLines", len(lines),
				"bytesWritten", written,
				"error", err,
			)

			// Update remaining lines for next retry
			currentLines = currentLines[recentlySent:]

			// Retry if we haven't exceeded max retries and there are remaining lines
			if attempt < c.maxRetries && len(currentLines) > 0 {
				delay := c.calculateBackoff(attempt)
				logging.Info("retrying remaining lines idempotently",
					"retryAttempt", attempt+1,
					"delay", delay,
					"linesAlreadySent", linesSent,
					"linesToRetry", len(currentLines),
				)

				// Wait before retry with backoff
				select {
				case <-ctx.Done():
					return linesSent, fmt.Errorf("context cancelled during retry backoff: %w", ctx.Err())
				case <-time.After(delay):
					continue // Continue retry loop
				}
			}

			// Max retries exceeded or no remaining lines
			return linesSent, fmt.Errorf("batch write failed after %d retries (linesSent=%d/%d): %w", attempt+1, linesSent, len(lines), err)
		}

		// Success - all remaining lines sent
		linesSent += len(currentLines)
		logging.Debug("ILP batch write success", "linesSent", linesSent, "bytesWritten", written)
		return linesSent, nil
	}

	// Should not reach here, but safety fallback
	return linesSent, fmt.Errorf("unexpected exit after %d retries", c.maxRetries)
}

// calculateBackoff calculates the delay for a given retry attempt using exponential backoff.
func (c *ILPClient) calculateBackoff(attempt int) time.Duration {
	// Exponential backoff: base * 2^attempt
	delay := c.retryBaseDelay
	for i := 0; i < attempt; i++ {
		delay *= 2
	}

	// Cap at max delay
	if delay > c.retryMaxDelay {
		delay = c.retryMaxDelay
	}

	return delay
}

func findColumn(header []string, name string) int {
	for i, col := range header {
		if col == name {
			return i
		}
	}
	return -1
}

func isSymbolColumn(col string, symbols []string) bool {
	for _, s := range symbols {
		if col == s {
			return true
		}
	}
	return false
}

// BufferedILPClient provides buffered ILP writes for better performance.
type BufferedILPClient struct {
	client    *ILPClient
	buffer    []string
	bufMutex  sync.Mutex
	flushSize int
}

// NewBufferedILPClient creates a new buffered ILP client.
func NewBufferedILPClient(client *ILPClient, flushSize int) *BufferedILPClient {
	return &BufferedILPClient{
		client:    client,
		buffer:    make([]string, 0, flushSize),
		flushSize: flushSize,
	}
}

// Write adds a line to the buffer.
func (bc *BufferedILPClient) Write(line string) {
	bc.bufMutex.Lock()
	defer bc.bufMutex.Unlock()
	bc.buffer = append(bc.buffer, line)
	if len(bc.buffer) >= bc.flushSize {
		bc.flushLocked()
	}
}

// Flush sends all buffered lines.
func (bc *BufferedILPClient) Flush() {
	bc.bufMutex.Lock()
	defer bc.bufMutex.Unlock()
	bc.flushLocked()
}

func (bc *BufferedILPClient) flushLocked() {
	if len(bc.buffer) > 0 {
		if _, err := bc.client.sendLines(context.Background(), bc.buffer); err != nil {
			logging.Warn("failed to flush buffered lines", "count", len(bc.buffer), "error", err)
		}
		bc.buffer = bc.buffer[:0]
	}
}
