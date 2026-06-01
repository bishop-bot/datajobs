package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"github.com/bishop-bot/datajobs/internal/database"
	"github.com/bishop-bot/datajobs/internal/logging"
	"github.com/bishop-bot/datajobs/internal/providers"
	"github.com/bishop-bot/datajobs/internal/worker"
)

// Instrument represents an instrument record from the database.
type Instrument struct {
	ID              string `json:"id"`
	Symbol          string `json:"symbol"`
	Name            string `json:"name"`
	Publisher       string `json:"publisher"`
	InstrumentClass string `json:"instrument_class"`
	Currency        string `json:"currency"`
	Exchange        string `json:"exchange"`
	Asset           string `json:"asset"`
	SecurityType    string `json:"security_type"`
	Group           string `json:"group"`
}

// MarketDataHandler handles market data endpoints.
type MarketDataHandler struct {
	pool     *worker.Pool
	ibClient *providers.IBClient
	sqliteDB *database.DB
	questdb  *database.QuestDB
}

// NewMarketDataHandler creates a new market data handler.
func NewMarketDataHandler(pool *worker.Pool, ibClient *providers.IBClient, sqliteDB *database.DB, questdb *database.QuestDB) *MarketDataHandler {
	return &MarketDataHandler{
		pool:     pool,
		ibClient: ibClient,
		sqliteDB: sqliteDB,
		questdb:  questdb,
	}
}

// DownloadHistoricalData downloads historical data from IB and stores OHLCV bars in QuestDB.
// Parameters:
//   - conid: Contract ID (e.g., "265598" for AAPL). If provided, symbol is ignored.
//   - symbol: Instrument symbol (e.g., "AAPL"). Used to look up conid if conid not provided.
//   - bar: Bar size (e.g., "1min", "5mins", "1hour", "1day")
//   - period: Time period (e.g., "1d", "1w", "1M", "1y")
//   - startTime: Start time in YYYYMMDD-HH:MM:SS format (optional, mutual exclusive with period)
//   - outsideRth: Include data outside regular trading hours (default: false)
func (h *MarketDataHandler) DownloadHistoricalData(
	ctx context.Context,
	conid, symbol, exchange, bar, period, startTime string,
	outsideRth bool,
) (*database.OHLCVUpsertResult, error) {
	logger := logging.FromContext(ctx)

	// Resolve conid from symbol if not provided
	if conid == "" && symbol == "" {
		return nil, fmt.Errorf("conid or symbol is required")
	}

	if h.ibClient == nil {
		return nil, fmt.Errorf("IB client not available")
	}

	if h.questdb == nil {
		return nil, fmt.Errorf("QuestDB not available")
	}

	// Look up conid from symbol if needed
	if conid == "" && symbol != "" {
		var err error
		conid, exchange, err = h.lookupConidAndExchange(ctx, symbol, exchange)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve conid for symbol %s: %w", symbol, err)
		}
		if conid == "" {
			return nil, fmt.Errorf("no conid found for symbol %s", symbol)
		}
	}

	logger.Info("fetching historical data",
		"conid", conid,
		"symbol", symbol,
		"exchange", exchange,
		"bar", bar,
		"period", period,
		"startTime", startTime,
		"outsideRth", outsideRth,
	)

	// Fetch historical data from IB
	req := providers.HistoricalDataRequest{
		Conid:      conid,
		Exchange:   exchange,
		Period:     period,
		Bar:        bar,
		StartTime:  startTime,
		OutsideRth: outsideRth,
	}

	ibData, err := h.ibClient.HistoricalData(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch historical data from IB: %w", err)
	}

	if len(ibData.Data) == 0 {
		logger.Info("no data returned from IB", "conid", conid)
		return &database.OHLCVUpsertResult{RowsAffected: 0}, nil
	}

	// Convert IB data to OHLCV bars
	// Determine publisher from exchange or default to IB
	publisher := exchange
	if publisher == "" {
		publisher = "IB"
	}

	bars := make([]database.OHLCVBar, 0, len(ibData.Data))
	for _, ibBar := range ibData.Data {
		bars = append(bars, database.OHLCVBar{
			Symbol:    ibData.Symbol,
			Publisher: publisher,
			Ts:        ibBar.T,
			TsEnd:     ibBar.T + (barDurationNs(bar)), // Estimate end time
			Open:      ibBar.O,
			High:      ibBar.H,
			Low:       ibBar.L,
			Close:     ibBar.C,
			Volume:    int64(ibBar.V),
		})
	}

	logger.Info("converting IB data to OHLCV bars", "count", len(bars))

	// Ensure table exists
	if err := h.questdb.EnsureTableOHLCV(ctx); err != nil {
		return nil, fmt.Errorf("failed to ensure ohlcv_bars table: %w", err)
	}

	// Upsert bars to QuestDB
	result, err := h.questdb.UpsertOHLCVBars(ctx, bars)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert OHLCV bars: %w", err)
	}

	logger.Info("stored historical data in QuestDB",
		"symbol", ibData.Symbol,
		"bars", len(bars),
		"rows_affected", result.RowsAffected,
	)

	return result, nil
}

// lookupConidAndExchange looks up the conid and exchange for a given symbol.
func (h *MarketDataHandler) lookupConidAndExchange(ctx context.Context, symbol, exchange string) (conid, exch string, err error) {
	if h.sqliteDB == nil {
		return "", "", fmt.Errorf("Instruments database not available")
	}

	var query string
	var args []interface{}

	if exchange != "" {
		query = `SELECT id, exchange FROM instruments WHERE symbol = ? COLLATE NOCASE AND exchange = ? LIMIT 1`
		args = []interface{}{symbol, exchange}
	} else {
		query = `SELECT id, exchange FROM instruments WHERE symbol = ? COLLATE NOCASE LIMIT 1`
		args = []interface{}{symbol}
	}

	var id, exchFromDB string
	err = h.sqliteDB.QueryRow(ctx, query, args...).Scan(&id, &exchFromDB)
	if err == sql.ErrNoRows {
		return "", "", nil
	}
	if err != nil {
		return "", "", err
	}

	return id, exchFromDB, nil
}

