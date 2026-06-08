package providers

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	ibapi "github.com/bishop-bot/ibapi-go"
	"github.com/bishop-bot/datajobs/internal/database"
	"github.com/bishop-bot/datajobs/internal/ingestion"
	"github.com/bishop-bot/datajobs/internal/logging"
	"github.com/bishop-bot/datajobs/internal/metadata"
	"github.com/bishop-bot/datajobs/internal/providers/ib"
	"github.com/bishop-bot/datajobs/internal/worker"
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

// HistoricalDataHandlerWithDB creates a handler with QuestDB and SQLite access.
// The ibProvider parameter accepts either *ib.Client or a mock implementation for testing.
func HistoricalDataHandlerWithDB(questDB *database.QuestDB, sqliteDB *database.DB, ibProvider ib.Provider) worker.JobFunc {
	return func(ctx context.Context, job worker.Job) (string, error) {
		return historicalDataHandlerImpl(ctx, job, questDB, sqliteDB, ibProvider)
	}
}

// historicalDataHandlerImpl is the main implementation.
func historicalDataHandlerImpl(ctx context.Context, job worker.Job, questDB *database.QuestDB, sqliteDB *database.DB, ibProvider ib.Provider) (string, error) {
	logger := logging.FromContext(ctx).With("job_id", job.ID)

	if ibProvider == nil {
		return "", fmt.Errorf("IB provider not available")
	}
	if questDB == nil {
		return "", fmt.Errorf("QuestDB not connected")
	}

	params := parseHistoricalParams(job.Metadata)
	instruments, err := getInstruments(ctx, job, params, sqliteDB)
	if err != nil {
		return "", fmt.Errorf("failed to get instruments: %w", err)
	}

	if len(instruments) == 0 {
		logger.Info("no instruments to fetch")
		return "no instruments to process", nil
	}

	logger.Info("fetching historical data",
		"count", len(instruments),
		"period", params.Period,
		"bar", params.Bar,
	)

	totalBars, failedSymbols := processInstruments(ctx, ibProvider, questDB, instruments, params)

	result := buildResultMessage(len(instruments), totalBars, failedSymbols)
	return result, nil
}

// processInstruments fetches and upserts OHLCV data for multiple instruments.
func processInstruments(ctx context.Context, ibProvider ib.Provider, questDB *database.QuestDB, instruments []instrument, params historicalParams) (totalBars int, failedSymbols []string) {
	logger := logging.FromContext(ctx)

	for _, instr := range instruments {
		bars, err := fetchOHLCV(ctx, ibProvider, instr, params)
		if err != nil {
			logger.Error("failed to fetch data", "symbol", instr.Symbol, "error", err)
			failedSymbols = append(failedSymbols, instr.Symbol)
			continue
		}

		if len(bars) == 0 {
			continue
		}

		if err := upsertOHLCVBatches(ctx, questDB, instr.Symbol, bars); err != nil {
			logger.Error("failed to upsert bars", "symbol", instr.Symbol, "error", err)
			failedSymbols = append(failedSymbols, instr.Symbol)
			continue
		}

		totalBars += len(bars)
		logger.Debug("upserted bars", "symbol", instr.Symbol, "count", len(bars))
	}

	return totalBars, failedSymbols
}

// buildResultMessage creates a summary message for the job result.
func buildResultMessage(instrumentCount, totalBars int, failedSymbols []string) string {
	parts := []string{
		fmt.Sprintf("processed %d instruments", instrumentCount),
		fmt.Sprintf("upserted %d bars", totalBars),
	}
	if len(failedSymbols) > 0 {
		parts = append(parts, fmt.Sprintf("failed: %s", strings.Join(failedSymbols, ", ")))
	}
	return strings.Join(parts, "; ")
}

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
		Period:     metadata.GetString(metadata_, "period", defaultPeriod),
		Bar:        metadata.GetString(metadata_, "bar", defaultBar),
		OutsideRth: metadata.GetBool(metadata_, "outsideRth", defaultOutsideRth),
		Instruments: metadata.GetStringSlice(metadata_, "instruments"),
	}
}

// instrument represents a tradeable instrument.
type instrument struct {
	Conid        string
	Symbol       string
	Exchange     string
	SecurityType string
}

