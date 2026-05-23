package migrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/bishop-bot/datajobs/internal/logging"
)

// filePattern matches migration files: YYYYMMDD_NNN_description.up.sql
var filePattern = regexp.MustCompile(`^(\d{8})_(\d+)_([^.]+)\.(up|down)\.sql$`)

// Migrator handles database migrations.
type Migrator struct {
	db        *sql.DB
	migrations string
}

// New creates a new Migrator.
func New(db *sql.DB, migrationsDir string) *Migrator {
	return &Migrator{
		db:        db,
		migrations: migrationsDir,
	}
}

// Migration represents a single migration.
type Migration struct {
	Version    int64
	Date       string
	Name       string
	Direction  string // "up" or "down"
	Path       string
	SQL        string
}

// Up runs pending migrations forward.
func (m *Migrator) Up(ctx context.Context) error {
	if err := m.ensureSchemaMigrations(); err != nil {
		return err
	}

	applied, err := m.getAppliedVersions()
	if err != nil {
		return fmt.Errorf("failed to get applied versions: %w", err)
	}

	migrations, err := m.readMigrations()
	if err != nil {
		return fmt.Errorf("failed to read migrations: %w", err)
	}

	upMigrations := filterDirection(migrations, "up")
	sort.Slice(upMigrations, func(i, j int) bool {
		return upMigrations[i].Version < upMigrations[j].Version
	})

	appliedCount := 0
	for _, migration := range upMigrations {
		if applied[migration.Version] {
			continue
		}

		logging.Info("applying migration", "version", migration.Version, "name", migration.Name)

		if err := m.applyMigration(ctx, migration); err != nil {
			return fmt.Errorf("failed to apply migration %d: %w", migration.Version, err)
		}
		appliedCount++
	}

	if appliedCount == 0 {
		logging.Info("no migrations to apply")
	} else {
		logging.Info("applied migrations", "count", appliedCount)
	}

	return nil
}

// Down rolls back the last migration.
func (m *Migrator) Down(ctx context.Context) error {
	if err := m.ensureSchemaMigrations(); err != nil {
		return err
	}

	migrations, err := m.readMigrations()
	if err != nil {
		return fmt.Errorf("failed to read migrations: %w", err)
	}

	upMigrations := filterDirection(migrations, "up")
	sort.Slice(upMigrations, func(i, j int) bool {
		return upMigrations[i].Version < upMigrations[j].Version
	})

	if len(upMigrations) == 0 {
		return errors.New("no migrations found")
	}

	// Get the latest applied version
	var latestVersion int64
	for i := len(upMigrations) - 1; i >= 0; i-- {
		downMigration := findDownMigration(migrations, upMigrations[i].Version)
		if downMigration != nil {
			// Check if this migration is applied
			var count int64
			err := m.db.QueryRowContext(ctx,
				"SELECT COUNT(*) FROM schema_migrations WHERE version = ?",
				upMigrations[i].Version,
			).Scan(&count)
			if err != nil {
				return fmt.Errorf("failed to check migration: %w", err)
			}
			if count > 0 {
				latestVersion = upMigrations[i].Version
				break
			}
		}
	}

	if latestVersion == 0 {
		return errors.New("no migrations to roll back")
	}

	downMigration := findDownMigration(migrations, latestVersion)
	if downMigration == nil {
		return fmt.Errorf("no down migration found for version %d", latestVersion)
	}

	logging.Info("rolling back migration", "version", latestVersion, "name", downMigration.Name)

	return m.applyMigration(ctx, *downMigration)
}

// Status shows the current migration status.
func (m *Migrator) Status(ctx context.Context) ([]MigrationStatus, error) {
	if err := m.ensureSchemaMigrations(); err != nil {
		return nil, err
	}

	applied, err := m.getAppliedVersions()
	if err != nil {
		return nil, fmt.Errorf("failed to get applied versions: %w", err)
	}

	migrations, err := m.readMigrations()
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations: %w", err)
	}

	upMigrations := filterDirection(migrations, "up")
	sort.Slice(upMigrations, func(i, j int) bool {
		return upMigrations[i].Version < upMigrations[j].Version
	})

	status := make([]MigrationStatus, 0, len(upMigrations))
	for _, migration := range upMigrations {
		s := MigrationStatus{
			Version: migration.Version,
			Name:    migration.Name,
			Applied: applied[migration.Version],
		}
		if s.Applied {
			// Get applied timestamp
			var appliedAt string
			err := m.db.QueryRowContext(ctx,
				"SELECT applied_at FROM schema_migrations WHERE version = ?",
				migration.Version,
			).Scan(&appliedAt)
			if err == nil {
				s.AppliedAt = appliedAt
			}
		}
		status = append(status, s)
	}

	return status, nil
}

// MigrationStatus represents the status of a migration.
type MigrationStatus struct {
	Version   int64
	Name      string
	Applied   bool
	AppliedAt string
}

