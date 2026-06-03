package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/bishop-bot/datajobs/internal/database"
)

// QuestDBHandler handles QuestDB database endpoints.
type QuestDBHandler struct {
	questdb *database.QuestDB
}

// NewQuestDBHandler creates a new QuestDB handler.
func NewQuestDBHandler(questdb *database.QuestDB) *QuestDBHandler {
	return &QuestDBHandler{
		questdb: questdb,
	}
}

// ListQuestDBTables handles GET /api/v1/questdb/tables.
func (h *QuestDBHandler) ListQuestDBTables(w http.ResponseWriter, r *http.Request) {
	if h.questdb == nil {
		respondJSON(w, http.StatusServiceUnavailable, Response{
			Success: false,
			Error:   "QuestDB not connected",
		})
		return
	}

	tables, err := h.questdb.ListTables(r.Context())
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    tables,
	})
}

// GetQuestDBTable handles GET /api/v1/questdb/tables/{name}.
func (h *QuestDBHandler) GetQuestDBTable(w http.ResponseWriter, r *http.Request) {
	if h.questdb == nil {
		respondJSON(w, http.StatusServiceUnavailable, Response{
			Success: false,
			Error:   "QuestDB not connected",
		})
		return
	}

	tableName := r.PathValue("name")

	columns, err := h.questdb.GetTableColumns(r.Context(), tableName)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"name":    tableName,
			"columns": columns,
		},
	})
}

// QueryQuestDBRequest is the request body for querying QuestDB.
type QueryQuestDBRequest struct {
	SQL string `json:"sql"`
}

// QueryQuestDB handles POST /api/v1/questdb/query.
func (h *QuestDBHandler) QueryQuestDB(w http.ResponseWriter, r *http.Request) {
	if h.questdb == nil {
		respondJSON(w, http.StatusServiceUnavailable, Response{
			Success: false,
			Error:   "QuestDB not connected",
		})
		return
	}

	var req QueryQuestDBRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "invalid request body",
		})
		return
	}

	rows, err := h.questdb.Query(r.Context(), req.SQL)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}
	defer rows.Close()

	results, err := scanRows(rows)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"rows":  results,
			"count": len(results),
		},
	})
}

// scanRows scans query results into a slice of maps.
func scanRows(rows database.Rows) ([]map[string]interface{}, error) {
	var results []map[string]interface{}
	for rows.Next() {
		// Scan all columns into interface{} slice
		cols := rows.FieldDescriptions()
		values := make([]interface{}, len(cols))
		for i := range values {
			values[i] = new(interface{})
		}
		if err := rows.Scan(values...); err != nil {
			continue
		}
		row := make(map[string]interface{})
		for i, v := range values {
			if iv, ok := v.(*interface{}); ok && iv != nil {
				if colName, ok := cols[i].(string); ok {
					row[colName] = *iv
				} else {
					row[fmt.Sprintf("col_%d", i)] = *iv
				}
			}
		}
		results = append(results, row)
	}
	return results, rows.Err()
}
