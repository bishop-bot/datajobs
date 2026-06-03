package database

import (
	"context"
	"fmt"
	"strings"
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

// ToSlice converts OHLCVBar slice to interface slice for generic handling.
func (b OHLCVBar) ToSlice() []interface{} {
	return []interface{}{
		b.Symbol,
		b.Publisher,
		b.Ts,
		b.TsEnd,
		b.Open,
		b.High,
		b.Low,
		b.Close,
		b.Volume,
	}
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

// UpsertOHLCVBars performs a bulk upsert of OHLCV bars into QuestDB.
// It uses ON CONFLICT DO UPDATE to handle re-ingestion of historical data.
// The upsert key is (symbol, ts) to avoid duplicates.
func (q *QuestDB) UpsertOHLCVBars(ctx context.Context, bars []OHLCVBar) (*OHLCVUpsertResult, error) {
	if len(bars) == 0 {
		return &OHLCVUpsertResult{RowsAffected: 0}, nil
	}

	start := time.Now()
	result := &OHLCVUpsertResult{}

	// Build the upsert query
	columns := OHLCVColumns()
	placeholders := make([]string, len(bars))
	values := make([]interface{}, 0, len(bars)*len(columns))

	for i, bar := range bars {
		// Create placeholders for each row: ($1, $2, ...), ($9, $10, ...), etc.
		base := i*len(columns) + 1
		rowPlaceholders := make([]string, len(columns))
		for j := range columns {
			rowPlaceholders[j] = fmt.Sprintf("$%d", base+j)
		}
		placeholders[i] = fmt.Sprintf("(%s)", strings.Join(rowPlaceholders, ", "))
		values = append(values, bar.ToSlice()...)
	}

	query := fmt.Sprintf(`
		INSERT INTO ohlcv_bars (%s)
		VALUES %s
		ON CONFLICT (symbol, ts) DO UPDATE SET
			publisher = EXCLUDED.publisher,
			ts_end = EXCLUDED.ts_end,
			open = EXCLUDED.open,
			high = EXCLUDED.high,
			low = EXCLUDED.low,
			close = EXCLUDED.close,
			volume = EXCLUDED.volume
	`, strings.Join(columns, ", "), strings.Join(placeholders, ", "))

	rows, err := q.pool.Exec(ctx, query, values...)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert OHLCV bars: %w", err)
	}

	result.RowsAffected = int(rows.RowsAffected())
	result.Duration = time.Since(start)

	logging.Debug("upserted OHLCV bars",
		"rows", result.RowsAffected,
		"duration", result.Duration,
	)

	return result, nil
}

// EnsureTableOHLCV creates the ohlcv_bars table if it doesn't exist.
func (q *QuestDB) EnsureTableOHLCV(ctx context.Context) error {
	const createSQL = `
		CREATE TABLE IF NOT EXISTS ohlcv_bars (
			symbol    SYMBOL,
			publisher SYMBOL,
			ts        TIMESTAMP_NS,
			ts_end    TIMESTAMP_NS,
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