// barDurationNs returns the approximate duration in nanoseconds for a bar size.
func barDurationNs(bar string) int64 {
	switch bar {
	case "1min":
		return 60 * 1_000_000_000
	case "2mins":
		return 2 * 60 * 1_000_000_000
	case "3mins":
		return 3 * 60 * 1_000_000_000
	case "5mins":
		return 5 * 60 * 1_000_000_000
	case "10mins":
		return 10 * 60 * 1_000_000_000
	case "15mins":
		return 15 * 60 * 1_000_000_000
	case "20mins":
		return 20 * 60 * 1_000_000_000
	case "30mins":
		return 30 * 60 * 1_000_000_000
	case "1hour":
		return 60 * 60 * 1_000_000_000
	case "2hour":
		return 2 * 60 * 60 * 1_000_000_000
	case "3hour":
		return 3 * 60 * 60 * 1_000_000_000
	case "4hour":
		return 4 * 60 * 60 * 1_000_000_000
	case "8hour":
		return 8 * 60 * 60 * 1_000_000_000
	case "1day":
		return 24 * 60 * 60 * 1_000_000_000
	case "1week":
		return 7 * 24 * 60 * 60 * 1_000_000_000
	default:
		return 5 * 60 * 1_000_000_000 // Default to 5min
	}
}

func (h *MarketDataHandler) GetHistoricalData(w http.ResponseWriter, r *http.Request) {
	if h.ibClient == nil {
		respondJSON(w, http.StatusServiceUnavailable, Response{
			Success: false,
			Error:   "IB client not available",
		})
		return
	}

	conid := r.URL.Query().Get("conid")
	exchange := r.URL.Query().Get("exchange")
	if conid == "" || exchange == "" {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "conid and exchange are required",
		})
		return
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "1d"
	}

	bar := r.URL.Query().Get("bar")
	if bar == "" {
		bar = "5mins"
	}

	startTime := r.URL.Query().Get("startTime")
	outsideRth := r.URL.Query().Get("outsideRth") == "true"

	req := providers.HistoricalDataRequest{
		Conid:      conid,
		Exchange:   exchange,
		Period:     period,
		Bar:        bar,
		StartTime:  startTime,
		OutsideRth: outsideRth,
	}

	data, err := h.ibClient.HistoricalData(r.Context(), req)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    data,
	})
}

// ListInstruments handles GET /api/v1/marketdata/instruments.
// Query params:
//   - exchange: Optional exchange filter (e.g., "XNAS", "XNYS", "ARCA")
//   - limit: Maximum number of results (default 100, max 1000)
//   - offset: Pagination offset (default 0)
func (h *MarketDataHandler) ListInstruments(w http.ResponseWriter, r *http.Request) {
	if h.sqliteDB == nil {
		respondJSON(w, http.StatusServiceUnavailable, Response{
			Success: false,
			Error:   "database not available",
		})
		return
	}

	exchange := r.URL.Query().Get("exchange")
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if _, err := fmt.Sscanf(l, "%d", &limit); err != nil || limit < 1 {
			limit = 100
		}
	}
	if limit > 1000 {
		limit = 1000
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		fmt.Sscanf(o, "%d", &offset)
	}

	var totalCount int
	var rows *sql.Rows
	var err error

	if exchange != "" {
		countQuery := `SELECT COUNT(*) FROM instruments WHERE exchange = ?`
		if err := h.sqliteDB.QueryRow(r.Context(), countQuery, exchange).Scan(&totalCount); err != nil {
			respondJSON(w, http.StatusInternalServerError, Response{
				Success: false,
				Error:   err.Error(),
			})
			return
		}

		query := `SELECT id, symbol, name, publisher, instrument_class, currency, exchange, asset, security_type, ` +
			`"group" FROM instruments WHERE exchange = ? ORDER BY symbol LIMIT ? OFFSET ?`
		rows, err = h.sqliteDB.Query(r.Context(), query, exchange, limit, offset)
	} else {
		countQuery := `SELECT COUNT(*) FROM instruments`
		if err := h.sqliteDB.QueryRow(r.Context(), countQuery).Scan(&totalCount); err != nil {
			respondJSON(w, http.StatusInternalServerError, Response{
				Success: false,
				Error:   err.Error(),
			})
			return
		}

		query := `SELECT id, symbol, name, publisher, instrument_class, currency, exchange, asset, security_type, ` +
			`"group" FROM instruments ORDER BY symbol LIMIT ? OFFSET ?`
		rows, err = h.sqliteDB.Query(r.Context(), query, limit, offset)
	}

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}
	defer rows.Close()

	var instruments []Instrument
	for rows.Next() {
		var inst Instrument
		if err := rows.Scan(&inst.ID, &inst.Symbol, &inst.Name, &inst.Publisher, &inst.InstrumentClass,
			&inst.Currency, &inst.Exchange, &inst.Asset, &inst.SecurityType, &inst.Group); err != nil {
			respondJSON(w, http.StatusInternalServerError, Response{
				Success: false,
				Error:   err.Error(),
			})
			return
		}
		instruments = append(instruments, inst)
	}

	if err := rows.Err(); err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"instruments": instruments,
			"limit":       limit,
			"offset":      offset,
			"total":       totalCount,
			"count":       len(instruments),
		},
	})
}
