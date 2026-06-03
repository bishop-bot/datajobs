package main

import (
	"fmt"
	"os"

	"github.com/bishop-bot/datajobs/internal/config"
	"github.com/bishop-bot/datajobs/internal/logging"
	"github.com/bishop-bot/datajobs/internal/metrics"
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

	// Initialize metrics
	m := metrics.New("datajobs")

	// Create application
	app, err := NewApp(cfg, m)
	if err != nil {
		return fmt.Errorf("failed to create app: %w", err)
	}

	// Setup graceful shutdown
	app.SetupShutdown()

	// Start application
	if err := app.Start(nil); err != nil {
		return fmt.Errorf("failed to start: %w", err)
	}

	// Wait for shutdown signal
	app.WaitForShutdown()

	logger.Info("server stopped")
	return nil
}