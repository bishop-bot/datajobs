package earnings

import (
	"testing"
)

func TestCountryCode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Standard country names
		{"United States full name", "United States", "US"},
		{"United Kingdom full name", "United Kingdom", "GB"},
		{"Japan", "Japan", "JP"},
		{"Germany", "Germany", "DE"},
		{"France", "France", "FR"},
		{"China", "China", "CN"},
		{"Canada", "Canada", "CA"},
		{"Australia", "Australia", "AU"},
		{"Brazil", "Brazil", "BR"},
		{"India", "India", "IN"},
		{"Mexico", "Mexico", "MX"},
		{"South Korea", "South Korea", "KR"},
		{"Switzerland", "Switzerland", "CH"},
		{"Hong Kong", "Hong Kong", "HK"},
		{"Singapore", "Singapore", "SG"},
		{"Taiwan", "Taiwan", "TW"},

		// Variations and abbreviations
		{"USA uppercase", "USA", "US"},
		{"US abbreviation", "US", "US"},
		{"UK abbreviation", "UK", "GB"},
		{"America", "America", "US"},
		{"Britain", "Britain", "GB"},
		{"Great Britain", "Great Britain", "GB"},

		// Euro Area (special code)
		{"Euro Zone", "Euro Zone", "EU"},
		{"Eurozone", "Eurozone", "EU"},
		{"Euro Area", "Euro Area", "EU"},
		{"EU abbreviation", "EU", "EU"},

		// Empty and invalid
		{"Empty string", "", ""},
		{"Unknown country", "Unknown Country XYZ", ""},
		{"Random text", "asdfghjkl", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CountryCode(tt.input)
			if result != tt.expected {
				t.Errorf("CountryCode(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCountryCodeCaseInsensitive(t *testing.T) {
	// Test that lookup is case-insensitive
	tests := []string{
		"united states",
		"UNITED STATES",
		"United States",
		"uNiTeD sTaTeS",
		"UNITED KINGDOM",
		"united kingdom",
		"Japan",
		"japan",
		"JAPAN",
	}

	for _, input := range tests {
		result := CountryCode(input)
		// All should return either US, GB, or JP depending on input
		if input == "united states" || input == "UNITED STATES" || input == "United States" || input == "uNiTeD sTaTeS" {
			if result != "US" {
				t.Errorf("CountryCode(%q) = %q, want US", input, result)
			}
		}
	}
}

func TestCleanCountryName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"United States (core)", "United States"},
		{"Euro Area (monthly)", "Euro Area"},
		{"Germany (yoy)", "Germany"},
		{"United States", "United States"}, // No change needed
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := cleanCountryName(tt.input)
			if result != tt.expected {
				t.Errorf("cleanCountryName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMapCountryName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"usa", "United States"},
		{"us", "United States"},
		{"uk", "United Kingdom"},
		{"hk", "Hong Kong"},
		{"swiss", "Switzerland"},
		{"holland", "Netherlands"},
		{"unknownxyz", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := mapCountryName(tt.input)
			if result != tt.expected {
				t.Errorf("mapCountryName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
