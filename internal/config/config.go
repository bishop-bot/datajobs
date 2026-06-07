package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"gopkg.in/yaml.v3"
)

// Config holds all application configuration.
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Worker    WorkerConfig    `yaml:"worker"`
	Scheduler SchedulerConfig `yaml:"scheduler"`
	Database  DatabaseConfig  `yaml:"database"`
	QuestDB   QuestDBConfig   `yaml:"questdb"`
	IB        IBConfig        `yaml:"ib"`
	Jobs      []JobConfig     `yaml:"jobs"`
	Metrics   MetricsConfig   `yaml:"metrics"`
	Tracing   TracingConfig   `yaml:"tracing"`
	Logging   LoggingConfig   `yaml:"logging"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host string `yaml:"host" env:"SERVER_HOST"`
	Port int    `yaml:"port" env:"SERVER_PORT"`
}

// WorkerConfig holds worker pool settings.
type WorkerConfig struct {
	PoolSize        int `yaml:"poolSize" env:"WORKER_POOL_SIZE"`
	QueueCapacity   int `yaml:"queueCapacity" env:"WORKER_QUEUE_CAPACITY"`
	ShutdownTimeout int `yaml:"shutdownTimeout" env:"WORKER_SHUTDOWN_TIMEOUT"`
}

// SchedulerConfig holds scheduler settings.
type SchedulerConfig struct {
	Timezone string `yaml:"timezone" env:"SCHEDULER_TIMEZONE"`
}

// DatabaseConfig holds SQLite settings.
type DatabaseConfig struct {
	Path          string `yaml:"path" env:"DATABASE_PATH"`
	JournalMode   string `yaml:"journalMode" env:"DATABASE_JOURNAL_MODE"`
	MigrationsDir string `yaml:"migrationsDir" env:"DATABASE_MIGRATIONS_DIR"`
}

// QuestDBConfig holds QuestDB settings.
type QuestDBConfig struct {
	Host     string `yaml:"host" env:"QUESTDB_HOST"`
	Port     int    `yaml:"port" env:"QUESTDB_PORT"`
	ILPPort  int    `yaml:"ilpPort" env:"QUESTDB_ILP_PORT"`
	User     string `yaml:"user" env:"QUESTDB_USER"`
	Password string `yaml:"password" env:"QUESTDB_PASSWORD"`
	Database string `yaml:"database" env:"QUESTDB_DATABASE"`
	PoolSize int    `yaml:"poolSize" env:"QUESTDB_POOL_SIZE"`
}

// IBConfig holds Interactive Brokers Web API settings.
type IBConfig struct {
	BaseURL           string        `yaml:"baseURL" env:"IB_BASE_URL"`
	InsecureSkipVerify bool        `yaml:"insecureSkipVerify" env:"IB_INSECURE_SKIP_VERIFY"`
	Timeout           time.Duration `yaml:"timeout" env:"IB_TIMEOUT"`

	// Authentication settings
	Username           string `yaml:"username" env:"IB_USERNAME"`
	Password           string `yaml:"password" env:"IB_PASSWORD"`
	AuthRequired       bool   `yaml:"authRequired" env:"IB_AUTH_REQUIRED"`
	SecondFactorMethod string `yaml:"secondFactorMethod" env:"IB_SECOND_FACTOR_METHOD"` // SMS, TOTP, IBKeyAndroid, IBKeyIOS
	TOTPSecret         string `yaml:"totpSecret" env:"IB_TOTP_SECRET"`
}

// JobConfig holds per-job configuration.
type JobConfig struct {
	ID       string                 `yaml:"id"`
	Name     string                 `yaml:"name"`
	Cron     string                 `yaml:"cron" env:"JOB_CRON"`
	Type     string                 `yaml:"type"`
	Enabled  bool                   `yaml:"enabled" env:"JOB_ENABLED"`
	Retry    RetryConfig            `yaml:"retry"`
	Handler  string                 `yaml:"handler" env:"JOB_HANDLER"`
	Timeout  int                    `yaml:"timeout" env:"JOB_TIMEOUT"` // seconds
	Metadata map[string]interface{} `yaml:"metadata"`
}

// RetryConfig holds retry settings for a job.
type RetryConfig struct {
	MaxAttempts  int     `yaml:"maxAttempts" env:"JOB_RETRY_MAX_ATTEMPTS"`
	InitialDelay int     `yaml:"initialDelay" env:"JOB_RETRY_INITIAL_DELAY"` // milliseconds
	MaxDelay     int     `yaml:"maxDelay" env:"JOB_RETRY_MAX_DELAY"`        // milliseconds
	Multiplier   float64 `yaml:"multiplier" env:"JOB_RETRY_MULTIPLIER"`
}

// MetricsConfig holds Prometheus metrics settings.
type MetricsConfig struct {
	Enabled bool   `yaml:"enabled" env:"METRICS_ENABLED"`
	Path    string `yaml:"path" env:"METRICS_PATH"`
}

// TracingConfig holds OpenTelemetry settings.
type TracingConfig struct {
	Enabled      bool   `yaml:"enabled" env:"TRACING_ENABLED"`
	ServiceName  string `yaml:"serviceName" env:"TRACING_SERVICE_NAME"`
	ExporterType string `yaml:"exporterType" env:"TRACING_EXPORTER"` // otlp, jaeger, stdout
	Endpoint     string `yaml:"endpoint" env:"TRACING_ENDPOINT"`
	Insecure     bool   `yaml:"insecure" env:"TRACING_INSECURE"`
}

// LoggingConfig holds structured logging settings.
type LoggingConfig struct {
	Level  string `yaml:"level" env:"LOG_LEVEL"`
	Format string `yaml:"format" env:"LOG_FORMAT"` // json, text
}

// Load reads configuration from a YAML file with environment variable overrides.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	applyEnvOverrides(&cfg)
	setDefaults(&cfg)

	return &cfg, nil
}

// Helper functions for environment variable overrides
func setString(v *string, key string) {
	if val := os.Getenv(key); val != "" {
		*v = val
	}
}

func setInt(v *int, key string) {
	if s := os.Getenv(key); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			*v = n
		}
	}
}

func setBool(v *bool, key string) {
	if s := os.Getenv(key); s != "" {
		*v = s == "true" || s == "1"
	}
}

func setDuration(v *time.Duration, key string) {
	if s := os.Getenv(key); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			*v = time.Duration(n) * time.Second
		}
	}
}

func setFloat(v *float64, key string) {
	if s := os.Getenv(key); s != "" {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			*v = f
		}
	}
}

func applyEnvOverrides(cfg *Config) {
	// Server
	setString(&cfg.Server.Host, "SERVER_HOST")
	setInt(&cfg.Server.Port, "SERVER_PORT")

	// Worker
	setInt(&cfg.Worker.PoolSize, "WORKER_POOL_SIZE")
	setInt(&cfg.Worker.QueueCapacity, "WORKER_QUEUE_CAPACITY")
	setInt(&cfg.Worker.ShutdownTimeout, "WORKER_SHUTDOWN_TIMEOUT")

	// Scheduler
	setString(&cfg.Scheduler.Timezone, "SCHEDULER_TIMEZONE")

	// Metrics
	setBool(&cfg.Metrics.Enabled, "METRICS_ENABLED")
	setString(&cfg.Metrics.Path, "METRICS_PATH")

	// Tracing
	setBool(&cfg.Tracing.Enabled, "TRACING_ENABLED")
	setString(&cfg.Tracing.ServiceName, "TRACING_SERVICE_NAME")
	setString(&cfg.Tracing.ExporterType, "TRACING_EXPORTER")
	setString(&cfg.Tracing.Endpoint, "TRACING_ENDPOINT")
	setBool(&cfg.Tracing.Insecure, "TRACING_INSECURE")

	// Logging
	setString(&cfg.Logging.Level, "LOG_LEVEL")
	setString(&cfg.Logging.Format, "LOG_FORMAT")

	// Database
	setString(&cfg.Database.Path, "DATABASE_PATH")
	setString(&cfg.Database.JournalMode, "DATABASE_JOURNAL_MODE")
	setString(&cfg.Database.MigrationsDir, "DATABASE_MIGRATIONS_DIR")

	// QuestDB
	setString(&cfg.QuestDB.Host, "QUESTDB_HOST")
	setInt(&cfg.QuestDB.Port, "QUESTDB_PORT")
	setInt(&cfg.QuestDB.ILPPort, "QUESTDB_ILP_PORT")
	setString(&cfg.QuestDB.User, "QUESTDB_USER")
	setString(&cfg.QuestDB.Password, "QUESTDB_PASSWORD")
	setString(&cfg.QuestDB.Database, "QUESTDB_DATABASE")
	setInt(&cfg.QuestDB.PoolSize, "QUESTDB_POOL_SIZE")

	// IB
	setString(&cfg.IB.BaseURL, "IB_BASE_URL")
	setBool(&cfg.IB.InsecureSkipVerify, "IB_INSECURE_SKIP_VERIFY")
	setDuration(&cfg.IB.Timeout, "IB_TIMEOUT")
	setString(&cfg.IB.Username, "IB_USERNAME")
	setString(&cfg.IB.Password, "IB_PASSWORD")
	setBool(&cfg.IB.AuthRequired, "IB_AUTH_REQUIRED")
	setString(&cfg.IB.SecondFactorMethod, "IB_SECOND_FACTOR_METHOD")
	setString(&cfg.IB.TOTPSecret, "IB_TOTP_SECRET")

	// Per-job overrides
	for i := range cfg.Jobs {
		prefix := "JOB_" + strconv.Itoa(i) + "_"
		setBool(&cfg.Jobs[i].Enabled, prefix+"ENABLED")
		setString(&cfg.Jobs[i].Cron, prefix+"CRON")
		setString(&cfg.Jobs[i].Handler, prefix+"HANDLER")
		setInt(&cfg.Jobs[i].Timeout, prefix+"TIMEOUT")
		setInt(&cfg.Jobs[i].Retry.MaxAttempts, prefix+"RETRY_MAX_ATTEMPTS")
		setInt(&cfg.Jobs[i].Retry.InitialDelay, prefix+"RETRY_INITIAL_DELAY")
		setInt(&cfg.Jobs[i].Retry.MaxDelay, prefix+"RETRY_MAX_DELAY")
		setFloat(&cfg.Jobs[i].Retry.Multiplier, prefix+"RETRY_MULTIPLIER")
	}
}

func setDefaults(cfg *Config) {
	// Server defaults
	missingString(&cfg.Server.Host, "0.0.0.0")
	zeroInt(&cfg.Server.Port, 8080)

	// Worker defaults
	zeroInt(&cfg.Worker.PoolSize, 10)
	zeroInt(&cfg.Worker.QueueCapacity, 100)
	zeroInt(&cfg.Worker.ShutdownTimeout, 30)

	// Scheduler defaults
	missingString(&cfg.Scheduler.Timezone, "UTC")

	// Database defaults
	missingString(&cfg.Database.Path, "datajobs.db")
	missingString(&cfg.Database.JournalMode, "WAL")
	missingString(&cfg.Database.MigrationsDir, "migrations")

	// QuestDB defaults
	missingString(&cfg.QuestDB.Host, "localhost")
	zeroInt(&cfg.QuestDB.Port, 8812)
	zeroInt(&cfg.QuestDB.ILPPort, 9009)
	missingString(&cfg.QuestDB.User, "admin")
	missingString(&cfg.QuestDB.Password, "quest")
	missingString(&cfg.QuestDB.Database, "qdb")
	zeroInt(&cfg.QuestDB.PoolSize, 10)

	// IB defaults
	missingString(&cfg.IB.BaseURL, "https://localhost:5001")
	zeroDuration(&cfg.IB.Timeout, 30*time.Second)

	// Metrics defaults
	missingString(&cfg.Metrics.Path, "/metrics")

	// Tracing defaults
	missingString(&cfg.Tracing.ServiceName, "datajobs")

	// Logging defaults
	missingString(&cfg.Logging.Level, "info")
	missingString(&cfg.Logging.Format, "json")

	// Job defaults
	for i := range cfg.Jobs {
		zeroInt(&cfg.Jobs[i].Timeout, 300)
		zeroInt(&cfg.Jobs[i].Retry.MaxAttempts, 3)
		zeroInt(&cfg.Jobs[i].Retry.InitialDelay, 1000)
		zeroInt(&cfg.Jobs[i].Retry.MaxDelay, 60000)
		zeroFloat(&cfg.Jobs[i].Retry.Multiplier, 2.0)
	}
}

// Defaults helpers
func missingString(v *string, defaultVal string) {
	if *v == "" {
		*v = defaultVal
	}
}

func zeroInt(v *int, defaultVal int) {
	if *v == 0 {
		*v = defaultVal
	}
}

func zeroDuration(v *time.Duration, defaultVal time.Duration) {
	if *v == 0 {
		*v = defaultVal
	}
}

func zeroFloat(v *float64, defaultVal float64) {
	if *v == 0 {
		*v = defaultVal
	}
}

// MergeJobConfig merges runtime overrides into a job config (for API updates).
func MergeJobConfig(base *JobConfig, overrides map[string]interface{}) error {
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		ErrorUnused: true,
		Result:      base,
	})
	if err != nil {
		return err
	}
	return dec.Decode(overrides)
}

// GetEnv returns environment variable or default value.
func GetEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// GetEnvInt returns environment variable as int or default value.
func GetEnvInt(key string, defaultVal int) int {
	if s := os.Getenv(key); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			return n
		}
	}
	return defaultVal
}

// GetEnvBool returns environment variable as bool or default value.
func GetEnvBool(key string, defaultVal bool) bool {
	if s := os.Getenv(key); s != "" {
		return s == "true" || s == "1"
	}
	return defaultVal
}

// GetEnvDuration returns environment variable as duration (seconds) or default value.
func GetEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if s := os.Getenv(key); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			return time.Duration(n) * time.Second
		}
	}
	return defaultVal
}

// PrefixEnv returns all environment variables with a given prefix as a map.
func PrefixEnv(prefix string) map[string]string {
	result := make(map[string]string)
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, prefix) {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimPrefix(parts[0], prefix)
				result[key] = parts[1]
			}
		}
	}
	return result
}