// getInstruments determines which instruments to fetch based on job params.
func getInstruments(ctx context.Context, job worker.Job, params historicalParams, sqliteDB *database.DB) ([]instrument, error) {
	if len(params.Instruments) > 0 {
		return getInstrumentsByConids(ctx, params.Instruments, sqliteDB)
	}
	return getAllInstruments(ctx, sqliteDB)
}

// getInstrumentsByConids fetches instruments by their conids from SQLite.
// Returns an empty slice (not nil) if no conids are provided or none are found.
func getInstrumentsByConids(ctx context.Context, conids []string, sqliteDB *database.DB) ([]instrument, error) {
	if len(conids) == 0 || sqliteDB == nil {
		return []instrument{}, nil
	}

	instruments, err := queryInstruments(ctx, sqliteDB, buildInClauseQuery(conids), conidsToArgs(conids))
	if err != nil {
		return nil, fmt.Errorf("failed to query instruments by conids: %w", err)
	}
	return instruments, nil
}

// getAllInstruments fetches all instruments from the SQLite database.
func getAllInstruments(ctx context.Context, sqliteDB *database.DB) ([]instrument, error) {
	if sqliteDB == nil {
		return nil, fmt.Errorf("SQLite DB not available")
	}

	instruments, err := queryInstruments(ctx, sqliteDB, queryAllInstrumentsSQL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query all instruments: %w", err)
	}
	return instruments, nil
}

// queryInstruments executes a query and scans the results into instrument slice.
func queryInstruments(ctx context.Context, db *database.DB, query string, args []interface{}) ([]instrument, error) {
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanInstruments(rows)
}

// scanInstruments scans rows into instrument slice.
func scanInstruments(rows *sql.Rows) ([]instrument, error) {
	var instruments []instrument
	var scanErrors int

	for rows.Next() {
		var instr instrument
		var securityType sql.NullString
		if err := rows.Scan(&instr.Conid, &instr.Symbol, &instr.Exchange, &securityType); err != nil {
			scanErrors++
			continue
		}
		if securityType.Valid {
			instr.SecurityType = securityType.String
		}
		instruments = append(instruments, instr)
	}

	if scanErrors > 0 {
		logging.Warn("dropped rows due to scan errors", "count", scanErrors)
	}
	return instruments, rows.Err()
}

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

	bars := make([]database.OHLCVBar, 0, len(ibBars))
	for _, ibBar := range ibBars {
		ts := ibBar.T * 1_000_000
		bars = append(bars, database.OHLCVBar{
			Symbol:    symbol,
			Publisher: defaultPublisher,
			Ts:        ts,
			TsEnd:     ts + ingestion.BarDurationNs(params.Bar),
			Open:      ibBar.O,
			High:      ibBar.H,
			Low:       ibBar.L,
			Close:     ibBar.C,
			Volume:    int64(ibBar.V),
		})
	}
	return bars
}

// upsertOHLCVBatches upserts bars to QuestDB in batches.
func upsertOHLCVBatches(ctx context.Context, questDB *database.QuestDB, symbol string, bars []database.OHLCVBar) error {
	for i := 0; i < len(bars); i += upsertBatchSize {
		end := min(i+upsertBatchSize, len(bars))
		batch := bars[i:end]

		_, err := questDB.UpsertOHLCVBars(ctx, batch)
		if err != nil {
			return fmt.Errorf("failed to upsert batch for %s (rows %d-%d): %w", symbol, i, end, err)
		}
	}
	return nil
}

// Query builders

const (
	queryAllInstrumentsSQL = `SELECT id, symbol, exchange, security_type FROM instruments ORDER BY symbol`
)

// buildInClauseQuery builds a SELECT query with IN clause for conids.
func buildInClauseQuery(conids []string) string {
	placeholders := make([]string, len(conids))
	for i := range conids {
		placeholders[i] = "?"
	}
	return fmt.Sprintf(
		"SELECT id, symbol, exchange, security_type FROM instruments WHERE id IN (%s)",
		strings.Join(placeholders, ", "),
	)
}

// conidsToArgs converts conids slice to interface slice for query args.
func conidsToArgs(conids []string) []interface{} {
	args := make([]interface{}, len(conids))
	for i, c := range conids {
		args[i] = c
	}
	return args
}