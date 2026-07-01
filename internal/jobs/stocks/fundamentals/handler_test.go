package fundamentals

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bishop-bot/datajobs/internal/config"
	"github.com/bishop-bot/datajobs/internal/database"
	"github.com/bishop-bot/datajobs/internal/providers/fmp"
	"github.com/bishop-bot/datajobs/internal/repository"
	"github.com/bishop-bot/datajobs/internal/worker"
)

// MockFMPProvider is a mock implementation of the FMP Provider interface.
type MockFMPProvider struct {
	FinancialRatiosFunc func(ctx context.Context, symbol string, period string) (*fmp.FinancialRatiosResponse, error)
	KeyMetricsFunc     func(ctx context.Context, symbol string, period string) (*fmp.KeyMetricsResponse, error)
	PingFunc           func(ctx context.Context) error
}

func (m *MockFMPProvider) FinancialRatios(ctx context.Context, symbol string, period string) (*fmp.FinancialRatiosResponse, error) {
	if m.FinancialRatiosFunc != nil {
		return m.FinancialRatiosFunc(ctx, symbol, period)
	}
	return nil, nil
}

func (m *MockFMPProvider) KeyMetrics(ctx context.Context, symbol string, period string) (*fmp.KeyMetricsResponse, error) {
	if m.KeyMetricsFunc != nil {
		return m.KeyMetricsFunc(ctx, symbol, period)
	}
	return nil, nil
}

func (m *MockFMPProvider) Ping(ctx context.Context) error {
	if m.PingFunc != nil {
		return m.PingFunc(ctx)
	}
	return nil
}

func (m *MockFMPProvider) Close() error {
	return nil
}

func floatPtr(v float64) *float64 {
	return &v
}

// Sample FinancialRatiosTTMResponse is a sample response from FMP API.
var sampleFinancialRatiosTTMResponse = &fmp.FinancialRatiosResponse{
	Symbol:           "AAPL",
	Date:             "2024-03-31",
	CurrentRatio:     floatPtr(1.0703),
	QuickRatio:       floatPtr(1.0202),
	CashRatio:        floatPtr(0.2698),
	DebtToEquity:     floatPtr(0.7954),
	ReturnOnAssets:   floatPtr(0.3303),
	ReturnOnEquity:   floatPtr(1.4668),
	DividendYield:    floatPtr(0.0036),
	PayoutRatio:      floatPtr(0.1268),
	PriceEarningsRatio:   floatPtr(34.9047),
	PriceToBookRatio:     floatPtr(39.9723),
	PriceToSalesRatio:    floatPtr(9.4141),
	PriceToFreeCashFlows: floatPtr(32.9008),
	PriceToOperatingCF:   floatPtr(30.3568),
}

// Sample KeyMetricsTTMResponse is a sample response from FMP API.
var sampleKeyMetricsTTMResponse = &fmp.KeyMetricsResponse{
	Symbol:                  "AAPL",
	Date:                    "2024-03-31",
	Period:                  "ttm",
	EnterpriseValue:         floatPtr(4298316332160),
	FreeCashFlow:            floatPtr(129174000000),
	ReturnOnCapitalEmployed: floatPtr(0.4523),
	EVToRevenue:             floatPtr(9.5),
	EVToEBITDA:              floatPtr(28.7),
}

func TestHandlerImpl_Success(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	watchlistRepo := repository.NewWatchlistRepository(db)

	// Insert test watchlist and symbols
	insertTestWatchlist(t, db, "fmpfree", []string{"AAPL", "GOOGL"})

	mockProvider := &MockFMPProvider{
		FinancialRatiosFunc: func(ctx context.Context, symbol string, period string) (*fmp.FinancialRatiosResponse, error) {
			return sampleFinancialRatiosTTMResponse, nil
		},
		KeyMetricsFunc: func(ctx context.Context, symbol string, period string) (*fmp.KeyMetricsResponse, error) {
			return sampleKeyMetricsTTMResponse, nil
		},
	}

	handler := HandlerWithDeps(db, nil, mockProvider, watchlistRepo)

	job := worker.Job{
		ID:       "test-job",
		Type:     "fundamentals",
		Handler:  "fundamentals_sync",
		Metadata: map[string]interface{}{"watchlistId": "fmpfree", "provider": "FMP"},
	}

	result, err := handler(context.Background(), job)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result == "" {
		t.Error("expected non-empty result")
	}

	t.Logf("Result: %s", result)

	// Verify data was inserted
	verifyMetricsInserted(t, db, "AAPL", "FMP")
	verifyMetricsInserted(t, db, "GOOGL", "FMP")
}

