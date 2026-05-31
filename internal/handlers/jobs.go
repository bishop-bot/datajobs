package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/bishop-bot/datajobs/internal/config"
	"github.com/bishop-bot/datajobs/internal/scheduler"
	"github.com/bishop-bot/datajobs/internal/worker"
)

// JobsHandler handles job management endpoints.
type JobsHandler struct {
	scheduler *scheduler.Scheduler
	pool      *worker.Pool
}

// NewJobsHandler creates a new jobs handler.
func NewJobsHandler(sched *scheduler.Scheduler, pool *worker.Pool) *JobsHandler {
	return&JobsHandler{
		scheduler: sched,
		pool:      pool,
	}
}

// ListJobs handles GET /api/v1/jobs.
func (h *JobsHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	jobs := h.scheduler.ListJobs()

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    jobs,
	})
}

// GetJob handles GET /api/v1/jobs/:id.
func (h *JobsHandler) GetJob(w http.ResponseWriter, r *http.Request) {
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
func (h *JobsHandler) CreateJob(w http.ResponseWriter, r *http.Request) {
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
	Cron     *string               `json:"cron,omitempty"`
	Handler  *string               `json:"handler,omitempty"`
	Enabled  *bool                 `json:"enabled,omitempty"`
	Timeout  *int                  `json:"timeout,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// UpdateJob handles PUT /api/v1/jobs/:id.
func (h *JobsHandler) UpdateJob(w http.ResponseWriter, r *http.Request) {
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
func (h *JobsHandler) DeleteJob(w http.ResponseWriter, r *http.Request) {
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
