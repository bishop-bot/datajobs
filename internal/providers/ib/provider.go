package ib

import (
	"context"

	ibapi "github.com/bishop-bot/ibapi-go"
)

// Provider defines the interface for IB data access.
// This allows for mocking in tests and dependency injection.
type Provider interface {
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

// AuthAwareProvider extends Provider with authentication support.
// Implement this interface to enable automatic authentication checks.
type AuthAwareProvider interface {
	Provider

	// EnsureAuthenticated ensures the client is authenticated.
	// Returns nil if already authenticated or auth succeeds.
	// Returns error if auth is required but fails.
	EnsureAuthenticated(ctx context.Context) error
}

// Ensure IBClient implements Provider at compile time.
var _ Provider = (*Client)(nil)

// Ensure MockClient implements Provider at compile time.
var _ Provider = (*MockClient)(nil)

// Ensure Client implements AuthAwareProvider at compile time.
var _ AuthAwareProvider = (*Client)(nil)