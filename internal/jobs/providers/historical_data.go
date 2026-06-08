package providers

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/bishop-bot/datajobs/internal/database"
	"github.com/bishop-bot/datajobs/internal/ingestion"
	"github.com/bishop-bot/datajobs/internal/logging"
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
// The ibProvider parameter accepts either *IBClient or a mock implementation for testing.
func HistoricalDataHandlerWithDB(questDB *database.QuestDB, sqliteDB *database.DB, ibProvider ib.Provider) worker.JobFunc {
	return func(ctx context.Context, job worker.Job) (string, error) {
		return historicalDataHandlerImpl(ctx, job, questDB, sqliteDB, ibProvider)
	}
}

// historicalDataHandlerImpl is the main implementation.
func historicalDataHandlerImpl(ctx context.Context, job worker.Job, questDB *database.QuestDB, sqliteDB *database.DB, ibProvider ib.Provider) (string, error) {
	logger := logging.FromContext(ctx).With("job_id", job.ID)

	// Validate IB provider
	if ibProvider == nil {
		return "", fmt.Errorf("IB provider not available")
	}

	// Validate QuestDB connection
	if questDB == nil {
		return "", fmt.Errorf("QuestDB not connected")
	}

	// Get parameters from job metadata
	params := parseHistoricalParams(job.Metadata)

	// Get instruments to fetch
	instruments, err := getInstrumentsForJob(ctx, job, params, sqliteDB)
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

	// Fetch and upsert for each instrument
	var totalBars int
	var failedSymbols []string

	for _, instr := range instruments {
		bars, err := fetchInstrumentOHLCV(ctx, ibProvider, instr, params)
		if err != nil {
			logger.Error("failed to fetch data", "symbol", instr.Symbol, "error", err)
			failedSymbols = append(failedSymbols, instr.Symbol)
			continue
		}

		if len(bars) == 0 {
			continue
		}

		// Upsert bars to QuestDB in batches
		if err := upsertOHLCVBatches(ctx, questDB, bars); err != nil {
			logger.Error("failed to upsert bars", "symbol", instr.Symbol, "error", err)
			failedSymbols = append(failedSymbols, instr.Symbol)
			continue
		}

		totalBars += len(bars)
		logger.Debug("upserted bars", "symbol", instr.Symbol, "count", len(bars))
	}

	// Build result message
	resultParts := []string{
		fmt.Sprintf("processed %d instruments", len(instruments)),
		fmt.Sprintf("upserted %d bars", totalBars),
	}
	if len(failedSymbols) > 0 {
		resultParts = append(resultParts, fmt.Sprintf("failed: %s", strings.Join(failedSymbols, ", ")))
	}

	return strings.Join(resultParts, "; "), nil
}

// historicalParams holds parameters for the historical data job.
type historicalParams struct {
	Period      string
	Bar         string
	OutsideRth  bool
	Instruments []string // Optional list of specific conids to fetch
}

// parseHistoricalParams extracts parameters from job metadata.
func parseHistoricalParams(metadata map[string]interface{}) historicalParams {
	return historicalParams{
		Period:     getStr(metadata, "period", defaultPeriod),
		Bar:        getStr(metadata, "bar", defaultBar),
		OutsideRth: getBool(metadata, "outsideRth", defaultOutsideRth),
		Instruments: getStrSlice(metadata, "instruments"),
	}
}

// instrument represents a tradeable instrument.
type instrument struct {
	Conid        string
	Symbol       string
	Exchange     string
	SecurityType string
}

// getInstrumentsForJob determines which instruments to fetch.
func getInstrumentsForJob(ctx context.Context, job worker.Job, params historicalParams, sqliteDB *database.DB) ([]instrument, error) {
	// If instruments are specified in job metadata, use those
	if len(params.Instruments) > 0 {
		return getInstrumentsByConids(ctx, params.Instruments, sqliteDB)
	}

	// Otherwise, fetch all instruments from SQLite database
	return getAllInstruments(ctx, sqliteDB)
}

