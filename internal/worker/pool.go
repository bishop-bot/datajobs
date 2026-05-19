package worker

import (
	"context"
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
	Job       Job
	Error     string
	Attempts  int
	FailedAt  time.Time
	Reason    string
}

// Pool manages the bounded worker pool.
type Pool struct {
	cfg         config.WorkerConfig
	metrics     *metrics.Metrics
	handlers    map[string]JobFunc
	deadLetter  chan DeadLetterJob
	deadLetterQ []DeadLetterJob
	mu          sync.RWMutex
	queueDepth  int
}

// NewPool creates a new worker pool.
func NewPool(cfg config.WorkerConfig, m *metrics.Metrics) *Pool {
	pool := &Pool{
		cfg:        cfg,
		metrics:    m,
		handlers:   make(map[string]JobFunc),
		deadLetter: make(chan DeadLetterJob, cfg.QueueCapacity),
		deadLetterQ: make([]DeadLetterJob, 0),
	}

	m.SetWorkerPoolSize(cfg.PoolSize)
	m.SetQueueCapacity(cfg.QueueCapacity)

	// Start dead letter queue processor
	go pool.processDeadLetter()

	return pool
}

// RegisterHandler registers a job handler.
func (p *Pool) RegisterHandler(name string, handler JobFunc) {
	p.handlers[name] = handler
}

// Submit submits a job to the pool.
func (p *Pool) Submit(ctx context.Context, job Job) error {
	logger := logging.FromContext(ctx)

	// Check queue capacity
	p.mu.Lock()
	if p.queueDepth >= p.cfg.QueueCapacity {
		p.mu.Unlock()
		p.metrics.RecordQueueFull(ctx)
		logger.Error("job rejected: queue full", "job_id", job.ID)
		return ErrQueueFull
	}
	p.queueDepth++
	p.metrics.SetQueueDepth(p.queueDepth)
	p.mu.Unlock()

	// Execute job with retry
	go p.executeWithRetry(context.Background(), job, 0, 0)

	return nil
}

func (p *Pool) executeWithRetry(ctx context.Context, job Job, attempt int, backoff time.Duration) {
	// Apply backoff delay if this is a retry
	if backoff > 0 {
		time.Sleep(backoff)
	}

	// Execute the job
	p.execute(ctx, job, attempt)
}

func (p *Pool) execute(ctx context.Context, job Job, attempt int) {
	logger := logging.FromContext(ctx).With("job_id", job.ID, "attempt", attempt+1)

	// Start tracing span
	ctx, span := tracing.Tracer().Start(ctx, "job.execute",
		trace.WithAttributes(
			attribute.String("job.id", job.ID),
			attribute.String("job.type", job.Type),
			attribute.String("job.handler", job.Handler),
			attribute.Int("job.attempt", attempt+1),
		),
	)
	defer span.End()

	// Get handler
	handler, ok := p.handlers[job.Handler]
	if !ok {
		logger.Error("handler not found", "handler", job.Handler)
		p.sendToDeadLetter(ctx, job, "handler_not_found", "handler not registered")
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

		// Check if we should retry
		if attempt < job.Retry.MaxAttempts-1 {
			nextAttempt := attempt + 1
			backoff := calculateBackoff(job.Retry, attempt)

			logger.Info("scheduling retry", "next_attempt", nextAttempt+1, "backoff", backoff)
			p.metrics.RecordJobRetry(ctx, job.ID)

			// Update queue depth
			p.mu.Lock()
			p.queueDepth--
			p.metrics.SetQueueDepth(p.queueDepth)
			p.mu.Unlock()

			go p.executeWithRetry(context.Background(), job, nextAttempt, backoff)
			return
		}

		// Max retries exceeded, send to dead letter
		p.sendToDeadLetter(ctx, job, "max_retries_exceeded", err.Error())
		p.metrics.RecordJobEnd(ctx, job.ID, "dead_letter")
		return
	}

	p.metrics.RecordJobEnd(ctx, job.ID, "success")
	logger.Info("job completed", "output", output)

	// Update queue depth
	p.mu.Lock()
	p.queueDepth--
	p.metrics.SetQueueDepth(p.queueDepth)
	p.mu.Unlock()
}

func (p *Pool) sendToDeadLetter(ctx context.Context, job Job, reason, errMsg string) {
	dl := DeadLetterJob{
		Job:      job,
		Error:    errMsg,
		Attempts: job.Retry.MaxAttempts,
		FailedAt: time.Now(),
		Reason:   reason,
	}

	p.mu.Lock()
	p.deadLetterQ = append(p.deadLetterQ, dl)
	p.mu.Unlock()

	select {
	case p.deadLetter <- dl:
		p.metrics.RecordDeadLetter(ctx, job.ID, reason)
	default:
		// Channel full, already stored in queue
	}
}

func (p *Pool) processDeadLetter() {
	for dl := range p.deadLetter {
		logging.Info("job sent to dead letter",
			"job_id", dl.Job.ID,
			"reason", dl.Reason,
			"error", dl.Error,
		)
	}
}

// GetDeadLetterQueue returns the current dead letter queue.
func (p *Pool) GetDeadLetterQueue() []DeadLetterJob {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]DeadLetterJob, len(p.deadLetterQ))
	copy(result, p.deadLetterQ)
	return result
}

// GetDeadLetterCount returns the number of jobs in the dead letter queue.
func (p *Pool) GetDeadLetterCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.deadLetterQ)
}

// GetQueueDepth returns the current queue depth.
func (p *Pool) GetQueueDepth() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.queueDepth
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