func TestHandlerImpl_APIError(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	watchlistRepo := repository.NewWatchlistRepository(db)

	// Insert test watchlist with one symbol
	insertTestWatchlist(t, db, "test-watchlist", []string{"ERROR", "GOOD"})

	mockProvider := &MockFMPProvider{
		FinancialRatiosFunc: func(ctx context.Context, symbol string, period string) (*fmp.FinancialRatiosResponse, error) {
			if symbol == "ERROR" {
				return nil, &fmp.APIError{StatusCode: 429, Message: "Rate limit exceeded"}
			}
			return sampleFinancialRatiosTTMResponse, nil
		},
		KeyMetricsFunc: func(ctx context.Context, symbol string, period string) (*fmp.KeyMetricsResponse, error) {
			if symbol == "ERROR" {
				return nil, &fmp.APIError{StatusCode: 429, Message: "Rate limit exceeded"}
			}
			return sampleKeyMetricsTTMResponse, nil
		},
	}

	handler := HandlerWithDeps(db, nil, mockProvider, watchlistRepo)

	job := worker.Job{
		ID:       "test-job",
		Type:     "fundamentals",
		Handler:  "fundamentals_sync",
		Metadata: map[string]interface{}{"watchlistId": "test-watchlist", "provider": "FMP"},
	}

	result, err := handler(context.Background(), job)
	if err != nil {
		t.Fatalf("expected no error (partial success), got: %v", err)
	}

	// Verify GOOD was inserted despite ERROR failing
	verifyMetricsInserted(t, db, "GOOD", "FMP")

	// Verify ERROR was not inserted
	var count int
	err = db.QueryRow(context.Background(), "SELECT COUNT(*) FROM stock_metrics_ttm WHERE symbol = ?", "ERROR").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 records for ERROR symbol, got %d", count)
	}

	t.Logf("Result: %s", result)
}

func TestHandlerImpl_DefaultParameters(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	watchlistRepo := repository.NewWatchlistRepository(db)
	insertTestWatchlist(t, db, "fmpfree", []string{"AAPL"})

	providerCalled := false
	mockProvider := &MockFMPProvider{
		FinancialRatiosFunc: func(ctx context.Context, symbol string, period string) (*fmp.FinancialRatiosResponse, error) {
			if period != fmp.PeriodTTM {
				t.Errorf("expected period TTM, got %s", period)
			}
			providerCalled = true
			return sampleFinancialRatiosTTMResponse, nil
		},
		KeyMetricsFunc: func(ctx context.Context, symbol string, period string) (*fmp.KeyMetricsResponse, error) {
			return sampleKeyMetricsTTMResponse, nil
		},
	}

	handler := HandlerWithDeps(db, nil, mockProvider, watchlistRepo)

	// Job with no metadata (should use defaults)
	job := worker.Job{
		ID:       "test-job",
		Type:     "fundamentals",
		Handler:  "fundamentals_sync",
		Metadata: map[string]interface{}{},
	}

	_, err := handler(context.Background(), job)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !providerCalled {
		t.Error("expected provider to be called")
	}
}

func TestHandlerImpl_EmptyWatchlist(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	watchlistRepo := repository.NewWatchlistRepository(db)
	// Don't insert any watchlist

	mockProvider := &MockFMPProvider{}

	handler := HandlerWithDeps(db, nil, mockProvider, watchlistRepo)

	job := worker.Job{
		ID:       "test-job",
		Type:     "fundamentals",
		Handler:  "fundamentals_sync",
		Metadata: map[string]interface{}{"watchlistId": "empty-watchlist", "provider": "FMP"},
	}

	result, err := handler(context.Background(), job)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result != "no symbols in watchlist" {
		t.Errorf("expected 'no symbols in watchlist', got: %s", result)
	}
}

