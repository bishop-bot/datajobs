package database

import (
	"testing"
	"time"
)

func TestOHLCVBarToSlice(t *testing.T) {
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

	slice := bar.ToSlice()

	if len(slice) != 9 {
		t.Errorf("expected slice length 9, got %d", len(slice))
	}

	if slice[0] != "AAPL" {
		t.Errorf("expected symbol AAPL, got %v", slice[0])
	}
	if slice[1] != "IB" {
		t.Errorf("expected publisher IB, got %v", slice[1])
	}
	if slice[2] != int64(1717200000000000000) {
		t.Errorf("expected Ts 1717200000000000000, got %v", slice[2])
	}
	if slice[3] != int64(1717200060000000000) {
		t.Errorf("expected TsEnd 1717200060000000000, got %v", slice[3])
	}
	if slice[4] != 189.50 {
		t.Errorf("expected Open 189.50, got %v", slice[4])
	}
	if slice[5] != 190.25 {
		t.Errorf("expected High 190.25, got %v", slice[5])
	}
	if slice[6] != 189.10 {
		t.Errorf("expected Low 189.10, got %v", slice[6])
	}
	if slice[7] != 190.00 {
		t.Errorf("expected Close 190.00, got %v", slice[7])
	}
	if slice[8] != int64(1000000) {
		t.Errorf("expected Volume 1000000, got %v", slice[8])
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
	start := time.Now()
	result := &OHLCVUpsertResult{
		RowsAffected: 100,
		Duration:     time.Since(start),
	}

	if result.RowsAffected != 100 {
		t.Errorf("expected RowsAffected 100, got %d", result.RowsAffected)
	}

	if result.Duration <= 0 {
		t.Error("expected positive duration")
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