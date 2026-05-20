package database

import (
	"context"
	"fmt"
	"time"

	"github.com/bishop-bot/datajobs/internal/config"
	"github.com/bishop-bot/datajobs/internal/logging"
	"github.com/jackc/pgx/v5/pgxpool"
)

// QuestDB wraps the QuestDB connection pool.
type QuestDB struct {
	pool  *pgxpool.Pool
	cfg   config.QuestDBConfig
	ilpAddr string
}

// NewQuestDB creates a new QuestDB connection pool.
func NewQuestDB(cfg config.QuestDBConfig) (*QuestDB, error) {
	// Build connection string
	connStr := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
	)

	poolConfig, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	// Configure pool
	poolConfig.MaxConns = int32(cfg.PoolSize)
	poolConfig.MinConns = 2
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute
	poolConfig.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping QuestDB: %w", err)
	}

	logging.Info("connected to QuestDB",
		"host", cfg.Host,
		"port", cfg.Port,
		"pool_size", cfg.PoolSize,
	)

	return &QuestDB{
		pool:     pool,
		cfg:      cfg,
		ilpAddr:  fmt.Sprintf("%s:%d", cfg.Host, cfg.ILPPort),
	}, nil
}

// Pool returns the underlying pgxpool.Pool for direct access.
func (q *QuestDB) Pool() *pgxpool.Pool {
	return q.pool
}

// ILPAddr returns the ILP address for ingestion.
func (q *QuestDB) ILPAddr() string {
	return q.ilpAddr
}

// Close closes the connection pool.
func (q *QuestDB) Close() {
	if q.pool != nil {
		q.pool.Close()
	}
}

// Ping checks database connectivity.
func (q *QuestDB) Ping(ctx context.Context) error {
	return q.pool.Ping(ctx)
}

// Exec executes a query without returning rows.
func (q *QuestDB) Exec(ctx context.Context, sql string, args ...interface{}) error {
	_, err := q.pool.Exec(ctx, sql, args...)
	return err
}

// Query executes a query that returns rows.
func (q *QuestDB) Query(ctx context.Context, sql string, args ...interface{}) (Rows, error) {
	rows, err := q.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return &questdbRows{rows: rows}, nil
}

// QueryRow executes a query that returns at most one row.
func (q *QuestDB) QueryRow(ctx context.Context, sql string, args ...interface{}) Row {
	return q.pool.QueryRow(ctx, sql, args...)
}

// Rows wraps pgx.Rows for interface compliance.
type Rows interface {
	Close()
	Next() bool
	Scan(dest ...interface{}) error
	Err() error
	Values() ([]interface{}, error)
	FieldDescriptions() []interface{}
}

type questdbRows struct {
	rows interface{ Close(); Next() bool; Scan(dest ...interface{}) error; Err() error }
}

func (r *questdbRows) Close() { r.rows.Close() }
func (r *questdbRows) Next() bool { return r.rows.Next() }
func (r *questdbRows) Scan(dest ...interface{}) error { return r.rows.Scan(dest...) }
func (r *questdbRows) Err() error { return r.rows.Err() }
func (r *questdbRows) Values() ([]interface{}, error) { return nil, nil }
func (r *questdbRows) FieldDescriptions() []interface{} { return nil }

// Row wraps pgx.Row.
type Row interface {
	Scan(dest ...interface{}) error
}

// HealthChecker provides health check functionality.
type HealthChecker interface {
	Ping(ctx context.Context) error
}

// TableInfo represents QuestDB table information.
type TableInfo struct {
	Name        string
	DesignatedTimestamp string
	PartitionBy string
	Columns     []ColumnInfo
}

// ColumnInfo represents a table column.
type ColumnInfo struct {
	Name     string
	Type     string
	Indexed  bool
	Signed   bool
}

// ListTables returns all tables in QuestDB.
func (q *QuestDB) ListTables(ctx context.Context) ([]TableInfo, error) {
	rows, err := q.Query(ctx, "SHOW TABLES")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []TableInfo
	for rows.Next() {
		var name, timestamp, partition string
		if err := rows.Scan(&name, &timestamp, &partition); err != nil {
			return nil, err
		}
		tables = append(tables, TableInfo{
			Name:        name,
			DesignatedTimestamp: timestamp,
			PartitionBy: partition,
		})
	}
	return tables, rows.Err()
}

// GetTableColumns returns column info for a table.
func (q *QuestDB) GetTableColumns(ctx context.Context, table string) ([]ColumnInfo, error) {
	rows, err := q.Query(ctx, fmt.Sprintf("SHOW COLUMNS FROM '%s'", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var name, colType string
		var indexed, signed bool
		if err := rows.Scan(&name, &colType, &indexed, &signed); err != nil {
			return nil, err
		}
		columns = append(columns, ColumnInfo{
			Name:    name,
			Type:    colType,
			Indexed: indexed,
			Signed:  signed,
		})
	}
	return columns, rows.Err()
}

// CreateTable creates a new table with the given schema.
func (q *QuestDB) CreateTable(ctx context.Context, createSQL string) error {
	return q.Exec(ctx, createSQL)
}

// AnalyzeTable runs ANALYZE on a table.
func (q *QuestDB) AnalyzeTable(ctx context.Context, table string) error {
	return q.Exec(ctx, fmt.Sprintf("ANALYZE TABLE '%s'", table))
}

// TruncateTable truncates a table.
func (q *QuestDB) TruncateTable(ctx context.Context, table string) error {
	return q.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE '%s'", table))
}

// DropTable drops a table.
func (q *QuestDB) DropTable(ctx context.Context, table string) error {
	return q.Exec(ctx, fmt.Sprintf("DROP TABLE '%s'", table))
}

// GetPartitionInfo returns partition information for a table.
func (q *QuestDB) GetPartitionInfo(ctx context.Context, table string) ([]string, error) {
	rows, err := q.Query(ctx, fmt.Sprintf("SHOW PARTITIONS FROM '%s'", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var partitions []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		partitions = append(partitions, name)
	}
	return partitions, rows.Err()
}