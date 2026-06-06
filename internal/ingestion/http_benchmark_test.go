package ingestion

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/bishop-bot/datajobs/internal/config"
	"github.com/bishop-bot/datajobs/internal/database"
)

// getEnvBenchmark returns environment variable or default value.
func getEnvBenchmark(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// BenchmarkHTTPIngest benchmarks the HTTP API ingestion approach.
func BenchmarkHTTPIngest(b *testing.B) {
	cfg := config.QuestDBConfig{
		Host:     getEnvBenchmark("QUESTDB_HOST", "localhost"),
		ILPPort:  9009,
		User:     getEnvBenchmark("QUESTDB_USER", "admin"),
		Password: getEnvBenchmark("QUESTDB_PASSWORD", "quest"),
	}

	client := NewHTTPClient(cfg)
	bars := generateTestBarsHTTP(1000)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_, err := client.IngestOHLCV(ctx, "ohlcv_bars", bars)
		cancel()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkHTTPIngest_VaryingSize benchmarks HTTP ingestion with different batch sizes.
func BenchmarkHTTPIngest_VaryingSize(b *testing.B) {
	cfg := config.QuestDBConfig{
		Host:     getEnvBenchmark("QUESTDB_HOST", "localhost"),
		ILPPort:  9009,
		User:     getEnvBenchmark("QUESTDB_USER", "admin"),
		Password: getEnvBenchmark("QUESTDB_PASSWORD", "quest"),
	}

	client := NewHTTPClient(cfg)
	sizes := []int{10, 100, 500, 1000, 2000}

	for _, size := range sizes {
		b.Run(httpBenchmarkName(size), func(b *testing.B) {
			bars := generateTestBarsHTTP(size)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				_, err := client.IngestOHLCV(ctx, "ohlcv_bars", bars)
				cancel()
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkCSVBuilding benchmarks just the CSV generation overhead.
func BenchmarkCSVBuilding(b *testing.B) {
	bars := generateTestBarsHTTP(1000)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		csv, err := barsToCSV(bars)
		if err != nil {
			b.Fatal(err)
		}
		_ = csv
	}
}

// BenchmarkMultipartBuilding benchmarks the multipart form building.
func BenchmarkMultipartBuilding(b *testing.B) {
	cfg := config.QuestDBConfig{
		Host:    getEnvBenchmark("QUESTDB_HOST", "localhost"),
		ILPPort: 9009,
	}

	_ = NewHTTPClient(cfg) // Used to ensure client creation works
	bars := generateTestBarsHTTP(1000)
	csvData, _ := barsToCSV(bars)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		body := new(bytes.Buffer)
		writer := newMultipartWriter(body)
		writer.WriteField("schema", `[{"name":"ts","type":"TIMESTAMP"},{"name":"ts_end","type":"TIMESTAMP"},{"name":"symbol","type":"SYMBOL"},{"name":"publisher","type":"SYMBOL"},{"name":"open","type":"DOUBLE"},{"name":"high","type":"DOUBLE"},{"name":"low","type":"DOUBLE"},{"name":"close","type":"DOUBLE"},{"name":"volume","type":"LONG"}]`)
		writer.WriteFile("data", "ohlcv.csv", "text/csv", csvData)
		writer.Close()
		_ = body.String()
	}
}

// generateTestBarsHTTP creates test OHLCV bars for HTTP benchmark.
func generateTestBarsHTTP(count int) []database.OHLCVBar {
	symbols := []string{"AAPL", "IBM", "GOOGL", "MSFT", "AMZN"}
	bars := make([]database.OHLCVBar, count)
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixNano()

	for i := 0; i < count; i++ {
		bars[i] = database.OHLCVBar{
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

// httpBenchmarkName formats the benchmark name.
func httpBenchmarkName(size int) string {
	if size == 0 {
		return "0"
	}
	digits := 0
	for temp := size; temp > 0; temp /= 10 {
		digits++
	}
	result := make([]byte, digits)
	for i := digits - 1; i >= 0; i-- {
		result[i] = byte('0' + size%10)
		size /= 10
	}
	return "size=" + string(result)
}