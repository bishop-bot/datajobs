package database

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/bishop-bot/datajobs/internal/config"
)

// BenchmarkUpsertOHLCVBars benchmarks the VALUES-based upsert approach.
func BenchmarkUpsertOHLCVBars(b *testing.B) {
	// Skip if no QuestDB connection
	cfg := config.QuestDBConfig{
		Host:     getEnv("QUESTDB_HOST", "localhost"),
		Port:     8812,
		User:     getEnv("QUESTDB_USER", "admin"),
		Password: getEnv("QUESTDB_PASSWORD", "quest"),
		Database: getEnv("QUESTDB_DATABASE", "qdb"),
		PoolSize: 10,
	}

	db, err := NewQuestDB(cfg)
	if err != nil {
		b.Skipf("skipping benchmark: QuestDB not available: %v", err)
	}
	defer db.Close()

	// Generate test data
	bars := generateTestBars(1000)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_, err := db.UpsertOHLCVBars(ctx, bars)
		cancel()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkUpsertOHLCVBars_VaryingSize benchmarks upsert with different batch sizes.
func BenchmarkUpsertOHLCVBars_VaryingSize(b *testing.B) {
	cfg := config.QuestDBConfig{
		Host:     getEnv("QUESTDB_HOST", "localhost"),
		Port:     8812,
		User:     getEnv("QUESTDB_USER", "admin"),
		Password: getEnv("QUESTDB_PASSWORD", "quest"),
		Database: getEnv("QUESTDB_DATABASE", "qdb"),
		PoolSize: 10,
	}

	db, err := NewQuestDB(cfg)
	if err != nil {
		b.Skipf("skipping benchmark: QuestDB not available: %v", err)
	}
	defer db.Close()

	sizes := []int{10, 100, 500, 1000, 2000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			bars := generateTestBars(size)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				_, err := db.UpsertOHLCVBars(ctx, bars)
				cancel()
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkQueryBuilding benchmarks just the VALUES clause query building.
// This isolates the string allocation overhead from the database operation.
func BenchmarkQueryBuilding(b *testing.B) {
	bars := generateTestBars(1000)
	columns := OHLCVColumns()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		query := buildValuesQuery(bars, columns)
		_ = query
	}
}

// buildValuesQuery is extracted from UpsertOHLCVBars to benchmark query building.
func buildValuesQuery(bars []OHLCVBar, columns []string) string {
	columnsStr := fmt.Sprintf("(%s)", joinStrings(columns, ", "))

	// Build values placeholders
	values := make([]string, len(bars))
	for i := range bars {
		base := i*len(columns) + 1
		rowPh := make([]string, len(columns))
		for j := range columns {
			rowPh[j] = fmt.Sprintf("$%d", base+j)
		}
		values[i] = fmt.Sprintf("(%s)", joinStrings(rowPh, ", "))
	}

	return fmt.Sprintf("INSERT INTO ohlcv_bars %s VALUES %s ON CONFLICT (symbol, ts) DO UPDATE SET publisher = EXCLUDED.publisher, ts_end = EXCLUDED.ts_end, open = EXCLUDED.open, high = EXCLUDED.high, low = EXCLUDED.low, close = EXCLUDED.close, volume = EXCLUDED.volume",
		columnsStr, joinStrings(values, ", "))
}

// generateTestBars creates test OHLCV bars for benchmarking.
func generateTestBars(count int) []OHLCVBar {
	symbols := []string{"AAPL", "IBM", "GOOGL", "MSFT", "AMZN"}
	bars := make([]OHLCVBar, count)
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixNano()

	for i := 0; i < count; i++ {
		bars[i] = OHLCVBar{
			Symbol:    symbols[i%len(symbols)],
			Publisher: "IB",
			Ts:        baseTime + int64(i)*int64(time.Hour),
			TsEnd:     baseTime + int64(i+1)*int64(time.Hour),
			Open:      100.0 + float64(i%100)/10.0,
			High:      101.0 + float64(i%100)/10.0,
			Low:      99.0 + float64(i%100)/10.0,
			Close:     100.5 + float64(i%100)/10.0,
			Volume:    int64(1000000 + i*1000),
		}
	}

	return bars
}

// joinStrings is a helper function for benchmarking.
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	n := len(sep) * (len(strs) - 1)
	for i := 0; i < len(strs); i++ {
		n += len(strs[i])
	}
	var b bytes.Buffer
	b.Grow(n)
	b.WriteString(strs[0])
	for i := 1; i < len(strs); i++ {
		b.WriteString(sep)
		b.WriteString(strs[i])
	}
	return b.String()
}

// getEnv returns environment variable or default value.
func getEnv(key, defaultVal string) string {
	if v := getEnvValue(key); v != "" {
		return v
	}
	return defaultVal
}

// getEnvValue is package-level to avoid import cycle.
var getEnvValue = func(key string) string {
	return os.Getenv(key)
}