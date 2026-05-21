# Task: Production-Grade Go Job Server

## Status: ✅ Complete

## Date: 2026-05-20

## Overview
Created a production-grade Go server for scheduled bulk data ingestion and incremental data updates.

## Architecture Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Job Scheduler | github.com/netresearch/go-cron | In-memory, lightweight, zero dependencies |
| Worker Pool | Bounded goroutines | Controlled resource usage |
| Metrics | Prometheus | Industry standard, /metrics endpoint |
| Tracing | OpenTelemetry | Distributed observability |
| Logging | Structured JSON (slog) | Machine-parseable with correlation IDs |
| Config | YAML + env overrides | Flexible deployment options |
| Health | Liveness/readiness probes | k8s/Docker compatibility |

## Features Implemented

### Core Infrastructure
- [x] In-memory cron scheduler with timezone support
- [x] Bounded worker pool with configurable size
- [x] Exponential backoff with configurable max retries
- [x] Dead letter queue for failed jobs
- [x] Graceful shutdown

### Observability
- [x] Prometheus metrics at `/metrics`
- [x] OpenTelemetry tracing (configurable exporter)
- [x] Structured JSON logging with request IDs
- [x] Health/readiness probes (`/healthz`, `/readyz`, `/status`)

### API
- [x] Job CRUD endpoints
- [x] Manual job trigger (`POST /jobs/:id/run`)
- [x] Dead letter queue view
- [x] Server statistics

### Database Integration
- [x] SQLite with WAL mode (job state, sync positions)
- [x] QuestDB connection pool (pgx)
- [x] ILP client for CSV ingestion
- [x] Database migration system

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
| `GET` | `/api/v1/questdb/tables` | List QuestDB tables |
| `GET` | `/api/v1/questdb/tables/{name}` | Get table columns |
| `POST` | `/api/v1/questdb/query` | Execute SQL query |
| `GET` | `/metrics` | Prometheus metrics |
| `GET` | `/healthz` | Liveness probe |
| `GET` | `/readyz` | Readiness probe |
| `GET` | `/status` | Full health status |

## Files Created

```
datajobs/
├── cmd/server/main.go          # Application entry point
├── config.yaml                 # Default configuration
├── config/examples.yaml        # Full configuration reference
├── internal/
│   ├── config/                 # YAML config + env overrides
│   │   ├── config.go
│   │   └── config_test.go
│   ├── database/               # Database clients
│   │   ├── sqlite.go          # SQLite with migrations
│   │   └── questdb.go          # QuestDB connection pool
│   ├── handlers/              # REST API handlers
│   │   └── handlers.go
│   ├── health/                # Health check endpoints
│   │   └── health.go
│   ├── ingestion/             # ILP CSV ingestion
│   │   └── ilp.go
│   ├── jobs/                  # Job handlers
│   │   └── registry.go
│   ├── logging/                # Structured logging
│   │   └── logging.go
│   ├── metrics/               # Prometheus metrics
│   │   └── metrics.go
│   ├── scheduler/              # Cron scheduler
│   │   └── scheduler.go
│   ├── tracing/               # OpenTelemetry tracing
│   │   └── tracing.go
│   └── worker/                # Bounded worker pool
│       ├── pool.go
│       └── pool_test.go
└── README.md
```

## Tests

- Config tests: 4 passing
- Worker pool tests: 9 passing (concurrency, retries, DLQ, backoff)

## Git Commits

- `a9e105a` - feat: add production-grade job server scaffold
- `4cd2e2c` - feat: add database integration and ILP ingestion support
- `aeb7412` - feat: add Interactive Brokers Web API client and ib_ping job

## Next Tasks

1. Implement full QuestDB ILP protocol communication
2. Implement SQLite→QuestDB sync job
3. Implement QuestDB ANALYZE maintenance
4. Add database package tests
5. Add IB market data fetching job