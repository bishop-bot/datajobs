package worker

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/bishop-bot/datajobs/internal/config"
	"github.com/bishop-bot/datajobs/internal/logging"
	"github.com/bishop-bot/datajobs/internal/metrics"
	"github.com/bishop-bot/datajobs/internal/tracing"
)

// Job represents a job to be executed.
type Job struct {
	ID       string
	Type     string
	Handler  string
	Metadata map[string]interface{}
	Retry    config.RetryConfig
	Timeout  time.Duration
	Ctx      context.Context // Carries parent context for tracing
	Attempt  int              // Current attempt number (0-based), persisted across retries
}

// JobResult represents the result of a job execution.
type JobResult struct {
	JobID     string
	Success   bool
	Output    string
	Error     error
	Attempts  int
	StartTime time.Time
	EndTime   time.Time
}

// JobFunc is the function type for job handlers.
type JobFunc func(ctx context.Context, job Job) (string, error)

// DeadLetterJob represents a job that has permanently failed.
type DeadLetterJob struct {
	Job      Job
	Error    string
	Attempts int
	FailedAt time.Time
	Reason   string
}

// Pool manages the bounded worker pool.
type Pool struct {
	cfg         config.WorkerConfig
	metrics     *metrics.Metrics
	handlers    map[string]JobFunc
	jobChan     chan Job
	deadLetter  chan DeadLetterJob
	deadLetterQ []DeadLetterJob
	deadLetterMu sync.Mutex
	stopCh      chan struct{}
	wg          sync.WaitGroup
	stopOnce    sync.Once // Ensure channels are closed only once
	mu          sync.Mutex
	running     bool
	handlersMu  sync.RWMutex // Protects handlers map
}

// MaxDeadLetterSize is the maximum number of dead letter entries to keep.
const MaxDeadLetterSize = 1000

// jobState tracks queue depth accurately.
// When a worker takes a job: decrement (job left queue)
// When a job completes: no change (already decremented)
// When retry is re-queued: no change (same job, already counted)
// When job goes to DL: no change (already left queue)

// NewPool creates a new worker pool with bounded concurrency.
func NewPool(cfg config.WorkerConfig, m *metrics.Metrics) *Pool {
	pool := &Pool{
		cfg:        cfg,
		metrics:    m,
		handlers:   make(map[string]JobFunc),
		jobChan:    make(chan Job, cfg.QueueCapacity),
		deadLetter: make(chan DeadLetterJob, cfg.QueueCapacity),
		deadLetterQ: make([]DeadLetterJob, 0),
		stopCh:     make(chan struct{}),
	}

	m.SetWorkerPoolSize(cfg.PoolSize)
	m.SetQueueCapacity(cfg.QueueCapacity)

	// Start workers
	pool.startWorkers()

	// Start dead letter processor
	pool.wg.Add(1)
	go pool.processDeadLetter()

	return pool
}

// startWorkers starts the fixed number of worker goroutines.
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

// worker pulls jobs from the channel and processes them.
func (p *Pool) worker() {
	defer p.wg.Done()

	for {
		select {
		case <-p.stopCh:
			return
		case job, ok := <-p.jobChan:
			if !ok {
				return // Channel closed, exit
			}
			p.executeJob(job, job.Attempt)
		}
	}
}

// RegisterHandler registers a job handler.
// Thread-safe: uses dedicated mutex for handlers map.
func (p *Pool) RegisterHandler(name string, handler JobFunc) {
	p.handlersMu.Lock()
	defer p.handlersMu.Unlock()
	p.handlers[name] = handler
}

// Submit submits a job to the pool.
// Returns ErrQueueFull if the job queue is at capacity.
func (p *Pool) Submit(ctx context.Context, job Job) error {
	logger := logging.FromContext(ctx)

	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return fmt.Errorf("pool is stopped")
	}
	p.mu.Unlock()

	// Propagate context for tracing through the channel
	job.Ctx = ctx

	// Check queue depth before submitting (for metrics only, actual backpressure via channel)
	p.metrics.SetQueueDepth(len(p.jobChan))

	// Use recover to handle race between running check and channel send
	defer func() {
		if r := recover(); r != nil {
			p.metrics.RecordQueueFull(ctx)
			logger.Error("job rejected: pool stopped", "job_id", job.ID)
		}
	}()

	select {
	case p.jobChan <- job:
		return nil
	default:
		// Queue is full
		p.metrics.RecordQueueFull(ctx)
		logger.Error("job rejected: queue full", "job_id", job.ID)
		return ErrQueueFull
	}
}

