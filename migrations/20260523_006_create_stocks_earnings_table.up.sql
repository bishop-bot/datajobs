-- Migration: Create stocks_earnings table
-- Version: 006

CREATE TABLE IF NOT EXISTS stocks_earnings (
    id INTEGER PRIMARY KEY,
    symbol TEXT NOT NULL,
    name TEXT,
    mic TEXT,
    isin TEXT,
    type TEXT,
    hour TEXT,
    status TEXT,
    eps REAL,
    eps_estimated REAL,
    revenue INTEGER,
    revenue_estimated INTEGER,
    date TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_stocks_earnings_symbol ON stocks_earnings(symbol);
CREATE INDEX IF NOT EXISTS idx_stocks_earnings_date ON stocks_earnings(date);
CREATE INDEX IF NOT EXISTS idx_stocks_earnings_mic ON stocks_earnings(mic);
CREATE UNIQUE INDEX IF NOT EXISTS idx_stocks_earnings_symbol_date ON stocks_earnings(symbol, date);
