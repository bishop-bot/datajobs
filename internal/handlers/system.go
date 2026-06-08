package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/bishop-bot/datajobs/internal/worker"
)

// SchedulerRunner defines the interface for triggering job execution.
type SchedulerRunner interface {
	RunNow(ctx context.Context, jobID string, metadata map[string]interface{}) error
	ListJobs() []JobInfo
	GetJob(jobID string) (JobInfo, bool)
}

// JobInfo represents basic job information.
type JobInfo struct {
	ID   string
	Name string
}

// SystemHandler handles system/monitoring endpoints.
type SystemHandler struct {
	scheduler SchedulerRunner
	pool      *worker.Pool
}

// NewSystemHandler creates a new system handler.
func NewSystemHandler(sched SchedulerRunner, pool *worker.Pool) *SystemHandler {
	return &SystemHandler{
		scheduler: sched,
		pool:      pool,
	}
}

// RunJob handles POST /api/v1/jobs/:id/run.
// Optionally accepts a JSON body with runtime metadata parameters.
// The metadata is passed directly to the job handler, which interprets
// fields as needed (e.g., "instruments", "period", "bar" for historical_data).
func (h *SystemHandler) RunJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")

	// Parse body as raw metadata (empty map if no body)
	metadata := make(map[string]interface{})
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&metadata); err != nil {
			respondJSON(w, http.StatusBadRequest, Response{
				Success: false,
				Error:   "invalid JSON body: " + err.Error(),
			})
			return
		}
	}

	if err := h.scheduler.RunNow(r.Context(), jobID, metadata); err != nil {
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
			"metadata":     metadata,
		},
	})
}

// GetDeadLetter handles GET /api/v1/dead-letter.
func (h *SystemHandler) GetDeadLetter(w http.ResponseWriter, r *http.Request) {
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
func (h *SystemHandler) GetStats(w http.ResponseWriter, r *http.Request) {
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
