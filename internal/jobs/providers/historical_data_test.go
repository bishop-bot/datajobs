package providers

import (
	"context"
	"testing"

	ibapi "github.com/bishop-bot/ibapi-go"
	"github.com/bishop-bot/datajobs/internal/ingestion"
	"github.com/bishop-bot/datajobs/internal/providers/ib"
	"github.com/bishop-bot/datajobs/internal/worker"
)

func TestParseHistoricalParams(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]interface{}
		want     historicalParams
	}{
		{
			name: "uses defaults when empty",
			metadata: map[string]interface{}{},
			want: historicalParams{
				Period:      defaultPeriod,
				Bar:         defaultBar,
				OutsideRth:  defaultOutsideRth,
				Instruments: nil,
			},
		},
		{
			name: "uses provided values",
			metadata: map[string]interface{}{
				"period":      "1y",
				"bar":         "1hour",
				"outsideRth":  true,
				"instruments": []any{"123", "456"},
			},
			want: historicalParams{
				Period:      "1y",
				Bar:         "1hour",
				OutsideRth:  true,
				Instruments: []string{"123", "456"},
			},
		},
		{
			name: "partial values use defaults",
			metadata: map[string]interface{}{
				"period": "2y",
			},
			want: historicalParams{
				Period:     "2y",
				Bar:        defaultBar,
				OutsideRth: defaultOutsideRth,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseHistoricalParams(tt.metadata)
			if got.Period != tt.want.Period {
				t.Errorf("Period = %q, want %q", got.Period, tt.want.Period)
			}
			if got.Bar != tt.want.Bar {
				t.Errorf("Bar = %q, want %q", got.Bar, tt.want.Bar)
			}
			if got.OutsideRth != tt.want.OutsideRth {
				t.Errorf("OutsideRth = %v, want %v", got.OutsideRth, tt.want.OutsideRth)
			}
			if len(got.Instruments) != len(tt.want.Instruments) {
				t.Errorf("Instruments len = %d, want %d", len(got.Instruments), len(tt.want.Instruments))
			}
		})
	}
}

func TestHistoricalParamsDefaults(t *testing.T) {
	if defaultPeriod != "5y" {
		t.Errorf("defaultPeriod = %q, want %q", defaultPeriod, "5y")
	}
	if defaultBar != "1d" {
		t.Errorf("defaultBar = %q, want %q", defaultBar, "1d")
	}
	if defaultOutsideRth != false {
		t.Errorf("defaultOutsideRth = %v, want false", defaultOutsideRth)
	}
	if defaultPublisher != "IB" {
		t.Errorf("defaultPublisher = %q, want %q", defaultPublisher, "IB")
	}
	if upsertBatchSize != 1000 {
		t.Errorf("upsertBatchSize = %d, want %d", upsertBatchSize, 1000)
	}
}

