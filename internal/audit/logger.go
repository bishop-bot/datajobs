package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bishop-bot/datajobs/internal/database"
	"github.com/bishop-bot/datajobs/internal/logging"
)

// Logger handles job run audit logging.
type Logger struct {
	db       *database.DB
	retention time.Duration
}

// NewLogger creates a new audit logger.
func NewLogger(db *database.DB) *Logger {
	return &Logger{
		db:       db,
		retention: 90 * 24 * time.Hour, // 90 days
	}
}

// NewLoggerWithRetention creates a new audit logger with custom retention.
func NewLoggerWithRetention(db *database.DB, retentionDays int) *Logger {
	return &Logger{
		db:       db,
		retention: time.Duration(retentionDays) * 24 * time.Hour,
	}
}

// Start begins tracking a job run. Returns functions to complete or fail the run.
func (l *Logger) Start(ctx context.Context, jobID, jobName, handler string, metadata map[string]interface{}, attempt int) (complete func(results map[string]interface{}), fail func(err error), err error) {
	// Serialize parameters to JSON
	paramsJSON, err := json.Marshal(metadata)
	if err != nil {
		paramsJSON = []byte("{}")
	}

	query := `
		INSERT INTO job_runs (
			job_id, job_name, started_at, status, parameters, attempt, handler
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now().UTC()
	result, err := l.db.Exec(ctx, query,
		jobID,
		jobName,
		now,
		StatusRunning,
		string(paramsJSON),
		attempt,
		handler,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to insert job run: %w", err)
	}

	runID, err := result.LastInsertId()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get run ID: %w", err)
	}

	// Create completion and failure functions
	complete = func(results map[string]interface{}) {
		l.complete(ctx, runID, jobID, StatusSuccess, results, "")
	}

	fail = func(err error) {
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		l.complete(ctx, runID, jobID, StatusFailure, nil, errMsg)
	}

	return complete, fail, nil
}

// complete finalizes a job run record.
func (l *Logger) complete(ctx context.Context, runID int64, jobID string, status JobRunStatus, results map[string]interface{}, errMsg string) {
	var resultsJSON []byte
	var err error

	if results != nil {
		resultsJSON, err = json.Marshal(results)
		if err != nil {
			resultsJSON = []byte("{}")
		}
	}

	completedAt := time.Now().UTC()
	durationMs := completedAt.Sub(time.Now().UTC()).Milliseconds()

	// Get duration by querying the record
	var startedAt time.Time
	err = l.db.QueryRow(ctx, "SELECT started_at FROM job_runs WHERE id = ?", runID).Scan(&startedAt)
	if err == nil {
		durationMs = int64(completedAt.Sub(startedAt).Milliseconds())
	}

	query := `
		UPDATE job_runs SET
			completed_at = ?,
			status = ?,
			error_message = ?,
			duration_ms = ?,
			results = ?
		WHERE id = ?
	`

	_, err = l.db.Exec(ctx, query,
		completedAt,
		status,
		errMsg,
		durationMs,
		string(resultsJSON),
		runID,
	)
	if err != nil {
		logging.Error("failed to update job run",
			"run_id", runID,
			"job_id", jobID,
			"error", err,
		)
	}
}

// CompleteSuccess records a successful job completion.
func (l *Logger) CompleteSuccess(ctx context.Context, runID int64, durationMs int64, results map[string]interface{}) error {
	resultsJSON, err := json.Marshal(results)
	if err != nil {
		resultsJSON = []byte("{}")
	}

	query := `
		UPDATE job_runs SET
			completed_at = ?,
			status = ?,
			duration_ms = ?,
			results = ?
		WHERE id = ?
	`

	_, err = l.db.Exec(ctx, query,
		time.Now().UTC(),
		StatusSuccess,
		durationMs,
		string(resultsJSON),
		runID,
	)
	return err
}

// CompleteFailure records a failed job completion.
func (l *Logger) CompleteFailure(ctx context.Context, runID int64, durationMs int64, errMsg string) error {
	query := `
		UPDATE job_runs SET
			completed_at = ?,
			status = ?,
			duration_ms = ?,
			error_message = ?
		WHERE id = ?
	`

	_, err := l.db.Exec(ctx, query,
		time.Now().UTC(),
		StatusFailure,
		durationMs,
		errMsg,
		runID,
	)
	return err
}

// GetByJobID retrieves job runs for a specific job.
func (l *Logger) GetByJobID(ctx context.Context, query JobRunQuery) ([]*JobRun, error) {
	if query.Limit <= 0 {
		query.Limit = 50
	}

	sqlQuery := `
		SELECT id, job_id, job_name, started_at, completed_at, status, 
		       error_message, duration_ms, parameters, results, attempt, handler, created_at
		FROM job_runs
		WHERE 1=1
	`
	args := []interface{}{}

	if query.JobID != "" {
		sqlQuery += " AND job_id = ?"
		args = append(args, query.JobID)
	}
	if query.Status != "" {
		sqlQuery += " AND status = ?"
		args = append(args, query.Status)
	}
	if query.StartDate != nil {
		sqlQuery += " AND started_at >= ?"
		args = append(args, *query.StartDate)
	}
	if query.EndDate != nil {
		sqlQuery += " AND started_at <= ?"
		args = append(args, *query.EndDate)
	}

	sqlQuery += " ORDER BY started_at DESC LIMIT ? OFFSET ?"
	args = append(args, query.Limit, query.Offset)

	return l.queryRuns(ctx, sqlQuery, args...)
}

// GetRecent retrieves recent job runs across all jobs.
func (l *Logger) GetRecent(ctx context.Context, limit int) ([]*JobRun, error) {
	if limit <= 0 {
		limit = 50
	}

	query := JobRunQuery{Limit: limit}
	return l.GetByJobID(ctx, query)
}

// GetByID retrieves a single job run by ID.
func (l *Logger) GetByID(ctx context.Context, runID int64) (*JobRun, error) {
	query := `
		SELECT id, job_id, job_name, started_at, completed_at, status, 
		       error_message, duration_ms, parameters, results, attempt, handler, created_at
		FROM job_runs
		WHERE id = ?
	`

	runs, err := l.queryRuns(ctx, query, runID)
	if err != nil {
		return nil, err
	}
	if len(runs) == 0 {
		return nil, nil
	}
	return runs[0], nil
}

// queryRuns executes a query and returns JobRun slice.
func (l *Logger) queryRuns(ctx context.Context, query string, args ...interface{}) ([]*JobRun, error) {
	rows, err := l.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query job runs: %w", err)
	}
	defer rows.Close()

	var runs []*JobRun
	for rows.Next() {
		var r JobRun
		var completedAt sql.NullTime
		var jobName, errorMsg, params, results, handler sql.NullString
		var durationMs sql.NullInt64

		err := rows.Scan(
			&r.ID, &r.JobID, &jobName, &r.StartedAt, &completedAt,
			&r.Status, &errorMsg, &durationMs, &params, &results,
			&r.Attempt, &handler, &r.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if completedAt.Valid {
			r.CompletedAt = &completedAt.Time
		}
		if jobName.Valid {
			r.JobName = jobName.String
		}
		if errorMsg.Valid {
			r.ErrorMessage = errorMsg.String
		}
		if params.Valid {
			r.Parameters = params.String
		}
		if results.Valid {
			r.Results = results.String
		}
		if handler.Valid {
			r.Handler = handler.String
		}
		if durationMs.Valid {
			r.DurationMs = durationMs.Int64
		}

		runs = append(runs, &r)
	}

	return runs, rows.Err()
}

// Cleanup deletes job runs older than the retention period.
func (l *Logger) Cleanup(ctx context.Context) (int64, error) {
	cutoff := time.Now().UTC().Add(-l.retention)

	query := `DELETE FROM job_runs WHERE created_at < ?`
	result, err := l.db.Exec(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup job runs: %w", err)
	}

	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get deleted count: %w", err)
	}

	if deleted > 0 {
		logging.Info("cleaned up old job runs", "deleted", deleted, "cutoff", cutoff)
	}

	return deleted, nil
}

// GetStats returns statistics about job runs.
func (l *Logger) GetStats(ctx context.Context) (*Stats, error) {
	stats := NewStats()

	// Total runs
	err := l.db.QueryRow(ctx, "SELECT COUNT(*) FROM job_runs").Scan(&stats.TotalRuns)
	if err != nil {
		return nil, err
	}

	// Runs by status
	queries := []struct {
		status JobRunStatus
		query  string
	}{
		{StatusSuccess, "SELECT COUNT(*) FROM job_runs WHERE status = 'success'"},
		{StatusFailure, "SELECT COUNT(*) FROM job_runs WHERE status = 'failure'"},
		{StatusRunning, "SELECT COUNT(*) FROM job_runs WHERE status = 'running'"},
	}

	for _, q := range queries {
		var count int64
		err := l.db.QueryRow(ctx, q.query).Scan(&count)
		if err != nil {
			return nil, err
		}
		stats.ByStatus[q.status] = count
	}

	// Recent runs
	recent, err := l.GetRecent(ctx, 5)
	if err != nil {
		return nil, err
	}
	stats.RecentRuns = recent

	return stats, nil
}

// Stats contains audit statistics.
type Stats struct {
	TotalRuns  int64                       `json:"total_runs"`
	ByStatus  map[JobRunStatus]int64       `json:"by_status"`
	RecentRuns []*JobRun                   `json:"recent_runs"`
}

// NewStats creates a new Stats instance with initialized maps.
func NewStats() *Stats {
	return &Stats{
		ByStatus: make(map[JobRunStatus]int64),
	}
}
