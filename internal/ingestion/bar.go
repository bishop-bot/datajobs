package ingestion

// ValidBarSizes contains all valid IB bar size values.
// These correspond to the bar_size field in OHLCVBar.
var ValidBarSizes = []string{
	"1min", "2min", "3min", "5min", "10min", "15min", "30min",
	"1h", "2h", "3h", "4h", "8h",
	"1d", "1w", "1m",
}

// IsValidBarSize returns true if the bar size is a valid IB bar size.
func IsValidBarSize(bar string) bool {
	for _, valid := range ValidBarSizes {
		if bar == valid {
			return true
		}
	}
	return false
}

// BarDurationNs returns the duration in nanoseconds for a bar size.
// IB API bar sizes: 1min, 2min, 3min, 5min, 10min, 15min, 30min,
// 1h, 2h, 3h, 4h, 8h, 1d, 1w, 1m
func BarDurationNs(bar string) int64 {
	switch bar {
	case "1min":
		return 1 * 60 * 1_000_000_000
	case "2min":
		return 2 * 60 * 1_000_000_000
	case "3min":
		return 3 * 60 * 1_000_000_000
	case "5min":
		return 5 * 60 * 1_000_000_000
	case "10min":
		return 10 * 60 * 1_000_000_000
	case "15min":
		return 15 * 60 * 1_000_000_000
	case "30min":
		return 30 * 60 * 1_000_000_000
	case "1h":
		return 1 * 60 * 60 * 1_000_000_000
	case "2h":
		return 2 * 60 * 60 * 1_000_000_000
	case "3h":
		return 3 * 60 * 60 * 1_000_000_000
	case "4h":
		return 4 * 60 * 60 * 1_000_000_000
	case "8h":
		return 8 * 60 * 60 * 1_000_000_000
	case "1d":
		return 24 * 60 * 60 * 1_000_000_000
	case "1w":
		return 7 * 24 * 60 * 60 * 1_000_000_000
	case "1m":
		return 30 * 24 * 60 * 60 * 1_000_000_000
	default:
		return 5 * 60 * 1_000_000_000 // Default to 5min
	}
}
