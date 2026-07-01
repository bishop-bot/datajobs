package fundamentals

import (
	"context"
	"fmt"
	"time"

	"github.com/bishop-bot/datajobs/internal/database"
	"github.com/bishop-bot/datajobs/internal/logging"
	"github.com/bishop-bot/datajobs/internal/providers/fmp"
	"github.com/bishop-bot/datajobs/internal/repository"
	"github.com/bishop-bot/datajobs/internal/worker"
)

// HandlerWithDeps creates a job handler with all required dependencies.
func HandlerWithDeps(
	db *database.DB,
	questDB *database.QuestDB,
	fmpProvider fmp.Provider,
	watchlistRepo *repository.WatchlistRepository,
) worker.JobFunc {
	return func(ctx context.Context, job worker.Job) (string, error) {
		return handlerImpl(ctx, job, db, questDB, fmpProvider, watchlistRepo)
	}
}

// handlerImpl performs the fundamentals sync.
func handlerImpl(
	ctx context.Context,
	job worker.Job,
	db *database.DB,
	questDB *database.QuestDB,
	fmpProvider fmp.Provider,
	watchlistRepo *repository.WatchlistRepository,
) (string, error) {
	logger := logging.FromContext(ctx).With("job_id", job.ID)

	// Validate dependencies
	if db == nil {
		return "", fmt.Errorf("SQLite database not available")
	}
	if fmpProvider == nil {
		return "", fmt.Errorf("FMP provider not available")
	}
	if watchlistRepo == nil {
		return "", fmt.Errorf("watchlist repository not available")
	}

	// Get parameters from job metadata
	watchlistID := "fmpfree" // Default watchlist
	provider := "FMP"        // Default provider

	if wlID, ok := job.Metadata["watchlistId"].(string); ok && wlID != "" {
		watchlistID = wlID
	}
	if p, ok := job.Metadata["provider"].(string); ok && p != "" {
		provider = p
	}

	logger.Info("starting fundamentals sync job",
		"watchlist_id", watchlistID,
		"provider", provider,
	)

	// Get symbols from watchlist
	symbols, err := watchlistRepo.GetSymbols(ctx, watchlistID)
	if err != nil {
		return "", fmt.Errorf("failed to get symbols from watchlist %s: %w", watchlistID, err)
	}

	if len(symbols) == 0 {
		logger.Warn("no symbols found in watchlist", "watchlist_id", watchlistID)
		return "no symbols in watchlist", nil
	}

	logger.Info("fetching fundamentals for symbols", "count", len(symbols))

	// Prepare results tracking
	var insertedSymbols []string
	var updatedSymbols []string
	var failedSymbols []string

	repo := NewRepository(db)
	currentYear := time.Now().UTC().Year()

	// Process each symbol
	for _, ws := range symbols {
		symbol := ws.Symbol

		// Fetch FinancialRatios (TTM)
		ratios, err := fmpProvider.FinancialRatios(ctx, symbol, fmp.PeriodTTM)
		if err != nil {
			logger.Error("failed to fetch financial ratios",
				"symbol", symbol,
				"error", err,
			)
			failedSymbols = append(failedSymbols, symbol)
			continue
		}

		// Fetch KeyMetrics (TTM)
		metrics, err := fmpProvider.KeyMetrics(ctx, symbol, fmp.PeriodTTM)
		if err != nil {
			logger.Error("failed to fetch key metrics",
				"symbol", symbol,
				"error", err,
			)
			failedSymbols = append(failedSymbols, symbol)
			continue
		}

		if ratios == nil && metrics == nil {
			logger.Warn("no data returned for symbol",
				"symbol", symbol,
			)
			failedSymbols = append(failedSymbols, symbol)
			continue
		}

		// Build StockMetricsTTM from ratios and metrics
		stockMetrics := buildStockMetrics(symbol, provider, currentYear, ratios, metrics)

		// Get latest closing price from QuestDB if available
		if questDB != nil {
			price, err := getLatestPriceFromQuestDB(ctx, questDB, symbol)
			if err != nil {
				logger.Debug("failed to get latest price from QuestDB",
					"symbol", symbol,
					"error", err,
				)
			} else if price != nil {
				stockMetrics.Price = price
			}
		}

		// Upsert to database
		isInsert, err := repo.UpsertMetrics(ctx, stockMetrics)
		if err != nil {
			logger.Error("failed to upsert stock metrics",
				"symbol", symbol,
				"error", err,
			)
			failedSymbols = append(failedSymbols, symbol)
			continue
		}

		if isInsert {
			insertedSymbols = append(insertedSymbols, symbol)
		} else {
			updatedSymbols = append(updatedSymbols, symbol)
		}
	}

	// Build result summary
	result := fmt.Sprintf(
		"fundamentals_sync completed: provider=%s, watchlist=%s, total=%d, inserted=%d (%s), updated=%d (%s), failed=%d (%s)",
		provider,
		watchlistID,
		len(symbols),
		len(insertedSymbols),
		formatSymbolList(insertedSymbols),
		len(updatedSymbols),
		formatSymbolList(updatedSymbols),
		len(failedSymbols),
		formatSymbolList(failedSymbols),
	)

	logger.Info("fundamentals sync completed",
		"inserted", len(insertedSymbols),
		"updated", len(updatedSymbols),
		"failed", len(failedSymbols),
	)

	return result, nil
}

