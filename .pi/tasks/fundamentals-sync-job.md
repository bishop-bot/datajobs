# Fundamentals Sync Job Implementation

## Overview

A new scheduled job `fundamentals_sync` has been implemented to fetch and store stock fundamental metrics from the Financial Modeling Prep (FMP) API into the SQLite database.

## Job Configuration

| Setting | Value |
|---------|-------|
| Job ID | `fundamentals-sync` |
| Schedule | `0 6 * * 6` (Weekly on Saturday at 06:00 HKT) |
| Handler | `fundamentals_sync` |
| Enabled | `true` |
| Audit | Enabled |
| Timeout | 3600 seconds |

### Job Metadata Parameters

```yaml
metadata:
  watchlistId: "fmpfree"  # Watchlist containing symbols to sync
  provider: "FMP"         # Data provider name (for multi-provider support)
```

## Data Flow

```
1. Job Scheduler ──triggers──> fundamentals_sync handler
2. Handler ──reads symbols──> watchlist (fmpfree)
3. For each symbol:
   a. Handler ──calls──> FMP FinancialRatios API (TTM period)
   b. Handler ──calls──> FMP KeyMetrics API (TTM period)
   c. Handler ──queries──> QuestDB ohlcv_bars (latest close price)
4. Handler ──upserts──> stock_metrics_ttm table
5. Audit log ──records──> job_runs table
```

## API Calls Per Symbol

Each symbol requires **2 API calls** to FMP:
1. `GET /v3/ratios-ttm/{symbol}` - Financial ratios
2. `GET /v3/key-metrics-ttm/{symbol}` - Key metrics

**Rate Limiting:** The FMP API is rate-limited (default: 30 requests/min). Consider implementing rate limiting wrapper.

## Field Mappings

| FMP API Response | FMP Field | Table Column |
|------------------|-----------|-------------|
| FinancialRatios | `symbol` | `symbol` |
| FinancialRatios | `date` | `date` |
| FinancialRatios | `cashRatio` | `cash` |
| FinancialRatios | `currentRatio` | `current` |
| FinancialRatios | `quickRatio` | `quick` |
| FinancialRatios | `debtToEquity` | `debt_to_equity` |
| FinancialRatios | `payoutRatio` | `dividend_payout` |
| FinancialRatios | `dividendYield` | `dividend_yield` |
| FinancialRatios | `priceEarningsRatio` | `price_to_earnings` |
| FinancialRatios | `priceToBookRatio` | `price_to_book` |
| FinancialRatios | `priceToSalesRatio` | `price_to_sales` |
| FinancialRatios | `priceToFreeCashFlows` | `price_to_free_cash_flow` |
| FinancialRatios | `priceToOperatingCF` | `price_to_cash_flow` |
| FinancialRatios | `returnOnAssets` | `return_on_assets` |
| FinancialRatios | `returnOnEquity` | `return_on_equity` |
| KeyMetrics | `enterpriseValue` | `enterprise_value` |
| KeyMetrics | `freeCashFlow` | `free_cash_flow` |
| QuestDB | `close` (latest) | `price` |

### Fields Left NULL

The following fields are not available from FinancialRatios/KeyMetrics APIs:
- `cik` - Would require separate company profile API call
- `currency` - Would require separate API call
- `year` - Set to current year from system time

## Database Schema

### Table: `stock_metrics_ttm`

```sql
CREATE TABLE stock_metrics_ttm (
    symbol TEXT NOT NULL PRIMARY KEY,
    cik TEXT,
    date TEXT NOT NULL,
    year INTEGER NOT NULL,
    provider TEXT NOT NULL,
    cash DOUBLE,
    current DOUBLE,
    currency TEXT,
    debt_to_equity DOUBLE,
    dividend_payout DOUBLE,
    dividend_yield DOUBLE,
    enterprise_value DOUBLE,
    free_cash_flow DOUBLE,
    price DOUBLE,
    price_to_book DOUBLE,
    price_to_cash_flow DOUBLE,
    price_to_earnings DOUBLE,
    price_to_free_cash_flow DOUBLE,
    price_to_sales DOUBLE,
    quick DOUBLE,
    return_on_assets DOUBLE,
    return_on_equity DOUBLE
);
```

### Sync Logic

- **New symbol + new provider:** INSERT new record
- **Existing symbol + same provider:** UPDATE existing record
- **Existing symbol + different provider:** INSERT additional record (allows multiple providers per symbol)

## Files Changed/Created

### New Files

| File | Description |
|------|-------------|
| `internal/jobs/stocks/fundamentals/types.go` | `StockMetricsTTM` struct definition |
| `internal/jobs/stocks/fundamentals/repository.go` | Database operations (upsert logic) |
| `internal/jobs/stocks/fundamentals/handler.go` | Job handler implementation |

### Modified Files

| File | Changes |
|------|---------|
| `internal/providers/fmp/types.go` | Added `EnterpriseValue` field to `KeyMetricsResponse` |
| `internal/jobs/registry.go` | Added `RegisterFundamentalsHandlers()` function |
| `cmd/server/app.go` | Added FMP client initialization and WatchlistRepository |
| `config.yaml` | Added `fundamentals-sync` job configuration |

## Audit Results

The job returns audit results with the following information:

```json
{
  "job_id": "fundamentals-sync",
  "status": "success",
  "results": "fundamentals_sync completed: provider=FMP, watchlist=fmpfree, total=87, inserted=5 (AAPL, GOOGL, MSFT, ...), updated=80, failed=2"
}
```

### Audit Result Format

```
fundamentals_sync completed: provider={provider}, watchlist={watchlistId}, 
  total={totalSymbols}, inserted={n} ({symbolList}), 
  updated={n} ({symbolList}), failed={n} ({symbolList})
```

## Dependencies

| Dependency | Purpose |
|------------|---------|
| SQLite DB | Read watchlists, write stock_metrics_ttm |
| QuestDB | Read latest closing prices from ohlcv_bars |
| FMP Provider | Fetch FinancialRatios and KeyMetrics |
| Watchlist Repository | Read symbols from watchlist |
| Audit Logger | Record job runs |

## Error Handling

- **API failures:** Symbol is logged as failed, job continues with remaining symbols
- **Missing data:** NULL values are inserted for unavailable fields
- **QuestDB unavailable:** Price is left as NULL, job continues
- **Invalid API response:** Symbol is logged and skipped

## Future Enhancements

1. **Batch API calls:** FMP supports batch endpoints for multiple symbols in one request
2. **Rate limiting wrapper:** Implement rate limiting similar to earnings provider
3. **Annual/Quarterly support:** Extend to populate `stock_metrics_annual` and `stock_metrics_quarter` tables
4. **CIK enrichment:** Add separate API call or lookups to get CIK codes
5. **Currency field:** Add currency lookup for each symbol

## Testing

Run the job tests:
```bash
go test ./internal/jobs/stocks/fundamentals/...
```

Run all tests:
```bash
go test ./...
```
