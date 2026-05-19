package jobs

import (
	"context"
	"fmt"

	"github.com/bishop-bot/datajobs/internal/logging"
	"github.com/bishop-bot/datajobs/internal/worker"
)

// Registry holds all job handlers.
type Registry struct {
	handlers map[string]worker.JobFunc
}

// NewRegistry creates a new job registry.
func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string]worker.JobFunc),
	}
}

// Register registers a job handler.
func (r *Registry) Register(name string, handler worker.JobFunc) {
	r.handlers[name] = handler
}

// Handlers returns all registered handlers.
func (r *Registry) Handlers() map[string]worker.JobFunc {
	return r.handlers
}

// Get returns a handler by name.
func (r *Registry) Get(name string) (worker.JobFunc, bool) {
	h, ok := r.handlers[name]
	return h, ok
}

// BuiltInHandlers returns the built-in handlers with default implementations.
func BuiltInHandlers() map[string]worker.JobFunc {
	return map[string]worker.JobFunc{
		"noop":             NoopHandler,
		"bulk_ingest":      BulkIngestHandler,
		"incremental_update": IncrementalUpdateHandler,
	}
}

// NoopHandler is a no-op handler for testing.
func NoopHandler(ctx context.Context, job worker.Job) (string, error) {
	logger := logging.FromContext(ctx)
	logger.Info("executing noop job", "job_id", job.ID, "metadata", job.Metadata)
	return "noop completed", nil
}

// BulkIngestHandler handles bulk data ingestion jobs.
// Placeholder for actual implementation.
func BulkIngestHandler(ctx context.Context, job worker.Job) (string, error) {
	logger := logging.FromContext(ctx)
	logger.Info("executing bulk ingest job", "job_id", job.ID, "metadata", job.Metadata)

	// Extract configuration from job metadata
	source, _ := job.Metadata["source"].(string)
	targetTable, _ := job.Metadata["targetTable"].(string)
	batchSize, _ := job.Metadata["batchSize"].(float64)

	if batchSize == 0 {
		batchSize = 10000
	}

	logger.Info("bulk ingest config",
		"source", source,
		"target_table", targetTable,
		"batch_size", batchSize,
	)

	// TODO: Implement actual bulk ingestion logic
	// This would typically involve:
	// 1. Connecting to data source (S3, database, API, etc.)
	// 2. Reading data in batches
	// 3. Transforming/validating data
	// 4. Writing to target database (QuestDB in this case)
	// 5. Tracking progress and handling failures

	return fmt.Sprintf("bulk ingest from %s to %s completed", source, targetTable), nil
}

// IncrementalUpdateHandler handles incremental data updates.
// Placeholder for actual implementation.
func IncrementalUpdateHandler(ctx context.Context, job worker.Job) (string, error) {
	logger := logging.FromContext(ctx)
	logger.Info("executing incremental update job", "job_id", job.ID, "metadata", job.Metadata)

	// Extract configuration from job metadata
	source, _ := job.Metadata["source"].(string)
	batchSize, _ := job.Metadata["batchSize"].(float64)

	if batchSize == 0 {
		batchSize = 1000
	}

	logger.Info("incremental update config",
		"source", source,
		"batch_size", batchSize,
	)

	// TODO: Implement actual incremental update logic
	// This would typically involve:
	// 1. Fetching data changes since last sync (using timestamps, offsets, etc.)
	// 2. Processing only new/updated records
	// 3. Writing to target database
	// 4. Updating sync checkpoint

	return fmt.Sprintf("incremental update from %s completed", source), nil
}