package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bishop-bot/datajobs/internal/config"
	"github.com/bishop-bot/datajobs/internal/logging"
	"github.com/bishop-bot/datajobs/internal/migrate"
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

	db, err := sql.Open("sqlite", cfg.Path)
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

	// Verify connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping SQLite: %w", err)
	}

	logging.Info("connected to SQLite", "path", cfg.Path)

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

// RunMigrations runs all pending migrations from the migrations directory.
func (d *DB) RunMigrations(ctx context.Context) error {
	migrator := migrate.New(d.sqlite, d.cfg.MigrationsDir)
	return migrator.Up(ctx)
}

// Migrator returns a new Migrator instance for this database.
func (d *DB) Migrator() *migrate.Migrator {
	return migrate.New(d.sqlite, d.cfg.MigrationsDir)
}

// reservedKeywords contains SQL reserved keywords that need quoting.
var reservedKeywords = map[string]bool{
	"group":      true,
	"order":      true,
	"limit":      true,
	"offset":     true,
	"primary":    true,
	"unique":     true,
	"index":      true,
	"trigger":    true,
	"view":       true,
	"virtual":    true,
	"indexed":    true,
	"if":         true,
	"not":        true,
	"null":       true,
	"default":    true,
	"foreign":    true,
	"references": true,
	"check":      true,
	"constraint": true,
}

// quoteIdentifier quotes a column name if it's a reserved keyword.
func quoteIdentifier(col string) string {
	if reservedKeywords[col] {
		return fmt.Sprintf("\"%s\"", col)
	}
	return col
}

// ImportInstrumentsBatch imports a batch of instrument records using INSERT OR REPLACE.
// The headers slice defines column names, and rows contains the values for each row.
func (d *DB) ImportInstrumentsBatch(ctx context.Context, headers []string, rows [][]interface{}) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}

	// Build the INSERT OR REPLACE statement with properly quoted column names
	columns := make([]string, len(headers))
	placeholders := make([]string, len(headers))
	for i, col := range headers {
		columns[i] = quoteIdentifier(col)
		placeholders[i] = "?"
	}

	query := fmt.Sprintf(
		"INSERT OR REPLACE INTO instruments (%s) VALUES (%s)",
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)

	// Execute within transaction for performance
	tx, err := d.BeginTx(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	imported := 0
	for _, row := range rows {
		_, err := stmt.ExecContext(ctx, row...)
		if err != nil {
			return imported, fmt.Errorf("failed to insert row: %w", err)
		}
		imported++
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return imported, nil
}