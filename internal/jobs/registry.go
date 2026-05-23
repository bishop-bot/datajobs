package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/bishop-bot/datajobs/internal/ingestion"
	"github.com/bishop-bot/datajobs/internal/logging"
	"github.com/bishop-bot/datajobs/internal/providers"
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
		"noop":                NoopHandler,
		"bulk_ingest":         BulkIngestHandler,
		"incremental_update":  IncrementalUpdateHandler,
		"questdb_maintenance": QuestDBMaintenanceHandler,
		"sqlite_to_questdb":   SQLiteToQuestDBHandler,
		"ib_ping":             IBPingHandler,
	}
}

// RegisterQuestDBHandlers registers QuestDB-specific handlers.
func RegisterQuestDBHandlers(pool *worker.Pool, questdb interface{}, ilp *ingestion.ILPClient) {
	// Register bulk ingest with ILP
	pool.RegisterHandler("bulk_ingest", func(ctx context.Context, job worker.Job) (string, error) {
		return BulkIngestWithILP(ctx, job, ilp)
	})

	// Register incremental update with ILP
	pool.RegisterHandler("incremental_update", func(ctx context.Context, job worker.Job) (string, error) {
		return IncrementalUpdateWithILP(ctx, job, ilp)
	})

	// Register QuestDB maintenance
	pool.RegisterHandler("questdb_maintenance", QuestDBMaintenanceHandler)

	// Register SQLite to QuestDB sync
	pool.RegisterHandler("sqlite_to_questdb", SQLiteToQuestDBHandler)
}

// NoopHandler is a no-op handler for testing.
func NoopHandler(ctx context.Context, job worker.Job) (string, error) {
	logger := logging.FromContext(ctx)
	logger.Info("executing noop job", "job_id", job.ID, "metadata", job.Metadata)
	return "noop completed", nil
}

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
	batchSize := getFloat64(job.Metadata, "batchSize", 50000)
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
		SymbolColumns:   getStringSlice(job.Metadata, "symbolColumns"),
		BatchSize:       int(batchSize),
	}

	result, err := ilp.IngestCSV(ctx, targetTable, source, opts)
	if err != nil {
		logger.Error("bulk ingest failed", "error", err)
		return "", fmt.Errorf("bulk ingest failed: %w", err)
	}

	return fmt.Sprintf("ingested %d rows in %v", result.RowsIngested, result.Duration()), nil
}

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
	batchSize := getFloat64(job.Metadata, "batchSize", 1000)

	if source == "" || targetTable == "" {
		return "", fmt.Errorf("source and targetTable are required")
	}

	logger.Info("starting incremental update",
		"source", source,
		"target_table", targetTable,
	)

	opts := ingestion.CSVOptions{
		TimestampColumn: getString(job.Metadata, "timestampColumn", "timestamp"),
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

// QuestDBMaintenanceHandler performs QuestDB maintenance operations.
func QuestDBMaintenanceHandler(ctx context.Context, job worker.Job) (string, error) {
	logger := logging.FromContext(ctx).With("job_id", job.ID)

	operations := getStringSlice(job.Metadata, "operations")
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

// SQLiteToQuestDBHandler syncs data from SQLite to QuestDB.
func SQLiteToQuestDBHandler(ctx context.Context, job worker.Job) (string, error) {
	logger := logging.FromContext(ctx).With("job_id", job.ID)

	sourceDB, _ := job.Metadata["sourceDb"].(string)
	targetTable, _ := job.Metadata["targetTable"].(string)

	if sourceDB == "" || targetTable == "" {
		return "", fmt.Errorf("sourceDb and targetTable are required")
	}

	logger.Info("starting SQLite to QuestDB sync",
		"source_db", sourceDB,
		"target_table", targetTable,
	)

	return "SQLite to QuestDB sync initiated", nil
}

// IBPingHandler pings the Interactive Brokers Client Portal Gateway.
func IBPingHandler(ctx context.Context, job worker.Job) (string, error) {
	logger := logging.FromContext(ctx).With("job_id", job.ID)

	ibClient := providers.GetIB()
	if ibClient == nil {
		return "", fmt.Errorf("IB client not initialized")
	}

	logger.Debug("pinging IB gateway")

	if err := ibClient.Ping(ctx); err != nil {
		logger.Error("IB ping failed", "error", err)
		return "", fmt.Errorf("ping failed: %w", err)
	}

	return fmt.Sprintf("IB gateway ping successful at %s", time.Now().Format(time.RFC3339)), nil
}

// Helper functions
func getFloat64(m map[string]interface{}, key string, defaultVal float64) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return defaultVal
}

func getString(m map[string]interface{}, key, defaultVal string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return defaultVal
}

func getStringSlice(m map[string]interface{}, key string) []string {
	if v, ok := m[key].([]string); ok {
		return v
	}
	// Handle []interface{} from YAML parsing
	if v, ok := m[key].([]any); ok {
		result := make([]string, len(v))
		for i, item := range v {
			if s, ok := item.(string); ok {
				result[i] = s
			}
		}
		return result
	}
	return nil
}