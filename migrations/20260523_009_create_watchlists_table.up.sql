-- Migration: Create watchlists table
-- Version: 009

CREATE TABLE IF NOT EXISTS watchlists (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    owner TEXT NOT NULL,
    is_public BOOLEAN DEFAULT true,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS watchlist_symbols (
    watchlist_id TEXT NOT NULL,
    symbol TEXT NOT NULL,
    note TEXT,
    position INTEGER,  -- display order within watchlist
    added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (watchlist_id, symbol),
    FOREIGN KEY (watchlist_id) REFERENCES watchlists(id) ON DELETE CASCADE,
    FOREIGN KEY (symbol) REFERENCES instruments(symbol) ON DELETE CASCADE
);

-- Index for looking up watchlists by owner
CREATE INDEX IF NOT EXISTS idx_watchlists_owner ON watchlists(owner);

-- Index for looking up symbols across all watchlists
CREATE INDEX IF NOT EXISTS idx_watchlist_symbols_symbol ON watchlist_symbols(symbol);

-- Index for ordered symbol lookups
CREATE INDEX IF NOT EXISTS idx_watchlist_symbols_position ON watchlist_symbols(watchlist_id, position);
