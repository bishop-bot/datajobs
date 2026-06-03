package jobs

import (
	"context"

	"github.com/bishop-bot/datajobs/internal/database"
	"github.com/bishop-bot/datajobs/internal/ingestion"
	jobingestion "github.com/bishop-bot/datajobs/internal/jobs/ingestion"
	jobquestdb "github.com/bishop-bot/datajobs/internal/jobs/questdb"
	"github.com/bishop-bot/datajobs/internal/jobs/system"
	"github.com/bishop-bot/datajobs/internal/jobs/providers"
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
		"noop":                system.NoopHandler,
		"bulk_ingest":         jobingestion.BulkIngestHandler,
		"incremental_update":  jobingestion.IncrementalUpdateHandler,
		"questdb_maintenance": jobquestdb.MaintenanceHandler,
		"sqlite_to_questdb":   jobingestion.SQLiteToQuestDBHandler,
		"ib_ping":             providers.PingHandler,
	}
}

// RegisterQuestDBHandlers registers QuestDB-specific handlers.
func RegisterQuestDBHandlers(pool *worker.Pool, questDB *database.QuestDB, sqliteDB *database.DB, ilp *ingestion.ILPClient) {
	// Register bulk ingest with ILP
	pool.RegisterHandler("bulk_ingest", func(ctx context.Context, job worker.Job) (string, error) {
		return jobingestion.BulkIngestWithILP(ctx, job, ilp)
	})

	// Register incremental update with ILP
	pool.RegisterHandler("incremental_update", func(ctx context.Context, job worker.Job) (string, error) {
		return jobingestion.IncrementalUpdateWithILP(ctx, job, ilp)
	})

	// Register QuestDB maintenance
	pool.RegisterHandler("questdb_maintenance", jobquestdb.MaintenanceHandler)

	// Register SQLite to QuestDB sync
	pool.RegisterHandler("sqlite_to_questdb", jobingestion.SQLiteToQuestDBHandler)

	// Register historical data handler
	if questDB != nil && sqliteDB != nil {
		pool.RegisterHandler("historical_data", providers.HistoricalDataHandlerWithDB(questDB, sqliteDB))
	}
}