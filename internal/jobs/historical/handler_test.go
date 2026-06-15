package historical

import (
	"context"
	"strings"
	"testing"

	ibapi "github.com/bishop-bot/ibapi-go"
	"github.com/bishop-bot/datajobs/internal/providers/ib"
	"github.com/bishop-bot/datajobs/internal/worker"
)

func TestBuildResultMessage(t *testing.T) {
	tests := []struct {
		name            string
		instrumentCount int
		totalBars       int
		failedSymbols   []string
		wantContains    []string
	}{
		{
			name:            "success with all instruments",
			instrumentCount: 5,
			totalBars:       1000,
			failedSymbols:   nil,
			wantContains:    []string{"processed 5 instruments", "upserted 1000 bars"},
		},
		{
			name:            "with failed symbols",
			instrumentCount: 3,
			totalBars:       500,
			failedSymbols:   []string{"AAPL", "TSLA"},
			wantContains:    []string{"processed 3 instruments", "upserted 500 bars", "failed: AAPL, TSLA"},
		},
		{
			name:            "no bars upserted",
			instrumentCount: 2,
			totalBars:       0,
			failedSymbols:   nil,
			wantContains:    []string{"processed 2 instruments", "upserted 0 bars"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildResultMessage(tt.instrumentCount, tt.totalBars, tt.failedSymbols)

			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("result = %q, want to contain %q", result, want)
				}
			}
		})
	}
}

func TestHandlerNilChecks(t *testing.T) {
	t.Run("returns error when ibProvider is nil", func(t *testing.T) {
		handler := HistoricalDataHandlerWithDB(nil, nil, nil)
		_, err := handler(context.Background(), worker.Job{ID: "test"})

		if err == nil {
			t.Error("expected error for nil ibProvider")
		}
		if err.Error() != "IB provider not available" {
			t.Errorf("error = %q, want %q", err.Error(), "IB provider not available")
		}
	})

	t.Run("returns error when questDB is nil", func(t *testing.T) {
		mockIB := ib.NewMockClient()
		handler := HistoricalDataHandlerWithDB(nil, nil, mockIB)
		_, err := handler(context.Background(), worker.Job{ID: "test"})

		if err == nil {
			t.Error("expected error for nil questDB")
		}
		if err.Error() != "QuestDB not connected" {
			t.Errorf("error = %q, want %q", err.Error(), "QuestDB not connected")
		}
	})
}

func TestHandlerSignature(t *testing.T) {
	// Verify handler has correct signature
	var handler worker.JobFunc = HistoricalDataHandlerWithDB(nil, nil, nil)
	if handler == nil {
		t.Error("handler should not be nil")
	}
}

func TestBuildResultMessageFormat(t *testing.T) {
	t.Run("single failed symbol", func(t *testing.T) {
		result := buildResultMessage(1, 100, []string{"FAIL"})
		if !strings.Contains(result, "failed: FAIL") {
			t.Errorf("unexpected result: %s", result)
		}
	})

	t.Run("multiple failed symbols", func(t *testing.T) {
		result := buildResultMessage(5, 500, []string{"A", "B", "C"})
		if !strings.Contains(result, "failed: A, B, C") {
			t.Errorf("unexpected result: %s", result)
		}
	})

	t.Run("no failures", func(t *testing.T) {
		result := buildResultMessage(3, 300, nil)
		if strings.Contains(result, "failed:") {
			t.Errorf("unexpected failure in result: %s", result)
		}
	})
}

// mockIBProviderFull implements ib.Provider for handler tests
type mockIBProviderFull struct{}

func (m *mockIBProviderFull) Ping(ctx context.Context) error {
	return nil
}

func (m *mockIBProviderFull) AuthStatus(ctx context.Context) (*ibapi.AuthStatusResponse, error) {
	return &ibapi.AuthStatusResponse{Authenticated: true}, nil
}

func (m *mockIBProviderFull) IsConnected(ctx context.Context) bool {
	return true
}

func (m *mockIBProviderFull) HistoricalData(ctx context.Context, req ib.HistoricalDataRequest) (*ibapi.HistoricalDataResponse, error) {
	return nil, nil
}

func (m *mockIBProviderFull) Close() error {
	return nil
}

