-- Migration: Create instruments table
-- Version: 005

CREATE TABLE IF NOT EXISTS instruments (
    id TEXT PRIMARY KEY,
    symbol TEXT NOT NULL,
    publisher TEXT NOT NULL,
    instrument_class TEXT NOT NULL,
    currency TEXT,
    exchange TEXT,
    asset TEXT,
    min_lot_size REAL,
    expiration DATETIME,
    max_price_variation REAL,
    unit_of_measure_qty REAL,
    min_price_increment REAL,
    display_factor REAL,
    price_display_format TEXT,
    price_ratio REAL,
    underlying_symbol TEXT,
    maturity_year INTEGER,
    maturity_month INTEGER,
    maturity_day INTEGER,
    group_ TEXT,
    tick_rule TEXT,
    strike_price REAL,
    strike_price_currency TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_instruments_symbol ON instruments(symbol);
CREATE INDEX IF NOT EXISTS idx_instruments_publisher ON instruments(publisher);
CREATE INDEX IF NOT EXISTS idx_instruments_exchange ON instruments(exchange);
CREATE INDEX IF NOT EXISTS idx_instruments_instrument_class ON instruments(instrument_class);
CREATE INDEX IF NOT EXISTS idx_instruments_underlying_symbol ON instruments(underlying_symbol);