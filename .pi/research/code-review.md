# Code Review: DataJobs Project

## Overview
- **Language**: Go
- **Lines of Code**: ~5,300 (excluding tests)
- **Architecture**: HTTP API server with background job processing
- **Key Components**: Chi router, SQLite, QuestDB, IB integration

---

## 🔴 High Priority (Bugs & Security)

### 1. **Duplicate `barDurationNs` Functions**
**Files**: `internal/handlers/marketdata.go`, `internal/jobs/providers/historical_data.go`

Both files have identical `barDurationNs` functions. This violates DRY.

**Proposed Fix**: Create `internal/ingestion/bar.go` with shared function:
```go
package ingestion

// BarDurationNs returns duration in nanoseconds for a bar size.
func BarDurationNs(bar string) int64 {
    switch bar {
    case "1s": return 1_000_000_000
    // ... all cases
    default: return 5 * 60 * 1_000_000_000
    }
}
```

### 2. **Duplicate `Instrument` Type**
**Files**: `internal/handlers/marketdata.go`, `internal/jobs/providers/historical_data.go` (as `instrument`)

Different structs with similar fields:
- `handlers.Instrument` (8 fields)
- `providers.instrument` (4 fields, private)

**Proposed Fix**: Create single `Instrument` type in `internal/database/sqlite.go`:
```go
type Instrument struct {
    ID              string
    Symbol          string
    Name            string
    Publisher       string
    InstrumentClass string
    Currency        string
    Exchange        string
    Asset           string
    SecurityType    string
    Group           string
}
```

### 3. **IB API Timestamp Handling Inconsistency**
**Issue**: In `handlers/marketdata.go`, timestamp is used directly (`ibBar.T` not converted to ns), but in `providers/historical_data.go` it's converted.

```go
// handlers/marketdata.go - WRONG (assumes ns already)
Ts: ibBar.T,  // ibBar.T is in milliseconds from IB!

// providers/historical_data.go - CORRECT
Ts: ibBar.T * 1_000_000,  // Convert ms to ns
```

**Proposed Fix**: Normalize in one place, use consistently.

### 4. **Hardcoded Retry Config in Scheduler**
**File**: `internal/scheduler/scheduler.go:103`

```go
Retry: config.RetryConfig{MaxAttempts: 3}, // Default retry config
```

This ignores job-level retry config and uses hardcoded values.

**Proposed Fix**: Pass job's `Retry` config:
```go
wj := worker.Job{
    // ...
    Retry: job.Retry,  // Use job's retry config
}
```

---

## 🟡 Medium Priority (Code Quality)

### 5. **Naming: `jobs` vs `handlers` Directory Collision**
**Issue**: `internal/jobs/` (job implementations) and `internal/handlers/` (HTTP handlers) create confusion.

| Package | Purpose |
|---------|---------|
| `internal/jobs` | Background task workers |
| `internal/handlers` | HTTP API handlers |

**Proposed Fix**: Document clearly or consider renaming `internal/jobs` → `internal/tasks`.

### 6. **Unused `_ = fmt.Sprintf` Import**
**File**: `internal/scheduler/scheduler.go:47`

```go
_ = fmt.Sprintf // suppress unused import warning
```

**Proposed Fix**: Remove or use proper import.

### 7. **Inconsistent Error Wrapping**
Some functions wrap errors, others don't:
```go
// Good
return nil, fmt.Errorf("failed to fetch instruments: %w", err)

// Bad
return nil, err  // No context
```

**Proposed Fix**: Consistent error wrapping with context.

### 8. **Global State: `DefaultILPClient`**
**File**: `internal/ingestion/ilp.go`

Global singleton makes testing harder and creates hidden dependencies.

**Proposed Fix**: Use dependency injection via `App` struct.

### 9. **Missing Request Validation**
**File**: `internal/handlers/jobs.go`

`CreateJobRequest` has no validation for empty strings, invalid cron expressions, etc.

**Proposed Fix**: Add validation:
```go
func (r CreateJobRequest) Validate() error {
    if r.ID == "" {
        return errors.New("job ID is required")
    }
    if _, err := cron.Parse(r.Cron); err != nil {
        return fmt.Errorf("invalid cron expression: %w", err)
    }
    return nil
}
```

