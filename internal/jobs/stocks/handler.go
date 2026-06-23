package stocks

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/bishop-bot/datajobs/internal/database"
	"github.com/bishop-bot/datajobs/internal/logging"
	"github.com/bishop-bot/datajobs/internal/providers/earnings"
	"github.com/bishop-bot/datajobs/internal/worker"
)

// HandlerWithDeps creates a job handler with database and earnings provider dependencies.
func HandlerWithDeps(db *database.DB, earningsProvider earnings.Provider) worker.JobFunc {
	// Validate the provider is actually usable.
	// Note: In Go, an interface holding a nil pointer of a concrete type is NOT nil.
	// We must use reflection to check if the underlying value is nil.
	if !isProviderUsable(earningsProvider) {
		return func(ctx context.Context, job worker.Job) (string, error) {
			return "", fmt.Errorf("earnings provider not configured")
		}
	}

	return func(ctx context.Context, job worker.Job) (string, error) {
		return earningsSyncHandlerImpl(ctx, job, db, earningsProvider)
	}
}

// isProviderUsable checks if the earnings provider is usable.
// In Go, an interface holding a nil pointer of a concrete type is NOT nil.
// This function uses reflection to detect "typed nils".
func isProviderUsable(p earnings.Provider) bool {
	if p == nil {
		return false
	}
	// Use reflection to check if the underlying value is nil
	v := reflect.ValueOf(p)
	if v.Kind() == reflect.Ptr {
		return !v.IsNil()
	}
	// If it's not a pointer type, check the Elem() for interface types
	if v.Kind() == reflect.Interface && !v.IsNil() {
		elem := v.Elem()
		if elem.Kind() == reflect.Ptr {
			return !elem.IsNil()
		}
	}
	return true
}

// Default look-forward period for earnings sync (in days)
const defaultLookForwardDays = 30

// earningsSyncHandlerImpl performs the daily earnings sync.
func earningsSyncHandlerImpl(ctx context.Context, job worker.Job, db *database.DB, earningsProvider earnings.Provider) (string, error) {
	logger := logging.FromContext(ctx).With("job_id", job.ID)

	// Defensive nil checks
	if db == nil {
		return "", fmt.Errorf("SQLite database not available")
	}
	if earningsProvider == nil {
		return "", fmt.Errorf("earnings provider not available")
	}

	logger.Debug("earnings sync handler started")

	// Get look-forward days from job metadata, default to 30
	lookForwardDays := defaultLookForwardDays
	if days, ok := job.Metadata["lookForwardDays"]; ok {
		if d, ok := days.(int); ok && d > 0 {
			lookForwardDays = d
		} else if d, ok := days.(float64); ok && d > 0 {
			lookForwardDays = int(d)
		}
	}
	logger.Info("starting earnings sync job", "lookForwardDays", lookForwardDays)

	// Sync earnings for each day in the look-forward period
	today := time.Now().UTC()
	totalUpserted := 0
	totalSkipped := 0
	daysProcessed := 0

	for i := 0; i <= lookForwardDays; i++ {
		syncDate := today.AddDate(0, 0, i)
		syncDateStr := syncDate.Format("2006-01-02")

		upserted, skipped, err := syncEarningsForDate(ctx, db, earningsProvider, syncDateStr)
		if err != nil {
			logger.Error("failed to sync earnings for date", "date", syncDateStr, "error", err)
			// Continue with next date rather than failing the entire job
			continue
		}

		totalUpserted += upserted
		totalSkipped += skipped
		daysProcessed++
	}

	// Build result message
	result := fmt.Sprintf(
		"synced lookForwardDays=%d: processed=%d, upserted=%d, skipped=%d",
		lookForwardDays,
		daysProcessed,
		totalUpserted,
		totalSkipped,
	)

	logger.Info("earnings sync completed", "result", result)
	return result, nil
}

// syncEarningsForDate fetches and syncs earnings for a specific date.
// Returns the number of records upserted and skipped.
func syncEarningsForDate(ctx context.Context, db *database.DB, earningsProvider earnings.Provider, date string) (upserted int, skipped int, err error) {
	logger := logging.FromContext(ctx).With("date", date)

	// Fetch earnings calendar from provider for this specific date
	parsedDate, _ := time.Parse("2006-01-02", date)
	calendarDate := earnings.NewCalendarDate(parsedDate)
	resp, err := earningsProvider.EarningsCalendar(ctx, calendarDate)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to fetch earnings calendar: %w", err)
	}

	// Convert response to StockEarnings entities
	allEarnings := convertResponseToEntities(resp)
	if len(allEarnings) == 0 {
		logger.Debug("no earnings data for date")
		return 0, 0, nil
	}

	logger.Debug("fetched earnings data",
		"total", len(allEarnings),
		"pre", len(resp.Pre),
		"after", len(resp.After),
		"notSupplied", len(resp.NotSupplied),
	)

	// Extract symbols for existing records check
	symbols := make([]string, len(allEarnings))
	for i, e := range allEarnings {
		symbols[i] = e.Symbol
	}

	// Query existing records
	repo := NewRepository(db)
	existing, err := repo.GetByDateAndSymbols(ctx, date, symbols)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to query existing records: %w", err)
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

	skipped = len(allEarnings) - len(newEarnings)

	if len(newEarnings) == 0 {
		return 0, skipped, nil
	}

	// Upsert new records
	u, err := repo.UpsertBatch(ctx, newEarnings)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to upsert earnings: %w", err)
	}

	return u, skipped, nil
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
