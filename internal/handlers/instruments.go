package handlers

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bishop-bot/datajobs/internal/database"
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

// InstrumentsHandler handles instruments endpoints.
type InstrumentsHandler struct {
	db *database.DB
}

// NewInstrumentsHandler creates a new instruments handler.
func NewInstrumentsHandler(db *database.DB) *InstrumentsHandler {
	return &InstrumentsHandler{db: db}
}

// ImportResult holds the result of a CSV import operation.
type ImportResult struct {
	Imported int      `json:"imported"`
	Skipped  int      `json:"skipped"`
	Errors   []string `json:"errors,omitempty"`
}

// ImportInstrumentsCSV handles POST /api/v1/instruments/import.
// Accepts a CSV file upload and imports instruments into the database.
func (h *InstrumentsHandler) ImportInstrumentsCSV(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		respondJSON(w, http.StatusServiceUnavailable, Response{
			Success: false,
			Error:   "database not available",
		})
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB max
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   fmt.Sprintf("failed to parse form: %v", err),
		})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "file field is required",
		})
		return
	}
	defer file.Close()

	// Validate file extension
	if filepath.Ext(header.Filename) != ".csv" {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "only CSV files are supported",
		})
		return
	}

	result, err := h.importCSV(r.Context(), file, header.Filename)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    result,
		Message: fmt.Sprintf("imported %d instruments", result.Imported),
	})
}

// ImportInstrumentsFromPath handles POST /api/v1/instruments/import-path.
// Imports instruments from a local file path (for CLI or internal use).
func (h *InstrumentsHandler) ImportInstrumentsFromPath(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		respondJSON(w, http.StatusServiceUnavailable, Response{
			Success: false,
			Error:   "database not available",
		})
		return
	}

	path := r.URL.Query().Get("path")
	if path == "" {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "path query parameter is required",
		})
		return
	}

	file, err := os.Open(path)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   fmt.Sprintf("failed to open file: %v", err),
		})
		return
	}
	defer file.Close()

	result, err := h.importCSV(r.Context(), file, filepath.Base(path))
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    result,
		Message: fmt.Sprintf("imported %d instruments from %s", result.Imported, path),
	})
}

// importCSV reads and imports CSV data into the instruments table.
func (h *InstrumentsHandler) importCSV(ctx context.Context, reader io.Reader, filename string) (*ImportResult, error) {
	csvReader := csv.NewReader(reader)

	// Read header row
	rawHeader, err := csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	// Normalize column names: trim whitespace and remove trailing type annotations
	// e.g., "price_ratio REAL" -> "price_ratio"
	header := make([]string, len(rawHeader))
	colIndex := make(map[string]int)
	for i, col := range rawHeader {
		// Trim whitespace
		col = strings.TrimSpace(col)
		// Remove trailing type annotations (e.g., "REAL", "INTEGER", "TEXT")
		col = strings.TrimSuffix(col, "REAL")
		col = strings.TrimSuffix(col, "INTEGER")
		col = strings.TrimSuffix(col, "TEXT")
		col = strings.TrimSpace(col) // Trim again after removing type
		header[i] = col
		colIndex[col] = i
	}

	// Validate required columns
	required := []string{"id", "symbol", "name", "publisher", "instrument_class"}
	for _, col := range required {
		if _, ok := colIndex[col]; !ok {
			return nil, fmt.Errorf("missing required column: %s", col)
		}
	}

	result := &ImportResult{}
	batchSize := 100
	var batch [][]interface{}

	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("parse error: %v", err))
			result.Skipped++
			continue
		}

		// Build row values
		row := make([]interface{}, len(header))
		for i := range header {
			val := record[i]
			// Try to convert integer columns first
			if isIntegerColumn(header[i]) && val != "" {
				if num, err := strconv.ParseInt(val, 10, 64); err == nil {
					row[i] = num
				} else {
					row[i] = val
				}
			// Try to convert float columns
			} else if isFloatColumn(header[i]) && val != "" {
				if num, err := strconv.ParseFloat(val, 64); err == nil {
					row[i] = num
				} else {
					row[i] = val
				}
			} else {
				row[i] = val
			}
		}

		batch = append(batch, row)

		// Process batch
		if len(batch) >= batchSize {
			imported, err := h.db.ImportInstrumentsBatch(ctx, header, batch)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("batch import error: %v", err))
			}
			result.Imported += imported
			batch = batch[:0]
		}
	}

	// Process remaining batch
	if len(batch) > 0 {
		imported, err := h.db.ImportInstrumentsBatch(ctx, header, batch)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("batch import error: %v", err))
		}
		result.Imported += imported
	}

	return result, nil
}

// isIntegerColumn returns true for columns that should be parsed as integers.
func isIntegerColumn(col string) bool {
	integerCols := map[string]bool{
		"id":             true,
		"maturity_year":   true,
		"maturity_month":  true,
		"maturity_day":    true,
	}
	return integerCols[col]
}

// isFloatColumn returns true for columns that should be parsed as floats.
func isFloatColumn(col string) bool {
	floatCols := map[string]bool{
		"min_lot_size":        true,
		"max_price_variation": true,
		"unit_of_measure_qty": true,
		"min_price_increment": true,
		"display_factor":      true,
		"price_ratio":         true,
		"strike_price":        true,
	}
	return floatCols[col]
}
