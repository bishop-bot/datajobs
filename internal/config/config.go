package config

import (
	"os"
	"strconv"

	"github.com/mitchellh/mapstructure"
	"gopkg.in/yaml.v3"
)

// Config holds all application configuration.
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Worker   WorkerConfig   `yaml:"worker"`
	Scheduler SchedulerConfig `yaml:"scheduler"`
	Jobs     []JobConfig    `yaml:"jobs"`
	Metrics  MetricsConfig  `yaml:"metrics"`
	Tracing  TracingConfig  `yaml:"tracing"`
	Logging  LoggingConfig  `yaml:"logging"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host string `yaml:"host" env:"SERVER_HOST"`
	Port int    `yaml:"port" env:"SERVER_PORT"`
}

// WorkerConfig holds worker pool settings.
type WorkerConfig struct {
	PoolSize       int `yaml:"poolSize" env:"WORKER_POOL_SIZE"`
	QueueCapacity  int `yaml:"queueCapacity" env:"WORKER_QUEUE_CAPACITY"`
	ShutdownTimeout int `yaml:"shutdownTimeout" env:"WORKER_SHUTDOWN_TIMEOUT"`
}

// SchedulerConfig holds scheduler settings.
type SchedulerConfig struct {
	Timezone string `yaml:"timezone" env:"SCHEDULER_TIMEZONE"`
}

// JobConfig holds per-job configuration.
type JobConfig struct {
	ID          string                 `yaml:"id" env:"JOB_ID"`
	Name        string                 `yaml:"name" env:"JOB_NAME"`
	Cron        string                 `yaml:"cron" env:"JOB_CRON"`
	Type        string                 `yaml:"type" env:"JOB_TYPE"`
	Enabled     bool                   `yaml:"enabled" env:"JOB_ENABLED"`
	Retry       RetryConfig            `yaml:"retry"`
	Handler     string                 `yaml:"handler" env:"JOB_HANDLER"`
	Timeout     int                    `yaml:"timeout" env:"JOB_TIMEOUT"` // seconds
	Metadata    map[string]interface{} `yaml:"metadata"`
}

// RetryConfig holds retry settings for a job.
type RetryConfig struct {
	MaxAttempts int     `yaml:"maxAttempts" env:"JOB_RETRY_MAX_ATTEMPTS"`
	InitialDelay int    `yaml:"initialDelay" env:"JOB_RETRY_INITIAL_DELAY"` // milliseconds
	MaxDelay     int     `yaml:"maxDelay" env:"JOB_RETRY_MAX_DELAY"`       // milliseconds
	Multiplier   float64 `yaml:"multiplier" env:"JOB_RETRY_MULTIPLIER"`
}

// MetricsConfig holds Prometheus metrics settings.
type MetricsConfig struct {
	Enabled bool   `yaml:"enabled" env:"METRICS_ENABLED"`
	Path    string `yaml:"path" env:"METRICS_PATH"`
}

// TracingConfig holds OpenTelemetry settings.
type TracingConfig struct {
	Enabled        bool   `yaml:"enabled" env:"TRACING_ENABLED"`
	ServiceName    string `yaml:"serviceName" env:"TRACING_SERVICE_NAME"`
	ExporterType   string `yaml:"exporterType" env:"TRACING_EXPORTER"` // otlp, jaeger, stdout
	Endpoint       string `yaml:"endpoint" env:"TRACING_ENDPOINT"`
	Insecure       bool   `yaml:"insecure" env:"TRACING_INSECURE"`
}

// LoggingConfig holds structured logging settings.
type LoggingConfig struct {
	Level      string `yaml:"level" env:"LOG_LEVEL"`
	Format     string `yaml:"format" env:"LOG_FORMAT"` // json, text
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

	// Apply environment variable overrides
	applyEnvOverrides(&cfg)

	// Set defaults
	setDefaults(&cfg)

	return &cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	// Server
	if v := os.Getenv("SERVER_HOST"); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv("SERVER_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = port
		}
	}

	// Worker
	if v := os.Getenv("WORKER_POOL_SIZE"); v != "" {
		if size, err := strconv.Atoi(v); err == nil {
			cfg.Worker.PoolSize = size
		}
	}
	if v := os.Getenv("WORKER_QUEUE_CAPACITY"); v != "" {
		if cap, err := strconv.Atoi(v); err == nil {
			cfg.Worker.QueueCapacity = cap
		}
	}
	if v := os.Getenv("WORKER_SHUTDOWN_TIMEOUT"); v != "" {
		if t, err := strconv.Atoi(v); err == nil {
			cfg.Worker.ShutdownTimeout = t
		}
	}

	// Scheduler
	if v := os.Getenv("SCHEDULER_TIMEZONE"); v != "" {
		cfg.Scheduler.Timezone = v
	}

	// Metrics
	if v := os.Getenv("METRICS_ENABLED"); v != "" {
		cfg.Metrics.Enabled = v == "true" || v == "1"
	}
	if v := os.Getenv("METRICS_PATH"); v != "" {
		cfg.Metrics.Path = v
	}

	// Tracing
	if v := os.Getenv("TRACING_ENABLED"); v != "" {
		cfg.Tracing.Enabled = v == "true" || v == "1"
	}
	if v := os.Getenv("TRACING_SERVICE_NAME"); v != "" {
		cfg.Tracing.ServiceName = v
	}
	if v := os.Getenv("TRACING_EXPORTER"); v != "" {
		cfg.Tracing.ExporterType = v
	}
	if v := os.Getenv("TRACING_ENDPOINT"); v != "" {
		cfg.Tracing.Endpoint = v
	}
	if v := os.Getenv("TRACING_INSECURE"); v != "" {
		cfg.Tracing.Insecure = v == "true" || v == "1"
	}

	// Logging
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.Logging.Level = v
	}
	if v := os.Getenv("LOG_FORMAT"); v != "" {
		cfg.Logging.Format = v
	}

	// Per-job overrides (iterates through env vars like JOB_0_ENABLED)
	for i := range cfg.Jobs {
		prefix := "JOB_" + strconv.Itoa(i) + "_"
		
		if v := os.Getenv(prefix + "ENABLED"); v != "" {
			cfg.Jobs[i].Enabled = v == "true" || v == "1"
		}
		if v := os.Getenv(prefix + "CRON"); v != "" {
			cfg.Jobs[i].Cron = v
		}
		if v := os.Getenv(prefix + "HANDLER"); v != "" {
			cfg.Jobs[i].Handler = v
		}
		if v := os.Getenv(prefix + "TIMEOUT"); v != "" {
			if t, err := strconv.Atoi(v); err == nil {
				cfg.Jobs[i].Timeout = t
			}
		}
		if v := os.Getenv(prefix + "RETRY_MAX_ATTEMPTS"); v != "" {
			if a, err := strconv.Atoi(v); err == nil {
				cfg.Jobs[i].Retry.MaxAttempts = a
			}
		}
	}
}

func setDefaults(cfg *Config) {
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Worker.PoolSize == 0 {
		cfg.Worker.PoolSize = 10
	}
	if cfg.Worker.QueueCapacity == 0 {
		cfg.Worker.QueueCapacity = 100
	}
	if cfg.Worker.ShutdownTimeout == 0 {
		cfg.Worker.ShutdownTimeout = 30
	}
	if cfg.Scheduler.Timezone == "" {
		cfg.Scheduler.Timezone = "UTC"
	}
	if cfg.Metrics.Path == "" {
		cfg.Metrics.Path = "/metrics"
	}
	if cfg.Tracing.ServiceName == "" {
		cfg.Tracing.ServiceName = "datajobs"
	}
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	if cfg.Logging.Format == "" {
		cfg.Logging.Format = "json"
	}

	for i := range cfg.Jobs {
		if cfg.Jobs[i].Timeout == 0 {
			cfg.Jobs[i].Timeout = 300 // 5 minutes default
		}
		if cfg.Jobs[i].Retry.MaxAttempts == 0 {
			cfg.Jobs[i].Retry.MaxAttempts = 3
		}
		if cfg.Jobs[i].Retry.InitialDelay == 0 {
			cfg.Jobs[i].Retry.InitialDelay = 1000 // 1 second
		}
		if cfg.Jobs[i].Retry.MaxDelay == 0 {
			cfg.Jobs[i].Retry.MaxDelay = 60000 // 1 minute
		}
		if cfg.Jobs[i].Retry.Multiplier == 0 {
			cfg.Jobs[i].Retry.Multiplier = 2.0
		}
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