package historical

import (
	"context"
	"errors"
	"strings"
	"testing"

	ibapi "github.com/bishop-bot/ibapi-go"
	"github.com/bishop-bot/datajobs/internal/providers/ib"
)

// mockIBProvider is a simple mock for testing
type mockIBProvider struct {
	historicalDataResp *ibapi.HistoricalDataResponse
	historicalDataErr  error
}

func (m *mockIBProvider) Ping(ctx context.Context) error {
	return nil
}

func (m *mockIBProvider) AuthStatus(ctx context.Context) (*ibapi.AuthStatusResponse, error) {
	return &ibapi.AuthStatusResponse{Authenticated: true}, nil
}

func (m *mockIBProvider) IsConnected(ctx context.Context) bool {
	return true
}

func (m *mockIBProvider) HistoricalData(ctx context.Context, req ib.HistoricalDataRequest) (*ibapi.HistoricalDataResponse, error) {
	return m.historicalDataResp, m.historicalDataErr
}

func (m *mockIBProvider) Close() error {
	return nil
}

func TestFetchOHLCV(t *testing.T) {
	t.Run("fetches and converts data successfully", func(t *testing.T) {
		mock := &mockIBProvider{
			historicalDataResp: &ibapi.HistoricalDataResponse{
				Symbol: "AAPL",
				Data: []ibapi.HistoricalDataBar{
					{T: 1719792000000, O: 185.50, H: 186.75, L: 184.90, C: 186.20, V: 50000000},
				},
			},
		}

		instr := instrument{Conid: "265598", Symbol: "AAPL", Exchange: "SMART"}
		params := historicalParams{Period: "1d", Bar: "1d", OutsideRth: false}

		bars, err := fetchOHLCV(context.Background(), mock, instr, params)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(bars) != 1 {
			t.Errorf("expected 1 bar, got %d", len(bars))
		}
		if bars[0].Symbol != "AAPL" {
			t.Errorf("Symbol = %q, want %q", bars[0].Symbol, "AAPL")
		}
	})

	t.Run("returns error on IB failure", func(t *testing.T) {
		mock := &mockIBProvider{
			historicalDataErr: errors.New("IB API error"),
		}

		instr := instrument{Conid: "265598", Symbol: "AAPL", Exchange: "SMART"}
		params := historicalParams{}

		_, err := fetchOHLCV(context.Background(), mock, instr, params)

		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("returns nil bars on empty response", func(t *testing.T) {
		mock := &mockIBProvider{
			historicalDataResp: &ibapi.HistoricalDataResponse{
				Symbol: "AAPL",
				Data:   []ibapi.HistoricalDataBar{},
			},
		}

		instr := instrument{Conid: "265598", Symbol: "AAPL", Exchange: "SMART"}
		params := historicalParams{}

		bars, err := fetchOHLCV(context.Background(), mock, instr, params)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if bars != nil {
			t.Errorf("expected nil bars, got %v", bars)
		}
	})

	t.Run("returns nil bars on nil response", func(t *testing.T) {
		mock := &mockIBProvider{
			historicalDataResp: nil,
		}

		instr := instrument{Conid: "265598", Symbol: "AAPL", Exchange: "SMART"}
		params := historicalParams{}

		bars, err := fetchOHLCV(context.Background(), mock, instr, params)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if bars != nil {
			t.Errorf("expected nil bars, got %v", bars)
		}
	})
}

func TestFetchOHLCV_ErrorIncludesSymbol(t *testing.T) {
	mock := &mockIBProvider{
		historicalDataErr: errors.New("connection timeout"),
	}

	instr := instrument{Conid: "123", Symbol: "TSLA", Exchange: "NASDAQ"}
	params := historicalParams{}

	_, err := fetchOHLCV(context.Background(), mock, instr, params)

	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "TSLA") {
		t.Errorf("error should contain symbol TSLA, got: %v", err)
	}
}