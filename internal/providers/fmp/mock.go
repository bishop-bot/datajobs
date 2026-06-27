package fmp

import (
	"context"
	"sync/atomic"
	"time"
)

// MockClient implements Provider for testing.
// It allows controlled responses without requiring a live FMP API.
type MockClient struct {
	// Configuration
	PingError error

	// Financial ratios responses
	FinancialRatiosResponse *FinancialRatiosResponse
	FinancialRatiosError    error

	// Key metrics responses
	KeyMetricsResponse *KeyMetricsResponse
	KeyMetricsError    error

	// Call tracking
	callCount atomic.Int32
	Calls     []MockCall

	// Closed flag
	closed bool
}

// MockCall records an invocation for assertion.
type MockCall struct {
	Method string
	Args   []any
	Time   time.Time
}

// NewMockClient creates a mock FMP client with default successful behavior.
func NewMockClient() *MockClient {
	return &MockClient{
		FinancialRatiosResponse: &FinancialRatiosResponse{},
		KeyMetricsResponse:      &KeyMetricsResponse{},
	}
}

// WithPingError sets the error to return on Ping.
func (m *MockClient) WithPingError(err error) *MockClient {
	m.PingError = err
	return m
}

// WithFinancialRatiosResponse sets the financial ratios response.
func (m *MockClient) WithFinancialRatiosResponse(resp *FinancialRatiosResponse) *MockClient {
	m.FinancialRatiosResponse = resp
	return m
}

// WithFinancialRatiosError sets the error to return on FinancialRatios.
func (m *MockClient) WithFinancialRatiosError(err error) *MockClient {
	m.FinancialRatiosError = err
	return m
}

// WithKeyMetricsResponse sets the key metrics response.
func (m *MockClient) WithKeyMetricsResponse(resp *KeyMetricsResponse) *MockClient {
	m.KeyMetricsResponse = resp
	return m
}

// WithKeyMetricsError sets the error to return on KeyMetrics.
func (m *MockClient) WithKeyMetricsError(err error) *MockClient {
	m.KeyMetricsError = err
	return m
}

// RecordCall records a method invocation for later assertion.
func (m *MockClient) RecordCall(method string, args ...any) {
	m.callCount.Add(1)
	m.Calls = append(m.Calls, MockCall{
		Method: method,
		Args:   args,
		Time:   time.Now(),
	})
}

// TotalCalls returns the number of times any method was called.
func (m *MockClient) TotalCalls() int {
	return int(m.callCount.Load())
}

// AssertCalled verifies a method was called at least once.
func (m *MockClient) AssertCalled(method string) bool {
	for _, call := range m.Calls {
		if call.Method == method {
			return true
		}
	}
	return false
}

// Reset clears call history and counters.
func (m *MockClient) Reset() {
	m.callCount.Store(0)
	m.Calls = m.Calls[:0]
}

// Ping implements Provider.
func (m *MockClient) Ping(ctx context.Context) error {
	m.RecordCall("Ping")
	if m.closed {
		return ErrClientClosed
	}
	return m.PingError
}

// FinancialRatios implements Provider.
func (m *MockClient) FinancialRatios(ctx context.Context, symbol string, period string) (*FinancialRatiosResponse, error) {
	m.RecordCall("FinancialRatios", symbol, period)
	if m.closed {
		return nil, ErrClientClosed
	}
	return m.FinancialRatiosResponse, m.FinancialRatiosError
}

// KeyMetrics implements Provider.
func (m *MockClient) KeyMetrics(ctx context.Context, symbol string, period string) (*KeyMetricsResponse, error) {
	m.RecordCall("KeyMetrics", symbol, period)
	if m.closed {
		return nil, ErrClientClosed
	}
	return m.KeyMetricsResponse, m.KeyMetricsError
}

// Close implements Provider.
func (m *MockClient) Close() error {
	m.RecordCall("Close")
	m.closed = true
	return nil
}

// Ensure MockClient implements Provider at compile time.
var _ Provider = (*MockClient)(nil)

// Mock helper functions

// MockFinancialRatiosResponse creates a test financial ratios response.
func MockFinancialRatiosResponse(symbol, date string) *FinancialRatiosResponse {
	pe := 25.5
	gpm := 0.45
	return &FinancialRatiosResponse{
		Symbol:               symbol,
		Date:                 date,
		PriceEarningsRatio:   &pe,
		GrossProfitMargin:    &gpm,
	}
}

// MockKeyMetricsResponse creates a test key metrics response.
func MockKeyMetricsResponse(symbol, date, period string) *KeyMetricsResponse {
	mc := 1_000_000_000_000.0
	pe := 25.5
	return &KeyMetricsResponse{
		Symbol:    symbol,
		Date:      date,
		Period:    period,
		MarketCap: &mc,
		PERatio:   &pe,
	}
}
