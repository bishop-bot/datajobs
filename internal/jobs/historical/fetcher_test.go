package historical

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	ibapi "github.com/bishop-bot/ibapi-go"
	"github.com/bishop-bot/datajobs/internal/database"
	"github.com/bishop-bot/datajobs/internal/providers/ib"
)

// mockIBProvider is a simple mock for testing single-page responses.
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

// mockPaginatedIBProvider simulates paginated responses for testing.
type mockPaginatedIBProvider struct {
	pages       []*ibapi.HistoricalDataResponse
	currentPage int
	err         error
	maxPages    int
}

func newMockPaginatedProvider(pages []*ibapi.HistoricalDataResponse) *mockPaginatedIBProvider {
	return &mockPaginatedIBProvider{
		pages:    pages,
		maxPages: 100,
	}
}

func (m *mockPaginatedIBProvider) Ping(ctx context.Context) error {
	return nil
}

func (m *mockPaginatedIBProvider) AuthStatus(ctx context.Context) (*ibapi.AuthStatusResponse, error) {
	return &ibapi.AuthStatusResponse{Authenticated: true}, nil
}

func (m *mockPaginatedIBProvider) IsConnected(ctx context.Context) bool {
	return true
}

func (m *mockPaginatedIBProvider) HistoricalData(ctx context.Context, req ib.HistoricalDataRequest) (*ibapi.HistoricalDataResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.currentPage >= len(m.pages) || m.currentPage >= m.maxPages {
		return &ibapi.HistoricalDataResponse{Data: []ibapi.HistoricalDataBar{}}, nil
	}
	resp := m.pages[m.currentPage]
	m.currentPage++
	return resp, nil
}

func (m *mockPaginatedIBProvider) Close() error {
	return nil
}

