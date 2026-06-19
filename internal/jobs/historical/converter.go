package historical

import (
	ibapi "github.com/bishop-bot/ibapi-go"
	"github.com/bishop-bot/datajobs/internal/database"
	"github.com/bishop-bot/datajobs/internal/ingestion"
	"github.com/bishop-bot/datajobs/internal/providers/ib"
)

// instrument represents a tradeable instrument.
type instrument struct {
	Conid        string
	Symbol       string
	Exchange     string
	SecurityType string
}

// buildHistoricalDataRequest creates a request from instrument and params.
func buildHistoricalDataRequest(instr instrument, params historicalParams) ib.HistoricalDataRequest {
	exchange := instr.Exchange
	if exchange == "" {
		exchange = "SMART"
	}

	return ib.HistoricalDataRequest{
		Conid:      instr.Conid,
		Exchange:   exchange,
		Period:     params.Period,
		Bar:        params.Bar,
		OutsideRth: params.OutsideRth,
	}
}

// convertIBBarsToOHLCV converts IB API response to database OHLCV bars.
func convertIBBarsToOHLCV(symbol string, ibBars []ibapi.HistoricalDataBar, params historicalParams) []database.OHLCVBar {
	if len(ibBars) == 0 {
		return nil
	}

	barSize := params.Bar
	bars := make([]database.OHLCVBar, 0, len(ibBars))
	for _, ibBar := range ibBars {
		ts := ibBar.T * 1_000_000
		bars = append(bars, database.OHLCVBar{
			Symbol:    symbol,
			Publisher: defaultPublisher,
			BarSize:   barSize,
			Ts:        ts,
			TsEnd:     ts + ingestion.BarDurationNs(barSize),
			Open:      ibBar.O,
			High:      ibBar.H,
			Low:       ibBar.L,
			Close:     ibBar.C,
			Volume:    int64(ibBar.V),
		})
	}
	return bars
}