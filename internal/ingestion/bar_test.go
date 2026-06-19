package ingestion

import (
	"testing"
)

func TestBarDurationNs(t *testing.T) {
	tests := []struct {
		bar      string
		expected int64
	}{
		{"1min", 1 * 60 * 1_000_000_000},
		{"2min", 2 * 60 * 1_000_000_000},
		{"3min", 3 * 60 * 1_000_000_000},
		{"5min", 5 * 60 * 1_000_000_000},
		{"10min", 10 * 60 * 1_000_000_000},
		{"15min", 15 * 60 * 1_000_000_000},
		{"30min", 30 * 60 * 1_000_000_000},
		{"1h", 1 * 60 * 60 * 1_000_000_000},
		{"2h", 2 * 60 * 60 * 1_000_000_000},
		{"3h", 3 * 60 * 60 * 1_000_000_000},
		{"4h", 4 * 60 * 60 * 1_000_000_000},
		{"8h", 8 * 60 * 60 * 1_000_000_000},
		{"1d", 24 * 60 * 60 * 1_000_000_000},
		{"1w", 7 * 24 * 60 * 60 * 1_000_000_000},
		{"1m", 30 * 24 * 60 * 60 * 1_000_000_000},
	}

	for _, tt := range tests {
		t.Run(tt.bar, func(t *testing.T) {
			result := BarDurationNs(tt.bar)
			if result != tt.expected {
				t.Errorf("BarDurationNs(%s): expected %d, got %d", tt.bar, tt.expected, result)
			}
		})
	}
}

func TestBarDurationNsDefault(t *testing.T) {
	// Test that unknown bar sizes default to 5min
	tests := []struct {
		bar      string
		expected int64
	}{
		{"", 5 * 60 * 1_000_000_000},
		{"unknown", 5 * 60 * 1_000_000_000},
		{"1sec", 5 * 60 * 1_000_000_000},
	}

	for _, tt := range tests {
		t.Run(tt.bar, func(t *testing.T) {
			result := BarDurationNs(tt.bar)
			if result != tt.expected {
				t.Errorf("BarDurationNs(%s): expected %d, got %d", tt.bar, tt.expected, result)
			}
		})
	}
}

func TestIsValidBarSize(t *testing.T) {
	validSizes := []string{
		"1min", "2min", "3min", "5min", "10min", "15min", "30min",
		"1h", "2h", "3h", "4h", "8h",
		"1d", "1w", "1m",
	}

	for _, size := range validSizes {
		if !IsValidBarSize(size) {
			t.Errorf("IsValidBarSize(%s): expected true, got false", size)
		}
	}

	invalidSizes := []string{"", "1s", "5s", "unknown", "1day", "1week", "1month"}
	for _, size := range invalidSizes {
		if IsValidBarSize(size) {
			t.Errorf("IsValidBarSize(%s): expected false, got true", size)
		}
	}
}
