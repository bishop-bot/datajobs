package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/bishop-bot/datajobs/internal/config"
	"github.com/bishop-bot/datajobs/internal/database"
	"github.com/bishop-bot/datajobs/internal/handlers"
	"github.com/bishop-bot/datajobs/internal/health"
	"github.com/bishop-bot/datajobs/internal/ingestion"
	"github.com/bishop-bot/datajobs/internal/jobs"
	"github.com/bishop-bot/datajobs/internal/logging"
	"github.com/bishop-bot/datajobs/internal/metrics"
	"github.com/bishop-bot/datajobs/internal/providers/ib"
	"github.com/bishop-bot/datajobs/internal/scheduler"
	"github.com/bishop-bot/datajobs/internal/tracing"
	"github.com/bishop-bot/datajobs/internal/worker"
)

// App encapsulates all application components and lifecycle management.
type App struct {
	cfg        *config.Config
	metrics    *metrics.Metrics
	logger     *slog.Logger

	// Core components
	sqliteDB  *database.DB
	questDB   *database.QuestDB
	ilpClient *ingestion.ILPClient
	ibClient  *ib.Client

	// Worker & scheduling
	pool      *worker.Pool
	scheduler *scheduler.Scheduler

	// Health monitoring
	healthServer *health.Server

	// HTTP server
	server *http.Server
	router *chi.Mux

	// Shutdown handling
	quit     chan struct{}
	shutdown chan os.Signal

	mu     sync.Mutex
	running bool
}

// NewApp creates a new application instance with all components initialized.
func NewApp(cfg *config.Config, m *metrics.Metrics) (*App, error) {
	logger := logging.NewLogger("app")

	app := &App{
		cfg:     cfg,
		metrics: m,
		logger:  logger,
	}

	// Initialize components
	if err := app.initTracing(); err != nil {
		logger.Warn("failed to init tracing", "error", err)
	}

	if err := app.initDatabases(); err != nil {
		return nil, err
	}

	if err := app.initIB(); err != nil {
		logger.Warn("failed to init IB client", "error", err)
		// Continue - IB is optional for server startup
	}

	app.initWorkerPool()
	app.initScheduler()
	app.initHealthServer()
	app.initHTTPServer()

	return app, nil
}

// Config returns the application configuration.
func (a *App) Config() *config.Config { return a.cfg }

// Metrics returns the metrics collector.
func (a *App) Metrics() *metrics.Metrics { return a.metrics }

// Scheduler returns the job scheduler.
func (a *App) Scheduler() *scheduler.Scheduler { return a.scheduler }

// Pool returns the worker pool.
func (a *App) Pool() *worker.Pool { return a.pool }

// HealthServer returns the health check server.
func (a *App) HealthServer() *health.Server { return a.healthServer }

// Start begins all application services.
func (a *App) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		return fmt.Errorf("app already running")
	}

	// Start scheduler
	if err := a.scheduler.Start(); err != nil {
		return fmt.Errorf("failed to start scheduler: %w", err)
	}

	// Start HTTP server in background
	go func() {
		addr := fmt.Sprintf("%s:%d", a.cfg.Server.Host, a.cfg.Server.Port)
		a.logger.Info("server listening", "address", addr)
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.logger.Error("server error", "error", err)
		}
	}()

	a.running = true
	return nil
}

// Stop gracefully shuts down all application services.
func (a *App) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return
	}

	a.logger.Info("shutting down...")

	// Shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(),
		time.Duration(a.cfg.Worker.ShutdownTimeout)*time.Second)
	defer cancel()

	// Stop scheduler first (no new jobs)
	a.scheduler.Stop()

	// Close clients
	if a.ilpClient != nil {
		a.ilpClient.Close()
	}
	if a.ibClient != nil {
		a.ibClient.Close()
	}

	// Shutdown HTTP server
	if a.server != nil {
		if err := a.server.Shutdown(shutdownCtx); err != nil {
			a.logger.Error("server shutdown error", "error", err)
		}
	}

	// Shutdown tracing
	if err := tracing.Shutdown(shutdownCtx); err != nil {
		a.logger.Error("tracing shutdown error", "error", err)
	}

	// Close database connections
	if a.sqliteDB != nil {
		a.sqliteDB.Close()
	}
	if a.questDB != nil {
		a.questDB.Close()
	}

	a.logger.Info("shutdown complete")
	a.running = false
}

