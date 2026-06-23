package stocks

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/bishop-bot/datajobs/internal/database"
	"github.com/bishop-bot/datajobs/internal/logging"
)

// Repository handles database operations for stock earnings.
type Repository struct {
	db *database.DB
}

// NewRepository creates a new Repository instance.
func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

// GetByDateAndSymbols fetches existing stock earnings for a given date.
func (r *Repository) GetByDateAndSymbols(ctx context.Context, date string, symbols []string) (map[string]*StockEarnings, error) {
	if len(symbols) == 0 {
		return make(map[string]*StockEarnings), nil
	}

	// Build placeholders for IN clause
	placeholders := make([]string, len(symbols))
	args := make([]interface{}, len(symbols)+1)
	args[0] = date
	for i, sym := range symbols {
		placeholders[i] = "?"
		args[i+1] = sym
	}

	query := fmt.Sprintf(`
		SELECT id, symbol, name, mic, isin, type, time, status,
		       eps, eps_estimated, revenue, revenue_estimated,
		       date, created_at, updated_at
		FROM stocks_earnings
		WHERE date = ? AND symbol IN (%s)
	`, joinStrings(placeholders))

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query stocks_earnings: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*StockEarnings)
	for rows.Next() {
		var e StockEarnings
		var name, mic, isin, typ, timeVal, status sql.NullString
		var eps, epsEst sql.NullFloat64
		var revenue, revenueEst sql.NullInt64

		err := rows.Scan(
			&e.ID, &e.Symbol, &name, &mic, &isin, &typ, &timeVal, &status,
			&eps, &epsEst, &revenue, &revenueEst,
			&e.Date, &e.CreatedAt, &e.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		e.Name = name.String
		e.MIC = mic.String
		e.ISIN = isin.String
		e.Type = typ.String
		e.Time = Time(timeVal.String)
		e.Status = Status(status.String)

		if eps.Valid {
			e.EPS = &eps.Float64
		}
		if epsEst.Valid {
			e.EPSEstimated = &epsEst.Float64
		}
		if revenue.Valid {
			e.Revenue = &revenue.Int64
		}
		if revenueEst.Valid {
			e.RevenueEstimated = &revenueEst.Int64
		}

		key := e.Symbol
		result[key] = &e
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return result, nil
}

// Upsert inserts or updates a stock earnings record.
func (r *Repository) Upsert(ctx context.Context, e *StockEarnings) error {
	query := `
		INSERT INTO stocks_earnings (
			symbol, name, mic, isin, type, time, status,
			eps, eps_estimated, revenue, revenue_estimated, date
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(symbol, date) DO UPDATE SET
			name = excluded.name,
			mic = excluded.mic,
			isin = excluded.isin,
			type = excluded.type,
			time = excluded.time,
			status = excluded.status,
			eps = excluded.eps,
			eps_estimated = excluded.eps_estimated,
			revenue = excluded.revenue,
			revenue_estimated = excluded.revenue_estimated,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := r.db.Exec(ctx, query,
		e.Symbol, e.Name, e.MIC, e.ISIN, e.Type, e.Time, e.Status,
		e.EPS, e.EPSEstimated, e.Revenue, e.RevenueEstimated, e.Date,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert stock earnings: %w", err)
	}

	return nil
}

// UpsertBatch inserts or updates multiple stock earnings records in a transaction.
func (r *Repository) UpsertBatch(ctx context.Context, earnings []*StockEarnings) (int, error) {
	if len(earnings) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
		INSERT INTO stocks_earnings (
			symbol, name, mic, isin, type, time, status,
			eps, eps_estimated, revenue, revenue_estimated, date
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(symbol, date) DO UPDATE SET
			name = excluded.name,
			mic = excluded.mic,
			isin = excluded.isin,
			type = excluded.type,
			time = excluded.time,
			status = excluded.status,
			eps = excluded.eps,
			eps_estimated = excluded.eps_estimated,
			revenue = excluded.revenue,
			revenue_estimated = excluded.revenue_estimated,
			updated_at = CURRENT_TIMESTAMP
	`

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	inserted := 0
	for _, e := range earnings {
		_, err := stmt.ExecContext(ctx,
			e.Symbol, e.Name, e.MIC, e.ISIN, e.Type, e.Time, e.Status,
			e.EPS, e.EPSEstimated, e.Revenue, e.RevenueEstimated, e.Date,
		)
		if err != nil {
			logging.Error("failed to upsert stock earnings",
				"symbol", e.Symbol,
				"date", e.Date,
				"error", err,
			)
			continue
		}
		inserted++
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return inserted, nil
}

// joinStrings joins string slices with commas.
func joinStrings(s []string) string {
	result := ""
	for i, v := range s {
		if i > 0 {
			result += ", "
		}
		result += v
	}
	return result
}