func TestFetchOHLCV(t *testing.T) {
	t.Run("fetches and converts data successfully", func(t *testing.T) {
		nowMs := time.Now().UnixMilli()
		mock := &mockIBProvider{
			historicalDataResp: &ibapi.HistoricalDataResponse{
				Symbol: "AAPL",
				Data: []ibapi.HistoricalDataBar{
					{T: nowMs, O: 185.50, H: 186.75, L: 184.90, C: 186.20, V: 50000000},
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

	t.Run("fetches paginated data across multiple pages", func(t *testing.T) {
		nowMs := time.Now().UnixMilli()
		// Use minute bars so pagination triggers (expectedBarsPerPage=100)
		// Create pages with 100 bars each to trigger pagination
		page1Bars := make([]ibapi.HistoricalDataBar, 100)
		page2Bars := make([]ibapi.HistoricalDataBar, 100)
		for i := 0; i < 100; i++ {
			page1Bars[i] = ibapi.HistoricalDataBar{T: nowMs - int64(i*60*1000), O: 185.50, H: 186.75, L: 184.90, C: 186.20, V: 50000000}
			page2Bars[i] = ibapi.HistoricalDataBar{T: nowMs - int64((100+i)*60*1000), O: 183.00, H: 184.00, L: 182.00, C: 183.50, V: 45000000}
		}

		mock := newMockPaginatedProvider([]*ibapi.HistoricalDataResponse{
			{Symbol: "AAPL", Data: page1Bars},
			{Symbol: "AAPL", Data: page2Bars},
		})

		instr := instrument{Conid: "265598", Symbol: "AAPL", Exchange: "SMART"}
		params := historicalParams{Period: "5y", Bar: "5mins", OutsideRth: false}

		bars, err := fetchOHLCV(context.Background(), mock, instr, params)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(bars) != 200 {
			t.Errorf("expected 200 bars across pages, got %d", len(bars))
		}
	})

	t.Run("returns error on IB failure", func(t *testing.T) {
		mock := &mockIBProvider{
			historicalDataErr: errors.New("IB API error"),
		}

		instr := instrument{Conid: "265598", Symbol: "AAPL", Exchange: "SMART"}
		params := historicalParams{Period: "1d", Bar: "1d", OutsideRth: false}

		_, err := fetchOHLCV(context.Background(), mock, instr, params)

		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("returns empty on empty response", func(t *testing.T) {
		mock := &mockIBProvider{
			historicalDataResp: &ibapi.HistoricalDataResponse{
				Symbol: "AAPL",
				Data:   []ibapi.HistoricalDataBar{},
			},
		}

		instr := instrument{Conid: "265598", Symbol: "AAPL", Exchange: "SMART"}
		params := historicalParams{Period: "1d", Bar: "1d", OutsideRth: false}

		bars, err := fetchOHLCV(context.Background(), mock, instr, params)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(bars) != 0 {
			t.Errorf("expected 0 bars, got %d", len(bars))
		}
	})

	t.Run("returns empty on nil response", func(t *testing.T) {
		mock := &mockIBProvider{
			historicalDataResp: nil,
		}

		instr := instrument{Conid: "265598", Symbol: "AAPL", Exchange: "SMART"}
		params := historicalParams{Period: "1d", Bar: "1d", OutsideRth: false}

		bars, err := fetchOHLCV(context.Background(), mock, instr, params)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(bars) != 0 {
			t.Errorf("expected 0 bars, got %d", len(bars))
		}
	})

	t.Run("stops pagination when reaching fromTime boundary", func(t *testing.T) {
		nowMs := time.Now().UnixMilli()
		fiveYearsAgoMs := nowMs - int64(5*365*24*3600*1000)

		// Use minute bars so pagination triggers (100 bars per page)
		// Page 1: 100 bars (full page to trigger pagination)
		page1Bars := make([]ibapi.HistoricalDataBar, 100)
		for i := 0; i < 100; i++ {
			page1Bars[i] = ibapi.HistoricalDataBar{T: nowMs - int64(i*60*1000), O: 185.50, H: 186.75, L: 184.90, C: 186.20, V: 50000000}
		}
		// Page 2 has data that crosses the 5y boundary
		page2Bars := []ibapi.HistoricalDataBar{
			{T: fiveYearsAgoMs - int64(24*3600*1000), O: 100.00, H: 101.00, L: 99.00, C: 100.50, V: 30000000},
			{T: fiveYearsAgoMs + int64(24*3600*1000), O: 101.00, H: 102.00, L: 100.00, C: 101.50, V: 31000000},
		}

		mock := newMockPaginatedProvider([]*ibapi.HistoricalDataResponse{
			{Symbol: "AAPL", Data: page1Bars},
			{Symbol: "AAPL", Data: page2Bars},
		})

		instr := instrument{Conid: "265598", Symbol: "AAPL", Exchange: "SMART"}
		params := historicalParams{Period: "5y", Bar: "5mins", OutsideRth: false}

		bars, err := fetchOHLCV(context.Background(), mock, instr, params)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should have 101 bars: 100 from page 1 + 1 from page 2 (one filtered at boundary)
		if len(bars) != 101 {
			t.Errorf("expected 101 bars (100 page1 + 1 filtered page2), got %d", len(bars))
		}
	})
}

func TestFetchOHLCV_ErrorIncludesSymbol(t *testing.T) {
	mock := &mockIBProvider{
		historicalDataErr: errors.New("connection timeout"),
	}

	instr := instrument{Conid: "123", Symbol: "TSLA", Exchange: "NASDAQ"}
	params := historicalParams{Period: "1d", Bar: "1d", OutsideRth: false}

	_, err := fetchOHLCV(context.Background(), mock, instr, params)

	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "TSLA") {
		t.Errorf("error should contain symbol TSLA, got: %v", err)
	}
}

func TestCalculateFromTime(t *testing.T) {
	tests := []struct {
		period string
		wantOk bool
	}{
		// Valid periods
		{"1min", true},
		{"30min", true},
		{"5mins", true},
		{"1h", true},
		{"8h", true},
		{"1d", true},
		{"365d", true},
		{"1w", true},
		{"52w", true},
		{"1m", true},
		{"12m", true},
		{"1y", true},
		{"5y", true},
		{"15y", true},
		// Invalid periods
		{"invalid", false},
		{"", false},
		{"0d", false},
		{"1001d", false},  // > 1000
		{"50min", false},  // > 30
		{"20h", false},    // > 8
		{"800w", false},   // > 792
		{"200m", false},   // > 182
		{"20y", false},    // > 15
	}

	for _, tt := range tests {
		t.Run(tt.period, func(t *testing.T) {
			ts, err := calculateFromTime(tt.period)
			if tt.wantOk && err != nil {
				t.Errorf("calculateFromTime(%q) unexpected error: %v", tt.period, err)
			}
			if !tt.wantOk && err == nil {
				t.Errorf("calculateFromTime(%q) expected error, got nil", tt.period)
			}
			if tt.wantOk {
				now := time.Now().UTC().UnixNano()
				if ts >= now {
					t.Errorf("calculateFromTime(%q) = %d, should be less than now (%d)", tt.period, ts, now)
				}
			}
		})
	}
}

func TestParsePeriodToDuration(t *testing.T) {
	tests := []struct {
		period   string
		wantHours float64
		wantOk    bool
	}{
		// Minutes
		{"5min", 5.0 / 60.0, true},
		{"30min", 30.0 / 60.0, true},
		// Hours
		{"1h", 1.0, true},
		{"8h", 8.0, true},
		// Days
		{"1d", 24.0, true},
		{"30d", 30.0 * 24.0, true},
		// Weeks
		{"1w", 7.0 * 24.0, true},
		// Months
		{"12m", 12.0 * 30.0 * 24.0, true},
		// Years
		{"1y", 365.0 * 24.0, true},
		{"5y", 5.0 * 365.0 * 24.0, true},
		// Handle "mins" suffix
		{"5mins", 5.0 / 60.0, true},
		// Invalid
		{"", 0, false},
		{"invalid", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.period, func(t *testing.T) {
			dur, err := parsePeriodToDuration(tt.period)
			if tt.wantOk && err != nil {
				t.Errorf("parsePeriodToDuration(%q) unexpected error: %v", tt.period, err)
			}
			if !tt.wantOk && err == nil {
				t.Errorf("parsePeriodToDuration(%q) expected error, got nil", tt.period)
			}
			if tt.wantOk {
				wantDur := time.Duration(tt.wantHours * float64(time.Hour))
				if dur != wantDur {
					t.Errorf("parsePeriodToDuration(%q) = %v, want %v", tt.period, dur, wantDur)
				}
			}
		})
	}
}

func TestFormatTimestampAsIB(t *testing.T) {
	tests := []struct {
		name    string
		ts      int64
		wantFmt string
	}{
		{
			name:    "standard timestamp",
			ts:      time.Date(2026, 6, 20, 16, 30, 0, 0, time.UTC).UnixNano(),
			wantFmt: "20260620-16:30:00",
		},
		{
			name:    "midnight",
			ts:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).UnixNano(),
			wantFmt: "20260101-00:00:00",
		},
		{
			name:    "end of day",
			ts:      time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC).UnixNano(),
			wantFmt: "20261231-23:59:59",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTimestampAsIB(tt.ts)
			if got != tt.wantFmt {
				t.Errorf("formatTimestampAsIB(%d) = %q, want %q", tt.ts, got, tt.wantFmt)
			}
		})
	}
}

func TestFilterBarsFromTime(t *testing.T) {
	now := time.Now().UnixNano()
	fromTime := now - int64(2*24*3600*1000*1e6) // 2 days ago

	olderTs := fromTime - int64(24*3600*1000*1e6) // 3 days ago
	newerTs1 := now - int64(24*3600*1000*1e6)    // 1 day ago
	newerTs2 := now                               // now

	bars := []database.OHLCVBar{
		{Symbol: "AAPL", Ts: olderTs},
		{Symbol: "AAPL", Ts: newerTs1},
		{Symbol: "AAPL", Ts: newerTs2},
	}

	filtered := filterBarsFromTime(bars, fromTime)

	if len(filtered) != 2 {
		t.Errorf("expected 2 bars after filtering, got %d", len(filtered))
	}
	if filtered[0].Ts != newerTs1 {
		t.Errorf("first bar Ts = %d, want %d", filtered[0].Ts, newerTs1)
	}
	if filtered[1].Ts != newerTs2 {
		t.Errorf("second bar Ts = %d, want %d", filtered[1].Ts, newerTs2)
	}
}

func TestFilterBarsFromTime_AllFiltered(t *testing.T) {
	now := time.Now().UnixNano()
	fromTime := now // in the future, all bars will be filtered

	bars := []database.OHLCVBar{
		{Symbol: "AAPL", Ts: now - int64(24*3600*1000*1e6)},
		{Symbol: "AAPL", Ts: now - int64(48*3600*1000*1e6)},
	}

	filtered := filterBarsFromTime(bars, fromTime)

	if len(filtered) != 0 {
		t.Errorf("expected 0 bars after filtering, got %d", len(filtered))
	}
}

func TestExpectedBarsPerPage(t *testing.T) {
	tests := []struct {
		bar  string
		want int
	}{
		{"1min", 100},
		{"5min", 100},
		{"30min", 100},
		{"5mins", 100},
		{"1h", 100},
		{"8h", 100},
		{"1d", 1000},
		{"365d", 1000},
		{"1w", 1000},
		{"52w", 1000},
		{"1m", 1000},
		{"12m", 1000},
		{"1y", 1000},
		{"5y", 1000},
		{"", 100},
		{"unknown", 100},
	}

	for _, tt := range tests {
		t.Run(tt.bar, func(t *testing.T) {
			got := expectedBarsPerPage(tt.bar)
			if got != tt.want {
				t.Errorf("expectedBarsPerPage(%q) = %d, want %d", tt.bar, got, tt.want)
			}
		})
	}
}