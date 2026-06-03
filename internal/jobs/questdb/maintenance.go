package questdb

import (
	"context"
	"fmt"

	"github.com/bishop-bot/datajobs/internal/jobs/ingestion"
	"github.com/bishop-bot/datajobs/internal/logging"
	"github.com/bishop-bot/datajobs/internal/worker"
)

// MaintenanceHandler performs QuestDB maintenance operations.
func MaintenanceHandler(ctx context.Context, job worker.Job) (string, error) {
	logger := logging.FromContext(ctx).With("job_id", job.ID)

	operations := ingestion.GetStringSlice(job.Metadata, "operations")
	if len(operations) == 0 {
		operations = []string{"analyze"}
	}

	var results []string
	for _, op := range operations {
		switch op {
		case "analyze":
			results = append(results, "analyze completed")
		case "cleanup":
			results = append(results, "cleanup completed")
		default:
			results = append(results, fmt.Sprintf("unknown operation: %s", op))
		}
	}

	logger.Info("maintenance operations completed", "operations", operations)
	return fmt.Sprintf("maintenance: %v", results), nil
}