package stocks

import (
	"testing"

	"github.com/bishop-bot/datajobs/internal/providers/earnings"
)

func TestConvertTime(t *testing.T) {
	tests := []struct {
		category string
		expected Time
	}{
		{"pre", TimeBMO},
		{"after", TimeAMC},
		{"during", TimeDMH},
		{"notSupplied", ""},
		{"unknown", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.category, func(t *testing.T) {
			result := ConvertTime(tt.category)
			if result != tt.expected {
				t.Errorf("ConvertTime(%q) = %v, want %v", tt.category, result, tt.expected)
			}
		})
	}
}

func TestEntryToStockEarnings(t *testing.T) {
	entry := earnings.EarningsEntry{
		Symbol:          "AAPL",
		Name:            "Apple Inc.",
		EpsEstimate:     1.53,
		Eps:             1.85,
		Revenue:         90000000000,
		RevenueEstimate: 89000000000,
	}

	se := entryToStockEarnings(entry, "2026-01-31", TimeBMO)

	if se.Symbol != "AAPL" {
		t.Errorf("expected Symbol AAPL, got %s", se.Symbol)
	}
	if se.Name != "Apple Inc." {
		t.Errorf("expected Name Apple Inc., got %s", se.Name)
	}
	if se.Date != "2026-01-31" {
		t.Errorf("expected Date 2026-01-31, got %s", se.Date)
	}
	if se.Time != TimeBMO {
		t.Errorf("expected Time BMO, got %s", se.Time)
	}
	if se.Status != StatusActual {
		t.Errorf("expected Status Actual, got %s", se.Status)
	}
	if se.EPS == nil || *se.EPS != 1.85 {
		t.Errorf("expected EPS 1.85, got %v", se.EPS)
	}
	if se.EPSEstimated == nil || *se.EPSEstimated != 1.53 {
		t.Errorf("expected EPSEstimated 1.53, got %v", se.EPSEstimated)
	}
	if se.Revenue == nil || *se.Revenue != 90000000000 {
		t.Errorf("expected Revenue 90000000000, got %v", se.Revenue)
	}
	if se.RevenueEstimated == nil || *se.RevenueEstimated != 89000000000 {
		t.Errorf("expected RevenueEstimated 89000000000, got %v", se.RevenueEstimated)
	}
}

func TestEntryToStockEarningsEstimateOnly(t *testing.T) {
	entry := earnings.EarningsEntry{
		Symbol:          "TSLA",
		Name:            "Tesla Inc.",
		EpsEstimate:     0.95,
		RevenueEstimate: 25000000000,
		// No actual EPS or Revenue
	}

	se := entryToStockEarnings(entry, "2026-02-01", TimeAMC)

	if se.Status != StatusEstimate {
		t.Errorf("expected Status Estimate, got %s", se.Status)
	}
	if se.EPS != nil {
		t.Error("expected EPS to be nil")
	}
	if se.EPSEstimated == nil || *se.EPSEstimated != 0.95 {
		t.Errorf("expected EPSEstimated 0.95, got %v", se.EPSEstimated)
	}
	if se.Revenue != nil {
		t.Error("expected Revenue to be nil")
	}
	if se.RevenueEstimated == nil || *se.RevenueEstimated != 25000000000 {
		t.Errorf("expected RevenueEstimated 25000000000, got %v", se.RevenueEstimated)
	}
}

func TestConvertResponseToEntities(t *testing.T) {
	resp := &earnings.EarningsCalendarResponse{
		Date: "2026-01-31",
		Pre: []earnings.EarningsEntry{
			{Symbol: "JPM", Name: "JPMorgan Chase"},
			{Symbol: "WFC", Name: "Wells Fargo"},
		},
		After: []earnings.EarningsEntry{
			{Symbol: "AAPL", Name: "Apple Inc."},
		},
		NotSupplied: []earnings.EarningsEntry{
			{Symbol: "BK", Name: "Bank of NY"},
		},
	}

	entities := convertResponseToEntities(resp)

	if len(entities) != 4 {
		t.Fatalf("expected 4 entities, got %d", len(entities))
	}

	// Check pre-market (BMO)
	if entities[0].Symbol != "JPM" || entities[0].Time != TimeBMO {
		t.Errorf("unexpected first entity: %+v", entities[0])
	}
	if entities[1].Symbol != "WFC" || entities[1].Time != TimeBMO {
		t.Errorf("unexpected second entity: %+v", entities[1])
	}

	// Check after-market (AMC)
	if entities[2].Symbol != "AAPL" || entities[2].Time != TimeAMC {
		t.Errorf("unexpected third entity: %+v", entities[2])
	}

	// Check not supplied (empty time)
	if entities[3].Symbol != "BK" || entities[3].Time != "" {
		t.Errorf("unexpected fourth entity: %+v", entities[3])
	}
}

func TestConvertResponseToEntitiesEmpty(t *testing.T) {
	resp := &earnings.EarningsCalendarResponse{
		Date:       "2026-01-31",
		Pre:        []earnings.EarningsEntry{},
		After:      []earnings.EarningsEntry{},
		NotSupplied: []earnings.EarningsEntry{},
	}

	entities := convertResponseToEntities(resp)

	if len(entities) != 0 {
		t.Errorf("expected 0 entities, got %d", len(entities))
	}
}
