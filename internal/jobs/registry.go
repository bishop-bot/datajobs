package jobs

import (
	"context"

	"github.com/bishop-bot/datajobs/internal/database"
	"github.com/bishop-bot/datajobs/internal/ingestion"
	"github.com/bishop-bot/datajobs/internal/jobs/calendar"
	"github.com/bishop-bot/datajobs/internal/jobs/historical"
	jobingestion "github.com/bishop-bot/datajobs/internal/jobs/ingestion"
	"github.com/bishop-bot/datajobs/internal/jobs/monitoring"
	jobquestdb "github.com/bishop-bot/datajobs/internal/jobs/questdb"
	corporateactions "github.com/bishop-bot/datajobs/internal/jobs/stocks"
	fundamentals "github.com/bishop-bot/datajobs/internal/jobs/stocks/fundamentals"
	"github.com/bishop-bot/datajobs/internal/jobs/system"
	"github.com/bishop-bot/datajobs/internal/logging"
	"github.com/bishop-bot/datajobs/internal/providers/earnings"
	"github.com/bishop-bot/datajobs/internal/providers/fmp"
	"github.com/bishop-bot/datajobs/internal/providers/ib"
	"github.com/bishop-bot/datajobs/internal/repository"
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
// Note: ib_ping requires IB provider and is registered separately in RegisterQuestDBHandlers.
func BuiltInHandlers() map[string]worker.JobFunc {
	return map[string]worker.JobFunc{
		"noop":                system.NoopHandler,
		"bulk_ingest":         jobingestion.BulkIngestHandler,
		"incremental_update":  jobingestion.IncrementalUpdateHandler,
		"questdb_maintenance": jobquestdb.MaintenanceHandler,
		"sqlite_to_questdb":   jobingestion.SQLiteToQuestDBHandler,
	}
}

// RegisterQuestDBHandlers registers QuestDB-specific handlers.
func RegisterQuestDBHandlers(pool *worker.Pool, questDB *database.QuestDB, sqliteDB *database.DB, ilp *ingestion.ILPClient, ibProvider ib.Provider, earningsProvider earnings.Provider) {
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

	// Register IB ping handler if provider is available
	if ibProvider != nil {
		pool.RegisterHandler("ib_ping", monitoring.PingHandler(ibProvider))
	}

	// Register historical data handler with IB provider
	if questDB != nil && sqliteDB != nil {
		pool.RegisterHandler("historical_data", historical.HistoricalDataHandlerWithDB(questDB, sqliteDB, ibProvider))
	} else {
		logging.Warn("historical_data handler not registered",
			"questDB_nil", questDB == nil,
			"sqliteDB_nil", sqliteDB == nil,
			"hint", "ensure QuestDB and SQLite are properly configured",
		)
	}

	// Register earnings sync handler
	// Note: We check the concrete *earnings.Client type directly so we can
	// properly check for nil before registration. This avoids the "typed nil"
	// issue where an interface holding a nil pointer is not actually nil.
	if sqliteDB != nil && earningsProvider != nil {
		pool.RegisterHandler("earnings_sync", corporateactions.HandlerWithDeps(sqliteDB, earningsProvider))
	} else {
		logging.Warn("earnings_sync handler not registered",
			"sqliteDB_nil", sqliteDB == nil,
			"earningsProvider_nil", earningsProvider == nil,
			"hint", "ensure SQLite and Earnings provider are properly configured",
		)
	}

	// Register economic calendar sync handler
	if sqliteDB != nil && earningsProvider != nil {
		pool.RegisterHandler("economic_calendar_sync", calendar.HandlerWithDeps(sqliteDB, earningsProvider))
	} else {
		logging.Warn("economic_calendar_sync handler not registered",
			"sqliteDB_nil", sqliteDB == nil,
			"earningsProvider_nil", earningsProvider == nil,
			"hint", "ensure SQLite and Earnings provider are properly configured",
		)
	}
}

// RegisterFundamentalsHandlers registers the fundamentals sync handlers.
func RegisterFundamentalsHandlers(pool *worker.Pool, sqliteDB *database.DB, questDB *database.QuestDB, fmpProvider fmp.Provider, watchlistRepo *repository.WatchlistRepository) {
	// Register fundamentals sync handler
	if sqliteDB != nil && fmpProvider != nil && watchlistRepo != nil {
		pool.RegisterHandler("fundamentals_sync", fundamentals.HandlerWithDeps(sqliteDB, questDB, fmpProvider, watchlistRepo))
	} else {
		logging.Warn("fundamentals_sync handler not registered",
			"sqliteDB_nil", sqliteDB == nil,
			"fmpProvider_nil", fmpProvider == nil,
			"watchlistRepo_nil", watchlistRepo == nil,
			"hint", "ensure SQLite, FMP provider, and Watchlist repository are properly configured",
		)
	}
}
