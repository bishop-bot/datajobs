package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bishop-bot/datajobs/internal/config"
	"github.com/bishop-bot/datajobs/internal/logging"
	"github.com/bishop-bot/datajobs/internal/migrate"
	_ "modernc.org/sqlite"
)

const (
	version     = "1.0.0"
	defaultName = "migrate"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Parse command line flags
	upCmd := flag.Bool("up", false, "Run pending migrations")
	downCmd := flag.Bool("down", false, "Rollback last migration")
	statusCmd := flag.Bool("status", false, "Show migration status")
	resetCmd := flag.Bool("reset", false, "Reset database (drop all tables)")
	createCmd := flag.String("create", "", "Create a new migration")
	forceCmd := flag.Int64("force", 0, "Force a specific migration version")
	configPath := flag.String("config", "config.yaml", "Path to config file")
	migrationsDir := flag.String("migrations", "migrations", "Path to migrations directory")
	versionCmd := flag.Bool("version", false, "Show version")
	helpCmd := flag.Bool("help", false, "Show help")

	flag.Parse()

	// Show help if no command specified
	if *helpCmd || (!*upCmd && !*downCmd && !*statusCmd && !*resetCmd && *createCmd == "" && *forceCmd == 0 && !*versionCmd) {
		printUsage()
		return nil
	}

	// Show version
	if *versionCmd {
		fmt.Printf("%s version %s\n", defaultName, version)
		return nil
	}

	// Initialize logging
	logging.Init(logging.Config{Level: "info", Format: "text"})
	logger := logging.NewLogger("migrate")
	logger.Info("migration tool started", "version", version)

	// Load config
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Warn("failed to load config, using defaults", "error", err)
		cfg = &config.Config{}
	}

	// Override migrations directory from config if not specified
	if *migrationsDir == "migrations" {
		if cfg.Database.MigrationsDir != "" {
			*migrationsDir = cfg.Database.MigrationsDir
		}
	}

	// Connect to database
	dbPath := cfg.Database.Path
	if dbPath == "" {
		dbPath = "datajobs.db"
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Configure SQLite
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// Create migrator
	migrator := migrate.New(db, *migrationsDir)
	ctx := context.Background()

	// Execute command
	switch {
	case *upCmd:
		if err := migrator.Up(ctx); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
		logger.Info("migrations completed successfully")

	case *downCmd:
		if err := migrator.Down(ctx); err != nil {
			return fmt.Errorf("rollback failed: %w", err)
		}
		logger.Info("rollback completed successfully")

	case *statusCmd:
		status, err := migrator.Status(ctx)
		if err != nil {
			return fmt.Errorf("failed to get status: %w", err)
		}
		printStatus(status)

	case *resetCmd:
		if err := migrator.Reset(ctx); err != nil {
			return fmt.Errorf("reset failed: %w", err)
		}
		logger.Info("database reset complete")

	case *createCmd != "":
		if err := migrator.Create(*createCmd); err != nil {
			return fmt.Errorf("failed to create migration: %w", err)
		}

	case *forceCmd > 0:
		if err := migrator.ForceVersion(ctx, *forceCmd, "forced"); err != nil {
			return fmt.Errorf("failed to force version: %w", err)
		}
		logger.Info("forced version", "version", *forceCmd)
	}

	return nil
}

func printUsage() {
	fmt.Printf(`Database Migration Tool v%s

Usage: %s [options]

Options:
  -up              Run all pending migrations
  -down            Rollback the last migration
  -status          Show current migration status
  -reset           Reset database (drop all tables)
  -create <name>   Create a new migration file pair
  -force <version> Force a specific migration version
  -config <path>   Path to config file (default: config.yaml)
  -migrations <dir> Path to migrations directory (default: migrations)
  -version         Show version
  -help            Show this help

Examples:
  %s -up                          Run pending migrations
  %s -down                        Rollback last migration
  %s -status                      Show status
  %s -create add_users_table      Create new migration
  %s -force 5                     Force version 5

`, version, defaultName, defaultName, defaultName, defaultName, defaultName, defaultName)
}

func printStatus(status []migrate.MigrationStatus) {
	fmt.Println("\nMigration Status:")
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("%-10s %-40s %s\n", "VERSION", "NAME", "STATUS")
	fmt.Println(strings.Repeat("-", 60))

	for _, s := range status {
		statusStr := "pending"
		if s.Applied {
			statusStr = fmt.Sprintf("applied %s", s.AppliedAt)
		}
		fmt.Printf("%-10d %-40s %s\n", s.Version, s.Name, statusStr)
	}
	fmt.Println(strings.Repeat("-", 60))

	applied := 0
	pending := 0
	for _, s := range status {
		if s.Applied {
			applied++
		} else {
			pending++
		}
	}
	fmt.Printf("\nTotal: %d applied, %d pending\n", applied, pending)
}

// resolveMigrationsDir resolves the migrations directory path.
func resolveMigrationsDir(migrationsDir string) string {
	// If absolute path, use it
	if filepath.IsAbs(migrationsDir) {
		return migrationsDir
	}

	// If starts with ./ or ../, keep relative
	if strings.HasPrefix(migrationsDir, "./") || strings.HasPrefix(migrationsDir, "../") {
		return migrationsDir
	}

	// Otherwise, make it relative to current working directory
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, migrationsDir)
}

// timestamp returns current date in YYYYMMDD format.
func timestamp() string {
	return time.Now().Format("20060102")
}