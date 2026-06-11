package handlers

import (
	"bytes"
	"context"
	"encoding/csv"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bishop-bot/datajobs/internal/config"
	"github.com/bishop-bot/datajobs/internal/database"
)

func TestInstrumentsHandler_ImportCSV(t *testing.T) {
	// Create a temporary database
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	cfg := config.DatabaseConfig{
		Path:         dbPath,
		JournalMode:  "WAL",
		MigrationsDir: filepath.Join("..", "..", "migrations"),
	}

	db, err := database.New(cfg)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}

	// Run migrations with context
	ctx := context.Background()
	if err := db.RunMigrations(ctx); err != nil {
		db.Close()
		t.Fatalf("failed to run migrations: %v", err)
	}

	defer db.Close()

	handler := NewInstrumentsHandler(db)

	// Create a test CSV file
	csvData := `id,symbol,name,publisher,instrument_class,currency,exchange,mic,asset,security_type,min_lot_size,expiration,max_price_variation,unit_of_measure_qty,min_price_increment,display_factor,price_display_format,price_ratio,underlying_symbol,maturity_year,maturity_month,maturity_day,group,tick_rule,strike_price,strike_price_currency
1234,TEST,Test Company,K,Stock,USD,NASDAQ,XNAS,Tech,Stock,,,,,,,,,,,,,,,Technology,,`

	// Test import from local path
	t.Run("ImportFromPath", func(t *testing.T) {
		// Write CSV to temp file
		tmpCSV := tmpDir + "/test.csv"
		if err := os.WriteFile(tmpCSV, []byte(csvData), 0644); err != nil {
			t.Fatalf("failed to write CSV file: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/api/v1/instruments/import-path?path="+tmpCSV, nil)
		w := httptest.NewRecorder()

		handler.ImportInstrumentsFromPath(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	// Test import from multipart file upload
	t.Run("ImportCSV", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		part, err := writer.CreateFormFile("file", "test.csv")
		if err != nil {
			t.Fatalf("failed to create form file: %v", err)
		}

		if _, err := io.Copy(part, bytes.NewBufferString(csvData)); err != nil {
			t.Fatalf("failed to copy CSV data: %v", err)
		}
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/instruments/import", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		handler.ImportInstrumentsCSV(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	// Test validation - missing required column
	t.Run("MissingRequiredColumn", func(t *testing.T) {
		// CSV missing 'id' column
		invalidCSV := `symbol,name,publisher,instrument_class,currency,exchange
TEST,Test,IB,Stock,USD,NASDAQ`

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		part, err := writer.CreateFormFile("file", "test.csv")
		if err != nil {
			t.Fatalf("failed to create form file: %v", err)
		}

		if _, err := io.Copy(part, bytes.NewBufferString(invalidCSV)); err != nil {
			t.Fatalf("failed to copy CSV data: %v", err)
		}
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/instruments/import", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		handler.ImportInstrumentsCSV(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestIsIntegerColumn(t *testing.T) {
	integerCols := []string{"id", "maturity_year", "maturity_month", "maturity_day"}
	nonIntegerCols := []string{"symbol", "name", "exchange", "min_lot_size", "strike_price", "price_ratio"}

	for _, col := range integerCols {
		t.Run("integer_"+col, func(t *testing.T) {
			if !isIntegerColumn(col) {
				t.Errorf("isIntegerColumn(%q) = false, want true", col)
			}
		})
	}

	for _, col := range nonIntegerCols {
		t.Run("non_integer_"+col, func(t *testing.T) {
			if isIntegerColumn(col) {
				t.Errorf("isIntegerColumn(%q) = true, want false", col)
			}
		})
	}
}

func TestIsFloatColumn(t *testing.T) {
	floatCols := []string{"min_lot_size", "max_price_variation", "unit_of_measure_qty", "min_price_increment", "display_factor", "price_ratio", "strike_price"}
	nonFloatCols := []string{"symbol", "name", "exchange", "id", "maturity_year"}

	for _, col := range floatCols {
		t.Run("float_"+col, func(t *testing.T) {
			if !isFloatColumn(col) {
				t.Errorf("isFloatColumn(%q) = false, want true", col)
			}
		})
	}

	for _, col := range nonFloatCols {
		t.Run("non_float_"+col, func(t *testing.T) {
			if isFloatColumn(col) {
				t.Errorf("isFloatColumn(%q) = true, want false", col)
			}
		})
	}
}

func TestImportStoresIntegersCorrectly(t *testing.T) {
	// Create a temporary database
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	cfg := config.DatabaseConfig{
		Path:         dbPath,
		JournalMode:  "WAL",
		MigrationsDir: filepath.Join("..", "..", "migrations"),
	}

	db, err := database.New(cfg)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}

	ctx := context.Background()
	if err := db.RunMigrations(ctx); err != nil {
		db.Close()
		t.Fatalf("failed to run migrations: %v", err)
	}

	defer db.Close()

	handler := NewInstrumentsHandler(db)

	// Test data with various numeric values - exactly 26 columns matching header
	csvData := "id,symbol,name,publisher,instrument_class,currency,exchange,mic,asset,security_type,min_lot_size,expiration,max_price_variation,unit_of_measure_qty,min_price_increment,display_factor,price_display_format,price_ratio,underlying_symbol,maturity_year,maturity_month,maturity_day,group,tick_rule,strike_price,strike_price_currency\n" +
		"1234567,TEST,Test Company,K,Stock,USD,NASDAQ,XNAS,Tech,Stock,,,,,,,,,TestUnderlying,2026,6,15,TechGroup,,," 

	// Write CSV to temp file
	tmpCSV := tmpDir + "/test.csv"
	if err := os.WriteFile(tmpCSV, []byte(csvData), 0644); err != nil {
		t.Fatalf("failed to write CSV file: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/instruments/import-path?path="+tmpCSV, nil)
	w := httptest.NewRecorder()

	handler.ImportInstrumentsFromPath(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the id was stored as an integer (not "1234567.0")
	var storedID interface{}
	err = db.QueryRow(ctx, "SELECT id FROM instruments WHERE symbol = ?", "TEST").Scan(&storedID)
	if err != nil {
		t.Fatalf("failed to query instrument: %v", err)
	}

	// Check that the ID is stored as an integer type, not a float
	switch v := storedID.(type) {
	case int64:
		// Correct - stored as integer
		if v != 1234567 {
			t.Errorf("id = %v, want 1234567", v)
		}
	case float64:
		t.Errorf("id stored as float64 %v instead of int64", v)
	case string:
		// SQLite might store as text if we pass a string
		// This is acceptable as long as it's not "1234567.0"
		if strings.Contains(v, ".") {
			t.Errorf("id stored as string with decimal: %q", v)
		}
	default:
		t.Logf("id stored as type %T with value %v", storedID, storedID)
	}

	// Verify maturity_year, month, day are integers
	var year, month, day int64
	err = db.QueryRow(ctx, "SELECT maturity_year, maturity_month, maturity_day FROM instruments WHERE symbol = ?", "TEST").
		Scan(&year, &month, &day)
	if err != nil {
		t.Fatalf("failed to query dates: %v", err)
	}

	if year != 2026 {
		t.Errorf("maturity_year = %v, want 2026", year)
	}
	if month != 6 {
		t.Errorf("maturity_month = %v, want 6", month)
	}
	if day != 15 {
		t.Errorf("maturity_day = %v, want 15", day)
	}
}

// BenchmarkImportCSV benchmarks the CSV import functionality.
func BenchmarkImportCSV(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := tmpDir + "/bench.db"

	cfg := config.DatabaseConfig{
		Path:         dbPath,
		JournalMode:  "WAL",
		MigrationsDir: filepath.Join("..", "..", "migrations"),
	}

	db, err := database.New(cfg)
	if err != nil {
		b.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	db.RunMigrations(nil)

	// Generate large CSV data
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write header
	writer.Write([]string{"id", "symbol", "name", "publisher", "instrument_class", "currency", "exchange", "mic", "asset", "security_type", "min_lot_size", "expiration", "max_price_variation", "unit_of_measure_qty", "min_price_increment", "display_factor", "price_display_format", "price_ratio", "underlying_symbol", "maturity_year", "maturity_month", "maturity_day", "group", "tick_rule", "strike_price", "strike_price_currency"})

	// Write 1000 rows
	for i := 0; i < 1000; i++ {
		writer.Write([]string{
			"1234", "TEST", "Test Company", "K", "Stock", "USD", "NASDAQ", "XNAS", "Tech", "Stock",
			"", "", "", "", "", "", "", "", "", "", "", "", "", "Technology", "", "", "",
		})
	}
	writer.Flush()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Reset database
		db.Close()
		os.Remove(dbPath)
		db, _ = database.New(cfg)
		db.RunMigrations(context.Background())

		// Import using the internal method directly
		csvReader := csv.NewReader(bytes.NewReader(buf.Bytes()))
		hdr, _ := csvReader.Read()
		rows, _ := csvReader.ReadAll()
		parsedRows := make([][]interface{}, len(rows))
		for j, row := range rows {
			parsedRow := make([]interface{}, len(hdr))
			for k, val := range row {
				parsedRow[k] = val
			}
			parsedRows[j] = parsedRow
		}
		db.ImportInstrumentsBatch(nil, hdr, parsedRows)
	}
}