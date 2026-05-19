package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  host: "127.0.0.1"
  port: 9090

worker:
  poolSize: 5
  queueCapacity: 50
  shutdownTimeout: 15

scheduler:
  timezone: "America/New_York"

jobs:
  - id: "test-job"
    name: "Test Job"
    cron: "*/5 * * * *"
    type: "test"
    handler: "noop"
    enabled: true
    timeout: 60
    retry:
      maxAttempts: 2
      initialDelay: 500
      maxDelay: 10000
      multiplier: 1.5

metrics:
  enabled: true
  path: "/custom-metrics"

tracing:
  enabled: true
  serviceName: "test-service"
  exporterType: "stdout"
  endpoint: "localhost:4318"
  insecure: true

logging:
  level: "debug"
  format: "text"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	// Test loading
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify server config
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("expected server host 127.0.0.1, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("expected server port 9090, got %d", cfg.Server.Port)
	}

	// Verify worker config
	if cfg.Worker.PoolSize != 5 {
		t.Errorf("expected pool size 5, got %d", cfg.Worker.PoolSize)
	}
	if cfg.Worker.QueueCapacity != 50 {
		t.Errorf("expected queue capacity 50, got %d", cfg.Worker.QueueCapacity)
	}

	// Verify scheduler config
	if cfg.Scheduler.Timezone != "America/New_York" {
		t.Errorf("expected timezone America/New_York, got %s", cfg.Scheduler.Timezone)
	}

	// Verify job config
	if len(cfg.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(cfg.Jobs))
	}
	job := cfg.Jobs[0]
	if job.ID != "test-job" {
		t.Errorf("expected job id test-job, got %s", job.ID)
	}
	if job.Retry.MaxAttempts != 2 {
		t.Errorf("expected max attempts 2, got %d", job.Retry.MaxAttempts)
	}
	if job.Retry.Multiplier != 1.5 {
		t.Errorf("expected multiplier 1.5, got %f", job.Retry.Multiplier)
	}

	// Verify metrics config
	if cfg.Metrics.Path != "/custom-metrics" {
		t.Errorf("expected metrics path /custom-metrics, got %s", cfg.Metrics.Path)
	}

	// Verify logging config
	if cfg.Logging.Level != "debug" {
		t.Errorf("expected log level debug, got %s", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "text" {
		t.Errorf("expected log format text, got %s", cfg.Logging.Format)
	}
}

func TestEnvOverrides(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  host: "0.0.0.0"
  port: 8080
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	// Set environment variables
	os.Setenv("SERVER_HOST", "localhost")
	os.Setenv("SERVER_PORT", "8888")
	defer os.Unsetenv("SERVER_HOST")
	defer os.Unsetenv("SERVER_PORT")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify env overrides
	if cfg.Server.Host != "localhost" {
		t.Errorf("expected host localhost, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 8888 {
		t.Errorf("expected port 8888, got %d", cfg.Server.Port)
	}
}

func TestDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Minimal config
	configContent := `
server:
  port: 8080
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Check defaults
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected default host 0.0.0.0, got %s", cfg.Server.Host)
	}
	if cfg.Worker.PoolSize != 10 {
		t.Errorf("expected default pool size 10, got %d", cfg.Worker.PoolSize)
	}
	if cfg.Worker.QueueCapacity != 100 {
		t.Errorf("expected default queue capacity 100, got %d", cfg.Worker.QueueCapacity)
	}
	if cfg.Scheduler.Timezone != "UTC" {
		t.Errorf("expected default timezone UTC, got %s", cfg.Scheduler.Timezone)
	}
	if cfg.Metrics.Path != "/metrics" {
		t.Errorf("expected default metrics path /metrics, got %s", cfg.Metrics.Path)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("expected default log level info, got %s", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("expected default log format json, got %s", cfg.Logging.Format)
	}
}

func TestLoadNonExistent(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}