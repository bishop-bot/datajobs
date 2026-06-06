package providers

import (
	"context"
	"sync/atomic"
	"time"

	ibapi "github.com/bishop-bot/ibapi-go"
)

// MockIBClient implements IBProvider for testing.
// It allows controlled responses without requiring a live IB Gateway.
type MockIBClient struct {
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
}

// MockCall records an invocation for assertion.
type MockCall struct {
	Method string
	Args   []any
	Time   time.Time
}

// NewMockIBClient creates a mock IB client with default successful behavior.
func NewMockIBClient() *MockIBClient {
	return &MockIBClient{
		Connected:              true,
		HistoricalDataResponse: &ibapi.HistoricalDataResponse{},
		AuthStatusResponse:     &ibapi.AuthStatusResponse{},
	}
}

// WithPingError sets the error to return on Ping.
func (m *MockIBClient) WithPingError(err error) *MockIBClient {
	m.PingError = err
	return m
}

// WithConnected sets the connected state.
func (m *MockIBClient) WithConnected(connected bool) *MockIBClient {
	m.Connected = connected
	return m
}

// WithHistoricalDataResponse sets the historical data response.
func (m *MockIBClient) WithHistoricalDataResponse(resp *ibapi.HistoricalDataResponse) *MockIBClient {
	m.HistoricalDataResponse = resp
	return m
}

// WithHistoricalDataError sets the error to return on HistoricalData.
func (m *MockIBClient) WithHistoricalDataError(err error) *MockIBClient {
	m.HistoricalDataError = err
	return m
}

// WithAuthStatusResponse sets the auth status response.
func (m *MockIBClient) WithAuthStatusResponse(resp *ibapi.AuthStatusResponse) *MockIBClient {
	m.AuthStatusResponse = resp
	return m
}

// RecordCall records a method invocation for later assertion.
func (m *MockIBClient) RecordCall(method string, args ...any) {
	m.callCount.Add(1)
	m.Calls = append(m.Calls, MockCall{
		Method: method,
		Args:   args,
		Time:   time.Now(),
	})
}

// TotalCalls returns the number of times any method was called.
func (m *MockIBClient) TotalCalls() int {
	return int(m.callCount.Load())
}

// AssertCalled verifies a method was called at least once.
func (m *MockIBClient) AssertCalled(method string) bool {
	for _, call := range m.Calls {
		if call.Method == method {
			return true
		}
	}
	return false
}

// Reset clears call history and counters.
func (m *MockIBClient) Reset() {
	m.callCount.Store(0)
	m.Calls = m.Calls[:0]
}

// Ping implements IBProvider.
func (m *MockIBClient) Ping(ctx context.Context) error {
	m.RecordCall("Ping")
	if m.closed {
		return ErrClientClosed
	}
	return m.PingError
}

// AuthStatus implements IBProvider.
func (m *MockIBClient) AuthStatus(ctx context.Context) (*ibapi.AuthStatusResponse, error) {
	m.RecordCall("AuthStatus")
	if m.closed {
		return nil, ErrClientClosed
	}
	return m.AuthStatusResponse, m.AuthStatusError
}

// IsConnected implements IBProvider.
func (m *MockIBClient) IsConnected(ctx context.Context) bool {
	m.RecordCall("IsConnected")
	if m.PingError != nil {
		return false
	}
	return m.Connected && !m.closed
}

// HistoricalData implements IBProvider.
func (m *MockIBClient) HistoricalData(ctx context.Context, req HistoricalDataRequest) (*ibapi.HistoricalDataResponse, error) {
	m.RecordCall("HistoricalData", req.Conid, req.Exchange, req.Period, req.Bar)
	if m.closed {
		return nil, ErrClientClosed
	}
	return m.HistoricalDataResponse, m.HistoricalDataError
}

// Close implements IBProvider.
func (m *MockIBClient) Close() error {
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

// Ensure MockIBClient implements IBProvider at compile time.
var _ IBProvider = (*MockIBClient)(nil)