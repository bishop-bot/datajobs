package fundamentals

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/bishop-bot/datajobs/internal/database"
	"github.com/bishop-bot/datajobs/internal/logging"
)

// Repository handles database operations for stock fundamentals.
type Repository struct {
	db *database.DB
}

// NewRepository creates a new Repository instance.
func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

// UpsertMetrics inserts or updates stock metrics TTM data.
// If a record with the same symbol and provider exists, it updates.
// If no record exists for the symbol (regardless of provider), it inserts.
func (r *Repository) UpsertMetrics(ctx context.Context, metrics *StockMetricsTTM) (bool, error) {
	// Check if a record with the same symbol exists
	var existingProvider string
	err := r.db.QueryRow(ctx, `
		SELECT provider FROM stock_metrics_ttm WHERE symbol = ?
	`, metrics.Symbol).Scan(&existingProvider)

	if err == sql.ErrNoRows {
		// No existing record - perform insert
		return true, r.insert(ctx, metrics)
	} else if err != nil {
		return false, fmt.Errorf("failed to check existing record: %w", err)
	}

	// Record exists - check if same provider (update) or different provider (insert)
	if existingProvider == metrics.Provider {
		// Same provider - update
		return false, r.update(ctx, metrics)
	}

	// Different provider - insert alongside existing
	return true, r.insert(ctx, metrics)
}

