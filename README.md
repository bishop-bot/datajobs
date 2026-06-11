# DataJobs

A production-grade Go server for scheduled bulk data ingestion and incremental data updates.

## Features

- **Cron-based scheduling** using `github.com/netresearch/go-cron`
- **Bounded worker pool** with configurable concurrency
- **Exponential backoff** with configurable retry limits
- **Dead letter queue** for failed jobs
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

See `config/examples.yaml` for full configuration documentation.

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `SERVER_HOST` | Bind host | `0.0.0.0` |
| `SERVER_PORT` | HTTP port | `8080` |
| `WORKER_POOL_SIZE` | Max concurrent jobs | `10` |
| `WORKER_QUEUE_CAPACITY` | Job queue size | `100` |
| `SCHEDULER_TIMEZONE` | Cron timezone | `UTC` |
| `METRICS_ENABLED` | Enable Prometheus | `true` |
| `METRICS_PATH` | Metrics endpoint | `/metrics` |
| `LOG_LEVEL` | Log level | `info` |
| `LOG_FORMAT` | `json` or `text` | `json` |

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/jobs` | List all jobs |
| `POST` | `/api/v1/jobs` | Create a job |
| `GET` | `/api/v1/jobs/{id}` | Get job details |
| `PUT` | `/api/v1/jobs/{id}` | Update a job |
| `DELETE` | `/api/v1/jobs/{id}` | Delete/disable a job |
| `POST` | `/api/v1/jobs/{id}/run` | Trigger immediate run |
| `GET` | `/api/v1/dead-letter` | View dead letter queue |
| `GET` | `/api/v1/stats` | View server stats |
| `GET` | `/api/v1/marketdata/instruments` | List instruments |
| `POST` | `/api/v1/instruments/import` | Import instruments from CSV file |
| `POST` | `/api/v1/instruments/import-path` | Import instruments from local path |
| `GET` | `/metrics` | Prometheus metrics |
| `GET` | `/healthz` | Liveness probe |
| `GET` | `/readyz` | Readiness probe |
| `GET` | `/status` | Full health status |

## Project Structure

```
.
├── cmd/server/main.go      # Application entry point
├── config.yaml             # Default configuration
├── config/examples.yaml    # Full configuration reference
├── internal/
│   ├── config/             # Configuration loading
│   ├── handlers/           # REST API handlers
│   ├── health/             # Health check endpoints
│   ├── jobs/               # Built-in job handlers
│   ├── logging/            # Structured logging
│   ├── metrics/            # Prometheus metrics
│   ├── scheduler/          # Cron scheduler
│   ├── tracing/            # OpenTelemetry tracing
│   └── worker/             # Bounded worker pool
└── internal/*_test.go      # Unit tests
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

## Instruments Import

Import instrument data from CSV files into the SQLite database.

### Endpoints

#### Upload CSV File
```bash
curl -X POST http://localhost:8080/api/v1/instruments/import \
  -F "file=@assets/XNAS.csv"
```

#### Import from Local Path
```bash
curl -X POST "http://localhost:8080/api/v1/instruments/import-path?path=./assets/XNAS.csv"
```

### Response

```json
{
  "success": true,
  "data": {
    "imported": 19,
    "skipped": 0,
    "errors": []
  },
  "message": "imported 19 instruments"
}
```

### CSV Format

The CSV must include these required columns:
- `id` - Instrument identifier
- `symbol` - Ticker symbol
- `name` - Full name
- `publisher` - Publisher/exchange code
- `instrument_class` - Instrument class (e.g., "K" for stock)

Optional columns: `currency`, `exchange`, `mic`, `asset`, `security_type`, `min_lot_size`, `expiration`, `max_price_variation`, `unit_of_measure_qty`, `min_price_increment`, `display_factor`, `price_display_format`, `price_ratio`, `underlying_symbol`, `maturity_year`, `maturity_month`, `maturity_day`, `group`, `tick_rule`, `strike_price`, `strike_price_currency`

### Upsert Behavior

Import uses `INSERT OR REPLACE`, so existing instruments with the same `id` are updated rather than duplicated.

## Built-in Job Handlers

- `noop` - No-op handler for testing
- `bulk_ingest` - Bulk data ingestion placeholder
- `incremental_update` - Incremental update placeholder
- `questdb_maintenance` - QuestDB maintenance placeholder
- `sqlite_to_questdb` - SQLite to QuestDB sync placeholder

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