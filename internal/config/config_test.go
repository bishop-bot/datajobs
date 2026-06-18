package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
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

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify values
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("expected server host 127.0.0.1, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("expected server port 9090, got %d", cfg.Server.Port)
	}
	if cfg.Worker.PoolSize != 5 {
		t.Errorf("expected pool size 5, got %d", cfg.Worker.PoolSize)
	}
	if cfg.Scheduler.Timezone != "America/New_York" {
		t.Errorf("expected timezone America/New_York, got %s", cfg.Scheduler.Timezone)
	}
	if len(cfg.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(cfg.Jobs))
	}
	if cfg.Jobs[0].Retry.Multiplier != 1.5 {
		t.Errorf("expected multiplier 1.5, got %f", cfg.Jobs[0].Retry.Multiplier)
	}
}

func TestEnvOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  host: "default"
  port: 8080
worker:
  poolSize: 1
  queueCapacity: 10
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	// Test all helper functions via env vars
	os.Setenv("SERVER_HOST", "localhost")
	os.Setenv("SERVER_PORT", "9999")
	os.Setenv("WORKER_POOL_SIZE", "20")
	os.Setenv("WORKER_QUEUE_CAPACITY", "200")
	os.Setenv("WORKER_SHUTDOWN_TIMEOUT", "60")
	os.Setenv("SCHEDULER_TIMEZONE", "Asia/Tokyo")
	os.Setenv("METRICS_ENABLED", "true")
	os.Setenv("METRICS_PATH", "/metrics-v2")
	os.Setenv("TRACING_ENABLED", "1")
	os.Setenv("TRACING_SERVICE_NAME", "custom")
	os.Setenv("TRACING_EXPORTER", "otlp")
	os.Setenv("TRACING_ENDPOINT", "otel:4317")
	os.Setenv("TRACING_INSECURE", "false")
	os.Setenv("LOG_LEVEL", "warn")
	os.Setenv("LOG_FORMAT", "text")
	os.Setenv("DATABASE_PATH", "/data/custom.db")
	os.Setenv("DATABASE_JOURNAL_MODE", "DELETE")
	os.Setenv("DATABASE_MIGRATIONS_DIR", "/mig")
	os.Setenv("QUESTDB_HOST", "questdb.example.com")
	os.Setenv("QUESTDB_PORT", "5432")
	os.Setenv("QUESTDB_ILP_PORT", "9009")
	os.Setenv("QUESTDB_USER", "custom-user")
	os.Setenv("QUESTDB_PASSWORD", "secret")
	os.Setenv("QUESTDB_DATABASE", "custom-qdb")
	os.Setenv("QUESTDB_POOL_SIZE", "25")
	os.Setenv("IB_BASE_URL", "https://ib.example.com")
	os.Setenv("IB_INSECURE_SKIP_VERIFY", "true")
	os.Setenv("IB_TIMEOUT", "120")
	defer func() {
		for _, k := range []string{
			"SERVER_HOST", "SERVER_PORT", "WORKER_POOL_SIZE", "WORKER_QUEUE_CAPACITY",
			"WORKER_SHUTDOWN_TIMEOUT", "SCHEDULER_TIMEZONE", "METRICS_ENABLED", "METRICS_PATH",
			"TRACING_ENABLED", "TRACING_SERVICE_NAME", "TRACING_EXPORTER", "TRACING_ENDPOINT",
			"TRACING_INSECURE", "LOG_LEVEL", "LOG_FORMAT", "DATABASE_PATH", "DATABASE_JOURNAL_MODE",
			"DATABASE_MIGRATIONS_DIR", "QUESTDB_HOST", "QUESTDB_PORT", "QUESTDB_ILP_PORT",
			"QUESTDB_USER", "QUESTDB_PASSWORD", "QUESTDB_DATABASE", "QUESTDB_POOL_SIZE",
			"IB_BASE_URL", "IB_INSECURE_SKIP_VERIFY", "IB_TIMEOUT",
		} {
			os.Unsetenv(k)
		}
	}()

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify all overrides
	if cfg.Server.Host != "localhost" {
		t.Errorf("SERVER_HOST: got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 9999 {
		t.Errorf("SERVER_PORT: got %d", cfg.Server.Port)
	}
	if cfg.Worker.PoolSize != 20 {
		t.Errorf("WORKER_POOL_SIZE: got %d", cfg.Worker.PoolSize)
	}
	if cfg.Worker.QueueCapacity != 200 {
		t.Errorf("WORKER_QUEUE_CAPACITY: got %d", cfg.Worker.QueueCapacity)
	}
	if cfg.Scheduler.Timezone != "Asia/Tokyo" {
		t.Errorf("SCHEDULER_TIMEZONE: got %s", cfg.Scheduler.Timezone)
	}
	if !cfg.Metrics.Enabled {
		t.Error("METRICS_ENABLED: expected true")
	}
	if cfg.Metrics.Path != "/metrics-v2" {
		t.Errorf("METRICS_PATH: got %s", cfg.Metrics.Path)
	}
	if !cfg.Tracing.Enabled {
		t.Error("TRACING_ENABLED: expected true")
	}
	if cfg.Tracing.ServiceName != "custom" {
		t.Errorf("TRACING_SERVICE_NAME: got %s", cfg.Tracing.ServiceName)
	}
	if cfg.Tracing.ExporterType != "otlp" {
		t.Errorf("TRACING_EXPORTER: got %s", cfg.Tracing.ExporterType)
	}
	if cfg.Tracing.Endpoint != "otel:4317" {
		t.Errorf("TRACING_ENDPOINT: got %s", cfg.Tracing.Endpoint)
	}
	if cfg.Tracing.Insecure {
		t.Error("TRACING_INSECURE: expected false")
	}
	if cfg.Logging.Level != "warn" {
		t.Errorf("LOG_LEVEL: got %s", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "text" {
		t.Errorf("LOG_FORMAT: got %s", cfg.Logging.Format)
	}
	if cfg.Database.Path != "/data/custom.db" {
		t.Errorf("DATABASE_PATH: got %s", cfg.Database.Path)
	}
	if cfg.QuestDB.Host != "questdb.example.com" {
		t.Errorf("QUESTDB_HOST: got %s", cfg.QuestDB.Host)
	}
	if cfg.QuestDB.Port != 5432 {
		t.Errorf("QUESTDB_PORT: got %d", cfg.QuestDB.Port)
	}
	if cfg.IB.BaseURL != "https://ib.example.com" {
		t.Errorf("IB_BASE_URL: got %s", cfg.IB.BaseURL)
	}
	if !cfg.IB.InsecureSkipVerify {
		t.Error("IB_INSECURE_SKIP_VERIFY: expected true")
	}
	if cfg.IB.Timeout != 120*time.Second {
		t.Errorf("IB_TIMEOUT: got %v", cfg.IB.Timeout)
	}
}

func TestEnvOverridesJob(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
jobs:
  - id: "job-0"
    name: "Job 0"
    enabled: true
  - id: "job-1"
    name: "Job 1"
    enabled: false
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	os.Setenv("JOB_0_ENABLED", "false")
	os.Setenv("JOB_0_CRON", "*/10 * * * *")
	os.Setenv("JOB_0_HANDLER", "custom-handler")
	os.Setenv("JOB_0_TIMEOUT", "120")
	os.Setenv("JOB_0_RETRY_MAX_ATTEMPTS", "5")
	os.Setenv("JOB_0_RETRY_INITIAL_DELAY", "5000")
	os.Setenv("JOB_0_RETRY_MAX_DELAY", "30000")
	os.Setenv("JOB_0_RETRY_MULTIPLIER", "3.0")
	defer func() {
		os.Unsetenv("JOB_0_ENABLED")
		os.Unsetenv("JOB_0_CRON")
		os.Unsetenv("JOB_0_HANDLER")
		os.Unsetenv("JOB_0_TIMEOUT")
		os.Unsetenv("JOB_0_RETRY_MAX_ATTEMPTS")
		os.Unsetenv("JOB_0_RETRY_INITIAL_DELAY")
		os.Unsetenv("JOB_0_RETRY_MAX_DELAY")
		os.Unsetenv("JOB_0_RETRY_MULTIPLIER")
	}()

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Job 0 should be overridden
	job := cfg.Jobs[0]
	if job.Enabled {
		t.Error("JOB_0_ENABLED: expected false")
	}
	if job.Cron != "*/10 * * * *" {
		t.Errorf("JOB_0_CRON: got %s", job.Cron)
	}
	if job.Handler != "custom-handler" {
		t.Errorf("JOB_0_HANDLER: got %s", job.Handler)
	}
	if job.Timeout != 120 {
		t.Errorf("JOB_0_TIMEOUT: got %d", job.Timeout)
	}
	if job.Retry.MaxAttempts != 5 {
		t.Errorf("JOB_0_RETRY_MAX_ATTEMPTS: got %d", job.Retry.MaxAttempts)
	}
	if job.Retry.InitialDelay != 5000 {
		t.Errorf("JOB_0_RETRY_INITIAL_DELAY: got %d", job.Retry.InitialDelay)
	}
	if job.Retry.MaxDelay != 30000 {
		t.Errorf("JOB_0_RETRY_MAX_DELAY: got %d", job.Retry.MaxDelay)
	}
	if job.Retry.Multiplier != 3.0 {
		t.Errorf("JOB_0_RETRY_MULTIPLIER: got %f", job.Retry.Multiplier)
	}

	// Job 1 should be unchanged
	if cfg.Jobs[1].Enabled {
		t.Error("JOB_1: expected to remain enabled=false")
	}
}

func TestDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

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
		t.Errorf("Server.Host: got %s", cfg.Server.Host)
	}
	if cfg.Worker.PoolSize != 10 {
		t.Errorf("Worker.PoolSize: got %d", cfg.Worker.PoolSize)
	}
	if cfg.Worker.QueueCapacity != 100 {
		t.Errorf("Worker.QueueCapacity: got %d", cfg.Worker.QueueCapacity)
	}
	if cfg.Scheduler.Timezone != "UTC" {
		t.Errorf("Scheduler.Timezone: got %s", cfg.Scheduler.Timezone)
	}
	if cfg.Metrics.Path != "/metrics" {
		t.Errorf("Metrics.Path: got %s", cfg.Metrics.Path)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("Logging.Level: got %s", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("Logging.Format: got %s", cfg.Logging.Format)
	}
	if cfg.QuestDB.Port != 8812 {
		t.Errorf("QuestDB.Port: got %d", cfg.QuestDB.Port)
	}
	if cfg.QuestDB.ILPPort != 9009 {
		t.Errorf("QuestDB.ILPPort: got %d", cfg.QuestDB.ILPPort)
	}
	if cfg.QuestDB.ILPHTTPPort != 9000 {
		t.Errorf("QuestDB.ILPHTTPPort: got %d", cfg.QuestDB.ILPHTTPPort)
	}
	if cfg.IB.Timeout != 30*time.Second {
		t.Errorf("IB.Timeout: got %v", cfg.IB.Timeout)
	}
}

func TestLoadNonExistent(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestEnvOverridesInvalidValues(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  port: 8080
worker:
  poolSize: 5
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	// Invalid values should be ignored
	os.Setenv("SERVER_PORT", "not-a-number")
	os.Setenv("WORKER_POOL_SIZE", "invalid")
	os.Setenv("IB_TIMEOUT", "bad")
	defer func() {
		os.Unsetenv("SERVER_PORT")
		os.Unsetenv("WORKER_POOL_SIZE")
		os.Unsetenv("IB_TIMEOUT")
	}()

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Original values should remain
	if cfg.Server.Port != 8080 {
		t.Errorf("SERVER_PORT: got %d", cfg.Server.Port)
	}
	if cfg.Worker.PoolSize != 5 {
		t.Errorf("WORKER_POOL_SIZE: got %d", cfg.Worker.PoolSize)
	}
}

func TestGetEnvHelpers(t *testing.T) {
	os.Setenv("TEST_STRING", "hello")
	os.Setenv("TEST_INT", "42")
	os.Setenv("TEST_BOOL_TRUE", "true")
	os.Setenv("TEST_BOOL_1", "1")
	os.Setenv("TEST_BOOL_FALSE", "false")
	os.Setenv("TEST_DURATION", "60")
	defer func() {
		os.Unsetenv("TEST_STRING")
		os.Unsetenv("TEST_INT")
		os.Unsetenv("TEST_BOOL_TRUE")
		os.Unsetenv("TEST_BOOL_1")
		os.Unsetenv("TEST_BOOL_FALSE")
		os.Unsetenv("TEST_DURATION")
	}()

	// Test GetEnv
	if got := GetEnv("TEST_STRING", "default"); got != "hello" {
		t.Errorf("GetEnv: got %s", got)
	}
	if got := GetEnv("MISSING", "default"); got != "default" {
		t.Errorf("GetEnv missing: got %s", got)
	}

	// Test GetEnvInt
	if got := GetEnvInt("TEST_INT", 0); got != 42 {
		t.Errorf("GetEnvInt: got %d", got)
	}
	if got := GetEnvInt("MISSING", 99); got != 99 {
		t.Errorf("GetEnvInt missing: got %d", got)
	}

	// Test GetEnvBool
	if !GetEnvBool("TEST_BOOL_TRUE", false) {
		t.Error("GetEnvBool true")
	}
	if !GetEnvBool("TEST_BOOL_1", false) {
		t.Error("GetEnvBool 1")
	}
	if GetEnvBool("TEST_BOOL_FALSE", true) {
		t.Error("GetEnvBool false")
	}
	if GetEnvBool("MISSING", false) {
		t.Error("GetEnvBool missing")
	}

	// Test GetEnvDuration
	if got := GetEnvDuration("TEST_DURATION", 0); got != 60*time.Second {
		t.Errorf("GetEnvDuration: got %v", got)
	}
	if got := GetEnvDuration("MISSING", 30*time.Second); got != 30*time.Second {
		t.Errorf("GetEnvDuration missing: got %v", got)
	}
}

func TestPrefixEnv(t *testing.T) {
	os.Setenv("TEST_PREFIX_A", "value-a")
	os.Setenv("TEST_PREFIX_B", "value-b")
	os.Setenv("TEST_PREFIX_C", "value-c")
	os.Setenv("OTHER_VAR", "ignored")
	defer func() {
		os.Unsetenv("TEST_PREFIX_A")
		os.Unsetenv("TEST_PREFIX_B")
		os.Unsetenv("TEST_PREFIX_C")
		os.Unsetenv("OTHER_VAR")
	}()

	result := PrefixEnv("TEST_PREFIX_")
	if len(result) != 3 {
		t.Fatalf("expected 3 results, got %d", len(result))
	}
	if result["A"] != "value-a" {
		t.Errorf("A: got %s", result["A"])
	}
	if result["B"] != "value-b" {
		t.Errorf("B: got %s", result["B"])
	}
	if result["C"] != "value-c" {
		t.Errorf("C: got %s", result["C"])
	}
}

func TestSetHelpers(t *testing.T) {
	// Test setString
	var s string
	setString(&s, "MISSING")
	if s != "" {
		t.Error("setString: should not set for missing env")
	}
	os.Setenv("TEST_SET_STRING", "value")
	setString(&s, "TEST_SET_STRING")
	if s != "value" {
		t.Errorf("setString: got %s", s)
	}
	os.Unsetenv("TEST_SET_STRING")

	// Test setInt
	var i int
	setInt(&i, "MISSING")
	if i != 0 {
		t.Error("setInt: should not set for missing env")
	}
	os.Setenv("TEST_SET_INT", "123")
	setInt(&i, "TEST_SET_INT")
	if i != 123 {
		t.Errorf("setInt: got %d", i)
	}
	os.Unsetenv("TEST_SET_INT")

	// Test setBool
	var b bool
	os.Setenv("TEST_SET_BOOL", "true")
	setBool(&b, "TEST_SET_BOOL")
	if !b {
		t.Error("setBool: got false")
	}
	os.Unsetenv("TEST_SET_BOOL")

	// Test setFloat
	var f float64
	os.Setenv("TEST_SET_FLOAT", "3.14")
	setFloat(&f, "TEST_SET_FLOAT")
	if f != 3.14 {
		t.Errorf("setFloat: got %f", f)
	}
	os.Unsetenv("TEST_SET_FLOAT")

	// Test setDuration
	var d time.Duration
	os.Setenv("TEST_SET_DURATION", "45")
	setDuration(&d, "TEST_SET_DURATION")
	if d != 45*time.Second {
		t.Errorf("setDuration: got %v", d)
	}
	os.Unsetenv("TEST_SET_DURATION")
}

func TestDefaultsHelpers(t *testing.T) {
	// Test missingString
	var s string = ""
	missingString(&s, "default")
	if s != "default" {
		t.Errorf("missingString: got %s", s)
	}
	s = "existing"
	missingString(&s, "default")
	if s != "existing" {
		t.Errorf("missingString: should not override")
	}

	// Test zeroInt
	var i int = 0
	zeroInt(&i, 99)
	if i != 99 {
		t.Errorf("zeroInt: got %d", i)
	}
	i = 5
	zeroInt(&i, 99)
	if i != 5 {
		t.Errorf("zeroInt: should not override")
	}

	// Test zeroFloat
	var f float64 = 0
	zeroFloat(&f, 1.5)
	if f != 1.5 {
		t.Errorf("zeroFloat: got %f", f)
	}
	f = 2.5
	zeroFloat(&f, 1.5)
	if f != 2.5 {
		t.Errorf("zeroFloat: should not override")
	}

	// Test zeroDuration
	var d time.Duration = 0
	zeroDuration(&d, 30*time.Second)
	if d != 30*time.Second {
		t.Errorf("zeroDuration: got %v", d)
	}
	d = 60 * time.Second
	zeroDuration(&d, 30*time.Second)
	if d != 60*time.Second {
		t.Errorf("zeroDuration: should not override")
	}
}

func TestMergeJobConfig(t *testing.T) {
	base := &JobConfig{
		Enabled: true,
		Timeout: 60,
		Retry: RetryConfig{
			MaxAttempts:  3,
			InitialDelay: 1000,
		},
	}

	overrides := map[string]interface{}{
		"enabled": false,
		"timeout": 120,
		"retry": map[string]interface{}{
			"maxAttempts":  5,
			"initialDelay": 2000,
			"multiplier":   2.5,
		},
	}

	err := MergeJobConfig(base, overrides)
	if err != nil {
		t.Fatalf("MergeJobConfig failed: %v", err)
	}

	if base.Enabled {
		t.Error("Enabled should be false")
	}
	if base.Timeout != 120 {
		t.Errorf("Timeout: got %d", base.Timeout)
	}
	if base.Retry.MaxAttempts != 5 {
		t.Errorf("Retry.MaxAttempts: got %d", base.Retry.MaxAttempts)
	}
	if base.Retry.InitialDelay != 2000 {
		t.Errorf("Retry.InitialDelay: got %d", base.Retry.InitialDelay)
	}
	if base.Retry.Multiplier != 2.5 {
		t.Errorf("Retry.Multiplier: got %f", base.Retry.Multiplier)
	}
}