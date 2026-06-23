package stocks

import (
	"time"
)

// Hour represents the time of day when earnings are reported.
type Hour string

const (
	// HourBMO = Before Market Open
	HourBMO Hour = "BMO"
	// HourAMC = After Market Close
	HourAMC Hour = "AMC"
	// HourDMH = During Market Hour
	HourDMH Hour = "DMH"
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
	Hour              Hour
	Status            Status
	EPS               *float64
	EPSEstimated      *float64
	Revenue           *int64
	RevenueEstimated  *int64
	Date              string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// ConvertHour converts an earnings calendar category to Hour enum.
func ConvertHour(category string) Hour {
	switch category {
	case "pre":
		return HourBMO
	case "after":
		return HourAMC
	case "during":
		return HourDMH
	default:
		return "" // notSupplied or unknown
	}
}
