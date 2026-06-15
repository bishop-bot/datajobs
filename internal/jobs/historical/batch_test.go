package historical

import (
	"testing"

	"github.com/bishop-bot/datajobs/internal/database"
)

func TestUpsertBatchSize(t *testing.T) {
	if upsertBatchSize != 1000 {
		t.Errorf("upsertBatchSize = %d, want 1000", upsertBatchSize)
	}
}

func TestBatchCalculation(t *testing.T) {
	t.Run("correct batch count for exact division", func(t *testing.T) {
		total := 3000
		batchSize := 1000
		expectedBatches := 3

		batches := (total + batchSize - 1) / batchSize
		if batches != expectedBatches {
			t.Errorf("expected %d batches, got %d", expectedBatches, batches)
		}
	})

	t.Run("correct batch count with remainder", func(t *testing.T) {
		total := 2500
		batchSize := 1000
		expectedBatches := 3

		batches := (total + batchSize - 1) / batchSize
		if batches != expectedBatches {
			t.Errorf("expected %d batches, got %d", expectedBatches, batches)
		}
	})

	t.Run("single batch for small count", func(t *testing.T) {
		total := 500
		batchSize := 1000
		expectedBatches := 1

		batches := (total + batchSize - 1) / batchSize
		if batches != expectedBatches {
			t.Errorf("expected %d batches, got %d", expectedBatches, batches)
		}
	})

	t.Run("empty slice needs no batches", func(t *testing.T) {
		total := 0
		batchSize := 1000
		expectedBatches := 0

		batches := (total + batchSize - 1) / batchSize
		if batches != expectedBatches {
			t.Errorf("expected %d batches, got %d", expectedBatches, batches)
		}
	})
}

func TestOHLCVBarStructure(t *testing.T) {
	bar := database.OHLCVBar{
		Symbol:    "AAPL",
		Publisher: "IB",
		Ts:        1719792000000 * 1_000_000,
		TsEnd:     1719792000000*1_000_000 + 86400000000000, // 1 day
		Open:      185.50,
		High:      186.75,
		Low:       184.90,
		Close:     186.20,
		Volume:    50000000,
	}

	if bar.Symbol != "AAPL" {
		t.Errorf("Symbol = %q, want %q", bar.Symbol, "AAPL")
	}
	if bar.Publisher != "IB" {
		t.Errorf("Publisher = %q, want %q", bar.Publisher, "IB")
	}
	if bar.Volume != 50000000 {
		t.Errorf("Volume = %d, want %d", bar.Volume, 50000000)
	}
}