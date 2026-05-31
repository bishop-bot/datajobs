package handlers

import (
	"net/http"
	"time"

	"github.com/bishop-bot/datajobs/internal/scheduler"
	"github.com/bishop-bot/datajobs/internal/worker"
)

// SystemHandler handles system/monitoring endpoints.
type SystemHandler struct {
	scheduler *scheduler.Scheduler
	pool      *worker.Pool
}

// NewSystemHandler creates a new system handler.
func NewSystemHandler(sched *scheduler.Scheduler, pool *worker.Pool) *SystemHandler {
	return &SystemHandler{
		scheduler: sched,
		pool:      pool,
	}
}

// RunJob handles POST /api/v1/jobs/:id/run.
func (h *SystemHandler) RunJob(w http.ResponseWriter, r *http.Request) {
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
