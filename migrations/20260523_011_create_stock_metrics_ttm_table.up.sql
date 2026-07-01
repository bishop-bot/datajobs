-- Migration: Create stock_metrics_ttm table
-- Version: 011
-- Description: Trailing twelve months (TTM) stock fundamental metrics

CREATE TABLE IF NOT EXISTS stock_metrics_ttm (
    symbol TEXT NOT NULL,
    cik TEXT,
    date TEXT NOT NULL,
    year INTEGER NOT NULL,
    provider TEXT,
    cash DOUBLE,
    current DOUBLE,
    currency TEXT,
    debt_to_equity DOUBLE,
    dividend_payout DOUBLE,
    dividend_yield DOUBLE,
    enterprise_value DOUBLE,
    ev_to_revenue DOUBLE,
    ev_to_ebitda DOUBLE,
    free_cash_flow DOUBLE,
    gross_profit_margin DOUBLE,
    net_profit_margin DOUBLE,
    operating_profit_margin DOUBLE,
    price DOUBLE,
    price_to_book DOUBLE,
    price_to_cash_flow DOUBLE,
    price_to_earnings DOUBLE,
    price_to_free_cash_flow DOUBLE,
    price_to_sales DOUBLE,
    quick DOUBLE,
    return_on_assets DOUBLE,
    return_on_equity DOUBLE,
    return_on_capital_employed DOUBLE,
    roic DOUBLE,
    created_at TEXT,
    updated_at TEXT,
    PRIMARY KEY (symbol)
);

-- Index for looking up by symbol
CREATE INDEX IF NOT EXISTS idx_stock_metrics_ttm_symbol ON stock_metrics_ttm(symbol);
