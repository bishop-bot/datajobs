package ingestion

import (
	"context"
	"fmt"

	"github.com/bishop-bot/datajobs/internal/ingestion"
	"github.com/bishop-bot/datajobs/internal/logging"
	"github.com/bishop-bot/datajobs/internal/worker"
)

// IncrementalUpdateHandler handles incremental data updates via ILP.
func IncrementalUpdateHandler(ctx context.Context, job worker.Job) (string, error) {
	if ingestion.DefaultILPClient == nil {
		return "", fmt.Errorf("ILP client not initialized")
	}
	return IncrementalUpdateWithILP(ctx, job, ingestion.DefaultILPClient)
}

// IncrementalUpdateWithILP performs incremental update using the provided ILP client.
func IncrementalUpdateWithILP(ctx context.Context, job worker.Job, ilp *ingestion.ILPClient) (string, error) {
	logger := logging.FromContext(ctx).With("job_id", job.ID)

	source, _ := job.Metadata["source"].(string)
	targetTable, _ := job.Metadata["targetTable"].(string)
	batchSize := GetFloat64(job.Metadata, "batchSize", 1000)

	if source == "" || targetTable == "" {
		return "", fmt.Errorf("source and targetTable are required")
	}

	logger.Info("starting incremental update",
		"source", source,
		"target_table", targetTable,
	)

	opts := ingestion.CSVOptions{
		TimestampColumn: GetString(job.Metadata, "timestampColumn", "timestamp"),
		BatchSize:       int(batchSize),
		MaxRows:         int(batchSize), // Incremental = only fetch one batch
	}

	result, err := ilp.IngestCSV(ctx, targetTable, source, opts)
	if err != nil {
		logger.Error("incremental update failed", "error", err)
		return "", fmt.Errorf("incremental update failed: %w", err)
	}

	return fmt.Sprintf("updated %d rows", result.RowsIngested), nil
}