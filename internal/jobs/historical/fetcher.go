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

	resp, err := ibProvider.HistoricalData(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("historical data request failed for %s: %w", instr.Symbol, err)
	}

	if resp == nil || len(resp.Data) == 0 {
		logging.Debug("no data returned", "symbol", instr.Symbol)
		return nil, nil
	}

	return convertIBBarsToOHLCV(instr.Symbol, resp.Data, params), nil
}