package ib

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	ibapi "github.com/bishop-bot/ibapi-go"
)

// MockClient implements Provider for testing.
// It allows controlled responses without requiring a live IB Gateway.
type MockClient struct {
	// Configuration
	PingError     error
	Connected     bool
	callCount     atomic.Int32
	Calls         []MockCall

	// Historical data responses
	HistoricalDataResponse *ibapi.HistoricalDataResponse
	HistoricalDataError    error

	// Auth status response
	AuthStatusResponse *ibapi.AuthStatusResponse
	AuthStatusError    error

	// Closed flag
	closed bool

	// Authenticator for testing auth scenarios
	Authenticator MockAuthenticator
}

// MockAuthenticator implements Authenticator for testing.
type MockAuthenticator struct {
	Authenticated bool
	AuthError     error
	AuthCallCount atomic.Int32
	CloseError    error
	httpClient    *http.Client
}

// NewMockClient creates a mock IB client with default successful behavior.
func NewMockClient() *MockClient {
	return &MockClient{
		Connected:              true,
		HistoricalDataResponse: &ibapi.HistoricalDataResponse{},
		AuthStatusResponse:     &ibapi.AuthStatusResponse{Authenticated: true},
		Authenticator:          MockAuthenticator{Authenticated: true},
	}
}

// WithPingError sets the error to return on Ping.
func (m *MockClient) WithPingError(err error) *MockClient {
	m.PingError = err
	return m
}

// WithConnected sets the connected state.
func (m *MockClient) WithConnected(connected bool) *MockClient {
	m.Connected = connected
	return m
}

// WithHistoricalDataResponse sets the historical data response.
func (m *MockClient) WithHistoricalDataResponse(resp *ibapi.HistoricalDataResponse) *MockClient {
	m.HistoricalDataResponse = resp
	return m
}

// WithHistoricalDataError sets the error to return on HistoricalData.
func (m *MockClient) WithHistoricalDataError(err error) *MockClient {
	m.HistoricalDataError = err
	return m
}

// WithAuthStatusResponse sets the auth status response.
func (m *MockClient) WithAuthStatusResponse(resp *ibapi.AuthStatusResponse) *MockClient {
	m.AuthStatusResponse = resp
	return m
}

// WithNotAuthenticated sets the auth status to not authenticated.
func (m *MockClient) WithNotAuthenticated() *MockClient {
	m.AuthStatusResponse = &ibapi.AuthStatusResponse{Authenticated: false}
	m.Authenticator.Authenticated = false
	return m
}

// WithAuthError sets the error to return on authentication.
func (m *MockClient) WithAuthError(err error) *MockClient {
	m.Authenticator.AuthError = err
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

// AuthStatus implements Provider.
func (m *MockClient) AuthStatus(ctx context.Context) (*ibapi.AuthStatusResponse, error) {
	m.RecordCall("AuthStatus")
	if m.closed {
		return nil, ErrClientClosed
	}
	return m.AuthStatusResponse, m.AuthStatusError
}

// IsConnected implements Provider.
func (m *MockClient) IsConnected(ctx context.Context) bool {
	m.RecordCall("IsConnected")
	if m.PingError != nil {
		return false
	}
	return m.Connected && !m.closed
}

// HistoricalData implements Provider.
func (m *MockClient) HistoricalData(ctx context.Context, req HistoricalDataRequest) (*ibapi.HistoricalDataResponse, error) {
	m.RecordCall("HistoricalData", req.Conid, req.Exchange, req.Period, req.Bar)
	if m.closed {
		return nil, ErrClientClosed
	}
	return m.HistoricalDataResponse, m.HistoricalDataError
}

// Close implements Provider.
func (m *MockClient) Close() error {
	m.RecordCall("Close")
	m.closed = true
	return nil
}

// MockHistoricalDataBar creates a test bar.
func MockHistoricalDataBar(t int64, o, h, l, c float64, v float64) ibapi.HistoricalDataBar {
	return ibapi.HistoricalDataBar{
		T: t,
		O: o,
		H: h,
		L: l,
		C: c,
		V: v,
	}
}

// MockHistoricalDataResponse creates a test response with bars.
func MockHistoricalDataResponse(symbol string, bars ...ibapi.HistoricalDataBar) *ibapi.HistoricalDataResponse {
	return &ibapi.HistoricalDataResponse{
		Symbol: symbol,
		Data:   bars,
	}
}

// Ensure MockClient implements Provider at compile time.
var _ Provider = (*MockClient)(nil)

// MockAuthenticator methods

// Authenticate implements Authenticator.
func (m *MockAuthenticator) Authenticate(ctx context.Context) error {
	m.AuthCallCount.Add(1)
	if m.AuthError != nil {
		return m.AuthError
	}
	m.Authenticated = true
	return nil
}

// IsAuthenticated implements Authenticator.
func (m *MockAuthenticator) IsAuthenticated(ctx context.Context) (bool, error) {
	return m.Authenticated, m.AuthError
}

// HTTPClient implements Authenticator (returns nil for mock).
func (m *MockAuthenticator) HTTPClient() *http.Client {
	return m.httpClient
}

// Close implements Authenticator.
func (m *MockAuthenticator) Close() error {
	return m.CloseError
}

// MockCall records an invocation for assertion.
type MockCall struct {
	Method string
	Args   []any
	Time   time.Time
}