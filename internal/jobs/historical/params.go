package historical

import (
	"github.com/bishop-bot/datajobs/internal/metadata"
)

const (
	// Default period for historical data requests (5 years)
	defaultPeriod = "5y"
	// Default bar size (1 day) - IB API uses "1d" not "1day"
	defaultBar = "1d"
	// Default outside regular trading hours
	defaultOutsideRth = false
	// Default publisher identifier
	defaultPublisher = "IB"
	// Batch size for QuestDB upserts
	upsertBatchSize = 1000
)

// historicalParams holds parameters for the historical data job.
type historicalParams struct {
	Period      string
	Bar         string
	OutsideRth  bool
	Instruments []string
}

// parseHistoricalParams extracts parameters from job metadata.
func parseHistoricalParams(metadata_ map[string]interface{}) historicalParams {
	return historicalParams{
		Period:      metadata.GetString(metadata_, "period", defaultPeriod),
		Bar:         metadata.GetString(metadata_, "bar", defaultBar),
		OutsideRth:  metadata.GetBool(metadata_, "outsideRth", defaultOutsideRth),
		Instruments: metadata.GetStringSlice(metadata_, "instruments"),
	}
}