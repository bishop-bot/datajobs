package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/bishop-bot/datajobs/internal/config"
	"github.com/bishop-bot/datajobs/internal/database"
	"github.com/bishop-bot/datajobs/internal/scheduler"
	"github.com/bishop-bot/datajobs/internal/worker"
)

// Handler holds all HTTP handlers and dependencies.
type Handler struct {
	scheduler *scheduler.Scheduler
	pool      *worker.Pool
	sqlite    *database.DB
	questdb   *database.QuestDB
}

// New creates a new handler instance.
func New(sched *scheduler.Scheduler, pool *worker.Pool, sqlite *database.DB, questdb *database.QuestDB) *Handler {
	return &Handler{
		scheduler: sched,
		pool:      pool,
		sqlite:    sqlite,
		questdb:   questdb,
	}
}

// Response is a standard API response.
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
}

// ListJobs handles GET /api/v1/jobs.
func (h *Handler) ListJobs(w http.ResponseWriter, r *http.Request) {
	jobs := h.scheduler.ListJobs()

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    jobs,
	})
}

// GetJob handles GET /api/v1/jobs/:id.
func (h *Handler) GetJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")

	job, ok := h.scheduler.GetJob(jobID)
	if !ok {
		respondJSON(w, http.StatusNotFound, Response{
			Success: false,
			Error:   "job not found",
		})
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    job,
	})
}

// CreateJobRequest is the request body for creating a job.
type CreateJobRequest struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Cron     string                 `json:"cron"`
	Type     string                 `json:"type"`
	Handler  string                 `json:"handler"`
	Enabled  bool                   `json:"enabled"`
	Timeout  int                    `json:"timeout"`
	Retry    config.RetryConfig     `json:"retry"`
	Metadata map[string]interface{} `json:"metadata"`
}

// CreateJob handles POST /api/v1/jobs.
func (h *Handler) CreateJob(w http.ResponseWriter, r *http.Request) {
	var req CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "invalid request body",
		})
		return
	}

	if req.ID == "" {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "job ID is required",
		})
		return
	}

	// Check if job already exists
	if _, ok := h.scheduler.GetJob(req.ID); ok {
		respondJSON(w, http.StatusConflict, Response{
			Success: false,
			Error:   "job already exists",
		})
		return
	}

	jobCfg := config.JobConfig{
		ID:       req.ID,
		Name:     req.Name,
		Cron:     req.Cron,
		Type:     req.Type,
		Handler:  req.Handler,
		Enabled:  req.Enabled,
		Timeout:  req.Timeout,
		Retry:    req.Retry,
		Metadata: req.Metadata,
	}

	if err := h.scheduler.AddJob(jobCfg); err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusCreated, Response{
		Success: true,
		Message: "job created",
		Data:    jobCfg,
	})
}

// UpdateJobRequest is the request body for updating a job.
type UpdateJobRequest struct {
	Name     *string               `json:"name,omitempty"`
	Cron      *string               `json:"cron,omitempty"`
	Handler   *string               `json:"handler,omitempty"`
	Enabled   *bool                 `json:"enabled,omitempty"`
	Timeout   *int                  `json:"timeout,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// UpdateJob handles PUT /api/v1/jobs/:id.
func (h *Handler) UpdateJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")

	var req UpdateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "invalid request body",
		})
		return
	}

	// Check if job exists
	job, ok := h.scheduler.GetJob(jobID)
	if !ok {
		respondJSON(w, http.StatusNotFound, Response{
			Success: false,
			Error:   "job not found",
		})
		return
	}

	// Note: Full update support would require modifying the scheduler
	// For now, we handle enable/disable which is the most common operation
	if req.Enabled != nil {
		if *req.Enabled {
			h.scheduler.EnableJob(r.Context(), jobID)
		} else {
			h.scheduler.DisableJob(r.Context(), jobID)
		}
		job.Enabled = *req.Enabled
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "job updated",
		Data:    job,
	})
}

// DeleteJob handles DELETE /api/v1/jobs/:id.
func (h *Handler) DeleteJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")

	// Check if job exists
	if _, ok := h.scheduler.GetJob(jobID); !ok {
		respondJSON(w, http.StatusNotFound, Response{
			Success: false,
			Error:   "job not found",
		})
		return
	}

	// Note: Full delete support would require modifying the scheduler
	// For now, we just disable the job
	h.scheduler.DisableJob(r.Context(), jobID)

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "job disabled",
	})
}

// RunJob handles POST /api/v1/jobs/:id/run.
func (h *Handler) RunJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")

	if err := h.scheduler.RunNow(r.Context(), jobID); err != nil {
		respondJSON(w, http.StatusNotFound, Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusAccepted, Response{
		Success: true,
		Message: "job triggered",
		Data: map[string]interface{}{
			"job_id":       jobID,
			"triggered_at": time.Now().UTC(),
		},
	})
}

// GetDeadLetter handles GET /api/v1/dead-letter.
func (h *Handler) GetDeadLetter(w http.ResponseWriter, r *http.Request) {
	dl := h.pool.GetDeadLetterQueue()

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"count": len(dl),
			"jobs":  dl,
		},
	})
}

// GetStats handles GET /api/v1/stats.
func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats := map[string]interface{}{
		"queue_depth":       h.pool.GetQueueDepth(),
		"queue_capacity":    100, // This should come from config
		"dead_letter_count": h.pool.GetDeadLetterCount(),
		"jobs":              h.scheduler.ListJobs(),
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    stats,
	})
}

// ListQuestDBTables handles GET /api/v1/questdb/tables.
func (h *Handler) ListQuestDBTables(w http.ResponseWriter, r *http.Request) {
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
func (h *Handler) GetQuestDBTable(w http.ResponseWriter, r *http.Request) {
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
func (h *Handler) QueryQuestDB(w http.ResponseWriter, r *http.Request) {
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

	// Collect results
	var results []map[string]interface{}
	for rows.Next() {
		// Get field descriptions for column names
		_ = rows.FieldDescriptions()
		// Note: In production, you'd want to scan into actual types
		values, _ := rows.Values()
		if values != nil {
			row := make(map[string]interface{})
			for i, v := range values {
				row[fmt.Sprintf("col_%d", i)] = v
			}
			results = append(results, row)
		}
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
			"rows": results,
			"count": len(results),
		},
	})
}

func respondJSON(w http.ResponseWriter, status int, resp Response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}