// getInstrumentsByConids fetches instruments by their conids from SQLite.
func getInstrumentsByConids(ctx context.Context, conids []string, sqliteDB *database.DB) ([]instrument, error) {
	if len(conids) == 0 || sqliteDB == nil {
		return nil, nil
	}

	placeholders := make([]string, len(conids))
	args := make([]interface{}, len(conids))
	for i, c := range conids {
		placeholders[i] = "?"
		args[i] = c
	}

	query := fmt.Sprintf(
		"SELECT id, symbol, exchange, security_type FROM instruments WHERE id IN (%s)",
		strings.Join(placeholders, ", "),
	)

	rows, err := sqliteDB.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query instruments: %w", err)
	}
	defer rows.Close()

	return scanInstruments(rows)
}

// getAllInstruments fetches all instruments from the SQLite database.
func getAllInstruments(ctx context.Context, sqliteDB *database.DB) ([]instrument, error) {
	if sqliteDB == nil {
		return nil, fmt.Errorf("SQLite DB not available")
	}

	query := "SELECT id, symbol, exchange, security_type FROM instruments ORDER BY symbol"
	rows, err := sqliteDB.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query instruments: %w", err)
	}
	defer rows.Close()

	return scanInstruments(rows)
}

// scanInstruments scans rows into instrument slice.
func scanInstruments(rows *sql.Rows) ([]instrument, error) {
	var instruments []instrument
	var scanErrors int
	for rows.Next() {
		var i instrument
		var securityType sql.NullString
		if err := rows.Scan(&i.Conid, &i.Symbol, &i.Exchange, &securityType); err != nil {
			scanErrors++
			logging.Warn("failed to scan instrument row", "error", err)
			continue
		}
		if securityType.Valid {
			i.SecurityType = securityType.String
		}
		instruments = append(instruments, i)
	}
	if scanErrors > 0 {
		logging.Warn("dropped rows due to scan errors", "count", scanErrors)
	}
	return instruments, rows.Err()
}

// fetchInstrumentOHLCV fetches OHLCV data for a single instrument.
func fetchInstrumentOHLCV(ctx context.Context, ibProvider ib.Provider, instr instrument, params historicalParams) ([]database.OHLCVBar, error) {
	logger := logging.FromContext(ctx)

	// Determine exchange - default to SMART
	exchange := instr.Exchange
	if exchange == "" {
		exchange = "SMART"
	}

	// Build request
	req := ib.HistoricalDataRequest{
		Conid:      instr.Conid,
		Exchange:   exchange,
		Period:     params.Period,
		Bar:        params.Bar,
		OutsideRth: params.OutsideRth,
	}

	// Fetch data
	resp, err := ibProvider.HistoricalData(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("historical data request failed: %w", err)
	}

	if resp == nil || len(resp.Data) == 0 {
		logger.Debug("no data returned", "symbol", instr.Symbol)
		return nil, nil
	}

	// Convert to OHLCV bars
	bars := make([]database.OHLCVBar, 0, len(resp.Data))
	for _, ibBar := range resp.Data {
		// Convert timestamp from milliseconds to nanoseconds
		ts := ibBar.T * 1_000_000
		bars = append(bars, database.OHLCVBar{
			Symbol:    instr.Symbol,
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

	return bars, nil
}

// upsertOHLCVBatches upserts bars to QuestDB in batches.
func upsertOHLCVBatches(ctx context.Context, questDB *database.QuestDB, bars []database.OHLCVBar) error {
	for i := 0; i < len(bars); i += upsertBatchSize {
		end := i + upsertBatchSize
		if end > len(bars) {
			end = len(bars)
		}

		result, err := questDB.UpsertOHLCVBars(ctx, bars[i:end])
		if err != nil {
			return err
		}

		logging.Debug("batch upsert complete",
			"start", i,
			"end", end,
			"rows_affected", result.RowsAffected,
		)
	}
	return nil
}

// getStr extracts a string from metadata with default.
func getStr(m map[string]interface{}, key, defaultVal string) string {
	if v, ok := m[key].(string); ok && v != "" {
		return v
	}
	return defaultVal
}

// getBool extracts a bool from metadata with default.
func getBool(m map[string]interface{}, key string, defaultVal bool) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return defaultVal
}

// getStrSlice extracts a string slice from metadata.
func getStrSlice(m map[string]interface{}, key string) []string {
	if v, ok := m[key].([]string); ok {
		return v
	}
	// Handle []any from YAML parsing
	if v, ok := m[key].([]any); ok {
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}