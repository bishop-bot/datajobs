package database

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/bishop-bot/datajobs/internal/config"
)

// BenchmarkUpsertOHLCVBars benchmarks the Line Protocol ingestion approach.
func BenchmarkUpsertOHLCVBars(b *testing.B) {
	// Skip if no QuestDB connection
	cfg := config.QuestDBConfig{
		Host:     getEnv("QUESTDB_HOST", "localhost"),
		Port:     8812, // PostgreSQL port
		ILPPort:  9009, // TCP ILP port (not used for HTTP)
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

// BenchmarkUpsertOHLCVBars_VaryingSize benchmarks ingestion with different batch sizes.
func BenchmarkUpsertOHLCVBars_VaryingSize(b *testing.B) {
	cfg := config.QuestDBConfig{
		Host:     getEnv("QUESTDB_HOST", "localhost"),
		Port:     8812, // PostgreSQL port
		ILPPort:  9009, // TCP ILP port (not used for HTTP)
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
			Low:       99.0 + float64(i%100)/10.0,
			Close:     100.5 + float64(i%100)/10.0,
			Volume:    int64(1000000 + i*1000),
		}
	}

	return bars
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
