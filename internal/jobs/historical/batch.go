package historical

import (
	"context"
	"fmt"

	"github.com/bishop-bot/datajobs/internal/database"
	"github.com/bishop-bot/datajobs/internal/logging"
)

// upsertOHLCVBatches upserts bars to QuestDB in batches.
func upsertOHLCVBatches(ctx context.Context, questDB *database.QuestDB, symbol string, bars []database.OHLCVBar) error {
	for i := 0; i < len(bars); i += upsertBatchSize {
		end := min(i+upsertBatchSize, len(bars))
		batch := bars[i:end]

		_, err := questDB.UpsertOHLCVBars(ctx, batch)
		if err != nil {
			return fmt.Errorf("failed to upsert batch for %s (rows %d-%d): %w", symbol, i, end, err)
		}

		logging.Debug("batch upsert complete",
			"symbol", symbol,
			"start", i,
			"end", end,
		)
	}
	return nil
}