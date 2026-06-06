package providers

import (
	"context"

	ibapi "github.com/bishop-bot/ibapi-go"
)

// IBProvider defines the interface for IB data access.
// This allows for mocking in tests and dependency injection.
type IBProvider interface {
	// Ping checks connectivity to IB Gateway.
	Ping(ctx context.Context) error

	// AuthStatus returns the current authentication status.
	AuthStatus(ctx context.Context) (*ibapi.AuthStatusResponse, error)

	// IsConnected checks if the client is connected to the gateway.
	IsConnected(ctx context.Context) bool

	// HistoricalData fetches historical market data.
	HistoricalData(ctx context.Context, req HistoricalDataRequest) (*ibapi.HistoricalDataResponse, error)

	// Close releases resources.
	Close() error
}

// Ensure IBClient implements IBProvider at compile time.
var _ IBProvider = (*IBClient)(nil)