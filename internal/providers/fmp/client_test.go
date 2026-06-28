package fmp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient_MissingAPIKey(t *testing.T) {
	t.Setenv("FMP_API_KEY", "")

	client, err := NewClient()
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if !errors.Is(err, ErrMissingAPIKey) {
		t.Errorf("expected ErrMissingAPIKey, got: %v", err)
	}
	if client != nil {
		t.Error("expected nil client")
	}
}

func TestMockClient_Ping(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	err := mock.Ping(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !mock.AssertCalled("Ping") {
		t.Error("expected Ping to be called")
	}
}

func TestMockClient_FinancialRatios(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	resp := MockFinancialRatiosResponse("AAPL", "2024-01-01")
	mock.WithFinancialRatiosResponse(resp)

	result, err := mock.FinancialRatios(ctx, "AAPL", PeriodAnnual)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Symbol != "AAPL" {
		t.Errorf("expected AAPL, got: %s", result.Symbol)
	}

	if !mock.AssertCalled("FinancialRatios") {
		t.Error("expected FinancialRatios to be called")
	}
}

func TestMockClient_KeyMetrics(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	resp := MockKeyMetricsResponse("AAPL", "2024-01-01", PeriodAnnual)
	mock.WithKeyMetricsResponse(resp)

	result, err := mock.KeyMetrics(ctx, "AAPL", PeriodAnnual)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Symbol != "AAPL" {
		t.Errorf("expected AAPL, got: %s", result.Symbol)
	}

	if !mock.AssertCalled("KeyMetrics") {
		t.Error("expected KeyMetrics to be called")
	}
}

func TestMockClient_ClosedError(t *testing.T) {
	mock := NewMockClient()
	mock.Close()
	ctx := context.Background()

	_, err := mock.FinancialRatios(ctx, "AAPL", PeriodAnnual)
	if !errors.Is(err, ErrClientClosed) {
		t.Errorf("expected ErrClientClosed, got: %v", err)
	}
}

func TestMockClient_FinancialRatiosError(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	mock.WithFinancialRatiosError(errors.New("API error"))

	_, err := mock.FinancialRatios(ctx, "AAPL", PeriodAnnual)
	if err == nil {
		t.Error("expected error")
	}
}

func TestIsValidPeriod(t *testing.T) {
	tests := []struct {
		period  string
		valid   bool
	}{
		{PeriodAnnual, true},
		{PeriodQuarter, true},
		{PeriodTTM, true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.period, func(t *testing.T) {
			result := isValidPeriod(tt.period)
			if result != tt.valid {
				t.Errorf("isValidPeriod(%q) = %v, want %v", tt.period, result, tt.valid)
			}
		})
	}
}

func TestClient_Ping(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Setenv("FMP_API_KEY", "test-key")

	client, err := NewClient(WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	err = client.Ping(ctx)
	if err != nil {
		t.Errorf("Ping failed: %v", err)
	}
}

func TestClient_InvalidPeriod(t *testing.T) {
	t.Setenv("FMP_API_KEY", "test-key")

	client, err := NewClient()
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	_, err = client.FinancialRatios(ctx, "AAPL", "invalid")
	if err == nil {
		t.Error("expected error for invalid period")
	}
}

func TestClient_InvalidSymbol(t *testing.T) {
	t.Setenv("FMP_API_KEY", "test-key")

	client, err := NewClient()
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	_, err = client.FinancialRatios(ctx, "", PeriodAnnual)
	if err == nil {
		t.Error("expected error for empty symbol")
	}
}
