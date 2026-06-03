package ingestion

import (
	"context"
	"fmt"

	"github.com/bishop-bot/datajobs/internal/ingestion"
	"github.com/bishop-bot/datajobs/internal/logging"
	"github.com/bishop-bot/datajobs/internal/worker"
)

// BulkIngestHandler handles bulk data ingestion jobs via ILP.
func BulkIngestHandler(ctx context.Context, job worker.Job) (string, error) {
	if ingestion.DefaultILPClient == nil {
		return "", fmt.Errorf("ILP client not initialized")
	}
	return BulkIngestWithILP(ctx, job, ingestion.DefaultILPClient)
}

// BulkIngestWithILP performs bulk ingestion using the provided ILP client.
func BulkIngestWithILP(ctx context.Context, job worker.Job, ilp *ingestion.ILPClient) (string, error) {
	logger := logging.FromContext(ctx).With("job_id", job.ID)

	source, _ := job.Metadata["source"].(string)
	targetTable, _ := job.Metadata["targetTable"].(string)
	batchSize := GetFloat64(job.Metadata, "batchSize", 50000)
	timestampCol, _ := job.Metadata["timestampColumn"].(string)
	if timestampCol == "" {
		timestampCol = "timestamp"
	}

	if source == "" || targetTable == "" {
		return "", fmt.Errorf("source and targetTable are required")
	}

	logger.Info("starting bulk ingest",
		"source", source,
		"target_table", targetTable,
		"batch_size", int(batchSize),
	)

	opts := ingestion.CSVOptions{
		TimestampColumn: timestampCol,
		SymbolColumns:   GetStringSlice(job.Metadata, "symbolColumns"),
		BatchSize:       int(batchSize),
	}

	result, err := ilp.IngestCSV(ctx, targetTable, source, opts)
	if err != nil {
		logger.Error("bulk ingest failed", "error", err)
		return "", fmt.Errorf("bulk ingest failed: %w", err)
	}

	return fmt.Sprintf("ingested %d rows in %v", result.RowsIngested, result.Duration()), nil
}