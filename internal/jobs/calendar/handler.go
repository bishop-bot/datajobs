package calendar

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
		return economicSyncHandlerImpl(ctx, job, db, earningsProvider)
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

// economicSyncHandlerImpl performs the economic calendar sync.
func economicSyncHandlerImpl(ctx context.Context, job worker.Job, db *database.DB, earningsProvider earnings.Provider) (string, error) {
	logger := logging.FromContext(ctx).With("job_id", job.ID)

	// Defensive nil checks
	if db == nil {
		return "", fmt.Errorf("SQLite database not available")
	}
	if earningsProvider == nil {
		return "", fmt.Errorf("earnings provider not available")
	}

	logger.Debug("economic calendar sync handler started")

	// Get today's date in YYYY-MM-DD format
	today := time.Now().UTC()
	todayStr := today.Format("2006-01-02")
	logger.Info("syncing economic calendar for date", "date", todayStr)

	// Fetch economic calendar from provider
	params := earnings.EconomicCalendarParams{
		Date:   earnings.NewCalendarDate(today),
		USMajor: false, // Fetch all events, not just US major
	}

	resp, err := earningsProvider.EconomicCalendar(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to fetch economic calendar: %w", err)
	}

	if len(resp.Events) == 0 {
		logger.Info("no economic events found for date", "date", todayStr)
		return fmt.Sprintf("synced date=%s: no events", todayStr), nil
	}

	logger.Info("fetched economic events",
		"date", todayStr,
		"total", len(resp.Events),
	)

	// Convert to entities
	entities := convertResponseToEntities(resp.Events, todayStr)
	if len(entities) == 0 {
		return fmt.Sprintf("synced date=%s: no events after conversion", todayStr), nil
	}

	// Query existing records
	repo := NewRepository(db)
	existing, err := repo.GetByDate(ctx, todayStr)
	if err != nil {
		return "", fmt.Errorf("failed to query existing records: %w", err)
	}

	// Defensive nil check
	if existing == nil {
		existing = make(map[string]*CalendarEconomic)
	}

	// Separate new events from existing (for logging purposes)
	newEvents := make([]*CalendarEconomic, 0, len(entities))
	updatedEvents := make([]*CalendarEconomic, 0, len(entities))

	for _, e := range entities {
		key := e.Country + "|" + e.EventName
		if existing[key] == nil {
			newEvents = append(newEvents, e)
		} else {
			updatedEvents = append(updatedEvents, e)
		}
	}

	// Upsert all events (handles both insert and update via ON CONFLICT)
	upserted, err := repo.UpsertBatch(ctx, entities)
	if err != nil {
		return "", fmt.Errorf("failed to upsert economic events: %w", err)
	}

	// Build result message
	result := fmt.Sprintf(
		"synced date=%s: total=%d, new=%d, updated=%d",
		todayStr,
		len(resp.Events),
		len(newEvents),
		len(updatedEvents),
	)

	logger.Info("economic calendar sync completed",
		"result", result,
		"upserted", upserted,
	)
	return result, nil
}

// convertResponseToEntities converts API response entries to CalendarEconomic entities.
func convertResponseToEntities(entries []earnings.EconomicEntry, date string) []*CalendarEconomic {
	entities := make([]*CalendarEconomic, 0, len(entries))

	for _, entry := range entries {
		// Convert country name to ISO code
		countryCode := earnings.CountryCode(entry.Country)
		if countryCode == "" {
			// Use original name if conversion fails
			countryCode = entry.Country
		}

		e := &CalendarEconomic{
			Country:    countryCode,
			EventName:  entry.EventName,
			Date:       entry.Date,
			Time:       entry.Time,
			Actual:     entry.Actual,
			Consensus:  entry.Consensus,
			Previous:   entry.Previous,
		}

		entities = append(entities, e)
	}

	return entities
}
