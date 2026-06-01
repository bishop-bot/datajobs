package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
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
	"github.com/bishop-bot/datajobs/internal/providers"
	"github.com/bishop-bot/datajobs/internal/scheduler"
	"github.com/bishop-bot/datajobs/internal/tracing"
	"github.com/bishop-bot/datajobs/internal/worker"
)

const (
	version    = "1.0.0"
	configPath = "config.yaml"
)

func main() {
	if err := run(); err != nil {
		logging.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize logging
	logging.Init(logging.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
	})

	logger := logging.NewLogger("main")
	logger.Info("starting server", "version", version)

	// Initialize tracing
	ctx := context.Background()
	if err := tracing.Init(ctx, tracing.Config{
		Enabled:      cfg.Tracing.Enabled,
		ServiceName:  cfg.Tracing.ServiceName,
		ExporterType: cfg.Tracing.ExporterType,
		Endpoint:     cfg.Tracing.Endpoint,
		Insecure:     cfg.Tracing.Insecure,
	}); err != nil {
		logger.Warn("failed to init tracing", "error", err)
	}

	// Initialize metrics
	m := metrics.New("datajobs")

	// Initialize SQLite database
	sqliteDB, err := database.New(cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to initialize SQLite: %w", err)
	}
	defer sqliteDB.Close()

	// Run migrations
	if err := sqliteDB.RunMigrations(ctx); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Initialize QuestDB connection pool
	var questDB *database.QuestDB
	questDB, err = database.NewQuestDB(cfg.QuestDB)
	if err != nil {
		logger.Warn("failed to connect to QuestDB", "error", err, "hint", "check QUESTDB_HOST and QUESTDB_PORT")
		questDB = nil // Allow server to start without QuestDB
	} else {
		defer questDB.Close()
	}

	// Initialize ILP client for QuestDB ingestion
	var ilpClient *ingestion.ILPClient
	if questDB != nil {
		ilpClient = ingestion.NewILPClient(cfg.QuestDB, m)
		ingestion.InitILP(cfg.QuestDB, m)
	}

	// Initialize IB client (optional - server starts even if IB is unavailable)
	if err := providers.InitIB(cfg.IB); err != nil {
		logger.Warn("failed to initialize IB client", "error", err, "hint", "check IB_BASE_URL")
	}

	// Initialize worker pool
	pool := worker.NewPool(cfg.Worker, m)

	// Register built-in job handlers
	for name, handler := range jobs.BuiltInHandlers() {
		pool.RegisterHandler(name, handler)
	}

	// Register QuestDB handlers
	jobs.RegisterQuestDBHandlers(pool, questDB, ilpClient)

	// Initialize scheduler
	sched := scheduler.New(cfg.Scheduler, pool)

	// Register jobs from config
	for _, jobCfg := range cfg.Jobs {
		if err := sched.AddJob(jobCfg); err != nil {
			logger.Error("failed to add job", "job_id", jobCfg.ID, "error", err)
		}
	}

	// Start scheduler
	if err := sched.Start(); err != nil {
		return fmt.Errorf("failed to start scheduler: %w", err)
	}

	// Initialize health server
	healthServer := health.New(version)

	// Add database health check
	if questDB != nil {
		healthServer.AddChecker(&dbHealthChecker{questdb: questDB})
	}
	healthServer.AddChecker(&sqliteHealthChecker{db: sqliteDB})

	healthServer.SetReady(true)

	// Initialize HTTP handlers
	jobsHandler := handlers.NewJobsHandler(sched, pool)
	systemHandler := handlers.NewSystemHandler(sched, pool)
	questdbHandler := handlers.NewQuestDBHandler(questDB)
	marketDataHandler := handlers.NewMarketDataHandler(pool, providers.GetIB(), sqliteDB, questDB)

	// Setup router
	router := setupRouter(cfg, healthServer, m, jobsHandler, systemHandler, questdbHandler, marketDataHandler)

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		logger.Info("server listening", "address", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Worker.ShutdownTimeout)*time.Second)
	defer cancel()

	// Stop scheduler
	sched.Stop()

	// Close ILP client
	if ilpClient != nil {
		ilpClient.Close()
	}

	// Close IB client
	if ibClient := providers.GetIB(); ibClient != nil {
		ibClient.Close()
	}

	// Shutdown HTTP server
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}

	// Shutdown tracing
	if err := tracing.Shutdown(shutdownCtx); err != nil {
		logger.Error("tracing shutdown error", "error", err)
	}

	logger.Info("server stopped")
	return nil
}

func setupRouter(
	cfg *config.Config,
	healthServer *health.Server,
	m *metrics.Metrics,
	jobsHandler *handlers.JobsHandler,
	systemHandler *handlers.SystemHandler,
	questdbHandler *handlers.QuestDBHandler,
	marketDataHandler *handlers.MarketDataHandler,
) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(middleware.CleanPath)

	// Add request logging middleware
	r.Use(func(next http.Handler) http.Handler {
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
	})

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
	})

	return r
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Health checkers
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