-- Migration: Create schemas table
-- Version: 004

CREATE TABLE IF NOT EXISTS schemas (
    table_name TEXT PRIMARY KEY,
    columns TEXT NOT NULL,
    timestamp_column TEXT,
    symbol_columns TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);