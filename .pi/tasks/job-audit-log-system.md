# Job Audit Log System - Implementation Plan

## Overview

Create a system to audit job executions with configurable per-job auditing, storing job run details in SQLite for querying and monitoring.

---

## 1. Database Schema

### New table: `job_runs`

```sql
CREATE TABLE job_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id TEXT NOT NULL,
    job_name TEXT,
    started_at DATETIME NOT NULL,
    completed_at DATETIME,
    status TEXT NOT NULL,           -- 'running', 'success', 'failure', 'cancelled'
    error_message TEXT,
    duration_ms INTEGER,
    
    -- Parameters and metadata
    parameters TEXT,                 -- JSON blob of job parameters/metadata
    results TEXT,                    -- JSON blob of job results
    
    -- Run context
    attempt INTEGER DEFAULT 1,
    handler TEXT,
    
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for common queries
CREATE INDEX idx_job_runs_job_id ON job_runs(job_id);
CREATE INDEX idx_job_runs_started_at ON job_runs(started_at);
CREATE INDEX idx_job_runs_status ON job_runs(status);
```

---

## 2. Configuration Changes

### `internal/config/config.go`

```go
type JobConfig struct {
    ID       string                 `yaml:"id"`
    Name     string                 `yaml:"name"`
    Cron     string                 `yaml:"cron"`
    Type     string                 `yaml:"type"`
    Enabled  bool                   `yaml:"enabled"`
    Audit    bool                   `yaml:"audit"`  // NEW: enable/disable auditing
    Retry    RetryConfig            `yaml:"retry"`
    Handler  string                 `yaml:"handler"`
    Timeout  int                    `yaml:"timeout"`
    Metadata map[string]interface{} `yaml:"metadata"`
}
```

### `config.yaml` Example

```yaml
- id: "earnings-sync"
  name: "Stock Earnings Daily Sync"
  audit: true  # NEW: enable auditing (default: false)
  handler: "earnings_sync"
  # ...
```

---

## 3. Component Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                       Worker Pool                            │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐    │
│  │ executeJob() │───▶│ AuditLogger │───▶│   SQLite    │    │
│  └─────────────┘    └─────────────┘    └─────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

### `internal/audit/logger.go`

```go
package audit

// Logger handles job run audit logging.
type Logger struct {
    db *database.DB
}

// JobRun represents a single job execution record.
type JobRun struct {
    ID           int64
    JobID        string
    JobName      string
    StartedAt    time.Time
    CompletedAt  *time.Time
    Status       string
    ErrorMessage string
    DurationMs   int64
    Parameters   string  // JSON
    Results      string  // JSON
    Attempt      int
    Handler      string
}

// Start begins tracking a job run. Returns audit context.
func (l *Logger) Start(ctx context.Context, job worker.Job) (context.Context, func(string), func(error), error)

// Complete finishes tracking a job run with success.
func (l *Logger) Complete(ctx context.Context, result string) error

// Fail finishes tracking a job run with failure.
func (l *Logger) Fail(ctx context.Context, err error) error

// GetByJobID retrieves job runs for a specific job.
func (l *Logger) GetByJobID(ctx context.Context, jobID string, limit int) ([]*JobRun, error)

// GetRecent retrieves recent job runs across all jobs.
func (l *Logger) GetRecent(ctx context.Context, limit int) ([]*JobRun, error)
```

---

## 4. Audit Context

```go
// auditContextKey is used to store audit data in context.
type auditContext struct {
    runID      int64
    jobID      string
    startTime  time.Time
    params     string
}

// WithAudit adds audit tracking to a context.
func WithAudit(ctx context.Context, job worker.Job, runID int64) context.Context

// FromContext retrieves audit data from context.
func FromContext(ctx context.Context) (*auditContext, bool)
```

---

## 5. Worker Pool Integration

### `internal/worker/pool.go`

