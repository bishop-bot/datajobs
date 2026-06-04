package ingestion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/bishop-bot/datajobs/internal/config"
	"github.com/bishop-bot/datajobs/internal/database"
	"github.com/bishop-bot/datajobs/internal/logging"
)

// HTTPClient sends data to QuestDB via the REST API.
type HTTPClient struct {
	baseURL    string
	user       string
	password   string
	httpClient *http.Client
	timeout    time.Duration
}

// HTTPIngestResult contains the result of an HTTP ingestion operation.
type HTTPIngestResult struct {
	Table       string
	RowsSent    int
	RowsImported int
	Duration    time.Duration
	ImportID    string
	Errors      []string
}

// NewHTTPClient creates a new QuestDB HTTP client.
func NewHTTPClient(cfg config.QuestDBConfig) *HTTPClient {
	return &HTTPClient{
		baseURL: fmt.Sprintf("http://%s:%d", cfg.Host, cfg.ILPPort),
		user:    cfg.User,
		password: cfg.Password,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		timeout: 30 * time.Second,
	}
}

// IngestOHLCV ingests OHLCV bars via the QuestDB HTTP REST API.
// It sends data in CSV format with optional schema specification.
// Returns the import ID and any errors encountered.
func (c *HTTPClient) IngestOHLCV(ctx context.Context, table string, bars []database.OHLCVBar) (*HTTPIngestResult, error) {
	if len(bars) == 0 {
		return &HTTPIngestResult{Table: table}, nil
	}

	start := time.Now()
	result := &HTTPIngestResult{
		Table:    table,
		RowsSent: len(bars),
	}

	// Build CSV data
	csvData, err := barsToCSV(bars)
	if err != nil {
		return nil, fmt.Errorf("failed to build CSV: %w", err)
	}

	// Build schema for timestamp column
	schema := []map[string]string{
		{"name": "ts", "type": "TIMESTAMP", "pattern": "yyyy-MM-dd'T'HH:mm:ss.SSS'Z'"},
		{"name": "ts_end", "type": "TIMESTAMP", "pattern": "yyyy-MM-dd'T'HH:mm:ss.SSS'Z'"},
		{"name": "symbol", "type": "SYMBOL"},
		{"name": "publisher", "type": "SYMBOL"},
		{"name": "open", "type": "DOUBLE"},
		{"name": "high", "type": "DOUBLE"},
		{"name": "low", "type": "DOUBLE"},
		{"name": "close", "type": "DOUBLE"},
		{"name": "volume", "type": "LONG"},
	}

	// Send request
	importID, err := c.sendImportRequest(ctx, table, csvData, schema)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result, err
	}

	result.ImportID = importID
	result.Duration = time.Since(start)

	logging.Debug("HTTP ingest completed",
		"table", table,
		"rows_sent", result.RowsSent,
		"import_id", result.ImportID,
		"duration", result.Duration,
	)

	return result, nil
}

