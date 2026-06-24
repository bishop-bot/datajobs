package earnings

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"
)

// MockClient implements Provider for testing.
// It allows controlled responses without requiring a live Earnings API.
type MockClient struct {
	// Configuration
	PingError error

	// Earnings calendar responses
	EarningsCalendarResponse *EarningsCalendarResponse
	EarningsCalendarError    error

	// Economic calendar responses
	EconomicCalendarResponse *EconomicCalendarResponse
	EconomicCalendarError    error

	// Call tracking
	callCount atomic.Int32
	Calls     []MockCall

	// Closed flag
	closed bool
}

// NewMockClient creates a mock Earnings API client with default successful behavior.
func NewMockClient() *MockClient {
	return &MockClient{
		EarningsCalendarResponse: &EarningsCalendarResponse{},
	}
}

// WithPingError sets the error to return on Ping.
func (m *MockClient) WithPingError(err error) *MockClient {
	m.PingError = err
	return m
}

// WithEarningsCalendarResponse sets the earnings calendar response.
func (m *MockClient) WithEarningsCalendarResponse(resp *EarningsCalendarResponse) *MockClient {
	m.EarningsCalendarResponse = resp
	return m
}

// WithEarningsCalendarError sets the error to return on EarningsCalendar.
func (m *MockClient) WithEarningsCalendarError(err error) *MockClient {
	m.EarningsCalendarError = err
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

// EarningsCalendar implements Provider.
func (m *MockClient) EarningsCalendar(ctx context.Context, date CalendarDate) (*EarningsCalendarResponse, error) {
	m.RecordCall("EarningsCalendar", date.Value)
	if m.closed {
		return nil, ErrClientClosed
	}
	return m.EarningsCalendarResponse, m.EarningsCalendarError
}

// EconomicCalendar implements Provider.
func (m *MockClient) EconomicCalendar(ctx context.Context, params EconomicCalendarParams) (*EconomicCalendarResponse, error) {
	m.RecordCall("EconomicCalendar", params.Date.Value, params.USMajor)
	if m.closed {
		return nil, ErrClientClosed
	}
	return m.EconomicCalendarResponse, m.EconomicCalendarError
}

// Close implements Provider.
func (m *MockClient) Close() error {
	m.RecordCall("Close")
	m.closed = true
	return nil
}

// MockCall records an invocation for assertion.
type MockCall struct {
	Method string
	Args   []any
	Time   time.Time
}

// Ensure MockClient implements Provider at compile time.
var _ Provider = (*MockClient)(nil)

// Mock helper functions

// MockEarningsEntry creates a test earnings entry.
func MockEarningsEntry(symbol, name string, epsEstimate, eps float64, revenue, revenueEstimate int64) EarningsEntry {
	return EarningsEntry{
		Symbol:          symbol,
		Name:            name,
		EpsEstimate:     epsEstimate,
		Eps:             eps,
		Revenue:         revenue,
		RevenueEstimate: revenueEstimate,
	}
}

// MockEarningsCalendarResponse creates a test response with earnings entries.
func MockEarningsCalendarResponse(date string, pre, after, notSupplied []EarningsEntry) *EarningsCalendarResponse {
	return &EarningsCalendarResponse{
		Date:       date,
		Pre:        pre,
		After:      after,
		NotSupplied: notSupplied,
	}
}

// MockHTTPResponse creates a mock HTTP response for testing HTTP transport.
type MockHTTPResponse struct {
	StatusCode int
	Body       string
	Headers    map[string]string
}

// MockTransport implements http.RoundTripper for testing.
type MockTransport struct {
	Response *MockHTTPResponse
	Error    error
}

// RoundTrip implements http.RoundTripper.
func (t *MockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.Error != nil {
		return nil, t.Error
	}

	return &http.Response{
		StatusCode: t.Response.StatusCode,
		Body:       http.NoBody,
		Header:     http.Header{},
	}, nil
}