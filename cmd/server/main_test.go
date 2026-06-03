package main

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/bishop-bot/datajobs/internal/config"
	"github.com/bishop-bot/datajobs/internal/logging"
	"github.com/bishop-bot/datajobs/internal/metrics"
)

// projectRoot returns the absolute path to the project root.
func projectRoot() string {
	// This assumes tests are run from within cmd/server
	return filepath.Join("..", "..")
}

func init() {
	// Initialize logging for tests
	logging.Init(logging.Config{Level: "error", Format: "json"})
}

func TestAppCreation(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Server:      config.ServerConfig{Host: "127.0.0.1", Port: 9090},
		Worker:      config.WorkerConfig{PoolSize: 2, QueueCapacity: 10},
		Database:    config.DatabaseConfig{Path: tmpDir + "/test.db", JournalMode: "WAL", MigrationsDir: filepath.Join(projectRoot(), "migrations")},
	}
	m := metrics.New("test-app")
	app, err := NewApp(cfg, m)
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}
	if app == nil {
		t.Fatal("expected non-nil app")
	}

	// Start and stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := app.Start(ctx); err != nil {
		t.Fatalf("failed to start app: %v", err)
	}
	app.Stop()
}

func TestAppHealthChecks(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Server:      config.ServerConfig{Host: "127.0.0.1", Port: 9091},
		Worker:      config.WorkerConfig{PoolSize: 1, QueueCapacity: 5},
		Database:    config.DatabaseConfig{Path: tmpDir + "/test.db", JournalMode: "WAL", MigrationsDir: filepath.Join(projectRoot(), "migrations")},
	}
	m := metrics.New("test-health")
	app, err := NewApp(cfg, m)
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	// App should have health server
	if app.HealthServer() == nil {
		t.Error("expected health server")
	}

	app.Stop()
}

func TestAppComponents(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Server:      config.ServerConfig{Host: "127.0.0.1", Port: 9092},
		Worker:      config.WorkerConfig{PoolSize: 1, QueueCapacity: 5},
		Scheduler:   config.SchedulerConfig{Timezone: "UTC"},
		Database:    config.DatabaseConfig{Path: tmpDir + "/test.db", JournalMode: "WAL", MigrationsDir: filepath.Join(projectRoot(), "migrations")},
	}
	m := metrics.New("test-components")
	app, err := NewApp(cfg, m)
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	// Check components are initialized
	if app.Scheduler() == nil {
		t.Error("expected scheduler")
	}
	if app.Pool() == nil {
		t.Error("expected worker pool")
	}
	if app.Config() == nil {
		t.Error("expected config")
	}

	app.Stop()
}

func TestAppDoubleStart(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Server:      config.ServerConfig{Host: "127.0.0.1", Port: 9093},
		Worker:      config.WorkerConfig{PoolSize: 1, QueueCapacity: 5},
		Database:    config.DatabaseConfig{Path: tmpDir + "/test.db", JournalMode: "WAL", MigrationsDir: filepath.Join(projectRoot(), "migrations")},
	}
	m := metrics.New("test-double")
	app, err := NewApp(cfg, m)
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	ctx := context.Background()
	if err := app.Start(ctx); err != nil {
		t.Fatalf("first start failed: %v", err)
	}

	// Second start should fail
	if err := app.Start(ctx); err == nil {
		t.Error("expected error on double start")
	}

	app.Stop()
}

func TestAppDoubleStop(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Server:      config.ServerConfig{Host: "127.0.0.1", Port: 9094},
		Worker:      config.WorkerConfig{PoolSize: 1, QueueCapacity: 5},
		Database:    config.DatabaseConfig{Path: tmpDir + "/test.db", JournalMode: "WAL", MigrationsDir: filepath.Join(projectRoot(), "migrations")},
	}
	m := metrics.New("test-double-stop")
	app, err := NewApp(cfg, m)
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	ctx := context.Background()
	app.Start(ctx)
	app.Stop()
	app.Stop() // Should not panic
}

func TestAppNotStarted(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Server:      config.ServerConfig{Host: "127.0.0.1", Port: 9095},
		Worker:      config.WorkerConfig{PoolSize: 1, QueueCapacity: 5},
		Database:    config.DatabaseConfig{Path: tmpDir + "/test.db", JournalMode: "WAL", MigrationsDir: filepath.Join(projectRoot(), "migrations")},
	}
	m := metrics.New("test-not-started")
	app, err := NewApp(cfg, m)
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	// Stop without start should not panic
	app.Stop()
}

func TestAppConcurrency(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Server:      config.ServerConfig{Host: "127.0.0.1", Port: 9096},
		Worker:      config.WorkerConfig{PoolSize: 5, QueueCapacity: 50},
		Database:    config.DatabaseConfig{Path: tmpDir + "/test.db", JournalMode: "WAL", MigrationsDir: filepath.Join(projectRoot(), "migrations")},
	}
	m := metrics.New("test-concurrent")
	app, err := NewApp(cfg, m)
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	ctx := context.Background()
	var wg sync.WaitGroup

	// Try starting from multiple goroutines
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			app.Start(ctx)
		}()
	}

	time.Sleep(100 * time.Millisecond)
	app.Stop()
	wg.Wait()
}