# Implementation Plan: IB Historical Data Ingest Job

## Objective
Create a scheduled job that fetches historical OHLCV market data from Interactive Brokers and stores it in QuestDB.

## Scope
- [ ] **Handler**: `internal/jobs/providers/historical_data.go`
- [ ] **Job Registration**: `internal/jobs/registry.go`
- [ ] **Config**: Add job definition to `config.yaml`
- [ ] **Tests**: Unit tests for the handler

## Job Parameters (from config/metadata)
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `period` | string | `5y` | Time period for historical data |
| `bar` | string | `1d` | Bar size |
| `outsideRth` | bool | `false` | Include data outside regular trading hours |
| `instruments` | []string | nil (all) | Specific conids to fetch |

## Instrument Source
- **Specific instruments**: Passed via `instruments` array in job metadata
- **All instruments**: Query SQLite `instruments` table

## Step-by-Step Implementation

### Step 1: Create the Handler File
**File**: `internal/jobs/providers/historical_data.go`

Contents:
- [ ] `HistoricalDataHandler` function (worker.JobFunc signature)
- [ ] Parse job metadata for parameters
- [ ] Get IB client via `providers.GetIB()`
- [ ] Get QuestDB via dependency injection (from registry)
- [ ] Get instruments (specific list or from SQLite)
- [ ] For each instrument:
  - [ ] Fetch historical data via IB API
  - [ ] Convert response to OHLCV bars
  - [ ] Upsert to QuestDB in batches
- [ ] Return summary (count of instruments, bars upserted, failures)

### Step 2: Update Registry
**File**: `internal/jobs/registry.go`

Changes:
- [ ] Update `RegisterQuestDBHandlers` signature to accept `*database.DB` (SQLite)
- [ ] Register `historical_data` handler with QuestDB and SQLite instances

### Step 3: Update App Initialization
**File**: `cmd/server/app.go`

Changes:
- [ ] Update `RegisterQuestDBHandlers` call to pass `a.sqliteDB`

### Step 4: Add Config
**File**: `config.yaml`

New job entry:
```yaml
- id: "historical-data-ingest"
  name: "Historical Market Data Ingest"
  cron: "0 6 * * *"
  type: "market_data"
  handler: "historical_data"
  enabled: false
  timeout: 7200
  metadata:
    period: "5y"
    bar: "1d"
    outsideRth: false
```

Scheduler timezone should be set to `Asia/Hong_Kong`.

### Step 5: Write Tests
**File**: `internal/jobs/providers/historical_data_test.go`

Tests:
- [ ] Parameter parsing
- [ ] Instrument fetching from SQLite
- [ ] Instrument filtering by conid list
- [ ] Bar conversion logic
- [ ] Batch upsert logic

## Dependencies
- `internal/providers` - IB client
- `internal/database` - QuestDB, SQLite connections
- `internal/worker` - JobFunc type
- `internal/logging` - Structured logging

## QuestDB Table Schema
```sql
CREATE TABLE IF NOT EXISTS ohlcv_bars (
    symbol    SYMBOL,
    publisher SYMBOL,
    ts        TIMESTAMP,
    ts_end    TIMESTAMP,
    open      DOUBLE,
    high      DOUBLE,
    low       DOUBLE,
    close     DOUBLE,
    volume    LONG
) TIMESTAMP(ts) PARTITION BY DAY WAL;
```

Upsert key: `(symbol, ts)` - handles re-ingestion of historical data.

## Error Handling
- IB API failures: Log and continue with next instrument
- QuestDB failures: Log and continue (partial success OK)
- SQLite failure: Return error (can't get instruments)
- Missing IB/QuestDB: Return error with clear message

## Testing Checklist
- [ ] Build passes
- [ ] Tests pass
- [ ] Handler compiles with correct IB API fields
- [ ] Config validates

## Files to Modify/Create
| File | Action |
|------|--------|
| `internal/jobs/providers/historical_data.go` | Create |
| `internal/jobs/registry.go` | Modify |
| `cmd/server/app.go` | Modify |
| `config.yaml` | Modify |
| `internal/jobs/providers/historical_data_test.go` | Create (optional) |