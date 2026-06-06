package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/bishop-bot/datajobs/internal/logging"
)

// Response is a standard API response.
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
}

func respondJSON(w http.ResponseWriter, status int, resp Response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logging.Warn("failed to encode JSON response", "status", status, "error", err)
	}
}
