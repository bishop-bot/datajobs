package stocks

import (
	"time"
)

// Time represents the time of day when earnings are reported.
type Time string

const (
	// TimeBMO = Before Market Open
	TimeBMO Time = "BMO"
	// TimeAMC = After Market Close
	TimeAMC Time = "AMC"
	// TimeDMH = During Market Hour
	TimeDMH Time = "DMH"
)

// Status represents the reporting status of earnings.
type Status string

const (
	// StatusActual indicates actual earnings have been reported
	StatusActual Status = "actual"
	// StatusEstimate indicates earnings are estimates
	StatusEstimate Status = "estimate"
	// StatusConfirmed indicates earnings date is confirmed
	StatusConfirmed Status = "confirmed"
)

// StockEarnings represents a stock's earnings data.
type StockEarnings struct {
	ID                int64
	Symbol            string
	Name              string
	MIC               string
	ISIN              string
	Type              string
	Time              Time
	Status            Status
	EPS               *float64
	EPSEstimated      *float64
	Revenue           *int64
	RevenueEstimated  *int64
	Date              string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// ConvertTime converts an earnings calendar category to Time enum.
func ConvertTime(category string) Time {
	switch category {
	case "pre":
		return TimeBMO
	case "after":
		return TimeAMC
	case "during":
		return TimeDMH
	default:
		return "" // notSupplied or unknown
	}
}