// SetupShutdown configures signal handling for graceful shutdown.
func (a *App) SetupShutdown() {
	a.quit = make(chan struct{})
	a.shutdown = make(chan os.Signal, 1)
	signal.Notify(a.shutdown, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-a.shutdown
		a.logger.Info("shutdown signal received")
		close(a.quit)
	}()
}

// WaitForShutdown blocks until a shutdown signal is received.
func (a *App) WaitForShutdown() {
	<-a.quit
}

// initTracing initializes OpenTelemetry tracing.
func (a *App) initTracing() error {
	ctx := context.Background()
	return tracing.Init(ctx, tracing.Config{
		Enabled:      a.cfg.Tracing.Enabled,
		ServiceName:  a.cfg.Tracing.ServiceName,
		ExporterType: a.cfg.Tracing.ExporterType,
		Endpoint:     a.cfg.Tracing.Endpoint,
		Insecure:     a.cfg.Tracing.Insecure,
	})
}

// initDatabases initializes SQLite and QuestDB connections.
func (a *App) initDatabases() error {
	// Initialize SQLite
	var err error
	a.sqliteDB, err = database.New(a.cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to initialize SQLite: %w", err)
	}

	// Run migrations
	if err := a.sqliteDB.RunMigrations(context.Background()); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Initialize QuestDB (optional)
	a.questDB, err = database.NewQuestDB(a.cfg.QuestDB)
	if err != nil {
		a.logger.Warn("failed to connect to QuestDB", "error", err,
			"hint", "check QUESTDB_HOST and QUESTDB_PORT")
		a.questDB = nil // Allow server to start without QuestDB
	} else {
		// Note: caller should defer Close()
	}

	// Initialize ILP client
	if a.questDB != nil {
		a.ilpClient = ingestion.NewILPClient(a.cfg.QuestDB, a.metrics)
	}

	return nil
}

// initIB initializes the IB client.
func (a *App) initIB() error {
	ibClient, err := ib.NewClient(a.cfg.IB)
	if err != nil {
		return err
	}
	a.ibClient = ibClient
	return nil
}

// IBClient returns the IB client.
func (a *App) IBClient() *ib.Client {
	return a.ibClient
}

// initWorkerPool initializes the worker pool and registers handlers.
func (a *App) initWorkerPool() {
	a.pool = worker.NewPool(a.cfg.Worker, a.metrics)

	// Register built-in handlers
	for name, handler := range jobs.BuiltInHandlers() {
		a.pool.RegisterHandler(name, handler)
	}

	// Register QuestDB handlers with all dependencies
	jobs.RegisterQuestDBHandlers(a.pool, a.questDB, a.sqliteDB, a.ilpClient, a.ibClient)
}

// initScheduler initializes the scheduler and registers jobs from config.
func (a *App) initScheduler() {
	a.scheduler = scheduler.New(a.cfg.Scheduler, a.pool)

	// Register jobs from config
	for _, jobCfg := range a.cfg.Jobs {
		if err := a.scheduler.AddJob(jobCfg); err != nil {
			a.logger.Error("failed to add job", "job_id", jobCfg.ID, "error", err)
		}
	}
}

// initHealthServer initializes the health check server with all health checkers.
func (a *App) initHealthServer() {
	a.healthServer = health.New("1.0.0")

	// Add health checkers
	if a.questDB != nil {
		a.healthServer.AddChecker(&dbHealthChecker{questdb: a.questDB})
	}
	if a.sqliteDB != nil {
		a.healthServer.AddChecker(&sqliteHealthChecker{db: a.sqliteDB})
	}

	a.healthServer.SetReady(true)
}

