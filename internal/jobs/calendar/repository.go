package calendar

import (
	"context"
	"fmt"

	"github.com/bishop-bot/datajobs/internal/database"
	"github.com/bishop-bot/datajobs/internal/logging"
)

// Repository handles database operations for economic calendar events.
type Repository struct {
	db *database.DB
}

// NewRepository creates a new Repository instance.
func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

// GetByDate fetches existing economic calendar events for a given date.
func (r *Repository) GetByDate(ctx context.Context, date string) (map[string]*CalendarEconomic, error) {
	query := `
		SELECT id, country, event_name, date, time, actual, consensus, previous, created_at, updated_at
		FROM calendar_economic
		WHERE date = ?
	`

	rows, err := r.db.Query(ctx, query, date)
	if err != nil {
		return nil, fmt.Errorf("failed to query calendar_economic: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*CalendarEconomic)
	for rows.Next() {
		var e CalendarEconomic
		var timeVal, actual, consensus, previous *string

		err := rows.Scan(
			&e.ID, &e.Country, &e.EventName, &e.Date, &timeVal, &actual, &consensus, &previous,
			&e.CreatedAt, &e.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		e.Time = derefString(timeVal)
		e.Actual = actual
		e.Consensus = consensus
		e.Previous = previous

		// Key is country + event_name for uniqueness
		key := e.Country + "|" + e.EventName
		result[key] = &e
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return result, nil
}

// Upsert inserts or updates an economic calendar event.
// If a record with the same country, date, and event_name exists, it updates.
// Otherwise, it inserts a new record.
func (r *Repository) Upsert(ctx context.Context, e *CalendarEconomic) error {
	query := `
		INSERT INTO calendar_economic (
			country, event_name, date, time, actual, consensus, previous
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(country, event_name, date) DO UPDATE SET
			time = excluded.time,
			actual = excluded.actual,
			consensus = excluded.consensus,
			previous = excluded.previous,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := r.db.Exec(ctx, query,
		e.Country, e.EventName, e.Date, e.Time, e.Actual, e.Consensus, e.Previous,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert calendar_economic: %w", err)
	}

	return nil
}

// UpsertBatch inserts or updates multiple economic calendar events in a transaction.
func (r *Repository) UpsertBatch(ctx context.Context, events []*CalendarEconomic) (int, error) {
	if len(events) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}

	query := `
		INSERT INTO calendar_economic (
			country, event_name, date, time, actual, consensus, previous
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(country, event_name, date) DO UPDATE SET
			time = excluded.time,
			actual = excluded.actual,
			consensus = excluded.consensus,
			previous = excluded.previous,
			updated_at = CURRENT_TIMESTAMP
	`

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		tx.Rollback()
		return 0, fmt.Errorf("failed to prepare statement: %w", err)
	}

	upserted := 0
	var lastErr error
	for _, e := range events {
		_, err := stmt.ExecContext(ctx,
			e.Country, e.EventName, e.Date, e.Time, e.Actual, e.Consensus, e.Previous,
		)
		if err != nil {
			logging.Error("failed to upsert calendar economic event",
				"country", e.Country,
				"eventName", e.EventName,
				"date", e.Date,
				"error", err,
			)
			lastErr = err
			continue
		}
		upserted++
	}

	// Close statement before commit/rollback
	stmt.Close()

	if err := tx.Commit(); err != nil {
		return upserted, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Return error if all inserts failed
	if lastErr != nil && upserted == 0 {
		return 0, fmt.Errorf("all %d economic events failed to upsert: %w", len(events), lastErr)
	}

	return upserted, nil
}

// derefString safely dereferences a string pointer, returning empty string if nil.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
