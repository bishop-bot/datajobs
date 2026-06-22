package historical

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bishop-bot/datajobs/internal/database"
	"github.com/bishop-bot/datajobs/internal/logging"
	"github.com/bishop-bot/datajobs/internal/providers/ib"
)

// maxPages is the maximum number of paginated requests per instrument to prevent infinite loops.
const maxPages = 100

// fetchOHLCV fetches OHLCV data for a single instrument with pagination.
func fetchOHLCV(ctx context.Context, ibProvider ib.Provider, instr instrument, params historicalParams) ([]database.OHLCVBar, error) {
	// Calculate the beginning of the requested duration (fromTime)
	fromTime, err := calculateFromTime(params.Period)
	if err != nil {
		return nil, fmt.Errorf("invalid period %q: %w", params.Period, err)
	}

	logging.Info("starting paginated fetch",
		"symbol", instr.Symbol,
		"period", params.Period,
		"fromTime", fromTime,
	)

	// Initial request: startTime is "now" (empty string triggers IB to use current time)
	var allBars []database.OHLCVBar
	startTime := ""
	pageCount := 0

	for {
		pageCount++
		if pageCount > maxPages {
			logging.Warn("max pages reached, stopping pagination",
				"symbol", instr.Symbol,
				"pages", pageCount,
				"totalBars", len(allBars),
			)
			break
		}

		req := buildHistoricalDataRequestWithStartTime(instr, params, startTime)

		logging.Info("fetching OHLCV page",
			"symbol", instr.Symbol,
			"page", pageCount,
			"startTime", startTime,
			"request", req,
		)

		resp, err := ibProvider.HistoricalData(ctx, req)
		if err != nil {
			logging.Error("IB HistoricalData failed",
				"symbol", instr.Symbol,
				"page", pageCount,
				"request", req,
				"error", err.Error(),
			)
			return nil, fmt.Errorf("historical data request failed for %s: %w", instr.Symbol, err)
		}

		if resp == nil {
			logging.Warn("IB response is nil", "symbol", instr.Symbol, "page", pageCount)
			break
		}

		if len(resp.Data) == 0 {
			logging.Info("IB returned empty data, stopping pagination",
				"symbol", instr.Symbol,
				"page", pageCount,
			)
			break
		}

		// Convert this page's data
		pageBars := convertIBBarsToOHLCV(instr.Symbol, resp.Data, params)

		if len(pageBars) == 0 {
			break
		}

		// Data is returned in ascending order (oldest → newest)
		oldestTs := pageBars[0].Ts
		newestTs := pageBars[len(pageBars)-1].Ts

		// Check if we've reached the fromTime boundary
		// Filter to only include bars >= fromTime
		filteredBars := filterBarsFromTime(pageBars, fromTime)
		if len(filteredBars) == 0 {
			// All bars are older than fromTime, stop
			logging.Info("all bars older than fromTime, stopping pagination",
				"symbol", instr.Symbol,
				"page", pageCount,
				"newestTs", newestTs,
				"fromTime", fromTime,
			)
			break
		}

		// Append filtered bars to collection
		allBars = append(allBars, filteredBars...)

		// If filteredBars < pageBars, some were filtered out (boundary case)
		if len(filteredBars) < len(pageBars) {
			logging.Info("reached fromTime boundary, stopping pagination",
				"symbol", instr.Symbol,
				"page", pageCount,
				"newestTs", newestTs,
				"fromTime", fromTime,
				"barsFiltered", len(pageBars)-len(filteredBars),
			)
			break
		}

		// For next request, use the oldest timestamp to get data BEFORE this page
		startTime = formatTimestampAsIB(oldestTs)

		logging.Debug("paginating to next page",
			"symbol", instr.Symbol,
			"page", pageCount,
			"barsOnPage", len(pageBars),
			"totalBars", len(allBars),
			"nextStartTime", startTime,
		)
	}

	logging.Info("completed paginated fetch",
		"symbol", instr.Symbol,
		"pages", pageCount,
		"totalBars", len(allBars),
	)

	return allBars, nil
}