// Reset drops all tables and clears migration tracking.
// WARNING: This will delete all data.
func (m *Migrator) Reset(ctx context.Context) error {
	logging.Warn("resetting database - all data will be lost")

	// Drop all tables
	tables, err := m.getTables(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tables: %w", err)
	}

	for _, table := range tables {
		if table == "schema_migrations" {
			continue // Drop it last
		}
		logging.Info("dropping table", "table", table)
		if _, err := m.db.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", table)); err != nil {
			return fmt.Errorf("failed to drop table %s: %w", table, err)
		}
	}

	// Drop schema_migrations
	if _, err := m.db.ExecContext(ctx, "DROP TABLE IF EXISTS schema_migrations"); err != nil {
		return fmt.Errorf("failed to drop schema_migrations: %w", err)
	}

	logging.Info("database reset complete")
	return nil
}

// Create generates a new migration file pair.
func (m *Migrator) Create(name string) error {
	date := "20260523" // Current date

	// Find the latest migration number for today
	files, err := os.ReadDir(m.migrations)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	latestNum := 0
	for _, f := range files {
		if matches := filePattern.FindStringSubmatch(f.Name()); matches != nil {
			if matches[1] == date {
				num, _ := strconv.Atoi(matches[2])
				if num > latestNum {
					latestNum = num
				}
			}
		}
	}

	nextNum := latestNum + 1
	filename := fmt.Sprintf("%s_%03d_%s", date, nextNum, name)

	upPath := filepath.Join(m.migrations, filename+".up.sql")
	downPath := filepath.Join(m.migrations, filename+".down.sql")

	upContent := fmt.Sprintf(`-- Migration: %s
-- Version: %s

-- Add UP migration SQL here

`, name, filename)
	downContent := fmt.Sprintf(`-- Migration: %s

-- Add DOWN migration SQL here (rollback)

`, name)

	if err := os.WriteFile(upPath, []byte(upContent), 0644); err != nil {
		return fmt.Errorf("failed to write up migration: %w", err)
	}
	if err := os.WriteFile(downPath, []byte(downContent), 0644); err != nil {
		return fmt.Errorf("failed to write down migration: %w", err)
	}

	logging.Info("created migration", "files", fmt.Sprintf("%s.up.sql, %s.down.sql", filename, filename))
	return nil
}

func (m *Migrator) ensureSchemaMigrations() error {
	_, err := m.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

func (m *Migrator) getAppliedVersions() (map[int64]bool, error) {
	rows, err := m.db.Query("SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[int64]bool)
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}

	return applied, nil
}

func (m *Migrator) readMigrations() ([]Migration, error) {
	entries, err := os.ReadDir(m.migrations)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("migrations directory does not exist: %s", m.migrations)
		}
		return nil, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	var migrations []Migration
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		matches := filePattern.FindStringSubmatch(entry.Name())
		if matches == nil {
			continue
		}

		version, _ := strconv.ParseInt(matches[2], 10, 64)

		content, err := os.ReadFile(filepath.Join(m.migrations, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("failed to read migration %s: %w", entry.Name(), err)
		}

		migrations = append(migrations, Migration{
			Version:   version,
			Date:      matches[1],
			Name:      matches[3],
			Direction: matches[4],
			Path:      filepath.Join(m.migrations, entry.Name()),
			SQL:       string(content),
		})
	}

	return migrations, nil
}

func (m *Migrator) applyMigration(ctx context.Context, migration Migration) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, migration.SQL); err != nil {
		return fmt.Errorf("failed to execute migration: %w", err)
	}

	// Record or remove migration tracking
	if migration.Direction == "up" {
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO schema_migrations (version, name) VALUES (?, ?)",
			migration.Version, migration.Name,
		); err != nil {
			return fmt.Errorf("failed to record migration: %w", err)
		}
	} else {
		if _, err := tx.ExecContext(ctx,
			"DELETE FROM schema_migrations WHERE version = ?",
			migration.Version,
		); err != nil {
			return fmt.Errorf("failed to remove migration record: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (m *Migrator) getTables(ctx context.Context) ([]string, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT name FROM sqlite_master
		WHERE type='table' AND name NOT LIKE 'sqlite_%'
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}

	return tables, nil
}

func filterDirection(migrations []Migration, direction string) []Migration {
	var result []Migration
	for _, m := range migrations {
		if m.Direction == direction {
			result = append(result, m)
		}
	}
	return result
}

func findDownMigration(migrations []Migration, version int64) *Migration {
	for _, m := range migrations {
		if m.Direction == "down" && m.Version == version {
			return &m
		}
	}
	return nil
}

// GetLatestAppliedVersion returns the highest applied migration version.
func (m *Migrator) GetLatestAppliedVersion(ctx context.Context) (int64, error) {
	var version int64
	err := m.db.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(version), 0) FROM schema_migrations",
	).Scan(&version)
	return version, err
}

// ForceVersion sets the migration version without running SQL.
// Useful for repair scenarios.
func (m *Migrator) ForceVersion(ctx context.Context, version int64, name string) error {
	_, err := m.db.ExecContext(ctx,
		"INSERT OR REPLACE INTO schema_migrations (version, name) VALUES (?, ?)",
		version, name,
	)
	return err
}

// ParseVersion extracts version and name from a migration filename.
func ParseVersion(filename string) (version int64, name string, err error) {
	matches := filePattern.FindStringSubmatch(filename)
	if matches == nil {
		return 0, "", fmt.Errorf("invalid migration filename: %s", filename)
	}
	version, _ = strconv.ParseInt(matches[2], 10, 64)
	name = strings.TrimSuffix(matches[3], ".sql")
	return version, name, nil
}