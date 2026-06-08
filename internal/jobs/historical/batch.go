package historical

import (
	"context"
	"fmt"

	"github.com/bishop-bot/datajobs/internal/database"
	"github.com/bishop-bot/datajobs/internal/logging"
)

// upsertOHLCVBatches upserts bars to QuestDB in batches.
func upsertOHLCVBatches(ctx context.Context, questDB *database.QuestDB, symbol string, bars []database.OHLCVBar) error {
	if questDB == nil {
		logging.Error("QuestDB is nil, cannot upsert", "symbol", symbol, "bars_count", len(bars))
		return fmt.Errorf("QuestDB not connected")
	}

	logging.Info("upserting bars to QuestDB",
		"symbol", symbol,
		"bars_count", len(bars),
	)

	for i := 0; i < len(bars); i += upsertBatchSize {
		end := min(i+upsertBatchSize, len(bars))
		batch := bars[i:end]

		count, err := questDB.UpsertOHLCVBars(ctx, batch)
		if err != nil {
			logging.Error("QuestDB upsert failed",
				"symbol", symbol,
				"error", err.Error(),
				"batch_start", i,
				"batch_end", end,
			)
			return fmt.Errorf("failed to upsert batch for %s (rows %d-%d): %w", symbol, i, end, err)
		}

		logging.Info("batch upsert complete",
			"symbol", symbol,
			"batch_start", i,
			"batch_end", end,
			"upserted_count", count,
		)
	}
	return nil
}