func TestHandlerImpl_NilAPIResponse(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	watchlistRepo := repository.NewWatchlistRepository(db)
	insertTestWatchlist(t, db, "test-watchlist", []string{"AAPL"})

	mockProvider := &MockFMPProvider{
		FinancialRatiosFunc: func(ctx context.Context, symbol string, period string) (*fmp.FinancialRatiosResponse, error) {
			return nil, nil // No data available
		},
		KeyMetricsFunc: func(ctx context.Context, symbol string, period string) (*fmp.KeyMetricsResponse, error) {
			return nil, nil // No data available
		},
	}

	handler := HandlerWithDeps(db, nil, mockProvider, watchlistRepo)

	job := worker.Job{
		ID:       "test-job",
		Type:     "fundamentals",
		Handler:  "fundamentals_sync",
		Metadata: map[string]interface{}{"watchlistId": "test-watchlist", "provider": "FMP"},
	}

	result, err := handler(context.Background(), job)
	if err != nil {
		t.Fatalf("expected no error (should skip nil responses), got: %v", err)
	}

	t.Logf("Result: %s", result)

	// Verify no data was inserted
	var count int
	err = db.QueryRow(context.Background(), "SELECT COUNT(*) FROM stock_metrics_ttm WHERE symbol = ?", "AAPL").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 records for nil response, got %d", count)
	}
}

func TestBuildStockMetrics(t *testing.T) {
	metrics := buildStockMetrics("AAPL", "FMP", 2024, sampleFinancialRatiosTTMResponse, sampleKeyMetricsTTMResponse)

	if metrics.Symbol != "AAPL" {
		t.Errorf("expected symbol AAPL, got %s", metrics.Symbol)
	}

	if metrics.Provider != "FMP" {
		t.Errorf("expected provider FMP, got %s", metrics.Provider)
	}

	if metrics.Year != 2024 {
		t.Errorf("expected year 2024, got %d", metrics.Year)
	}

	// Verify ratio mappings from FinancialRatios
	if metrics.Current == nil || *metrics.Current != 1.0703 {
		t.Errorf("expected current ratio 1.0703, got %v", metrics.Current)
	}

	if metrics.Quick == nil || *metrics.Quick != 1.0202 {
		t.Errorf("expected quick ratio 1.0202, got %v", metrics.Quick)
	}

	if metrics.Cash == nil || *metrics.Cash != 0.2698 {
		t.Errorf("expected cash ratio 0.2698, got %v", metrics.Cash)
	}

	if metrics.PriceToEarnings == nil || *metrics.PriceToEarnings != 34.9047 {
		t.Errorf("expected PE 34.9047, got %v", metrics.PriceToEarnings)
	}

	if metrics.ReturnOnAssets == nil || *metrics.ReturnOnAssets != 0.3303 {
		t.Errorf("expected ROA 0.3303, got %v", metrics.ReturnOnAssets)
	}

	// Verify KeyMetrics mappings
	if metrics.EnterpriseValue == nil || *metrics.EnterpriseValue != 4298316332160 {
		t.Errorf("expected EV 4298316332160, got %v", metrics.EnterpriseValue)
	}

	if metrics.FreeCashFlow == nil || *metrics.FreeCashFlow != 129174000000 {
		t.Errorf("expected FCF 129174000000, got %v", metrics.FreeCashFlow)
	}

	// Verify new KeyMetrics mappings (from FinancialRatios)
	if metrics.ReturnOnCapitalEmployed == nil || *metrics.ReturnOnCapitalEmployed != 0.4523 {
		t.Errorf("expected ROCE 0.4523, got %v", metrics.ReturnOnCapitalEmployed)
	}

	if metrics.EVToRevenue == nil || *metrics.EVToRevenue != 9.5 {
		t.Errorf("expected EV/Revenue 9.5, got %v", metrics.EVToRevenue)
	}

	if metrics.EVToEBITDA == nil || *metrics.EVToEBITDA != 28.7 {
		t.Errorf("expected EV/EBITDA 28.7, got %v", metrics.EVToEBITDA)
	}
}

