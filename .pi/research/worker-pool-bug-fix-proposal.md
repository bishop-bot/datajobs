# Worker Pool Bug Analysis & Fix Proposal

## Current Bug: Unbounded Goroutines

### Problem

```go
// internal/worker/pool.go

func (p *Pool) Submit(ctx context.Context, job Job) error {
    // Queue check only limits incoming jobs, NOT concurrent execution
    p.mu.Lock()
    if p.queueDepth >= p.cfg.QueueCapacity {
        p.mu.Unlock()
        return ErrQueueFull
    }
    p.queueDepth++
    p.mu.Unlock()

    // BUG: Every job spawns a new goroutine, PoolSize is NEVER enforced!
    go p.executeWithRetry(context.Background(), job, 0, 0)
    return nil
}
```

### Attack Scenario

1. Scheduler triggers 1000 jobs at 06:00 HKT
2. Each job spawns a goroutine: `go p.executeWithRetry(...)`
3. 1000 concurrent goroutines running (regardless of `PoolSize: 10`)
4. System overwhelmed: OOM, CPU thrashing, or crashes

### Root Cause

`PoolSize` is only used for metrics display, not actual concurrency control:
```go
m.SetWorkerPoolSize(cfg.PoolSize)  // Just a label!
```

---

## Proposed Fix: True Bounded Pool Pattern

### Design

```
                           ┌─────────────────┐
                           │  Job Channel    │  (buffered queue)
                           │  capacity = Q   │
                           └────────┬────────┘
                                    │
        ┌────────────────────────────┼────────────────────────────┐
        │                            │                            │
   ┌────▼────┐              ┌────▼────┐              ┌────▼────┐
   │ Worker 1│              │ Worker 2│      ...     │ Worker N│
   │ (goroutine)│           │ (goroutine)│            │ (goroutine)│
   └────┬────┘              └────┬────┘              └────┬────┘
        │                         │                         │
        └─────────────────────────┼─────────────────────────┘
                                  │
                           Sync execution
                           (max N concurrent)
```

### Implementation

```go
// Pool manages the bounded worker pool.
type Pool struct {
    cfg       config.WorkerConfig
    metrics   *metrics.Metrics
    handlers  map[string]JobFunc
    jobChan   chan Job              // Buffered job channel
    stopCh    chan struct{}
    wg        sync.WaitGroup        // Track running workers
    mu        sync.Mutex
    running   bool
}

func NewPool(cfg config.WorkerConfig, m *metrics.Metrics) *Pool {
    pool := &Pool{
        cfg:      cfg,
        metrics:  m,
        handlers: make(map[string]JobFunc),
        jobChan:  make(chan Job, cfg.QueueCapacity),
        stopCh:   make(chan struct{}),
    }

    m.SetWorkerPoolSize(cfg.PoolSize)
    m.SetQueueCapacity(cfg.QueueCapacity)

    // Start fixed number of worker goroutines
    pool.startWorkers()

    // Start dead letter processor
    go pool.processDeadLetter()

    return pool
}

func (p *Pool) startWorkers() {
    p.mu.Lock()
    defer p.mu.Unlock()

    if p.running {
        return
    }
    p.running = true

    for i := 0; i < p.cfg.PoolSize; i++ {
        p.wg.Add(1)
        go p.worker()
    }
}

// worker pulls jobs from the channel (blocks when idle)
func (p *Pool) worker() {
    defer p.wg.Done()

    for {
        select {
        case <-p.stopCh:
            return
        case job := <-p.jobChan:
            p.executeJob(job, 0)
        }
    }
}

// Submit enqueues job (blocks if queue is full)
func (p *Pool) Submit(ctx context.Context, job Job) error {
    p.mu.Lock()
    if !p.running {
        p.mu.Unlock()
        return fmt.Errorf("pool is stopped")
    }
    p.mu.Unlock()

    select {
    case p.jobChan <- job:
        p.metrics.SetQueueDepth(len(p.jobChan))
        return nil
    default:
        // Queue is full
        p.metrics.RecordQueueFull(ctx)
        return ErrQueueFull
    }
}
```

### Retry Handling

Retries go BACK into the channel, not spawn new goroutines:
```go
func (p *Pool) executeJob(job Job, attempt int) {
    // ...
    if attempt < job.Retry.MaxAttempts-1 {
        // Re-queue for retry (bounded by channel capacity)
        time.Sleep(calculateBackoff(job.Retry, attempt))
        p.jobChan <- job  // Re-use existing worker slot
        return
    }
    // ...
}
```

---

## Key Changes

| Aspect | Before | After |
|--------|--------|-------|
| Concurrency | Unbounded (goroutine per job) | Bounded (PoolSize workers) |
| Job storage | Queue depth counter | Buffered channel |
| Workers | Ephemeral goroutines | Fixed worker pool |
| Backpressure | None (unbounded spawn) | Channel backpressure |
| Retry | New goroutine | Re-queue in channel |

---

## Benefits

1. **True bounded concurrency** - max `PoolSize` jobs executing
2. **Backpressure** - submit blocks/fails when queue full
3. **Graceful shutdown** - `wg.Wait()` ensures all workers finish
4. **Efficient retries** - re-use worker slots, no goroutine explosion
5. **Cleaner shutdown** - stop channel signals workers to exit

---

## Backward Compatibility

- `Submit()` signature unchanged
- `GetQueueDepth()` still works (len of channel)
- `GetDeadLetterQueue()` still works
- Error types unchanged

---

## Testing Checklist

- [ ] Submit N jobs where N > PoolSize, verify max PoolSize concurrent
- [ ] Submit when queue full, verify ErrQueueFull returned
- [ ] Graceful shutdown completes all in-flight jobs
- [ ] Retry re-queues without spawning new goroutines
- [ ] Metrics accurately reflect queue depth