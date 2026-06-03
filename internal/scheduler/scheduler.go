package scheduler

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/bishop-bot/datajobs/internal/config"
	"github.com/bishop-bot/datajobs/internal/logging"
	"github.com/bishop-bot/datajobs/internal/tracing"
	"github.com/bishop-bot/datajobs/internal/worker"

	cron "github.com/netresearch/go-cron"
)

// Job represents a schedulable job.
type Job struct {
	ID      string
	Name    string
	Cron    string
	Type    string
	Handler string
	Enabled bool
	Timeout time.Duration
	Retry   config.RetryConfig
}

// Scheduler manages scheduled jobs.
type Scheduler struct {
	cfg       config.SchedulerConfig
	pool      *worker.Pool
	cron      *cron.Cron
	jobs      map[string]Job
	mu        sync.RWMutex
	stopCh    chan struct{}
}

// New creates a new scheduler.
func New(cfg config.SchedulerConfig, pool *worker.Pool) *Scheduler {
	s := &Scheduler{
		cfg:    cfg,
		pool:   pool,
		cron:   cron.New(cron.WithLocation(time.UTC)),
		jobs:   make(map[string]Job),
		stopCh: make(chan struct{}),
	}

	// Set timezone
	if cfg.Timezone != "" {
		loc, err := time.LoadLocation(cfg.Timezone)
		if err != nil {
			logging.Warn("invalid timezone, using UTC", "tz", cfg.Timezone, "error", err)
		} else {
			s.cron = cron.New(cron.WithLocation(loc))
		}
	}

	return s
}

// AddJob adds a job to the scheduler.
func (s *Scheduler) AddJob(cfg config.JobConfig) error {
	job := Job{
		ID:      cfg.ID,
		Name:    cfg.Name,
		Cron:    cfg.Cron,
		Type:    cfg.Type,
		Handler: cfg.Handler,
		Enabled: cfg.Enabled,
		Timeout: time.Duration(cfg.Timeout) * time.Second,
		Retry:   cfg.Retry,
	}

	s.mu.Lock()
	s.jobs[job.ID] = job

	if job.Enabled && job.Cron != "" {
		_, err := s.cron.AddFunc(job.Cron, func() {
			s.executeJob(context.Background(), job)
		})
		if err != nil {
			s.mu.Unlock()
			return err
		}
	}

	s.mu.Unlock()

	logging.Info("job added to scheduler",
		"job_id", job.ID,
		"cron", job.Cron,
		"enabled", job.Enabled,
	)

	return nil
}

// RunNow triggers immediate execution of a job.
func (s *Scheduler) RunNow(ctx context.Context, jobID string) error {
	s.mu.RLock()
	job, ok := s.jobs[jobID]
	s.mu.RUnlock()

	if !ok {
		return ErrJobNotFound
	}

	go s.executeJob(ctx, job)
	return nil
}

// executeJob executes a job through the worker pool.
func (s *Scheduler) executeJob(ctx context.Context, job Job) {
	logger := logging.FromContext(ctx).With("job_id", job.ID)
	logger.Info("triggering scheduled job", "cron", job.Cron)

	// Start tracing span
	ctx, span := tracing.Tracer().Start(ctx, "scheduler.trigger",
		trace.WithAttributes(
			attribute.String("job.id", job.ID),
			attribute.String("job.type", job.Type),
			attribute.String("job.handler", job.Handler),
		),
	)
	defer span.End()

	wjob := worker.Job{
		ID:       job.ID,
		Type:     job.Type,
		Handler:  job.Handler,
		Metadata: make(map[string]interface{}),
		Retry:    config.RetryConfig{MaxAttempts: 3},
		Timeout:  job.Timeout,
	}

	// Use job config's retry settings if available (not zero values)
	if job.Retry.MaxAttempts > 0 {
		wjob.Retry = job.Retry
	}

	if err := s.pool.Submit(ctx, wjob); err != nil {
		logger.Error("failed to submit job", "error", err)
	}
}

// ListJobs returns all registered jobs.
func (s *Scheduler) ListJobs() []Job {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

// GetJob returns a job by ID.
func (s *Scheduler) GetJob(jobID string) (Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	job, ok := s.jobs[jobID]
	return job, ok
}

// EnableJob enables a job.
func (s *Scheduler) EnableJob(ctx context.Context, jobID string) error {
	return s.setJobEnabled(ctx, jobID, true)
}

// DisableJob disables a job.
func (s *Scheduler) DisableJob(ctx context.Context, jobID string) error {
	return s.setJobEnabled(ctx, jobID, false)
}

func (s *Scheduler) setJobEnabled(ctx context.Context, jobID string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[jobID]
	if !ok {
		return ErrJobNotFound
	}

	job.Enabled = enabled
	s.jobs[jobID] = job

	logging.Info("job enabled/disabled", "job_id", jobID, "enabled", enabled)
	return nil
}

// Start starts the scheduler.
func (s *Scheduler) Start() error {
	s.cron.Start()
	logging.Info("scheduler started", "timezone", s.cfg.Timezone)
	return nil
}

// Stop stops the scheduler.
func (s *Scheduler) Stop() {
	s.cron.Stop()
	close(s.stopCh)
	logging.Info("scheduler stopped")
}

// NextRun returns the next scheduled run time for a job.
func (s *Scheduler) NextRun(jobID string) (time.Time, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Note: go-cron's public API doesn't expose next run times
	return time.Time{}, false
}

// Errors
var (
	ErrJobNotFound = &JobNotFoundError{}
)

type JobNotFoundError struct{}

func (e *JobNotFoundError) Error() string {
	return "job not found"
}