// UpsertBatch inserts or updates multiple stock metrics in a transaction.
// Returns (insertedCount, updatedCount, error)
func (r *Repository) UpsertBatch(ctx context.Context, metrics []*StockMetricsTTM) (int, int, error) {
	if len(metrics) == 0 {
		return 0, 0, nil
	}

	tx, err := r.db.BeginTx(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	insertQuery := `
		INSERT INTO stock_metrics_ttm (
			symbol, cik, date, year, provider,
			cash, current, currency,
			debt_to_equity, dividend_payout, dividend_yield,
			enterprise_value, free_cash_flow, price,
			price_to_book, price_to_cash_flow, price_to_earnings,
			price_to_free_cash_flow, price_to_sales,
			quick, return_on_assets, return_on_equity,
			gross_profit_margin, operating_profit_margin, net_profit_margin,
			return_on_capital_employed, roic, ev_to_revenue, ev_to_ebitda,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	insertStmt, err := tx.PrepareContext(ctx, insertQuery)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer insertStmt.Close()

	now := time.Now().UTC().Format(time.RFC3339)

	inserted := 0
	updated := 0

	for _, m := range metrics {
		// Check existing provider for this symbol
		var existingProvider sql.NullString
		err := tx.QueryRowContext(ctx, `
			SELECT provider FROM stock_metrics_ttm WHERE symbol = ?
		`, m.Symbol).Scan(&existingProvider)

		if err == sql.ErrNoRows {
			// No existing record - insert
			_, err = insertStmt.ExecContext(ctx,
				m.Symbol, m.CIK, m.Date, m.Year, m.Provider,
				m.Cash, m.Current, m.Currency,
				m.DebtToEquity, m.DividendPayout, m.DividendYield,
				m.EnterpriseValue, m.FreeCashFlow, m.Price,
				m.PriceToBook, m.PriceToCashFlow, m.PriceToEarnings,
				m.PriceToFreeCashFlow, m.PriceToSales,
				m.Quick, m.ReturnOnAssets, m.ReturnOnEquity,
				m.GrossProfitMargin, m.OperatingProfitMargin, m.NetProfitMargin,
				m.ReturnOnCapitalEmployed, m.ROIC, m.EVToRevenue, m.EVToEBITDA,
				now, now,
			)
			if err != nil {
				logging.Error("failed to insert stock metrics",
					"symbol", m.Symbol,
					"error", err,
				)
				continue
			}
			inserted++
		} else if err != nil {
			logging.Error("failed to query existing record",
				"symbol", m.Symbol,
				"error", err,
			)
			continue
		} else if existingProvider.Valid && existingProvider.String == m.Provider {
			// Same provider - update
			_, err = tx.ExecContext(ctx, `
				UPDATE stock_metrics_ttm SET
					cik = ?, date = ?, year = ?,
					cash = ?, current = ?, currency = ?,
					debt_to_equity = ?, dividend_payout = ?, dividend_yield = ?,
					enterprise_value = ?, free_cash_flow = ?, price = ?,
					price_to_book = ?, price_to_cash_flow = ?, price_to_earnings = ?,
					price_to_free_cash_flow = ?, price_to_sales = ?,
					quick = ?, return_on_assets = ?, return_on_equity = ?,
					gross_profit_margin = ?, operating_profit_margin = ?, net_profit_margin = ?,
					return_on_capital_employed = ?, roic = ?, ev_to_revenue = ?, ev_to_ebitda = ?,
					updated_at = ?
				WHERE symbol = ?
			`,
				m.CIK, m.Date, m.Year,
				m.Cash, m.Current, m.Currency,
				m.DebtToEquity, m.DividendPayout, m.DividendYield,
				m.EnterpriseValue, m.FreeCashFlow, m.Price,
				m.PriceToBook, m.PriceToCashFlow, m.PriceToEarnings,
				m.PriceToFreeCashFlow, m.PriceToSales,
				m.Quick, m.ReturnOnAssets, m.ReturnOnEquity,
				m.GrossProfitMargin, m.OperatingProfitMargin, m.NetProfitMargin,
				m.ReturnOnCapitalEmployed, m.ROIC, m.EVToRevenue, m.EVToEBITDA,
				now,
				m.Symbol,
			)
			if err != nil {
				logging.Error("failed to update stock metrics",
					"symbol", m.Symbol,
					"error", err,
				)
				continue
			}
			updated++
		} else {
			// Different provider - insert (allows multiple providers per symbol)
			_, err = insertStmt.ExecContext(ctx,
				m.Symbol, m.CIK, m.Date, m.Year, m.Provider,
				m.Cash, m.Current, m.Currency,
				m.DebtToEquity, m.DividendPayout, m.DividendYield,
				m.EnterpriseValue, m.FreeCashFlow, m.Price,
				m.PriceToBook, m.PriceToCashFlow, m.PriceToEarnings,
				m.PriceToFreeCashFlow, m.PriceToSales,
				m.Quick, m.ReturnOnAssets, m.ReturnOnEquity,
				m.GrossProfitMargin, m.OperatingProfitMargin, m.NetProfitMargin,
				m.ReturnOnCapitalEmployed, m.ROIC, m.EVToRevenue, m.EVToEBITDA,
				now, now,
			)
			if err != nil {
				logging.Error("failed to insert stock metrics (provider conflict)",
					"symbol", m.Symbol,
					"existing_provider", existingProvider.String,
					"new_provider", m.Provider,
					"error", err,
				)
				continue
			}
			inserted++
		}
	}

	if err := tx.Commit(); err != nil {
		return inserted, updated, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return inserted, updated, nil
}

// insert performs an INSERT for new stock metrics.
func (r *Repository) insert(ctx context.Context, m *StockMetricsTTM) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.Exec(ctx, `
		INSERT INTO stock_metrics_ttm (
			symbol, cik, date, year, provider,
			cash, current, currency,
			debt_to_equity, dividend_payout, dividend_yield,
			enterprise_value, free_cash_flow, price,
			price_to_book, price_to_cash_flow, price_to_earnings,
			price_to_free_cash_flow, price_to_sales,
			quick, return_on_assets, return_on_equity,
			gross_profit_margin, operating_profit_margin, net_profit_margin,
			return_on_capital_employed, roic, ev_to_revenue, ev_to_ebitda,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		m.Symbol, m.CIK, m.Date, m.Year, m.Provider,
		m.Cash, m.Current, m.Currency,
		m.DebtToEquity, m.DividendPayout, m.DividendYield,
		m.EnterpriseValue, m.FreeCashFlow, m.Price,
		m.PriceToBook, m.PriceToCashFlow, m.PriceToEarnings,
		m.PriceToFreeCashFlow, m.PriceToSales,
		m.Quick, m.ReturnOnAssets, m.ReturnOnEquity,
		m.GrossProfitMargin, m.OperatingProfitMargin, m.NetProfitMargin,
		m.ReturnOnCapitalEmployed, m.ROIC, m.EVToRevenue, m.EVToEBITDA,
		now, now,
	)
	return err
}

// update performs an UPDATE for existing stock metrics.
func (r *Repository) update(ctx context.Context, m *StockMetricsTTM) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.Exec(ctx, `
		UPDATE stock_metrics_ttm SET
			cik = ?, date = ?, year = ?,
			cash = ?, current = ?, currency = ?,
			debt_to_equity = ?, dividend_payout = ?, dividend_yield = ?,
			enterprise_value = ?, free_cash_flow = ?, price = ?,
			price_to_book = ?, price_to_cash_flow = ?, price_to_earnings = ?,
			price_to_free_cash_flow = ?, price_to_sales = ?,
			quick = ?, return_on_assets = ?, return_on_equity = ?,
			gross_profit_margin = ?, operating_profit_margin = ?, net_profit_margin = ?,
			return_on_capital_employed = ?, roic = ?, ev_to_revenue = ?, ev_to_ebitda = ?,
			updated_at = ?
		WHERE symbol = ?
	`,
		m.CIK, m.Date, m.Year,
		m.Cash, m.Current, m.Currency,
		m.DebtToEquity, m.DividendPayout, m.DividendYield,
		m.EnterpriseValue, m.FreeCashFlow, m.Price,
		m.PriceToBook, m.PriceToCashFlow, m.PriceToEarnings,
		m.PriceToFreeCashFlow, m.PriceToSales,
		m.Quick, m.ReturnOnAssets, m.ReturnOnEquity,
		m.GrossProfitMargin, m.OperatingProfitMargin, m.NetProfitMargin,
		m.ReturnOnCapitalEmployed, m.ROIC, m.EVToRevenue, m.EVToEBITDA,
		now,
		m.Symbol,
	)
	return err
}
