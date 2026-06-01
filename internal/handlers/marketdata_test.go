package handlers

import (
	"testing"
)

func TestBarDurationNs(t *testing.T) {
	tests := []struct {
		bar      string
		expected int64
	}{
		{"1min", 60 * 1_000_000_000},
		{"2mins", 2 * 60 * 1_000_000_000},
		{"5mins", 5 * 60 * 1_000_000_000},
		{"10mins", 10 * 60 * 1_000_000_000},
		{"15mins", 15 * 60 * 1_000_000_000},
		{"30mins", 30 * 60 * 1_000_000_000},
		{"1hour", 60 * 60 * 1_000_000_000},
		{"2hour", 2 * 60 * 60 * 1_000_000_000},
		{"3hour", 3 * 60 * 60 * 1_000_000_000},
		{"4hour", 4 * 60 * 60 * 1_000_000_000},
		{"8hour", 8 * 60 * 60 * 1_000_000_000},
		{"1day", 24 * 60 * 60 * 1_000_000_000},
		{"1week", 7 * 24 * 60 * 60 * 1_000_000_000},
		{"unknown", 5 * 60 * 1_000_000_000}, // default to 5min
	}

	for _, tt := range tests {
		t.Run(tt.bar, func(t *testing.T) {
			result := barDurationNs(tt.bar)
			if result != tt.expected {
				t.Errorf("barDurationNs(%s): expected %d, got %d", tt.bar, tt.expected, result)
			}
		})
	}
}

func TestBarDurationNsEdgeCases(t *testing.T) {
	// Test empty string falls back to default
	result := barDurationNs("")
	if result != 5*60*1_000_000_000 {
		t.Errorf("expected default 5min duration for empty string, got %d", result)
	}

	// Test variations - note function expects "1hour", "1day" not "1h", "1d"
	tests := []struct {
		bar      string
		expected int64
	}{
		{"1m", 5 * 60 * 1_000_000_000},                     // unrecognized, defaults to 5min
		{"5m", 5 * 60 * 1_000_000_000},                     // unrecognized, defaults to 5min
		{"1hour", 60 * 60 * 1_000_000_000},                 // recognized
		{"1day", 24 * 60 * 60 * 1_000_000_000},             // recognized
		{"2hour", 2 * 60 * 60 * 1_000_000_000},              // recognized
	}

	for _, tt := range tests {
		result := barDurationNs(tt.bar)
		if result != tt.expected {
			t.Errorf("barDurationNs(%s): expected %d, got %d", tt.bar, tt.expected, result)
		}
	}
}