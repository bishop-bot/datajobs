package ingestion

// BarDurationNs returns the duration in nanoseconds for a bar size.
// IB API bar sizes: 1s, 5s, 10s, 15s, 30s, 1m, 2m, 3m, 5m, 10m, 15m, 20m, 30m, 1h, 2h, 3h, 4h, 8h, 1d, 1w
func BarDurationNs(bar string) int64 {
	switch bar {
	case "1s":
		return 1 * 1_000_000_000
	case "5s":
		return 5 * 1_000_000_000
	case "10s":
		return 10 * 1_000_000_000
	case "15s":
		return 15 * 1_000_000_000
	case "30s":
		return 30 * 1_000_000_000
	case "1m":
		return 60 * 1_000_000_000
	case "2m":
		return 2 * 60 * 1_000_000_000
	case "3m":
		return 3 * 60 * 1_000_000_000
	case "5m":
		return 5 * 60 * 1_000_000_000
	case "10m":
		return 10 * 60 * 1_000_000_000
	case "15m":
		return 15 * 60 * 1_000_000_000
	case "20m":
		return 20 * 60 * 1_000_000_000
	case "30m":
		return 30 * 60 * 1_000_000_000
	case "1h":
		return 60 * 60 * 1_000_000_000
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
	default:
		return 5 * 60 * 1_000_000_000 // Default to 5 minutes
	}
}