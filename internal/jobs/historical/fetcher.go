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
		"request", req,
	)

	resp, err := ibProvider.HistoricalData(ctx, req)
	if err != nil {
		logging.Error("IB HistoricalData failed",
			"symbol", instr.Symbol,
			"request", req,
			"error", err.Error(),
		)
		return nil, fmt.Errorf("historical data request failed for %s: %w", instr.Symbol, err)
	}

	if resp == nil {
		logging.Warn("IB response is nil", "symbol", instr.Symbol, "request", req)
		return nil, nil
	}

	if len(resp.Data) == 0 {
		logging.Warn("IB returned no data",
			"symbol", instr.Symbol,
			"request", req,
			"response", resp,
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