// calculateFromTime calculates the beginning datetime based on the period.
// Valid period formats: {1-30}min, {1-8}h, {1-1000}d, {1-792}w, {1-182}m, {1-15}y.
// Examples: "5min", "2h", "30d", "52w", "12m", "5y".
func calculateFromTime(period string) (int64, error) {
	now := time.Now().UTC()

	duration, err := parsePeriodToDuration(period)
	if err != nil {
		return 0, err
	}

	fromTime := now.Add(-duration)
	return fromTime.UnixNano(), nil
}

// parsePeriodToDuration converts a period string to a time.Duration.
// Supported formats: {n}min, {n}h, {n}d, {n}w, {n}m (months), {n}y (years).
func parsePeriodToDuration(period string) (time.Duration, error) {
	if period == "" {
		return 0, fmt.Errorf("period cannot be empty")
	}

	// Normalize: handle both "min" and "mins"
	period = strings.ToLower(period)
	period = strings.TrimSuffix(period, "s")

	var value int
	var unit string

	// Parse the numeric part and unit
	for i, r := range period {
		if r >= '0' && r <= '9' {
			continue
		}
		unit = period[i:]
		numStr := period[:i]
		value, _ = strconv.Atoi(numStr)
		break
	}

	if unit == "" || value <= 0 {
		return 0, fmt.Errorf("invalid period format: %q", period)
	}

	// Validate ranges and convert to duration
	hoursPerDay := 24.0
	daysPerWeek := 7.0
	daysPerMonth := 30.0 // approximate
	daysPerYear := 365.0 // approximate

	var totalHours float64

	switch unit {
	case "min":
		if value < 1 || value > 30 {
			return 0, fmt.Errorf("period value %d out of range for min (1-30)", value)
		}
		totalHours = float64(value) / 60.0
	case "h":
		if value < 1 || value > 8 {
			return 0, fmt.Errorf("period value %d out of range for h (1-8)", value)
		}
		totalHours = float64(value)
	case "d":
		if value < 1 || value > 1000 {
			return 0, fmt.Errorf("period value %d out of range for d (1-1000)", value)
		}
		totalHours = float64(value) * hoursPerDay
	case "w":
		if value < 1 || value > 792 {
			return 0, fmt.Errorf("period value %d out of range for w (1-792)", value)
		}
		totalHours = float64(value) * daysPerWeek * hoursPerDay
	case "m":
		if value < 1 || value > 182 {
			return 0, fmt.Errorf("period value %d out of range for m (1-182)", value)
		}
		totalHours = float64(value) * daysPerMonth * hoursPerDay
	case "y":
		if value < 1 || value > 15 {
			return 0, fmt.Errorf("period value %d out of range for y (1-15)", value)
		}
		totalHours = float64(value) * daysPerYear * hoursPerDay
	default:
		return 0, fmt.Errorf("unsupported period unit: %q", unit)
	}

	return time.Duration(totalHours * float64(time.Hour)), nil
}

// formatTimestampAsIB formats a Unix timestamp (nanoseconds) as IB's YYYYMMDD-HH:mm:ss.
func formatTimestampAsIB(ts int64) string {
	t := time.Unix(0, ts).UTC()
	return t.Format("20060102-15:04:05")
}

// buildHistoricalDataRequestWithStartTime creates a request with an optional startTime.
func buildHistoricalDataRequestWithStartTime(instr instrument, params historicalParams, startTime string) ib.HistoricalDataRequest {
	exchange := instr.Exchange
	if exchange == "" {
		exchange = "SMART"
	}

	return ib.HistoricalDataRequest{
		Conid:      instr.Conid,
		Exchange:   exchange,
		Period:     params.Period,
		Bar:        params.Bar,
		StartTime:  startTime,
		OutsideRth: params.OutsideRth,
	}
}

// filterBarsFromTime filters bars to only include those with timestamp >= fromTime.
func filterBarsFromTime(bars []database.OHLCVBar, fromTime int64) []database.OHLCVBar {
	for i, bar := range bars {
		if bar.Ts >= fromTime {
			return bars[i:]
		}
	}
	return nil
}

