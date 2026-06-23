package earnings

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestClientClosedError(t *testing.T) {
	err := &ClientClosedError{}

	if err.Error() != "earnings client is closed" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestMissingAPIKeyError(t *testing.T) {
	err := &MissingAPIKeyError{}

	if err.Error() != "EARNINGS_API_KEY environment variable is not set" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestInvalidDateError(t *testing.T) {
	err := &InvalidDateError{Date: "2026-13-45"}

	expected := "invalid date format: 2026-13-45 (expected YYYY-MM-DD or today/yesterday/tomorrow)"
	if err.Error() != expected {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestAPIError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		message    string
		expected   string
	}{
		{
			name:       "with message",
			statusCode: 401,
			message:    "unauthorized",
			expected:   "earnings API error: unauthorized",
		},
		{
			name:       "without message",
			statusCode: 500,
			expected:   "earnings API error: status 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &APIError{
				StatusCode: tt.statusCode,
				Message:    tt.message,
			}

			if err.Error() != tt.expected {
				t.Errorf("unexpected error message: got %s, want %s", err.Error(), tt.expected)
			}
		})
	}
}

func TestIsAPIError(t *testing.T) {
	err := &APIError{StatusCode: 401}

	if !IsAPIError(err, 401) {
		t.Error("expected IsAPIError(401) to return true")
	}

	if IsAPIError(err, 500) {
		t.Error("expected IsAPIError(500) to return false")
	}

	// Non-API error should return false
	if IsAPIError(context.DeadlineExceeded, 401) {
		t.Error("expected IsAPIError on non-API error to return false")
	}
}

func TestNewClientMissingAPIKey(t *testing.T) {
	// Clear the environment variable
	oldKey := os.Getenv("EARNINGS_API_KEY")
	os.Unsetenv("EARNINGS_API_KEY")
	defer func() {
		if oldKey != "" {
			os.Setenv("EARNINGS_API_KEY", oldKey)
		}
	}()

	_, err := NewClient()
	if err == nil {
		t.Error("expected error when API key is missing")
	}

	if err != ErrMissingAPIKey {
		t.Errorf("expected ErrMissingAPIKey, got %v", err)
	}
}

func TestNewClientWithAPIKey(t *testing.T) {
	// Set a test API key
	oldKey := os.Getenv("EARNINGS_API_KEY")
	os.Setenv("EARNINGS_API_KEY", "test-api-key")
	defer func() {
		if oldKey != "" {
			os.Setenv("EARNINGS_API_KEY", oldKey)
		} else {
			os.Unsetenv("EARNINGS_API_KEY")
		}
	}()

	client, err := NewClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client == nil {
		t.Fatal("expected non-nil client")
	}

	if client.apiKey != "test-api-key" {
		t.Errorf("unexpected API key: %s", client.apiKey)
	}

	if client.baseURL != DefaultBaseURL {
		t.Errorf("unexpected base URL: %s", client.baseURL)
	}
}

