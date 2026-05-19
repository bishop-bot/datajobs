package ingestion

import (
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

// DefaultILPClient is the global ILP client instance.
var DefaultILPClient *ILPClient

// InitILP initializes the global ILP client.
func InitILP(cfg config.QuestDBConfig, m *metrics.Metrics) {
	DefaultILPClient = NewILPClient(cfg, m)
}

// ILPClient sends data to QuestDB via the InfluxDB Line Protocol.
type ILPClient struct {
	addr       string
	user       string
	password   string
	conn       net.Conn
	mu         sync.Mutex
	connected  bool
	reconnect  bool
	timeout    time.Duration
	metrics    *metrics.Metrics
}

// NewILPClient creates a new ILP client.
func NewILPClient(cfg config.QuestDBConfig, m *metrics.Metrics) *ILPClient {
	return &ILPClient{
		addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.ILPPort),
		user:     cfg.User,
		password: cfg.Password,
		timeout:  30 * time.Second,
		metrics:  m,
	}
}

// Connect establishes a connection to QuestDB ILP.
func (c *ILPClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return nil
	}

	conn, err := net.DialTimeout("tcp", c.addr, c.timeout)
	if err != nil {
		return fmt.Errorf("failed to connect to ILP: %w", err)
	}

	c.conn = conn
	c.connected = true

	// Send authentication if needed
	if c.user != "" {
		authLine := fmt.Sprintf("auth\t%s:%s\n", c.user, c.password)
		if _, err := conn.Write([]byte(authLine)); err != nil {
			conn.Close()
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
		lines, err := batchToILP(batch, header, timestampIdx, table, opts)
		if err != nil {
			errors++
			result.Errors = append(result.Errors, err.Error())
			continue
		}

		if err := c.sendLines(ctx, lines); err != nil {
			errors++
			result.Errors = append(result.Errors, err.Error())
			continue
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

func batchToILP(batch [][]string, header []string, timestampIdx int, table string, opts CSVOptions) ([]string, error) {
	lines := make([]string, 0, len(batch))

	for _, row := range batch {
		if len(row) != len(header) {
			continue // Skip malformed rows
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

	return lines, nil
}

func formatTags(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	return "," + joinStrings(tags, ",")
}

func formatFields(fields []string) string {
	return joinStrings(fields, ",")
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

func parseTimestamp(value string) string {
	// Handle various timestamp formats
	// QuestDB ILP expects Unix timestamp in nanoseconds or RFC3339
	// For now, assume Unix timestamp in milliseconds
	return value
}

func (c *ILPClient) sendLines(ctx context.Context, lines []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.conn == nil {
		if err := c.Connect(ctx); err != nil {
			return err
		}
	}

	for _, line := range lines {
		c.conn.SetWriteDeadline(time.Now().Add(c.timeout))
		if _, err := c.conn.Write([]byte(line + "\n")); err != nil {
			c.connected = false
			return fmt.Errorf("failed to send line: %w", err)
		}
	}

	return nil
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
	client   *ILPClient
	buffer   []string
	bufMutex sync.Mutex
	flushSize int
}

// NewBufferedILPClient creates a new buffered ILP client.
func NewBufferedILPClient(client *ILPClient, flushSize int) *BufferedILPClient {
	return &BufferedILPClient{
		client:    client,
		buffer:     make([]string, 0, flushSize),
		flushSize: flushSize,
	}
}

// Write adds a line to the buffer.
func (bc *BufferedILPClient) Write(line string) {
	bc.bufMutex.Lock()
	defer bc.bufMutex.Unlock()
	bc.buffer = append(bc.buffer, line)
	if len(bc.buffer) >= bc.flushSize {
		bc.flush()
	}
}

// Flush sends all buffered lines.
func (bc *BufferedILPClient) Flush() {
	bc.bufMutex.Lock()
	defer bc.bufMutex.Unlock()
	bc.flush()
}

func (bc *BufferedILPClient) flush() {
	if len(bc.buffer) > 0 {
		bc.client.sendLines(context.Background(), bc.buffer)
		bc.buffer = bc.buffer[:0]
	}
}