package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bishop-bot/datajobs/internal/config"
	"github.com/bishop-bot/datajobs/internal/logging"
	_ "modernc.org/sqlite" // SQLite driver
)

// DB wraps the SQLite database connection.
type DB struct {
	sqlite  *sql.DB
	cfg     config.DatabaseConfig
}

// New creates a new SQLite database connection.
func New(cfg config.DatabaseConfig) (*DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(cfg.Path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite3", cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(1) // SQLite doesn't support concurrent writes
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0) // Connection lives forever

	// Set WAL journal mode
	if _, err := db.Exec(fmt.Sprintf("PRAGMA journal_mode=%s", cfg.JournalMode)); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set journal mode: %w", err)
	}

	// Set other pragmas for better performance
	pragmas := []string{
		"PRAGMA synchronous=NORMAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA cache_size=-64000", // 64MB cache
		"PRAGMA temp_store=MEMORY",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			logging.Warn("failed to set pragma", "pragma", pragma, "error", err)
		}
	}

	return &DB{sqlite: db, cfg: cfg}, nil
}

// DB returns the underlying sql.DB for direct queries.
func (d *DB) DB() *sql.DB {
	return d.sqlite
}

// Close closes the database connection.
func (d *DB) Close() error {
	if d.sqlite != nil {
		return d.sqlite.Close()
	}
	return nil
}

// Ping checks database connectivity.
func (d *DB) Ping(ctx context.Context) error {
	return d.sqlite.PingContext(ctx)
}

// BeginTx starts a new transaction with the given context.
func (d *DB) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return d.sqlite.BeginTx(ctx, nil)
}

// Exec executes a query without returning rows.
func (d *DB) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return d.sqlite.ExecContext(ctx, query, args...)
}

// Query executes a query that returns rows.
func (d *DB) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return d.sqlite.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that returns at most one row.
func (d *DB) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return d.sqlite.QueryRowContext(ctx, query, args...)
}

// RunMigrations runs all pending migrations.
func (d *DB) RunMigrations(ctx context.Context) error {
	// Create migrations table if not exists
	if _, err := d.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get applied migrations
	rows, err := d.Query(ctx, "SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return fmt.Errorf("failed to get migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[int64]bool)
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return fmt.Errorf("failed to scan migration: %w", err)
		}
		applied[v] = true
	}

	// Run pending migrations in order
	for _, m := range allMigrations {
		if applied[m.Version] {
			continue
		}

		logging.Info("running migration", "version", m.Version, "name", m.Name)

		tx, err := d.BeginTx(ctx)
		if err != nil {
			return fmt.Errorf("failed to start transaction: %w", err)
		}

		if _, err := tx.ExecContext(ctx, m.SQL); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to execute migration %d: %w", m.Version, err)
		}

		if _, err := tx.ExecContext(ctx,
			"INSERT INTO schema_migrations (version, name) VALUES (?, ?)",
			m.Version, m.Name); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration: %w", err)
		}
	}

	return nil
}

// Migration represents a database migration.
type Migration struct {
	Version int64
	Name    string
	SQL     string
}

// allMigrations contains all database migrations in order.
var allMigrations = []Migration{
	{
		Version: 1,
		Name:    "create_jobs_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS jobs (
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL,
				cron TEXT,
				type TEXT NOT NULL,
				handler TEXT NOT NULL,
				enabled INTEGER DEFAULT 1,
				timeout INTEGER DEFAULT 300,
				metadata TEXT,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)
		`,
	},
	{
		Version: 2,
		Name:    "create_job_runs_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS job_runs (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				job_id TEXT NOT NULL,
				status TEXT NOT NULL,
				started_at DATETIME NOT NULL,
				finished_at DATETIME,
				output TEXT,
				error TEXT,
				attempts INTEGER DEFAULT 1,
				 FOREIGN KEY (job_id) REFERENCES jobs(id)
			)
		`,
	},
	{
		Version: 3,
		Name:    "create_sync_state_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS sync_state (
				id TEXT PRIMARY KEY,
				source_type TEXT NOT NULL,
				last_sync_at DATETIME,
				last_synced_key TEXT,
				status TEXT DEFAULT 'idle',
				records_synced INTEGER DEFAULT 0,
				updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)
		`,
	},
	{
		Version: 4,
		Name:    "create_schemas_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS schemas (
				table_name TEXT PRIMARY KEY,
				columns TEXT NOT NULL,
				timestamp_column TEXT,
				symbol_columns TEXT,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)
		`,
	},
}