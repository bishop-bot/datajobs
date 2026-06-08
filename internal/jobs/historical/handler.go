package historical

import (
	"context"
	"fmt"
	"strings"

	"github.com/bishop-bot/datajobs/internal/database"
	"github.com/bishop-bot/datajobs/internal/logging"
	"github.com/bishop-bot/datajobs/internal/providers/ib"
	"github.com/bishop-bot/datajobs/internal/worker"
)

// HistoricalDataHandlerWithDB creates a handler with QuestDB and SQLite access.
// The ibProvider parameter accepts either *ib.Client or a mock implementation for testing.
func HistoricalDataHandlerWithDB(questDB *database.QuestDB, sqliteDB *database.DB, ibProvider ib.Provider) worker.JobFunc {
	return func(ctx context.Context, job worker.Job) (string, error) {
		return historicalDataHandlerImpl(ctx, job, questDB, sqliteDB, ibProvider)
	}
}

// historicalDataHandlerImpl is the main implementation.
func historicalDataHandlerImpl(ctx context.Context, job worker.Job, questDB *database.QuestDB, sqliteDB *database.DB, ibProvider ib.Provider) (string, error) {
	logger := logging.FromContext(ctx).With("job_id", job.ID)

	if ibProvider == nil {
		return "", fmt.Errorf("IB provider not available")
	}
	if questDB == nil {
		return "", fmt.Errorf("QuestDB not connected")
	}

	params := parseHistoricalParams(job.Metadata)
	instruments, err := getInstruments(ctx, job, params, sqliteDB)
	if err != nil {
		return "", fmt.Errorf("failed to get instruments: %w", err)
	}

	if len(instruments) == 0 {
		logger.Info("no instruments to fetch")
		return "no instruments to process", nil
	}

	logger.Info("fetching historical data",
		"count", len(instruments),
		"period", params.Period,
		"bar", params.Bar,
	)

	totalBars, failedSymbols := processInstruments(ctx, ibProvider, questDB, instruments, params)

	result := buildResultMessage(len(instruments), totalBars, failedSymbols)
	return result, nil
}

// processInstruments fetches and upserts OHLCV data for multiple instruments.
func processInstruments(ctx context.Context, ibProvider ib.Provider, questDB *database.QuestDB, instruments []instrument, params historicalParams) (totalBars int, failedSymbols []string) {
	logger := logging.FromContext(ctx)

	for _, instr := range instruments {
		bars, err := fetchOHLCV(ctx, ibProvider, instr, params)
		if err != nil {
			logger.Error("failed to fetch data", "symbol", instr.Symbol, "error", err)
			failedSymbols = append(failedSymbols, instr.Symbol)
			continue
		}

		if len(bars) == 0 {
			continue
		}

		if err := upsertOHLCVBatches(ctx, questDB, instr.Symbol, bars); err != nil {
			logger.Error("failed to upsert bars", "symbol", instr.Symbol, "error", err)
			failedSymbols = append(failedSymbols, instr.Symbol)
			continue
		}

		totalBars += len(bars)
		logger.Debug("upserted bars", "symbol", instr.Symbol, "count", len(bars))
	}

	return totalBars, failedSymbols
}

// buildResultMessage creates a summary message for the job result.
func buildResultMessage(instrumentCount, totalBars int, failedSymbols []string) string {
	parts := []string{
		fmt.Sprintf("processed %d instruments", instrumentCount),
		fmt.Sprintf("upserted %d bars", totalBars),
	}
	if len(failedSymbols) > 0 {
		parts = append(parts, fmt.Sprintf("failed: %s", strings.Join(failedSymbols, ", ")))
	}
	return strings.Join(parts, "; ")
}