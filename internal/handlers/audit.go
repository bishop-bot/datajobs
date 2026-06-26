package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/bishop-bot/datajobs/internal/audit"
)

// AuditHandler handles audit log API requests.
type AuditHandler struct {
	logger *audit.Logger
}

// NewAuditHandler creates a new audit handler.
func NewAuditHandler(logger *audit.Logger) *AuditHandler {
	return &AuditHandler{logger: logger}
}

// ListRuns returns job runs with optional filtering.
func (h *AuditHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	query := h.parseJobRunQuery(r)

	ctx := r.Context()
	runs, err := h.logger.GetByJobID(ctx, query)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "failed to fetch job runs: " + err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"runs":   runs,
			"count":  len(runs),
			"limit":  query.Limit,
			"offset": query.Offset,
		},
	})
}

// GetJobRuns returns job runs for a specific job.
func (h *AuditHandler) GetJobRuns(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")
	if jobID == "" {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "job_id is required",
		})
		return
	}

	query := h.parseJobRunQuery(r)
	query.JobID = jobID

	ctx := r.Context()
	runs, err := h.logger.GetByJobID(ctx, query)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "failed to fetch job runs: " + err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"runs":   runs,
			"count":  len(runs),
			"limit":  query.Limit,
			"offset": query.Offset,
		},
	})
}

// GetRun returns a single job run by ID.
func (h *AuditHandler) GetRun(w http.ResponseWriter, r *http.Request) {
	runIDStr := chi.URLParam(r, "runId")
	runID, err := strconv.ParseInt(runIDStr, 10, 64)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "invalid run_id",
		})
		return
	}

	ctx := r.Context()
	run, err := h.logger.GetByID(ctx, runID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "failed to fetch job run: " + err.Error(),
		})
		return
	}
	if run == nil {
		respondJSON(w, http.StatusNotFound, Response{
			Success: false,
			Error:   "job run not found",
		})
		return
	}

	// Parse JSON fields for cleaner response
	responseData := map[string]interface{}{
		"id":            run.ID,
		"job_id":        run.JobID,
		"started_at":    run.StartedAt,
		"completed_at":  run.CompletedAt,
		"status":        run.Status,
		"error_message": run.ErrorMessage,
		"duration_ms":   run.DurationMs,
		"attempt":       run.Attempt,
		"handler":       run.Handler,
	}

	// Parse JSON fields if present
	if run.Parameters != "" {
		var params map[string]interface{}
		if err := json.Unmarshal([]byte(run.Parameters), &params); err == nil {
			responseData["parameters"] = params
		}
	}
	if run.Results != "" {
		var results map[string]interface{}
		if err := json.Unmarshal([]byte(run.Results), &results); err == nil {
			responseData["results"] = results
		}
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    responseData,
	})
}

// GetStats returns audit statistics.
func (h *AuditHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stats, err := h.logger.GetStats(ctx)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "failed to fetch stats: " + err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    stats,
	})
}

// parseJobRunQuery parses query parameters into JobRunQuery.
func (h *AuditHandler) parseJobRunQuery(r *http.Request) audit.JobRunQuery {
	query := audit.JobRunQuery{
		Limit:  50,
		Offset: 0,
	}

	// Parse limit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			query.Limit = limit
			if query.Limit > 100 {
				query.Limit = 100 // Max limit
			}
		}
	}

	// Parse offset
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			query.Offset = offset
		}
	}

	// Parse status filter
	if status := r.URL.Query().Get("status"); status != "" {
		query.Status = audit.JobRunStatus(status)
	}

	// Parse start date
	if startDateStr := r.URL.Query().Get("start_date"); startDateStr != "" {
		if startDate, err := time.Parse(time.RFC3339, startDateStr); err == nil {
			query.StartDate = &startDate
		}
	}

	// Parse end date
	if endDateStr := r.URL.Query().Get("end_date"); endDateStr != "" {
		if endDate, err := time.Parse(time.RFC3339, endDateStr); err == nil {
			query.EndDate = &endDate
		}
	}

	return query
}