// initHTTPServer initializes the HTTP server with routes.
func (a *App) initHTTPServer() {
	// Initialize handlers
	jobsHandler := handlers.NewJobsHandler(a.scheduler, a.pool)
	systemHandler := handlers.NewSystemHandler(handlers.NewSchedulerAdapter(a.scheduler), a.pool)
	questdbHandler := handlers.NewQuestDBHandler(a.questDB)
	marketDataHandler := handlers.NewMarketDataHandler(a.pool, a.ibClient, a.sqliteDB, a.questDB)
	instrumentsHandler := handlers.NewInstrumentsHandler(a.sqliteDB)

	// Setup router
	a.router = setupRouter(a.cfg, a.healthServer, a.metrics,
		jobsHandler, systemHandler, questdbHandler, marketDataHandler, instrumentsHandler)

	// Create server
	addr := fmt.Sprintf("%s:%d", a.cfg.Server.Host, a.cfg.Server.Port)
	a.server = &http.Server{
		Addr:         addr,
		Handler:      a.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

// setupRouter configures the HTTP router with all routes and middleware.
func setupRouter(
	cfg *config.Config,
	healthServer *health.Server,
	m *metrics.Metrics,
	jobsHandler *handlers.JobsHandler,
	systemHandler *handlers.SystemHandler,
	questdbHandler *handlers.QuestDBHandler,
	marketDataHandler *handlers.MarketDataHandler,
	instrumentsHandler *handlers.InstrumentsHandler,
) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(middleware.CleanPath)

	// Request logging middleware
	r.Use(requestLoggerMiddleware(m))

	// Health endpoints
	r.Get("/healthz", healthServer.LivenessHandler)
	r.Get("/readyz", healthServer.ReadinessHandler)
	r.Get("/status", healthServer.StatusHandler)

	// Metrics endpoint
	if cfg.Metrics.Enabled {
		r.Handle(cfg.Metrics.Path, promhttp.Handler())
	}

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Job management
		r.Get("/jobs", jobsHandler.ListJobs)
		r.Post("/jobs", jobsHandler.CreateJob)
		r.Get("/jobs/{id}", jobsHandler.GetJob)
		r.Put("/jobs/{id}", jobsHandler.UpdateJob)
		r.Delete("/jobs/{id}", jobsHandler.DeleteJob)
		r.Post("/jobs/{id}/run", systemHandler.RunJob)

		// Dead letter and stats
		r.Get("/dead-letter", systemHandler.GetDeadLetter)
		r.Get("/stats", systemHandler.GetStats)

		// Database endpoints (QuestDB)
		r.Get("/questdb/tables", questdbHandler.ListQuestDBTables)
		r.Get("/questdb/tables/{name}", questdbHandler.GetQuestDBTable)
		r.Post("/questdb/query", questdbHandler.QueryQuestDB)

		// Market data endpoints (IB)
		r.Get("/marketdata/history", marketDataHandler.GetHistoricalData)
		r.Get("/marketdata/instruments", marketDataHandler.ListInstruments)

		// Instruments import endpoints
		r.Post("/instruments/import", instrumentsHandler.ImportInstrumentsCSV)
		r.Post("/instruments/import-path", instrumentsHandler.ImportInstrumentsFromPath)
	})

	return r
}

// requestLoggerMiddleware returns middleware that logs HTTP requests.
func requestLoggerMiddleware(m *metrics.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			requestID := middleware.GetReqID(r.Context())

			logger := logging.NewLogger("http",
				"method", r.Method,
				"path", r.URL.Path,
				"request_id", requestID,
			)

			// Attach logger to context
			r = r.WithContext(logging.WithContext(r.Context(), logger))

			// Wrap response writer to capture status
			wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)
			status := strconv.Itoa(wrapped.status)

			m.RecordHTTPRequest(r.Context(), r.Method, r.URL.Path, status, duration)

			logger.Info("request completed",
				"status", status,
				"duration", duration.String(),
			)
		})
	}
}

// Health check implementations
type dbHealthChecker struct {
	questdb *database.QuestDB
}

func (c *dbHealthChecker) Name() string { return "questdb" }
func (c *dbHealthChecker) Check() (health.Status, string, error) {
	if err := c.questdb.Ping(context.Background()); err != nil {
		return health.StatusUnhealthy, "connection failed", err
	}
	return health.StatusHealthy, "", nil
}

type sqliteHealthChecker struct {
	db *database.DB
}

func (c *sqliteHealthChecker) Name() string { return "sqlite" }
func (c *sqliteHealthChecker) Check() (health.Status, string, error) {
	if err := c.db.Ping(context.Background()); err != nil {
		return health.StatusUnhealthy, "connection failed", err
	}
	return health.StatusHealthy, "", nil
}

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}