func TestNewClientWithOptions(t *testing.T) {
	oldKey := os.Getenv("EARNINGS_API_KEY")
	os.Setenv("EARNINGS_API_KEY", "test-key")
	defer func() {
		if oldKey != "" {
			os.Setenv("EARNINGS_API_KEY", oldKey)
		} else {
			os.Unsetenv("EARNINGS_API_KEY")
		}
	}()

	customURL := "https://custom.earningsapi.com"
	customTimeout := 60 * time.Second

	client, err := NewClient(
		WithBaseURL(customURL),
		WithTimeout(customTimeout),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.baseURL != customURL {
		t.Errorf("expected base URL %s, got %s", customURL, client.baseURL)
	}

	if client.timeout != customTimeout {
		t.Errorf("expected timeout %v, got %v", customTimeout, client.timeout)
	}
}

func TestMockClientPing(t *testing.T) {
	mock := NewMockClient()

	err := mock.Ping(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !mock.AssertCalled("Ping") {
		t.Error("expected Ping to be called")
	}
}

func TestMockClientPingError(t *testing.T) {
	expectedErr := &InvalidDateError{Date: "today"}
	mock := NewMockClient().WithPingError(expectedErr)

	err := mock.Ping(context.Background())
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestMockClientEarningsCalendar(t *testing.T) {
	mock := NewMockClient()

	expectedResp := &EarningsCalendarResponse{
		Date: "2026-01-31",
		Pre: []EarningsEntry{
			{Symbol: "JPM", Name: "J P Morgan Chase & Co"},
		},
		After: []EarningsEntry{
			{Symbol: "AAPL", Name: "Apple Inc."},
		},
	}
	mock.WithEarningsCalendarResponse(expectedResp)

	resp, err := mock.EarningsCalendar(context.Background(), Today())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Date != "2026-01-31" {
		t.Errorf("unexpected date: %s", resp.Date)
	}

	if len(resp.Pre) != 1 {
		t.Errorf("expected 1 pre-market entry, got %d", len(resp.Pre))
	}

	if !mock.AssertCalled("EarningsCalendar") {
		t.Error("expected EarningsCalendar to be called")
	}
}

func TestMockClientEarningsCalendarError(t *testing.T) {
	expectedErr := &APIError{StatusCode: 429, Message: "rate limited"}
	mock := NewMockClient().WithEarningsCalendarError(expectedErr)

	_, err := mock.EarningsCalendar(context.Background(), Today())
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestMockClientClosed(t *testing.T) {
	mock := NewMockClient()

	// Close the client
	err := mock.Close()
	if err != nil {
		t.Fatalf("unexpected error on close: %v", err)
	}

	// Operations should return ErrClientClosed
	_, err = mock.EarningsCalendar(context.Background(), Today())
	if err != ErrClientClosed {
		t.Errorf("expected ErrClientClosed, got %v", err)
	}

	err = mock.Ping(context.Background())
	if err != ErrClientClosed {
		t.Errorf("expected ErrClientClosed, got %v", err)
	}
}

func TestMockClientRecordCall(t *testing.T) {
	mock := NewMockClient()

	if mock.TotalCalls() != 0 {
		t.Errorf("expected 0 calls, got %d", mock.TotalCalls())
	}

	mock.Ping(context.Background())
	mock.Ping(context.Background())

	if mock.TotalCalls() != 2 {
		t.Errorf("expected 2 calls, got %d", mock.TotalCalls())
	}

	mock.Reset()
	if mock.TotalCalls() != 0 {
		t.Errorf("expected 0 calls after reset, got %d", mock.TotalCalls())
	}
}

func TestCalendarDateHelpers(t *testing.T) {
	tests := []struct {
		name      string
		date      CalendarDate
		expected  string
		relative  bool
	}{
		{
			name:     "today",
			date:     Today(),
			expected: "today",
			relative: true,
		},
		{
			name:     "yesterday",
			date:     Yesterday(),
			expected: "yesterday",
			relative: true,
		},
		{
			name:     "tomorrow",
			date:     Tomorrow(),
			expected: "tomorrow",
			relative: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.date.Value != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.date.Value)
			}
			if tt.date.IsRelative != tt.relative {
				t.Errorf("expected IsRelative %v, got %v", tt.relative, tt.date.IsRelative)
			}
		})
	}
}

func TestMockEarningsEntry(t *testing.T) {
	entry := MockEarningsEntry("AAPL", "Apple Inc.", 1.53, 1.85, 90000000000, 89000000000)

	if entry.Symbol != "AAPL" {
		t.Errorf("expected Symbol AAPL, got %s", entry.Symbol)
	}
	if entry.Name != "Apple Inc." {
		t.Errorf("expected Name Apple Inc., got %s", entry.Name)
	}
	if entry.EpsEstimate != 1.53 {
		t.Errorf("expected EpsEstimate 1.53, got %f", entry.EpsEstimate)
	}
	if entry.Eps != 1.85 {
		t.Errorf("expected Eps 1.85, got %f", entry.Eps)
	}
	if entry.Revenue != 90000000000 {
		t.Errorf("expected Revenue 90000000000, got %d", entry.Revenue)
	}
	if entry.RevenueEstimate != 89000000000 {
		t.Errorf("expected RevenueEstimate 89000000000, got %d", entry.RevenueEstimate)
	}
}

func TestMockEarningsCalendarResponse(t *testing.T) {
	pre := []EarningsEntry{{Symbol: "JPM", Name: "JPMorgan"}}
	after := []EarningsEntry{{Symbol: "AAPL", Name: "Apple"}}
	notSupplied := []EarningsEntry{{Symbol: "BK", Name: "Bank of NY"}}

	resp := MockEarningsCalendarResponse("2026-01-31", pre, after, notSupplied)

	if resp.Date != "2026-01-31" {
		t.Errorf("expected Date 2026-01-31, got %s", resp.Date)
	}
	if len(resp.Pre) != 1 {
		t.Errorf("expected 1 pre entry, got %d", len(resp.Pre))
	}
	if len(resp.After) != 1 {
		t.Errorf("expected 1 after entry, got %d", len(resp.After))
	}
	if len(resp.NotSupplied) != 1 {
		t.Errorf("expected 1 notSupplied entry, got %d", len(resp.NotSupplied))
	}
}

// Ensure Client implements Provider at compile time.
var _ Provider = (*Client)(nil)

// Ensure MockClient implements Provider at compile time.
var _ Provider = (*MockClient)(nil)

// Suppress unused import warning
var _ = http.StatusOK