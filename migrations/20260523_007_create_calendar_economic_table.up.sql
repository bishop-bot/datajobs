-- Migration: Create calendar_economic table
-- Version: 007

CREATE TABLE IF NOT EXISTS calendar_economic (
    id INTEGER PRIMARY KEY,
    country TEXT NOT NULL,
    event_name TEXT NOT NULL,
    date TEXT NOT NULL,
    time TEXT,
    actual TEXT,
    consensus TEXT,
    previous TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_calendar_economic_country ON calendar_economic(country);
CREATE INDEX IF NOT EXISTS idx_calendar_economic_date ON calendar_economic(date);
CREATE UNIQUE INDEX IF NOT EXISTS idx_calendar_economic_country_event_date ON calendar_economic(country, event_name, date);
