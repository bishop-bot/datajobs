-- Migration: Create job_runs table for audit logging
-- Version: 002

CREATE TABLE IF NOT EXISTS job_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id TEXT NOT NULL,
    started_at DATETIME NOT NULL,
    completed_at DATETIME,
    status TEXT NOT NULL,              -- 'running', 'success', 'failure', 'cancelled'
    error_message TEXT,
    duration_ms INTEGER,
    
    -- Parameters and results as JSON
    parameters TEXT,                    -- JSON blob of job parameters/metadata
    results TEXT,                      -- JSON blob of structured job results
    
    -- Run context
    attempt INTEGER DEFAULT 1,
    handler TEXT,
    
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (job_id) REFERENCES jobs(id)
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_job_runs_job_id ON job_runs(job_id);
CREATE INDEX IF NOT EXISTS idx_job_runs_started_at ON job_runs(started_at);
CREATE INDEX IF NOT EXISTS idx_job_runs_status ON job_runs(status);
CREATE INDEX IF NOT EXISTS idx_job_runs_created_at ON job_runs(created_at);