```go
type Pool struct {
    // ... existing fields
    auditLogger *audit.Logger
    jobConfigs  map[string]*config.JobConfig  // Store job configs for audit check
}

func (p *Pool) executeJob(job Job, attempt int) {
    // Check if job should be audited
    shouldAudit := p.shouldAudit(job)
    
    ctx := context.Background()
    var completeAudit func(string)
    var failAudit func(error)
    
    if shouldAudit && p.auditLogger != nil {
        ctx, completeAudit, failAudit, _ = p.auditLogger.Start(ctx, job)
        defer func() {
            if completeAudit != nil {
                completeAudit("")
            }
        }()
    }
    
    // Execute handler
    output, err := handler(execCtx, job)
    
    if err != nil {
        if failAudit != nil {
            failAudit(err)
        }
        // ... existing retry logic
        return
    }
    
    if completeAudit != nil {
        completeAudit(output)
    }
    // ... existing success logic
}

func (p *Pool) shouldAudit(job Job) bool {
    cfg, ok := p.jobConfigs[job.ID]
    if !ok {
        return false
    }
    return cfg.Audit
}
```

---

## 6. Implementation Steps

| Step | Task | Files |
|------|------|-------|
| 1 | Create migration for `job_runs` table | `migrations/YYYYMMDD_XXX_job_runs.up.sql` |
| 2 | Add `Audit` field to `JobConfig` | `internal/config/config.go` |
| 3 | Add default audit config handling | `internal/config/config.go` |
| 4 | Create audit logger package | `internal/audit/logger.go`, `types.go` |
| 5 | Integrate with worker pool | `internal/worker/pool.go` |
| 6 | Update config.yaml with `audit: true` for selected jobs | `config.yaml` |
| 7 | Add API endpoint to query audit logs | `internal/handlers/` |
| 8 | Write tests | `internal/audit/*_test.go` |

---

## 7. API Endpoints (Optional Enhancement)

```bash
# Get job run history
GET /api/v1/jobs/{id}/runs?limit=10

# Get recent runs across all jobs
GET /api/v1/runs?limit=50&status=failure

# Response
{
  "data": [
    {
      "id": 123,
      "job_id": "earnings-sync",
      "started_at": "2026-06-26T04:00:00Z",
      "completed_at": "2026-06-26T04:05:32Z",
      "status": "success",
      "duration_ms": 332000,
      "results": {"processed": 31, "upserted": 450}
    }
  ]
}
```

---

## 8. Configuration Defaults

```go
// Default audit setting - off by default
missingBool(&cfg.Jobs[i].Audit, false)
```

---

## 9. Open Questions

1. **Retention**: Should we auto-delete old audit logs? (e.g., keep 90 days)
2. **Sensitive Data**: Should we redact sensitive parameters in audit logs?
3. **Bulk Runs**: For jobs that run multiple times (like scheduled), should each run be logged separately or aggregated?
4. **Query API**: Do we need REST endpoints to query audit logs?

---

## 10. File Structure

```
internal/
├── audit/
│   ├── logger.go      # Main audit logger implementation
│   ├── types.go       # JobRun struct definition
│   └── logger_test.go # Tests
└── ...
```

---

## 11. Migration Files

### `migrations/YYYYMMDD_XXX_job_runs.up.sql`

```sql
CREATE TABLE IF NOT EXISTS job_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id TEXT NOT NULL,
    job_name TEXT,
    started_at DATETIME NOT NULL,
    completed_at DATETIME,
    status TEXT NOT NULL,
    error_message TEXT,
    duration_ms INTEGER,
    parameters TEXT,
    results TEXT,
    attempt INTEGER DEFAULT 1,
    handler TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_job_runs_job_id ON job_runs(job_id);
CREATE INDEX IF NOT EXISTS idx_job_runs_started_at ON job_runs(started_at);
CREATE INDEX IF NOT EXISTS idx_job_runs_status ON job_runs(status);
```

### `migrations/YYYYMMDD_XXX_job_runs.down.sql`

```sql
DROP TABLE IF EXISTS job_runs;
```
