# DataJobs

A production-grade Go server for scheduled bulk data ingestion and incremental data updates.

## Features

- **Cron-based scheduling** with configurable timezone
- **Bounded worker pool** with configurable concurrency
- **Exponential backoff** with configurable retry limits
- **Dead letter queue** for failed jobs
- **Job audit logging** with 90-day retention
- **Prometheus metrics** at `/metrics`
- **OpenTelemetry tracing** for distributed observability
- **Structured JSON logging** with request correlation IDs
- **REST API** for job management and manual triggers
- **Health checks** (liveness/readiness probes)
- **YAML configuration** with environment variable overrides

## Quick Start

```bash
# Build
go build -o datajobs ./cmd/server

# Run with default config
./datajobs

# Run with custom config
CONFIG_PATH=/path/to/config.yaml ./datajobs

# Run with env overrides
SERVER_PORT=9090 WORKER_POOL_SIZE=20 ./datajobs
```

## Configuration

See `config.yaml` for full configuration documentation.

### Core Settings

| Variable | Description | Default |
|----------|-------------|---------|
| `SERVER_HOST` | Bind host | `0.0.0.0` |
| `SERVER_PORT` | HTTP port | `8080` |
| `WORKER_POOL_SIZE` | Max concurrent jobs | `10` |
| `WORKER_QUEUE_CAPACITY` | Job queue size | `100` |
| `SCHEDULER_TIMEZONE` | Cron timezone | `Asia/Hong_Kong` |
| `LOG_LEVEL` | Log level | `info` |
| `LOG_FORMAT` | `json` or `text` | `json` |

### Database Settings

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_PATH` | SQLite database path | `assets/datajobs.db` |
| `QUESTDB_HOST` | QuestDB host | `localhost` |
| `QUESTDB_PORT` | QuestDB port | `8812` |

### API Providers

| Variable | Description | Default |
|----------|-------------|---------|
| `IB_BASE_URL` | IB Gateway URL | `https://localhost:5001` |
| `EARNINGS_BASE_URL` | Earnings API URL | `https://api.earningsapi.com` |
| `EARNINGS_RATE_LIMIT_PER_MIN` | API rate limit | `30` |
| `FMP_BASE_URL` | FMP API URL | `https://financialmodelingprep.com/api` |
| `FMP_RATE_LIMIT_PER_MIN` | API rate limit | `30` |

## API Endpoints

### Job Management

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/jobs` | List all jobs |
| `POST` | `/api/v1/jobs` | Create a job |
| `GET` | `/api/v1/jobs/{id}` | Get job details |
| `PUT` | `/api/v1/jobs/{id}` | Update a job |
| `DELETE` | `/api/v1/jobs/{id}` | Delete/disable a job |
| `POST` | `/api/v1/jobs/{id}/run` | Trigger immediate run |

### Audit Logs

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/audit/runs` | List all job runs |
| `GET` | `/api/v1/audit/runs/stats` | Get audit statistics |
| `GET` | `/api/v1/audit/jobs/{id}/runs` | Get runs for a specific job |
| `GET` | `/api/v1/audit/runs/{runId}` | Get single run details |

Query parameters: `status`, `start_date`, `end_date`, `limit`, `offset`

### Data & Instruments

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/marketdata/instruments` | List instruments |
| `GET` | `/api/v1/marketdata/history` | Get historical market data |
| `POST` | `/api/v1/instruments/import` | Import instruments from CSV |
| `POST` | `/api/v1/instruments/import-path` | Import from local path |

### QuestDB

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/questdb/tables` | List QuestDB tables |
| `GET` | `/api/v1/questdb/tables/{name}` | Get table schema |
| `POST` | `/api/v1/questdb/query` | Execute SQL query |

