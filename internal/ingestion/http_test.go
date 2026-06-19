package ingestion

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/bishop-bot/datajobs/internal/config"
	"github.com/bishop-bot/datajobs/internal/database"
)

func TestBarsToCSV(t *testing.T) {
	bars := []database.OHLCVBar{
		{
			Symbol:    "AAPL",
			Publisher: "IB",
			BarSize:   "5mins",
			Ts:        1719792000000000000, // 2024-07-01 00:00:00 UTC
			TsEnd:     1719878400000000000, // 2024-07-02 00:00:00 UTC
			Open:      185.50,
			High:      186.75,
			Low:      184.90,
			Close:     186.20,
			Volume:    50000000,
		},
		{
			Symbol:    "IBM",
			Publisher: "IB",
			BarSize:   "5mins",
			Ts:        1719792000000000000,
			TsEnd:     1719878400000000000,
			Open:      190.00,
			High:      191.00,
			Low:      189.50,
			Close:     190.50,
			Volume:    3000000,
		},
	}

	csv, err := barsToCSV(bars)
	if err != nil {
		t.Fatalf("barsToCSV failed: %v", err)
	}

	// Check header
	expectedHeader := "symbol,publisher,bar_size,ts,ts_end,open,high,low,close,volume\n"
	if string(csv[:len(expectedHeader)]) != expectedHeader {
		t.Errorf("header mismatch:\ngot: %s\nwant: %s", string(csv[:len(expectedHeader)]), expectedHeader)
	}

	// Check data rows are present
	data := string(csv)
	if len(data) < len(expectedHeader) {
		t.Error("CSV data is empty")
	}

	// Should contain AAPL and IBM symbols
	if !containsSubstring(data, "AAPL") {
		t.Error("CSV missing AAPL symbol")
	}
	if !containsSubstring(data, "IBM") {
		t.Error("CSV missing IBM symbol")
	}

	// Should contain bar_size
	if !containsSubstring(data, "5mins") {
		t.Error("CSV missing bar_size")
	}

	t.Logf("Generated CSV:\n%s", string(csv))
}

func TestBarsToCSV_Empty(t *testing.T) {
	bars := []database.OHLCVBar{}
	csv, err := barsToCSV(bars)
	if err != nil {
		t.Fatalf("barsToCSV failed: %v", err)
	}

	// Should just have header
	expected := "symbol,publisher,bar_size,ts,ts_end,open,high,low,close,volume\n"
	if string(csv) != expected {
		t.Errorf("empty bars CSV mismatch:\ngot: %s\nwant: %s", string(csv), expected)
	}
}

func TestBarsToCSV_WithSpecialChars(t *testing.T) {
	bars := []database.OHLCVBar{
		{
			Symbol:    "A,B", // Comma in symbol
			Publisher: "IB",
			BarSize:   "1hour",
			Ts:        1719792000000000000,
			TsEnd:     1719878400000000000,
			Open:      185.50,
			High:      186.75,
			Low:      184.90,
			Close:     186.20,
			Volume:    50000000,
		},
	}

	csv, err := barsToCSV(bars)
	if err != nil {
		t.Fatalf("barsToCSV failed: %v", err)
	}

	// Symbol with comma should be quoted
	data := string(csv)
	if !containsSubstring(data, "\"A,B\"") {
		t.Errorf("CSV should quote symbol with comma:\n%s", data)
	}

	t.Logf("CSV with special chars:\n%s", data)
}

func TestFormatTimestamp(t *testing.T) {
	tests := []struct {
		name     string
		ns       int64
		expected string
	}{
		{
			name:     "epoch",
			ns:       0,
			expected: "1970-01-01T00:00:00.000Z",
		},
		{
			name:     "2024-07-01 00:00:00 UTC",
			ns:       1719792000000000000,
			expected: "2024-07-01T00:00:00.000Z",
		},
		{
			name:     "2024-07-01 12:30:45.123 UTC",
			ns:       1719829845123000000,
			expected: "2024-07-01T10:30:45.123Z", // UTC timestamp - verify actual conversion
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTimestamp(tt.ns)
			if result != tt.expected {
				t.Errorf("formatTimestamp(%d) = %s, want %s", tt.ns, result, tt.expected)
			}
		})
	}
}

func TestEscapeCSV(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no escaping needed",
			input:    "AAPL",
			expected: "AAPL",
		},
		{
			name:     "comma",
			input:    "A,B",
			expected: `"A,B"`,
		},
		{
			name:     "quotes",
			input:    `A"B`,
			expected: `"A""B"`,
		},
		{
			name:     "newline",
			input:    "A\nB",
			expected: `"A
B"`,
		},
		{
			name:     "empty",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeCSV(tt.input)
			if result != tt.expected {
				t.Errorf("escapeCSV(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMultipartWriter(t *testing.T) {
	buf := &bytes.Buffer{}
	writer := newMultipartWriter(buf)

	writer.WriteField("name", "value")
	writer.WriteFile("file", "test.csv", "text/csv", []byte("col1,col2\n1,2"))
	writer.Close()

	data := buf.String()

	// Should contain boundary markers
	if !containsSubstring(data, "--boundary-") {
		t.Error("missing boundary marker")
	}
	if !containsSubstring(data, "Content-Disposition: form-data") {
		t.Error("missing Content-Disposition")
	}
	if !containsSubstring(data, "name=\"name\"") {
		t.Error("missing field name")
	}
	if !containsSubstring(data, "name=\"file\"") {
		t.Error("missing file field name")
	}

	t.Logf("Multipart body:\n%s", data)
}

func TestNewHTTPClient(t *testing.T) {
	client := NewHTTPClient(config.QuestDBConfig{
		Host:         "localhost",
		ILPHTTPPort:  9000,
		User:         "admin",
		Password:     "quest",
	})

	if client.baseURL != "http://localhost:9000" {
		t.Errorf("baseURL = %s, want http://localhost:9000", client.baseURL)
	}
	if client.user != "admin" {
		t.Errorf("user = %s, want admin", client.user)
	}
}

func TestHTTPIngestResult_Duration(t *testing.T) {
	result := &HTTPIngestResult{
		Table:    "test",
		RowsSent: 100,
	}

	// Verify struct fields exist and can be set
	result.RowsImported = 100
	result.Duration = 100 * time.Millisecond

	if result.Duration < 0 {
		t.Error("Duration should be non-negative")
	}
}

func containsSubstring(s, substr string) bool {
	return strings.Contains(s, substr)
}