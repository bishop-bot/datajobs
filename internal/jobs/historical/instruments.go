package historical

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/bishop-bot/datajobs/internal/database"
	"github.com/bishop-bot/datajobs/internal/logging"
	"github.com/bishop-bot/datajobs/internal/worker"
)

const (
	queryAllInstrumentsSQL = `SELECT id, symbol, exchange, security_type FROM instruments ORDER BY symbol`
)

// getInstruments determines which instruments to fetch based on job params.
func getInstruments(ctx context.Context, job worker.Job, params historicalParams, sqliteDB *database.DB) ([]instrument, error) {
	if len(params.Instruments) > 0 {
		return getInstrumentsByConids(ctx, params.Instruments, sqliteDB)
	}
	return getAllInstruments(ctx, sqliteDB)
}

// getInstrumentsByConids fetches instruments by their conids from SQLite.
// Returns an empty slice (not nil) if no conids are provided or none are found.
func getInstrumentsByConids(ctx context.Context, conids []string, sqliteDB *database.DB) ([]instrument, error) {
	if len(conids) == 0 || sqliteDB == nil {
		return []instrument{}, nil
	}

	query := buildInClauseQuery(conids)
	args := conidsToArgs(conids)

	logging.Debug("querying instruments by conids",
		"conids", conids,
		"count", len(conids),
	)

	instruments, err := queryInstruments(ctx, sqliteDB, query, args)
	if err != nil {
		return nil, fmt.Errorf("failed to query instruments by conids (count=%d): %w", len(conids), err)
	}
	return instruments, nil
}

// getAllInstruments fetches all instruments from the SQLite database.
func getAllInstruments(ctx context.Context, sqliteDB *database.DB) ([]instrument, error) {
	if sqliteDB == nil {
		return nil, fmt.Errorf("SQLite DB not available")
	}

	instruments, err := queryInstruments(ctx, sqliteDB, queryAllInstrumentsSQL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query all instruments: %w", err)
	}
	return instruments, nil
}

// queryInstruments executes a query and scans the results into instrument slice.
func queryInstruments(ctx context.Context, db *database.DB, query string, args []interface{}) ([]instrument, error) {
	logging.Debug("querying instruments",
		"query", query,
		"args_count", len(args),
	)

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		// Return wrapped error with context for debugging
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	return scanInstruments(rows)
}

// scanInstruments scans rows into instrument slice.
func scanInstruments(rows *sql.Rows) ([]instrument, error) {
	var instruments []instrument
	var scanErrors int

	for rows.Next() {
		var instr instrument
		var securityType sql.NullString
		if err := rows.Scan(&instr.Conid, &instr.Symbol, &instr.Exchange, &securityType); err != nil {
			scanErrors++
			continue
		}
		if securityType.Valid {
			instr.SecurityType = securityType.String
		}
		instruments = append(instruments, instr)
	}

	if scanErrors > 0 {
		logging.Warn("dropped rows due to scan errors", "count", scanErrors)
	}
	return instruments, rows.Err()
}

// buildInClauseQuery builds a SELECT query with IN clause for conids.
func buildInClauseQuery(conids []string) string {
	placeholders := make([]string, len(conids))
	for i := range conids {
		placeholders[i] = "?"
	}
	return fmt.Sprintf(
		"SELECT id, symbol, exchange, security_type FROM instruments WHERE id IN (%s)",
		strings.Join(placeholders, ", "),
	)
}

// conidsToArgs converts conids slice to interface slice for query args.
func conidsToArgs(conids []string) []interface{} {
	args := make([]interface{}, len(conids))
	for i, c := range conids {
		args[i] = c
	}
	return args
}