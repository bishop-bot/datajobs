package fmp

import "context"

// Provider defines the interface for FMP API data access.
// This allows for mocking in tests and dependency injection.
type Provider interface {
	// Ping checks connectivity to the FMP API.
	Ping(ctx context.Context) error

	// FinancialRatios fetches financial ratios for a symbol.
	FinancialRatios(ctx context.Context, symbol string, period string) (*FinancialRatiosResponse, error)

	// KeyMetrics fetches key financial metrics for a symbol.
	KeyMetrics(ctx context.Context, symbol string, period string) (*KeyMetricsResponse, error)

	// Close releases resources.
	Close() error
}

// Ensure Client implements Provider at compile time.
var _ Provider = (*Client)(nil)
