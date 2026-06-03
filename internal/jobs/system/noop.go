package system

import (
	"context"

	"github.com/bishop-bot/datajobs/internal/logging"
	"github.com/bishop-bot/datajobs/internal/worker"
)

// NoopHandler is a no-op handler for testing.
func NoopHandler(ctx context.Context, job worker.Job) (string, error) {
	logger := logging.FromContext(ctx)
	logger.Info("executing noop job", "job_id", job.ID, "metadata", job.Metadata)
	return "noop completed", nil
}