func TestConvertIBBarsToOHLCV(t *testing.T) {
	t.Run("converts ib bars to ohlcv bars", func(t *testing.T) {
		ibBars := []ibapi.HistoricalDataBar{
			{T: 1719792000000, O: 185.50, H: 186.75, L: 184.90, C: 186.20, V: 50000000},
			{T: 1719878400000, O: 186.00, H: 187.00, L: 185.50, C: 186.50, V: 45000000},
		}
		symbol := "AAPL"
		params := historicalParams{
			Period:     "1d",
			Bar:        "1d",
			OutsideRth: false,
		}

		bars := convertIBBarsToOHLCV(symbol, ibBars, params)

		if len(bars) != 2 {
			t.Fatalf("expected 2 bars, got %d", len(bars))
		}

		// First bar
		if bars[0].Symbol != symbol {
			t.Errorf("Symbol = %q, want %q", bars[0].Symbol, symbol)
		}
		if bars[0].Publisher != defaultPublisher {
			t.Errorf("Publisher = %q, want %q", bars[0].Publisher, defaultPublisher)
		}
		if bars[0].Ts != 1719792000000*1_000_000 {
			t.Errorf("Ts = %d, want %d", bars[0].Ts, 1719792000000*1_000_000)
		}
		if bars[0].Open != 185.50 {
			t.Errorf("Open = %f, want %f", bars[0].Open, 185.50)
		}
		if bars[0].High != 186.75 {
			t.Errorf("High = %f, want %f", bars[0].High, 186.75)
		}
		if bars[0].Low != 184.90 {
			t.Errorf("Low = %f, want %f", bars[0].Low, 184.90)
		}
		if bars[0].Close != 186.20 {
			t.Errorf("Close = %f, want %f", bars[0].Close, 186.20)
		}
		if bars[0].Volume != 50000000 {
			t.Errorf("Volume = %d, want %d", bars[0].Volume, 50000000)
		}
	})

	t.Run("handles empty ib bars", func(t *testing.T) {
		bars := convertIBBarsToOHLCV("AAPL", []ibapi.HistoricalDataBar{}, historicalParams{})
		if len(bars) != 0 {
			t.Errorf("expected 0 bars, got %d", len(bars))
		}
	})

	t.Run("calculates TsEnd correctly for different bar sizes", func(t *testing.T) {
		ibBars := []ibapi.HistoricalDataBar{
			{T: 1719792000000, O: 100, H: 101, L: 99, C: 100.5, V: 1000},
		}

		// Test 1-day bar
		bars1d := convertIBBarsToOHLCV("AAPL", ibBars, historicalParams{Bar: "1d"})
		expectedEnd1d := int64(1719792000000*1_000_000) + ingestion.BarDurationNs("1d")
		if bars1d[0].TsEnd != expectedEnd1d {
			t.Errorf("1d TsEnd = %d, want %d", bars1d[0].TsEnd, expectedEnd1d)
		}

		// Test 1-hour bar
		bars1h := convertIBBarsToOHLCV("AAPL", ibBars, historicalParams{Bar: "1hour"})
		expectedEnd1h := int64(1719792000000*1_000_000) + ingestion.BarDurationNs("1hour")
		if bars1h[0].TsEnd != expectedEnd1h {
			t.Errorf("1hour TsEnd = %d, want %d", bars1h[0].TsEnd, expectedEnd1h)
		}
	})
}

func TestBuildHistoricalDataRequest(t *testing.T) {
	t.Run("uses instrument exchange when provided", func(t *testing.T) {
		instr := instrument{Conid: "123", Symbol: "AAPL", Exchange: "NASDAQ"}
		params := historicalParams{Period: "1d", Bar: "1d", OutsideRth: false}

		req := buildHistoricalDataRequest(instr, params)

		if req.Conid != "123" {
			t.Errorf("Conid = %q, want %q", req.Conid, "123")
		}
		if req.Exchange != "NASDAQ" {
			t.Errorf("Exchange = %q, want %q", req.Exchange, "NASDAQ")
		}
	})

	t.Run("defaults to SMART when exchange empty", func(t *testing.T) {
		instr := instrument{Conid: "123", Symbol: "AAPL", Exchange: ""}
		params := historicalParams{Period: "1d", Bar: "1d", OutsideRth: false}

		req := buildHistoricalDataRequest(instr, params)

		if req.Exchange != "SMART" {
			t.Errorf("Exchange = %q, want %q", req.Exchange, "SMART")
		}
	})

	t.Run("passes through period, bar, and outsideRth", func(t *testing.T) {
		instr := instrument{Conid: "123", Exchange: "SMART"}
		params := historicalParams{Period: "1y", Bar: "1hour", OutsideRth: true}

		req := buildHistoricalDataRequest(instr, params)

		if req.Period != "1y" {
			t.Errorf("Period = %q, want %q", req.Period, "1y")
		}
		if req.Bar != "1hour" {
			t.Errorf("Bar = %q, want %q", req.Bar, "1hour")
		}
		if !req.OutsideRth {
			t.Error("OutsideRth = false, want true")
		}
	})
}

func TestHistoricalDataHandlerWithDB_NilChecks(t *testing.T) {
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