// sendImportRequest sends a multipart form to QuestDB's /imp endpoint.
func (c *HTTPClient) sendImportRequest(ctx context.Context, table string, csvData []byte, schema []map[string]string) (string, error) {
	// Create multipart form
	body := &bytes.Buffer{}
	writer := newMultipartWriter(body)

	// Add schema
	schemaJSON, err := json.Marshal(schema)
	if err != nil {
		return "", fmt.Errorf("failed to encode schema: %w", err)
	}
	writer.WriteField("schema", string(schemaJSON))

	// Add CSV data
	writer.WriteFile("data", "ohlcv.csv", "text/csv", csvData)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/imp", body)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Add basic auth if configured
	if c.user != "" {
		req.SetBasicAuth(c.user, c.password)
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return "", fmt.Errorf("QuestDB returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response to get import ID
	// QuestDB returns the import ID in the response body
	// Format: {"ok": true, ...} or just the id
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		// Try to extract import ID from plain text response
		return string(respBody), nil
	}

	// Check for import id in response
	if id, ok := result["id"].(string); ok {
		return id, nil
	}

	// Check for OK status
	if success, ok := result["ok"].(bool); ok && success {
		return "", nil
	}

	return "", nil
}

// GetImportStatus checks the status of an import by ID.
// Returns the import status and any rows imported/errors.
func (c *HTTPClient) GetImportStatus(ctx context.Context, importID string) (*ImportStatus, error) {
	query := fmt.Sprintf("SELECT * FROM 'sys.text_import_log' WHERE id = '%s'", importID)

	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/exec?query="+query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.user != "" {
		req.SetBasicAuth(c.user, c.password)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("status check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status check returned %d: %s", resp.StatusCode, string(body))
	}

	// Parse JSON response
	var result struct {
		Columns []string `json:"columns"`
		Dataset [][]interface{} `json:"dataset"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result.Dataset) == 0 {
		return nil, fmt.Errorf("no import found with id: %s", importID)
	}

	// Parse row into ImportStatus
	status := &ImportStatus{}
	cols := result.Columns

	for i, col := range cols {
		if i >= len(result.Dataset[0]) {
			continue
		}
		val := result.Dataset[0][i]

		switch col {
		case "status":
			if s, ok := val.(string); ok {
				status.Status = s
			}
		case "table":
			if s, ok := val.(string); ok {
				status.Table = s
			}
		case "rows_imported":
			if f, ok := val.(float64); ok {
				status.RowsImported = int(f)
			}
		case "errors":
			if f, ok := val.(float64); ok {
				status.Errors = int(f)
			}
		case "message":
			if s, ok := val.(string); ok {
				status.Message = s
			}
		}
	}

	return status, nil
}

// ImportStatus represents the status of a QuestDB import operation.
type ImportStatus struct {
	Table        string
	Status       string
	RowsImported int
	Errors       int
	Message      string
}

// multipartWriter handles multipart form writing.
type multipartWriter struct {
	writer *bytes.Buffer
	writer2 io.Writer
	boundary string
}

func newMultipartWriter(buf *bytes.Buffer) *multipartWriter {
	return &multipartWriter{
		writer:   buf,
		writer2:   buf,
		boundary: "boundary-" + strconv.FormatInt(time.Now().UnixNano(), 16),
	}
}

func (m *multipartWriter) FormDataContentType() string {
	return "multipart/form-data; boundary=" + m.boundary
}

func (m *multipartWriter) WriteField(key, value string) {
	fmt.Fprintf(m.writer2, "--%s\r\n", m.boundary)
	fmt.Fprintf(m.writer2, "Content-Disposition: form-data; name=\"%s\"\r\n", key)
	fmt.Fprintf(m.writer2, "\r\n")
	fmt.Fprintf(m.writer2, "%s\r\n", value)
}

func (m *multipartWriter) WriteFile(fieldname, filename, contentType string, data []byte) {
	fmt.Fprintf(m.writer2, "--%s\r\n", m.boundary)
	fmt.Fprintf(m.writer2, "Content-Disposition: form-data; name=\"%s\"; filename=\"%s\"\r\n", fieldname, filename)
	fmt.Fprintf(m.writer2, "Content-Type: %s\r\n", contentType)
	fmt.Fprintf(m.writer2, "\r\n")
	m.writer2.Write(data)
	fmt.Fprintf(m.writer2, "\r\n")
}

func (m *multipartWriter) Close() {
	fmt.Fprintf(m.writer2, "--%s--\r\n", m.boundary)
}

// barsToCSV converts OHLCV bars to CSV format.
// Header: symbol,publisher,ts,ts_end,open,high,low,close,volume
func barsToCSV(bars []database.OHLCVBar) ([]byte, error) {
	buf := &bytes.Buffer{}

	// Write header
	buf.WriteString("symbol,publisher,ts,ts_end,open,high,low,close,volume\n")

	// Write data rows
	for _, bar := range bars {
		// Convert nanosecond timestamps to ISO8601 format
		tsStart := formatTimestamp(bar.Ts)
		tsEnd := formatTimestamp(bar.TsEnd)

		// Format row: symbol,publisher,ts,ts_end,open,high,low,close,volume
		fmt.Fprintf(buf, "%s,%s,%s,%s,%.8f,%.8f,%.8f,%.8f,%d\n",
			escapeCSV(bar.Symbol),
			escapeCSV(bar.Publisher),
			tsStart,
			tsEnd,
			bar.Open,
			bar.High,
			bar.Low,
			bar.Close,
			bar.Volume,
		)
	}

	return buf.Bytes(), nil
}

// formatTimestamp converts nanoseconds to ISO8601 format.
func formatTimestamp(ns int64) string {
	// Convert nanoseconds to time.Time
	t := time.Unix(0, ns).UTC()
	return t.Format("2006-01-02T15:04:05.000Z")
}

// escapeCSV escapes a string for CSV output.
func escapeCSV(s string) string {
	// Check if escaping is needed
	needsQuotes := false
	for _, c := range s {
		if c == ',' || c == '"' || c == '\n' || c == '\r' {
			needsQuotes = true
			break
		}
	}

	if !needsQuotes {
		return s
	}

	// Escape quotes and wrap in quotes
	var buf bytes.Buffer
	buf.WriteByte('"')
	for _, c := range s {
		if c == '"' {
			buf.WriteByte('"')
		}
		buf.WriteRune(c)
	}
	buf.WriteByte('"')
	return buf.String()
}

// IngestOHLCVBatch ingests bars in batches with progress tracking.
// This is useful for large datasets to avoid memory issues.
func (c *HTTPClient) IngestOHLCVBatch(ctx context.Context, table string, bars []database.OHLCVBar, batchSize int) (*HTTPIngestResult, error) {
	if len(bars) == 0 {
		return &HTTPIngestResult{Table: table}, nil
	}

	if batchSize <= 0 {
		batchSize = 10000 // Default batch size
	}

	result := &HTTPIngestResult{
		Table:    table,
		RowsSent: len(bars),
	}

	start := time.Now()
	var lastImportID string

	for i := 0; i < len(bars); i += batchSize {
		end := i + batchSize
		if end > len(bars) {
			end = len(bars)
		}

		batch := bars[i:end]
		ingestResult, err := c.IngestOHLCV(ctx, table, batch)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("batch %d-%d: %v", i, end, err))
			continue
		}

		result.RowsImported += len(batch)
		if ingestResult.ImportID != "" {
			lastImportID = ingestResult.ImportID
		}

		// Progress logging
		logging.Debug("batch ingest progress",
			"table", table,
			"batch", i/batchSize+1,
			"rows_imported", result.RowsImported,
			"total", len(bars),
		)
	}

	result.ImportID = lastImportID
	result.Duration = time.Since(start)

	return result, nil
}