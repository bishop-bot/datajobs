package logging

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"
)

type contextKey string

const (
	LoggerKey      contextKey = "logger"
	RequestIDKey   contextKey = "request_id"
)

var (
	defaultLogger *slog.Logger
	initOnce      sync.Once
)

// Config holds logging configuration.
type Config struct {
	Level  string
	Format string
}

// Init initializes the global logger based on config.
func Init(cfg Config) {
	initOnce.Do(func() {
		var level slog.Level
		switch strings.ToLower(cfg.Level) {
		case "debug":
			level = slog.LevelDebug
		case "warn", "warning":
			level = slog.LevelWarn
		case "error":
			level = slog.LevelError
		default:
			level = slog.LevelInfo
		}

		opts := &slog.HandlerOptions{
			Level: level,
		}

		var handler slog.Handler
		if strings.ToLower(cfg.Format) == "text" {
			handler = slog.NewTextHandler(os.Stdout, opts)
		} else {
			handler = slog.NewJSONHandler(os.Stdout, opts)
		}

		defaultLogger = slog.New(handler)
		slog.SetDefault(defaultLogger)
	})
}

// initDefault initializes a default logger if not already initialized.
func initDefault() {
	initOnce.Do(func() {
		defaultLogger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
		slog.SetDefault(defaultLogger)
	})
}

// FromContext returns the logger from context, or the default logger.
func FromContext(ctx context.Context) *slog.Logger {
	// Initialize default if not set
	if defaultLogger == nil {
		initDefault()
	}
	if logger, ok := ctx.Value(LoggerKey).(*slog.Logger); ok && logger != nil {
		return logger
	}
	return defaultLogger
}

// WithContext returns a new context with the logger attached.
func WithContext(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, LoggerKey, logger)
}

// WithRequestID returns a new logger with the request ID as an attribute.
func WithRequestID(ctx context.Context, requestID string) *slog.Logger {
	logger := FromContext(ctx)
	return logger.With(slog.String("request_id", requestID))
}

// WithJobID returns a new logger with the job ID as an attribute.
func WithJobID(ctx context.Context, jobID string) *slog.Logger {
	logger := FromContext(ctx)
	return logger.With(slog.String("job_id", jobID))
}

// WithCorrelationID is an alias for WithRequestID for compatibility.
func WithCorrelationID(ctx context.Context, correlationID string) *slog.Logger {
	return WithRequestID(ctx, correlationID)
}

// NewLogger returns a new logger with the given attributes.
func NewLogger(component string, attrs ...any) *slog.Logger {
	return defaultLogger.With(append([]any{"component", component}, attrs...)...)
}

// Info logs an info message.
func Info(msg string, attrs ...any) {
	if defaultLogger == nil {
		initDefault()
	}
	defaultLogger.Info(msg, attrs...)
}

// Error logs an error message.
func Error(msg string, attrs ...any) {
	if defaultLogger == nil {
		initDefault()
	}
	defaultLogger.Error(msg, attrs...)
}

// Debug logs a debug message.
func Debug(msg string, attrs ...any) {
	if defaultLogger == nil {
		initDefault()
	}
	defaultLogger.Debug(msg, attrs...)
}

// Warn logs a warning message.
func Warn(msg string, attrs ...any) {
	if defaultLogger == nil {
		initDefault()
	}
	defaultLogger.Warn(msg, attrs...)
}