### 10. **Handler Method Order Inconsistency**
**Issue**: Some handlers take `http.ResponseWriter, *http.Request`, others have custom signatures (e.g., `MarketDataHandler.DownloadHistoricalData`).

**Proposed Fix**: Standardize handler interface or clearly separate HTTP handlers from service methods.

---

## 🟢 Low Priority (Improvements)

### 11. **Missing Test Coverage**
**Statistics**:
- `internal/jobs/providers/`: No tests
- `internal/scheduler/`: No tests
- `internal/health/`: No tests
- `internal/handlers/`: Only 1 test file (`marketdata_test.go`)

**Proposed Fix**: Add tests for:
- `HistoricalDataHandler` parameter parsing
- Instrument fetching from SQLite
- `barDurationNs` function

### 12. **Configuration Defaults Scattered**
**File**: `internal/config/config.go`

Defaults are set in `setDefaults()`, but some are also set in constructor (`NewPool`).

**Proposed Fix**: Centralize all defaults in config package.

### 13. **Logging Duplication**
Both `logging.Info` and `slog.Logger` methods used inconsistently.

**Proposed Fix**: Standardize on `slog.Logger` (already used in `app.go`).

### 14. **QuestDB Table Creation on Every Request**
**File**: `internal/handlers/marketdata.go:95`

```go
if err := h.questdb.EnsureTableOHLCV(ctx); err != nil {
```

This is called on every request. Should be called once at startup.

**Proposed Fix**: Move to `initDatabases()` or use lazy initialization flag.

### 15. **Config File Line Limit**
**Issue**: `config.yaml` is 130+ lines with inline comments.

**Proposed Fix**: Consider splitting into `config.d/*.yaml` or using environment-based config.

---

## 📊 Refactoring Opportunities

### A. **Extract Common HTTP Utilities**
Current `handlers.go` has basic `respondJSON`. Could expand:
```go
func respondError(w http.ResponseWriter, err error, status int)
func respondValidationError(w http.ResponseWriter, field, msg string)
```

### B. **Job Handler Interface**
All job handlers have same signature but aren't enforced:
```go
type JobHandler interface {
    Handle(ctx context.Context, job worker.Job) (string, error)
}
```

### C. **Repository Pattern for Database**
SQLite and QuestDB access could use repository interfaces:
```go
type InstrumentRepository interface {
    GetAll(ctx context.Context) ([]Instrument, error)
    GetByID(ctx context.Context, id string) (*Instrument, error)
}
```

---

## 🧪 Test Cases to Add

### Priority 1: Historical Data Handler
```go
func TestParseHistoricalParams(t *testing.T) {
    // Test defaults
    // Test custom values
    // Test missing metadata
}

func TestBarDurationNs(t *testing.T) {
    // 1d = 86400000000000 ns
    // Unknown defaults to 1d
}

func TestGetInstrumentsByConids(t *testing.T) {
    // With valid conids
    // With empty conids
    // With nil sqliteDB
}
```

### Priority 2: Job Handler Registry
```go
func TestRegisterQuestDBHandlers(t *testing.T) {
    // Registers historical_data handler
    // Nil checks for questDB/sqliteDB
}

func TestBuiltInHandlers(t *testing.T) {
    // All expected handlers present
    // Each handler is non-nil
}
```

---

## 📋 Summary Checklist

| Item | Priority | Effort | Impact |
|------|----------|--------|--------|
| Fix timestamp bug | 🔴 High | Low | Critical |
| Extract `barDurationNs` | 🟡 Medium | Medium | Quality |
| Extract `Instrument` type | 🟡 Medium | Medium | Quality |
| Fix hardcoded retry | 🟡 Medium | Low | Correctness |
| Add validation | 🟡 Medium | Low | Robustness |
| Add test coverage | 🟢 Low | High | Confidence |
| Remove global state | 🟢 Low | High | Testability |

---

## 🤔 Questions for Review

1. **Global State**: Is `DefaultILPClient` intentional for simplicity, or should we move to full DI?
2. **Naming**: Is renaming `internal/jobs` worth the migration effort?
3. **Error Handling**: Should we use custom error types (e.g., `ErrInstrumentNotFound`) or stick with wrapped errors?
4. **Configuration**: Should we support environment-only config (no YAML)?

---

*Review date: 2026-06-03*
*Reviewer: AI Code Review Assistant*