### System

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/dead-letter` | View dead letter queue |
| `GET` | `/api/v1/stats` | View server stats |
| `GET` | `/metrics` | Prometheus metrics |
| `GET` | `/healthz` | Liveness probe |
| `GET` | `/readyz` | Readiness probe |
| `GET` | `/status` | Full health status |

## Project Structure

```
.
├── cmd/
│   ├── server/           # Application entry point
│   └── migrate/          # Database migration tool
├── config.yaml           # Default configuration
├── internal/
│   ├── config/           # Configuration loading
│   ├── handlers/         # REST API handlers
│   ├── health/           # Health check endpoints
│   ├── audit/            # Job audit logging
│   ├── database/          # SQLite & QuestDB clients
│   ├── jobs/             # Job handlers
│   │   ├── calendar/     # Economic calendar sync
│   │   ├── historical/   # Historical market data
│   │   ├── ingestion/    # Data ingestion
│   │   └── stocks/      # Stock earnings sync
│   ├── logging/          # Structured logging
│   ├── metrics/          # Prometheus metrics
│   ├── providers/        # External API providers
│   │   ├── earnings/     # Earnings calendar API
│   │   ├── fmp/         # Financial Modeling Prep API
│   │   ├── ib/          # Interactive Brokers API
│   │   └── databento/   # Databento API
│   ├── ratelimiter/      # Generic token bucket rate limiter
│   ├── scheduler/        # Cron scheduler
│   ├── tracing/          # OpenTelemetry tracing
│   └── worker/          # Bounded worker pool
├── migrations/           # Database migrations
└── assets/              # Static assets
```

## Built-in Job Handlers

| Handler | Description |
|---------|-------------|
| `noop` | No-op handler for testing |
| `bulk_ingest` | Bulk data ingestion from CSV |
| `incremental_update` | Incremental data updates |
| `earnings_sync` | Stock earnings calendar sync |
| `economic_calendar_sync` | Economic calendar sync |
| `historical_data` | Historical OHLCV data from IB |
| `questdb_maintenance` | QuestDB maintenance tasks |

### Job Configuration

Jobs are configured in `config.yaml` with these options:

```yaml
jobs:
  - id: "earnings-sync"
    name: "Stock Earnings Daily Sync"
    cron: "0 4 * * *"  # Daily at 04:00 HKT
    handler: "earnings_sync"
    enabled: false
    audit: true         # Enable audit logging
    timeout: 3600
    retry:
      maxAttempts: 3
      initialDelay: 1000
      maxDelay: 60000
      multiplier: 2.0
    metadata:
      lookForwardDays: 30
```

## API Providers

### Earnings API

Fetches stock earnings calendar and economic calendar events.

```go
provider := earnings.NewProvider(client, earnings.WithRateLimit(30))
calendar, err := provider.EarningsCalendar(ctx, date)
```

### FMP API

Financial Modeling Prep API for financial ratios and metrics.

```go
client, err := fmp.NewClient()
provider := fmp.NewRateLimitedProvider(client, 30)

// Annual, quarterly, or TTM data
ratios, err := provider.FinancialRatios(ctx, "AAPL", fmp.PeriodAnnual)
metrics, err := provider.KeyMetrics(ctx, "AAPL", fmp.PeriodTTM)
```

### Interactive Brokers

Market data and historical OHLCV data via IB Gateway.

```go
client, _ := ib.NewClient(opts...)
history, err := client.HistoricalData(ctx, req)
```

## Rate Limiter

A generic token bucket rate limiter available for any provider:

```go
bucket := ratelimiter.NewTokenBucket(30) // 30 req/min
bucket.Allow(ctx)
```

Or wrap any interface:

```go
wrapper, _ := ratelimiter.NewWrapper(target, 30)
```

## Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package
go test ./internal/worker/...
```

## Metrics

Available at `/metrics`:

- `datajobs_jobs_executed_total` - Total jobs executed (by job_id, status)
- `datajobs_job_duration_seconds` - Job execution duration histogram
- `datajobs_job_retries_total` - Total job retries
- `datajobs_jobs_running` - Currently running jobs gauge
- `datajobs_job_queue_depth` - Current queue depth
- `datajobs_dead_letter_total` - Dead letter entries
- `datajobs_http_requests_total` - HTTP requests (by method, path, status)
- `datajobs_http_request_duration_seconds` - HTTP request duration

## Audit Logging

Job runs can be audited with configurable retention (default: 90 days).

```go
// Enable auditing per job in config.yaml
jobs:
  - id: "earnings-sync"
    audit: true
```

Audit data includes:
- Job ID and handler
- Start/completion timestamps
- Status (running, success, failure)
- Duration
- Parameters and results (as JSON)
- Error messages
