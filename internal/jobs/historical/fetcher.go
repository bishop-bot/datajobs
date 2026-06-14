package historical

import (
	"context"
	"fmt"

	"github.com/bishop-bot/datajobs/internal/database"
	"github.com/bishop-bot/datajobs/internal/logging"
	"github.com/bishop-bot/datajobs/internal/providers/ib"
)

// fetchOHLCV fetches OHLCV data for a single instrument.
func fetchOHLCV(ctx context.Context, ibProvider ib.Provider, instr instrument, params historicalParams) ([]database.OHLCVBar, error) {
	req := buildHistoricalDataRequest(instr, params)

	logging.Info("fetching OHLCV from IB",
		"symbol", instr.Symbol,
		"conid", instr.Conid,
		"exchange", instr.Exchange,
		"period", params.Period,
		"bar", params.Bar,
		"outsideRth", req.OutsideRth,
		"startTime", req.StartTime,
		"source", req.Source,
		// Full request details for debugging
		"request_conid", req.Conid,
		"request_exchange", req.Exchange,
		"request_period", req.Period,
		"request_bar", req.Bar,
	)

	resp, err := ibProvider.HistoricalData(ctx, req)
	if err != nil {
		logging.Error("IB HistoricalData failed",
			"symbol", instr.Symbol,
			"conid", req.Conid,
			"exchange", req.Exchange,
			"error", err.Error(),
		)
		return nil, fmt.Errorf("historical data request failed for %s: %w", instr.Symbol, err)
	}

	if resp == nil {
		logging.Warn("IB response is nil", "symbol", instr.Symbol, "conid", req.Conid)
		return nil, nil
	}

	if len(resp.Data) == 0 {
		logging.Warn("IB returned no data",
			"symbol", instr.Symbol,
			"conid", req.Conid,
			"exchange", req.Exchange,
			"period", req.Period,
			"bar", req.Bar,
			"response_symbol", resp.Symbol,
			"response_serverID", resp.ServerID,
			"response_text", resp.Text,
		)
		return nil, nil
	}

	bars := convertIBBarsToOHLCV(instr.Symbol, resp.Data, params)
	logging.Info("converted bars from IB",
		"symbol", instr.Symbol,
		"bars_count", len(bars),
	)

	return bars, nil
}