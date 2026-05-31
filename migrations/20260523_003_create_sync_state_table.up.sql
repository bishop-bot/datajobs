-- Migration: Create sync_state table
-- Version: 003

CREATE TABLE IF NOT EXISTS sync_state (
    id TEXT PRIMARY KEY,
    source_type TEXT NOT NULL,
    last_sync_at DATETIME,
    last_synced_key TEXT,
    status TEXT DEFAULT 'idle',
    records_synced INTEGER DEFAULT 0,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);