// executeJob processes a single job with retry support.
func (p *Pool) executeJob(job Job, attempt int) {
	// Use parent context from job if available, otherwise create new root span
	parentCtx := job.Ctx
	if parentCtx == nil {
		parentCtx = context.Background()
	}

	// Use job.Attempt for span attributes (persists across retries)
	attemptNum := job.Attempt

	ctx, span := tracing.Tracer().Start(parentCtx, "job.execute",
		trace.WithAttributes(
			attribute.String("job.id", job.ID),
			attribute.String("job.type", job.Type),
			attribute.String("job.handler", job.Handler),
			attribute.Int("job.attempt", attemptNum+1),
		),
	)
	defer span.End()

	logger := logging.FromContext(ctx).With("job_id", job.ID, "attempt", attemptNum+1)

	// Get handler (thread-safe read)
	p.handlersMu.RLock()
	handler, ok := p.handlers[job.Handler]
	p.handlersMu.RUnlock()

	if !ok {
		logger.Error("handler not found", "handler", job.Handler)
		p.sendToDeadLetter(ctx, job, "handler_not_found", "handler not registered")
		// Job left queue, queue depth is now len(p.jobChan)
		return
	}

	// Create job context with timeout
	jobCtx, cancel := context.WithTimeout(ctx, job.Timeout)
	defer cancel()

	// Record metrics
	done := p.metrics.RecordJobStart(ctx, job.ID)
	defer done()

	logger.Info("job started", "handler", job.Handler)

	// Execute handler
	output, err := handler(jobCtx, job)

	// Update metrics
	if err != nil {
		p.metrics.RecordJobEnd(ctx, job.ID, "failure")
		logger.Error("job failed", "error", err.Error(), "output", output)

		// Check if we should retry (use job.Attempt which persists across retries)
		if job.Attempt < job.Retry.MaxAttempts-1 {
			backoff := calculateBackoff(job.Retry, job.Attempt)

			logger.Info("scheduling retry", "next_attempt", job.Attempt+2, "backoff", backoff)
			p.metrics.RecordJobRetry(ctx, job.ID)

			// Sleep for backoff using timer for proper cleanup
			// Check parent context (not span context) for cancellation
			timer := time.NewTimer(backoff)
			select {
			case <-timer.C:
				// Backoff complete, proceed to re-queue
			case <-parentCtx.Done():
				timer.Stop()
				logger.Warn("retry cancelled due to parent context", "job_id", job.ID)
				p.sendToDeadLetter(ctx, job, "parent_context_cancelled", parentCtx.Err().Error())
				return
			}

			// Increment attempt before re-queuing so worker sees updated value
			job.Attempt++

			// Re-queue the job (bounded by channel capacity)
			// Propagate current context through channel for span continuity
			job.Ctx = ctx

			// Use non-blocking send with recover to handle race between isRunning check and send
			defer func() {
				if r := recover(); r != nil {
					logger.Warn("recovered from panic during retry enqueue, sending to dead letter",
						"job_id", job.ID, "panic", r)
					p.sendToDeadLetter(ctx, job, "pool_stopped_panic", fmt.Sprintf("%v", r))
				}
			}()

			if !p.isRunning() {
				logger.Warn("pool stopped during retry, sending to dead letter")
				p.sendToDeadLetter(ctx, job, "pool_stopped", "pool stopped during retry backoff")
				return
			}

			// Attempt non-blocking send - will hit default if queue is full
			select {
			case p.jobChan <- job:
				// Job re-queued successfully
			default:
				// Queue full, send to dead letter
				logger.Error("retry rejected: queue full", "job_id", job.ID)
				p.sendToDeadLetter(ctx, job, "queue_full_on_retry", err.Error())
				p.metrics.RecordJobEnd(ctx, job.ID, "dead_letter")
			}
			return
		}

		// Max retries exceeded, send to dead letter
		p.sendToDeadLetter(ctx, job, "max_retries_exceeded", err.Error())
		p.metrics.RecordJobEnd(ctx, job.ID, "dead_letter")
		return
	}

	p.metrics.RecordJobEnd(ctx, job.ID, "success")
	logger.Info("job completed", "output", output)
	// Job left queue (completed), queue depth is now len(p.jobChan)
}

