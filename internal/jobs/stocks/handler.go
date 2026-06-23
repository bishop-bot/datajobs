package stocks

import (
	"context"
	"fmt"
	"time"

	"github.com/bishop-bot/datajobs/internal/database"
	"github.com/bishop-bot/datajobs/internal/logging"
	"github.com/bishop-bot/datajobs/internal/providers/earnings"
	"github.com/bishop-bot/datajobs/internal/worker"
)

// HandlerWithDeps creates a job handler with database and earnings provider dependencies.
func HandlerWithDeps(db *database.DB, earningsProvider earnings.Provider) worker.JobFunc {
	return func(ctx context.Context, job worker.Job) (string, error) {
		return earningsSyncHandlerImpl(ctx, job, db, earningsProvider)
	}
}

// earningsSyncHandlerImpl performs the daily earnings sync.
func earningsSyncHandlerImpl(ctx context.Context, job worker.Job, db *database.DB, earningsProvider earnings.Provider) (string, error) {
	logger := logging.FromContext(ctx).With("job_id", job.ID)

	logger.Debug("earnings sync handler started",
		"db_nil", db == nil,
		"earningsProvider_nil", earningsProvider == nil,
	)

	if db == nil {
		return "", fmt.Errorf("SQLite database not available")
	}
	if earningsProvider == nil {
		return "", fmt.Errorf("earnings provider not available")
	}

	// Get today's date in YYYY-MM-DD format
	today := time.Now().UTC().Format("2006-01-02")
	logger.Info("syncing earnings for date", "date", today)

	// Fetch earnings calendar from provider
	resp, err := earningsProvider.EarningsCalendar(ctx, earnings.NewCalendarDate(time.Now().UTC()))
	if err != nil {
		return "", fmt.Errorf("failed to fetch earnings calendar: %w", err)
	}

	// Convert response to StockEarnings entities
	allEarnings := convertResponseToEntities(resp)
	logger.Info("fetched earnings data",
		"total", len(allEarnings),
		"pre", len(resp.Pre),
		"after", len(resp.After),
		"notSupplied", len(resp.NotSupplied),
	)

	if len(allEarnings) == 0 {
		return "no earnings data to sync", nil
	}

	// Extract symbols for existing records check
	symbols := make([]string, len(allEarnings))
	for i, e := range allEarnings {
		symbols[i] = e.Symbol
	}

	// Query existing records
	repo := NewRepository(db)
	existing, err := repo.GetByDateAndSymbols(ctx, today, symbols)
	if err != nil {
		return "", fmt.Errorf("failed to query existing records: %w", err)
	}

	// Defensive nil check (should never happen due to repository contract)
	if existing == nil {
		existing = make(map[string]*StockEarnings)
	}

	// Filter out existing records (no changes needed)
	newEarnings := make([]*StockEarnings, 0, len(allEarnings))
	for _, e := range allEarnings {
		if existing[e.Symbol] == nil {
			newEarnings = append(newEarnings, e)
		}
	}

	// Upsert new records
	upserted, err := repo.UpsertBatch(ctx, newEarnings)
	if err != nil {
		return "", fmt.Errorf("failed to upsert earnings: %w", err)
	}

	// Build result message
	result := fmt.Sprintf(
		"synced date=%s: total=%d, pre=%d, after=%d, notSupplied=%d, upserted=%d, skipped=%d",
		today,
		len(allEarnings),
		len(resp.Pre),
		len(resp.After),
		len(resp.NotSupplied),
		upserted,
		len(allEarnings)-upserted,
	)

	logger.Info("earnings sync completed", "result", result)
	return result, nil
}

// convertResponseToEntities converts the API response to StockEarnings entities.
func convertResponseToEntities(resp *earnings.EarningsCalendarResponse) []*StockEarnings {
	entities := make([]*StockEarnings, 0, len(resp.Pre)+len(resp.After)+len(resp.NotSupplied))

	// Process pre-market (BMO)
	for _, entry := range resp.Pre {
		entities = append(entities, entryToStockEarnings(entry, resp.Date, TimeBMO))
	}

	// Process after-market (AMC)
	for _, entry := range resp.After {
		entities = append(entities, entryToStockEarnings(entry, resp.Date, TimeAMC))
	}

	// Process not supplied (empty time)
	for _, entry := range resp.NotSupplied {
		entities = append(entities, entryToStockEarnings(entry, resp.Date, ""))
	}

	return entities
}

// entryToStockEarnings converts an EarningsEntry to a StockEarnings entity.
func entryToStockEarnings(entry earnings.EarningsEntry, date string, timeVal Time) *StockEarnings {
	e := &StockEarnings{
		Symbol: entry.Symbol,
		Name:   entry.Name,
		Date:   date,
		Time:   timeVal,
		Status: StatusEstimate, // Default to estimate
	}

	// Set EPS values
	if entry.EpsEstimate != 0 {
		e.EPSEstimated = &entry.EpsEstimate
	}
	if entry.Eps != 0 {
		e.EPS = &entry.Eps
		e.Status = StatusActual // If actual EPS is reported, status is actual
	}

	// Set revenue values
	if entry.RevenueEstimate != 0 {
		e.RevenueEstimated = &entry.RevenueEstimate
	}
	if entry.Revenue != 0 {
		e.Revenue = &entry.Revenue
	}

	return e
}
