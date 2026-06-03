package handlers

import (
	"testing"

	"github.com/bishop-bot/datajobs/internal/ingestion"
)

func TestBarDurationNs(t *testing.T) {
	tests := []struct {
		bar      string
		expected int64
	}{
		// IB API format bar sizes
		{"1s", 1 * 1_000_000_000},
		{"5s", 5 * 1_000_000_000},
		{"10s", 10 * 1_000_000_000},
		{"15s", 15 * 1_000_000_000},
		{"30s", 30 * 1_000_000_000},
		{"1m", 60 * 1_000_000_000},
		{"2m", 2 * 60 * 1_000_000_000},
		{"3m", 3 * 60 * 1_000_000_000},
		{"5m", 5 * 60 * 1_000_000_000},
		{"10m", 10 * 60 * 1_000_000_000},
		{"15m", 15 * 60 * 1_000_000_000},
		{"30m", 30 * 60 * 1_000_000_000},
		{"1h", 60 * 60 * 1_000_000_000},
		{"2h", 2 * 60 * 60 * 1_000_000_000},
		{"3h", 3 * 60 * 60 * 1_000_000_000},
		{"4h", 4 * 60 * 60 * 1_000_000_000},
		{"8h", 8 * 60 * 60 * 1_000_000_000},
		{"1d", 24 * 60 * 60 * 1_000_000_000},
		{"1w", 7 * 24 * 60 * 60 * 1_000_000_000},
	}

	for _, tt := range tests {
		t.Run(tt.bar, func(t *testing.T) {
			result := ingestion.BarDurationNs(tt.bar)
			if result != tt.expected {
				t.Errorf("BarDurationNs(%s): expected %d, got %d", tt.bar, tt.expected, result)
			}
		})
	}
}

func TestBarDurationNsEdgeCases(t *testing.T) {
	// Test empty string falls back to default (5min)
	result := ingestion.BarDurationNs("")
	if result != 5*60*1_000_000_000 {
		t.Errorf("expected default 5min duration for empty string, got %d", result)
	}

	// Test unrecognized formats fall back to default
	tests := []struct {
		bar      string
		expected int64
	}{
		{"1min", 5 * 60 * 1_000_000_000},   // unrecognized, defaults to 5min
		{"1hour", 5 * 60 * 1_000_000_000},   // unrecognized, defaults to 5min
		{"1day", 5 * 60 * 1_000_000_000},    // unrecognized, defaults to 5min
		{"unknown", 5 * 60 * 1_000_000_000}, // unrecognized, defaults to 5min
	}

	for _, tt := range tests {
		result := ingestion.BarDurationNs(tt.bar)
		if result != tt.expected {
			t.Errorf("BarDurationNs(%s): expected %d, got %d", tt.bar, tt.expected, result)
		}
	}
}