package handlers

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/bishop-bot/datajobs/internal/database"
	"github.com/bishop-bot/datajobs/internal/providers"
	"github.com/bishop-bot/datajobs/internal/worker"
)

// Instrument represents an instrument record from the database.
type Instrument struct {
	ID             string `json:"id"`
	Symbol         string `json:"symbol"`
	Name           string `json:"name"`
	Publisher      string `json:"publisher"`
	InstrumentClass string `json:"instrument_class"`
	Currency       string `json:"currency"`
	Exchange       string `json:"exchange"`
	Asset          string `json:"asset"`
	SecurityType   string `json:"security_type"`
	Group          string `json:"group"`
}

// MarketDataHandler handles market data endpoints.
type MarketDataHandler struct {
	pool      *worker.Pool
	ibClient  *providers.IBClient
	sqliteDB  *database.DB
}

// NewMarketDataHandler creates a new market data handler.
func NewMarketDataHandler(pool *worker.Pool, ibClient *providers.IBClient, sqliteDB *database.DB) *MarketDataHandler {
	return &MarketDataHandler{
		pool:     pool,
		ibClient: ibClient,
		sqliteDB: sqliteDB,
	}
}

// GetHistoricalData handles GET /api/v1/marketdata/history.
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