func (p *Pool) sendToDeadLetter(ctx context.Context, job Job, reason, errMsg string) {
	dl := DeadLetterJob{
		Job:      job,
		Error:    errMsg,
		Attempts: job.Retry.MaxAttempts,
		FailedAt: time.Now(),
		Reason:   reason,
	}

	p.deadLetterMu.Lock()
	if len(p.deadLetterQ) > MaxDeadLetterSize {
		// Trim to max when exceeding
		keep := MaxDeadLetterSize / 2
		p.deadLetterQ = p.deadLetterQ[len(p.deadLetterQ)-keep:]
	}
	p.deadLetterQ = append(p.deadLetterQ, dl)
	p.deadLetterMu.Unlock()

	select {
	case p.deadLetter <- dl:
		p.metrics.RecordDeadLetter(ctx, job.ID, reason)
	default:
		// Channel full or closed, already stored in slice
	}
}

func (p *Pool) processDeadLetter() {
	defer p.wg.Done()

	for {
		select {
		case <-p.stopCh:
			return
		case dl, ok := <-p.deadLetter:
			if !ok {
				return // Channel closed
			}
			// Trim dead letter queue if it exceeds max size
			p.deadLetterMu.Lock()
			if len(p.deadLetterQ) > MaxDeadLetterSize {
				// Keep only half when exceeding max
				keep := MaxDeadLetterSize / 2
				p.deadLetterQ = p.deadLetterQ[len(p.deadLetterQ)-keep:]
			}
			p.deadLetterQ = append(p.deadLetterQ, dl)
			currentSize := len(p.deadLetterQ)
			p.deadLetterMu.Unlock()

			logging.Info("job sent to dead letter",
				"job_id", dl.Job.ID,
				"reason", dl.Reason,
				"error", dl.Error,
				"dlq_size", currentSize,
			)
		}
	}
}

// Stop gracefully shuts down the worker pool.
// Waits for all in-progress jobs to complete.
func (p *Pool) Stop() {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return
	}
	p.running = false
	p.mu.Unlock()

	// Use Once to prevent double-close panic
	p.stopOnce.Do(func() {
		// Signal workers to stop by closing stopCh and jobChan
		close(p.stopCh)
		close(p.jobChan)  // Workers exit when jobChan closes
		close(p.deadLetter) // Dead letter processor exits
	})

	// Wait for all workers and processors to finish
	p.wg.Wait()
}

// GetDeadLetterQueue returns the current dead letter queue.
func (p *Pool) GetDeadLetterQueue() []DeadLetterJob {
	p.deadLetterMu.Lock()
	defer p.deadLetterMu.Unlock()

	result := make([]DeadLetterJob, len(p.deadLetterQ))
	copy(result, p.deadLetterQ)
	return result
}

// GetDeadLetterCount returns the number of jobs in the dead letter queue.
func (p *Pool) GetDeadLetterCount() int {
	p.deadLetterMu.Lock()
	defer p.deadLetterMu.Unlock()
	return len(p.deadLetterQ)
}

// GetQueueDepth returns the current number of jobs in the queue.
func (p *Pool) GetQueueDepth() int {
	return len(p.jobChan)
}

// isRunning checks if the pool is still accepting jobs.
func (p *Pool) isRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

// GetActiveWorkers returns the number of active workers.
func (p *Pool) GetActiveWorkers() int {
	return p.cfg.PoolSize
}

// CalculateBackoff calculates the next backoff duration using exponential backoff.
func CalculateBackoff(retry config.RetryConfig, attempt int) time.Duration {
	delay := float64(retry.InitialDelay) * math.Pow(retry.Multiplier, float64(attempt))
	if delay > float64(retry.MaxDelay) {
		delay = float64(retry.MaxDelay)
	}
	return time.Duration(delay) * time.Millisecond
}

func calculateBackoff(retry config.RetryConfig, attempt int) time.Duration {
	return CalculateBackoff(retry, attempt)
}

// ErrQueueFull is returned when the job queue is full.
var ErrQueueFull = &QueueFullError{}

type QueueFullError struct{}

func (e *QueueFullError) Error() string {
	return "job queue is full"
}