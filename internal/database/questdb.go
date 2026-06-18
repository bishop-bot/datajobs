package database

import (
	"context"
	"fmt"
	"time"

	"github.com/bishop-bot/datajobs/internal/config"
	"github.com/bishop-bot/datajobs/internal/logging"
	"github.com/jackc/pgx/v5/pgxpool"
	qdb "github.com/questdb/go-questdb-client/v4"
)

// QuestDB wraps the QuestDB connection pool and line sender.
type QuestDB struct {
	pool       *pgxpool.Pool
	lineSender qdb.LineSender
	cfg        config.QuestDBConfig
	httpAddr   string
}

// NewQuestDB creates a new QuestDB connection pool and line sender.
func NewQuestDB(cfg config.QuestDBConfig) (*QuestDB, error) {
	q := &QuestDB{
		cfg: cfg,
	}

	// HTTP ILP uses cfg.ILPHTTPPort (default 9000, same as web UI)
	// TCP ILP uses cfg.ILPPort (default 9009)
	q.httpAddr = fmt.Sprintf("%s:%d", cfg.Host, cfg.ILPHTTPPort)

	// Create PostgreSQL connection pool for SQL queries
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}
	q.pool = pool

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping QuestDB: %w", err)
	}

	// Create line sender for ingestion (HTTP transport uses same port as web UI)
	lineSender, err := qdb.NewLineSender(ctx,
		qdb.WithHttp(),
		qdb.WithAddress(q.httpAddr),
		qdb.WithBasicAuth(cfg.User, cfg.Password),
	)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to create line sender: %w", err)
	}
	q.lineSender = lineSender

	logging.Info("connected to QuestDB",
		"host", cfg.Host,
		"http_ilp_port", cfg.ILPHTTPPort,
		"pg_port", cfg.Port,
		"tcp_ilp_port", cfg.ILPPort,
		"pool_size", cfg.PoolSize,
	)

	return q, nil
}

// Pool returns the underlying pgxpool.Pool for direct access.
func (q *QuestDB) Pool() *pgxpool.Pool {
	return q.pool
}

// LineSender returns the QuestDB line sender for ingestion.
func (q *QuestDB) LineSender() qdb.LineSender {
	return q.lineSender
}



// Close closes the connection pool and line sender.
func (q *QuestDB) Close() {
	if q.lineSender != nil {
		q.lineSender.Close(context.Background())
	}
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

// OHLCVBar represents a single OHLCV candlestick bar for QuestDB ingestion.
type OHLCVBar struct {
	// Symbol is the instrument symbol (e.g., "AAPL", "IBM")
	Symbol string `json:"symbol"`
	// Publisher is the data source/publisher (e.g., "IB", "NASDAQ")
	Publisher string `json:"publisher"`
	// Ts is the bar start timestamp (nanoseconds since epoch)
	Ts int64 `json:"ts"`
	// TsEnd is the bar end timestamp (nanoseconds since epoch)
	TsEnd int64 `json:"ts_end"`
	// Open is the opening price
	Open float64 `json:"open"`
	// High is the highest price
	High float64 `json:"high"`
	// Low is the lowest price
	Low float64 `json:"low"`
	// Close is the closing price
	Close float64 `json:"close"`
	// Volume is the trading volume
	Volume int64 `json:"volume"`
}

// OHLCVColumns returns the column names for the ohlcv_bars table.
func OHLCVColumns() []string {
	return []string{"symbol", "publisher", "ts", "ts_end", "open", "high", "low", "close", "volume"}
}

// OHLCVUpsertResult contains the result of an upsert operation.
type OHLCVUpsertResult struct {
	RowsAffected int
	Duration     time.Duration
	Errors       []string
}

// UpsertOHLCVBars ingests OHLCV bars into QuestDB using the Line Protocol.
// Uses the table Auto-Creation feature - table is created automatically if it doesn't exist.
func (q *QuestDB) UpsertOHLCVBars(ctx context.Context, bars []OHLCVBar) (*OHLCVUpsertResult, error) {
	if len(bars) == 0 {
		return &OHLCVUpsertResult{RowsAffected: 0}, nil
	}

	if q == nil || q.lineSender == nil {
		logging.Error("QuestDB is nil in UpsertOHLCVBars")
		return nil, fmt.Errorf("QuestDB line sender is nil")
	}

	start := time.Now()
	result := &OHLCVUpsertResult{}

	for _, bar := range bars {
		// Convert nanoseconds to time.Time for QuestDB
		ts := time.Unix(0, bar.Ts).UTC()
		tsEnd := time.Unix(0, bar.TsEnd).UTC()

		err := q.lineSender.Table("ohlcv_bars").
			Symbol("symbol", bar.Symbol).
			Symbol("publisher", bar.Publisher).
			TimestampColumn("ts_end", tsEnd).
			Float64Column("open", bar.Open).
			Float64Column("high", bar.High).
			Float64Column("low", bar.Low).
			Float64Column("close", bar.Close).
			Int64Column("volume", bar.Volume).
			At(ctx, ts)

		if err != nil {
			logging.Error("QuestDB line sender failed",
				"symbol", bar.Symbol,
				"error", err.Error(),
			)
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", bar.Symbol, err))
			continue
		}
		result.RowsAffected++
	}

	// Flush the buffer to ensure all data is sent
	if err := q.lineSender.Flush(ctx); err != nil {
		// Log the error but still return partial results
		// The go-questdb-client buffers data, so if Flush fails,
		// some rows may still have been sent successfully
		logging.Error("QuestDB flush failed (partial results may have been sent)",
			"rows_queued", result.RowsAffected,
			"error", err.Error(),
		)
		// Include the flush error in result so caller knows about it
		result.Errors = append(result.Errors, fmt.Sprintf("flush failed: %v", err))
	}

	result.Duration = time.Since(start)

	logging.Info("ingested OHLCV bars via ILP",
		"rows", result.RowsAffected,
		"duration", result.Duration,
		"errors", len(result.Errors),
	)

	return result, nil
}

// EnsureTableOHLCV creates the ohlcv_bars table if it doesn't exist using SQL.
// This is optional since the Line Protocol can auto-create tables.
func (q *QuestDB) EnsureTableOHLCV(ctx context.Context) error {
	const createSQL = `
		CREATE TABLE IF NOT EXISTS ohlcv_bars (
			symbol    SYMBOL,
			publisher SYMBOL,
			ts        TIMESTAMP,
			ts_end    TIMESTAMP,
			open      DOUBLE,
			high      DOUBLE,
			low       DOUBLE,
			close     DOUBLE,
			volume    LONG
		) TIMESTAMP(ts) PARTITION BY DAY
	`
	return q.Exec(ctx, createSQL)
}

// TableInfo represents QuestDB table information.
type TableInfo struct {
	Name                 string
	DesignatedTimestamp  string
	PartitionBy          string
	Columns              []ColumnInfo
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
			Name:                 name,
			DesignatedTimestamp: timestamp,
			PartitionBy:          partition,
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
