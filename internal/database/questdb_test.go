package database

import (
	"testing"
	"time"
)

func TestOHLCVBar(t *testing.T) {
	bar := OHLCVBar{
		Symbol:    "AAPL",
		Publisher: "IB",
		Ts:        1717200000000000000,
		TsEnd:     1717200060000000000,
		Open:      189.50,
		High:      190.25,
		Low:       189.10,
		Close:     190.00,
		Volume:    1000000,
	}

	// Verify bar fields are correctly populated
	if bar.Symbol != "AAPL" {
		t.Errorf("expected symbol AAPL, got %v", bar.Symbol)
	}
	if bar.Publisher != "IB" {
		t.Errorf("expected publisher IB, got %v", bar.Publisher)
	}
	if bar.Ts != 1717200000000000000 {
		t.Errorf("expected Ts 1717200000000000000, got %v", bar.Ts)
	}
	if bar.Open != 189.50 {
		t.Errorf("expected Open 189.50, got %v", bar.Open)
	}
}

func TestOHLCVColumns(t *testing.T) {
	columns := OHLCVColumns()

	expected := []string{"symbol", "publisher", "ts", "ts_end", "open", "high", "low", "close", "volume"}
	if len(columns) != len(expected) {
		t.Fatalf("expected %d columns, got %d", len(expected), len(columns))
	}

	for i, col := range columns {
		if col != expected[i] {
			t.Errorf("column %d: expected %s, got %s", i, expected[i], col)
		}
	}
}

func TestOHLCVUpsertResultDuration(t *testing.T) {
	// Use a known duration instead of time.Since() which can return 0 on fast machines
	expectedDuration := 250 * time.Millisecond
	result := &OHLCVUpsertResult{
		RowsAffected: 100,
		Duration:     expectedDuration,
	}

	if result.RowsAffected != 100 {
		t.Errorf("expected RowsAffected 100, got %d", result.RowsAffected)
	}

	if result.Duration != expectedDuration {
		t.Errorf("expected duration %v, got %v", expectedDuration, result.Duration)
	}
}

func TestOHLCVUpsertResultErrors(t *testing.T) {
	result := &OHLCVUpsertResult{
		RowsAffected: 50,
		Errors:       []string{"error 1", "error 2"},
	}

	if len(result.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(result.Errors))
	}
}