-- Migration: Create jobs table
-- Version: 001

CREATE TABLE IF NOT EXISTS jobs (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    cron TEXT,
    type TEXT NOT NULL,
    handler TEXT NOT NULL,
    enabled INTEGER DEFAULT 1,
    timeout INTEGER DEFAULT 300,
    metadata TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);