// buildStockMetrics creates StockMetricsTTM from FinancialRatios and KeyMetrics.
func buildStockMetrics(symbol, provider string, year int, ratios *fmp.FinancialRatiosResponse, metrics *fmp.KeyMetricsResponse) *StockMetricsTTM {
	m := &StockMetricsTTM{
		Symbol:    symbol,
		Provider:  provider,
		Year:      year,
	}

	// Map FinancialRatios fields (from TTM endpoint)
	if ratios != nil {
		// Cash ratio maps to 'cash' column
		m.Cash = ratios.CashRatio
		// Current ratio maps to 'current' column
		m.Current = ratios.CurrentRatio
		// Quick ratio maps to 'quick' column
		m.Quick = ratios.QuickRatio
		// Debt to equity
		m.DebtToEquity = ratios.DebtToEquity
		// Payout ratio maps to 'dividend_payout'
		m.DividendPayout = ratios.PayoutRatio
		// Dividend yield
		m.DividendYield = ratios.DividendYield
		// Price to earnings
		m.PriceToEarnings = ratios.PriceEarningsRatio
		// Price to book
		m.PriceToBook = ratios.PriceToBookRatio
		// Price to sales
		m.PriceToSales = ratios.PriceToSalesRatio
		// Price to free cash flows
		m.PriceToFreeCashFlow = ratios.PriceToFreeCashFlows
		// Price to operating cash flow
		m.PriceToCashFlow = ratios.PriceToOperatingCF
		// Return on assets
		m.ReturnOnAssets = ratios.ReturnOnAssets
		// Return on equity
		m.ReturnOnEquity = ratios.ReturnOnEquity
		// Profitability margins
		m.GrossProfitMargin = ratios.GrossProfitMargin
		m.OperatingProfitMargin = ratios.OperatingProfitMargin
		m.NetProfitMargin = ratios.NetProfitMargin
		// Return on capital
		m.ReturnOnCapitalEmployed = ratios.ReturnOnCapitalEmployed
		m.ROIC = ratios.ROIC
		// Valuation multiples
		m.EVToRevenue = ratios.EVToRevenue
		m.EVToEBITDA = ratios.EVToEBITDA
	}

	// Map KeyMetrics fields (these can override or supplement ratios)
	if metrics != nil {
		// Enterprise value
		if metrics.EnterpriseValue != nil {
			m.EnterpriseValue = metrics.EnterpriseValue
		}
		// EV to revenue
		if metrics.EVToRevenue != nil {
			m.EVToRevenue = metrics.EVToRevenue
		}
		// EV to EBITDA
		if metrics.EVToEBITDA != nil {
			m.EVToEBITDA = metrics.EVToEBITDA
		}
		// Free cash flow
		if metrics.FreeCashFlow != nil {
			m.FreeCashFlow = metrics.FreeCashFlow
		}
		// Override ratios with metrics if metrics has value (metrics is more recent TTM data)
		if metrics.CurrentRatio != nil {
			m.Current = metrics.CurrentRatio
		}
		if metrics.QuickRatio != nil {
			m.Quick = metrics.QuickRatio
		}
		if metrics.DebtToEquity != nil {
			m.DebtToEquity = metrics.DebtToEquity
		}
		if metrics.PriceToBookRatio != nil {
			m.PriceToBook = metrics.PriceToBookRatio
		}
		if metrics.PriceToSalesRatio != nil {
			m.PriceToSales = metrics.PriceToSalesRatio
		}
		if metrics.PriceToFreeCashFlows != nil {
			m.PriceToFreeCashFlow = metrics.PriceToFreeCashFlows
		}
		if metrics.ReturnOnAssets != nil {
			m.ReturnOnAssets = metrics.ReturnOnAssets
		}
		if metrics.ReturnOnEquity != nil {
			m.ReturnOnEquity = metrics.ReturnOnEquity
		}
		if metrics.ReturnOnCapitalEmployed != nil {
			m.ReturnOnCapitalEmployed = metrics.ReturnOnCapitalEmployed
		}
		if metrics.ROIC != nil {
			m.ROIC = metrics.ROIC
		}
		if metrics.NetProfitMargin != nil {
			m.NetProfitMargin = metrics.NetProfitMargin
		}
		if metrics.GrossProfitMargin != nil {
			m.GrossProfitMargin = metrics.GrossProfitMargin
		}
		if metrics.OperatingProfitMargin != nil {
			m.OperatingProfitMargin = metrics.OperatingProfitMargin
		}
	}

	return m
}

// getLatestPriceFromQuestDB fetches the latest closing price for a symbol from QuestDB.
func getLatestPriceFromQuestDB(ctx context.Context, questDB *database.QuestDB, symbol string) (*float64, error) {
	if questDB == nil {
		return nil, fmt.Errorf("QuestDB not available")
	}

	query := `
		SELECT close 
		FROM 'ohlcv_bars' 
		WHERE symbol = ?
		ORDER BY ts DESC 
		LIMIT 1
	`

	var price float64
	err := questDB.QueryRow(ctx, query, symbol).Scan(&price)
	if err != nil {
		return nil, err
	}

	return &price, nil
}

// formatSymbolList formats a list of symbols for logging.
func formatSymbolList(symbols []string) string {
	if len(symbols) == 0 {
		return ""
	}
	if len(symbols) <= 5 {
		return joinStrings(symbols)
	}
	return joinStrings(symbols[:5]) + "..."
}

// joinStrings joins string slice with commas.
func joinStrings(s []string) string {
	if len(s) == 0 {
		return ""
	}
	result := s[0]
	for i := 1; i < len(s); i++ {
		result += ", " + s[i]
	}
	return result
}