func TestBuildStockMetrics_NilRatios(t *testing.T) {
	metrics := buildStockMetrics("TEST", "FMP", 2024, nil, sampleKeyMetricsTTMResponse)

	if metrics.Symbol != "TEST" {
		t.Errorf("expected symbol TEST, got %s", metrics.Symbol)
	}

	// Should have values from KeyMetrics
	if metrics.EnterpriseValue == nil {
		t.Error("expected enterprise value from KeyMetrics")
	}

	// Should have nil values from missing FinancialRatios
	if metrics.Current != nil {
		t.Error("expected nil current ratio")
	}
}

func TestBuildStockMetrics_NilMetrics(t *testing.T) {
	metrics := buildStockMetrics("TEST", "FMP", 2024, sampleFinancialRatiosTTMResponse, nil)

	if metrics.Symbol != "TEST" {
		t.Errorf("expected symbol TEST, got %s", metrics.Symbol)
	}

	// Should have values from FinancialRatios
	if metrics.Current == nil {
		t.Error("expected current ratio from FinancialRatios")
	}

	// Should have nil values from missing KeyMetrics
	if metrics.EnterpriseValue != nil {
		t.Error("expected nil enterprise value")
	}
}

func TestFormatSymbolList(t *testing.T) {
	tests := []struct {
		input    []string
		expected string
	}{
		{[]string{}, ""},
		{[]string{"AAPL"}, "AAPL"},
		{[]string{"AAPL", "GOOGL"}, "AAPL, GOOGL"},
		{[]string{"AAPL", "GOOGL", "MSFT", "AMZN", "NVDA", "META"}, "AAPL, GOOGL, MSFT, AMZN, NVDA..."},
	}

	for _, tt := range tests {
		result := formatSymbolList(tt.input)
		if result != tt.expected {
			t.Errorf("formatSymbolList(%v) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// setupTestDB creates a temporary SQLite database for testing.
func setupTestDB(t *testing.T) (*database.DB, func()) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Get absolute path to migrations directory (4 levels up from internal/jobs/stocks/fundamentals)
	migrationsDir := filepath.Join("..", "..", "..", "..", "migrations")
	migrationsDir, err := filepath.Abs(migrationsDir)
	if err != nil {
		t.Fatalf("failed to get migrations path: %v", err)
	}

	cfg := config.DatabaseConfig{
		Path:          dbPath,
		JournalMode:   "WAL",
		MigrationsDir: migrationsDir,
	}

	db, err := database.New(cfg)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}

	ctx := context.Background()
	if err := db.RunMigrations(ctx); err != nil {
		db.Close()
		t.Fatalf("failed to run migrations: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.Remove(dbPath)
	}

	return db, cleanup
}

func insertTestWatchlist(t *testing.T, db *database.DB, watchlistID string, symbols []string) {
	ctx := context.Background()

	// Insert watchlist
	_, err := db.Exec(ctx, `
		INSERT INTO watchlists (id, name, is_public)
		VALUES (?, ?, ?)
	`, watchlistID, watchlistID, true)
	if err != nil {
		t.Fatalf("failed to insert watchlist: %v", err)
	}

	// Insert symbols
	for i, symbol := range symbols {
		_, err := db.Exec(ctx, `
			INSERT INTO watchlist_symbols (watchlist_id, symbol, position)
			VALUES (?, ?, ?)
		`, watchlistID, symbol, i)
		if err != nil {
			t.Fatalf("failed to insert symbol %s: %v", symbol, err)
		}
	}
}

func verifyMetricsInserted(t *testing.T, db *database.DB, symbol, provider string) {
	var count int
	err := db.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM stock_metrics_ttm WHERE symbol = ? AND provider = ?
	`, symbol, provider).Scan(&count)
	if err != nil {
		t.Fatalf("failed to query metrics: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 record for %s/%s, got %d", symbol, provider, count)
	}
}
