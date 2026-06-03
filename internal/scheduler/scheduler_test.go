package scheduler

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/bishop-bot/datajobs/internal/config"
	"github.com/bishop-bot/datajobs/internal/logging"
	"github.com/bishop-bot/datajobs/internal/metrics"
	"github.com/bishop-bot/datajobs/internal/worker"
)

// sharedMetrics ensures we don't re-register Prometheus metrics across tests.
var (
	sharedMetrics     *metrics.Metrics
	sharedMetricsOnce sync.Once
)

func getMetrics() *metrics.Metrics {
	sharedMetricsOnce.Do(func() {
		sharedMetrics = metrics.New("test-scheduler")
	})
	return sharedMetrics
}

// sharedPool creates a test worker pool.
func sharedPool() *worker.Pool {
	return worker.NewPool(config.WorkerConfig{PoolSize: 2, QueueCapacity: 10}, getMetrics())
}

func init() {
	// Initialize logging for tests
	logging.Init(logging.Config{Level: "error", Format: "json"})
}

func TestSchedulerCreation(t *testing.T) {
	cfg := config.SchedulerConfig{Timezone: "UTC"}
	pool := sharedPool()
	defer pool.Stop()

	sched := New(cfg, pool)
	if sched == nil {
		t.Fatal("expected non-nil scheduler")
	}
}

func TestSchedulerAddJob(t *testing.T) {
	cfg := config.SchedulerConfig{Timezone: "UTC"}
	pool := sharedPool()
	defer pool.Stop()

	sched := New(cfg, pool)

	jobCfg := config.JobConfig{
		ID:      "test-job",
		Name:    "Test Job",
		Cron:    "*/5 * * * *",
		Type:    "test",
		Handler: "test-handler",
		Enabled: true,
		Timeout: 60,
	}

	if err := sched.AddJob(jobCfg); err != nil {
		t.Fatalf("failed to add job: %v", err)
	}

	// Verify job is in scheduler
	job, ok := sched.GetJob("test-job")
	if !ok {
		t.Fatal("job not found in scheduler")
	}

	if job.Name != "Test Job" {
		t.Errorf("expected job name 'Test Job', got '%s'", job.Name)
	}

	if !job.Enabled {
		t.Error("expected job to be enabled")
	}
}

func TestSchedulerAddJobInvalidCron(t *testing.T) {
	cfg := config.SchedulerConfig{Timezone: "UTC"}
	pool := sharedPool()
	defer pool.Stop()

	sched := New(cfg, pool)

	// Test clearly invalid cron expressions
	invalidCrons := []string{
		"invalid",
		"* * *",
		"60 * * * *", // minute > 59
		"* 25 * * *", // hour > 23
	}

	for _, cron := range invalidCrons {
		jobCfg := config.JobConfig{
			ID:      "cron-test",
			Name:    "Cron Test",
			Cron:    cron,
			Type:    "test",
			Handler: "test-handler",
			Enabled: true,
			Timeout: 60,
		}

		err := sched.AddJob(jobCfg)
		if err == nil {
			t.Errorf("expected error for cron '%s', got nil", cron)
		}
	}

	// Empty cron should be valid for manual jobs
	jobCfg := config.JobConfig{
		ID:      "manual-job",
		Name:    "Manual Job",
		Cron:    "", // No cron, runs only via RunNow
		Type:    "test",
		Handler: "test-handler",
		Enabled: true,
		Timeout: 60,
	}

	err := sched.AddJob(jobCfg)
	if err != nil {
		t.Errorf("empty cron should be valid for manual jobs: %v", err)
	}
}

func TestSchedulerAddDuplicateJob(t *testing.T) {
	cfg := config.SchedulerConfig{Timezone: "UTC"}
	pool := sharedPool()
	defer pool.Stop()

	sched := New(cfg, pool)

	jobCfg := config.JobConfig{
		ID:      "dup-job",
		Name:    "Dup Job",
		Cron:    "*/5 * * * *",
		Type:    "test",
		Handler: "test-handler",
		Enabled: true,
		Timeout: 60,
	}

	// Add first job
	if err := sched.AddJob(jobCfg); err != nil {
		t.Fatalf("failed to add first job: %v", err)
	}

	// Try to add same job again - should overwrite (no error)
	if err := sched.AddJob(jobCfg); err != nil {
		t.Fatalf("should not error on duplicate: %v", err)
	}

	// Job should still exist
	if _, ok := sched.GetJob("dup-job"); !ok {
		t.Fatal("job should still exist")
	}
}

func TestSchedulerListJobs(t *testing.T) {
	cfg := config.SchedulerConfig{Timezone: "UTC"}
	pool := sharedPool()
	defer pool.Stop()

	sched := New(cfg, pool)

	// Add multiple jobs
	for i := 0; i < 5; i++ {
		jobCfg := config.JobConfig{
			ID:      "list-test",
			Name:    "List Test",
			Cron:    "*/5 * * * *",
			Type:    "test",
			Handler: "test-handler",
			Enabled: true,
			Timeout: 60,
		}

		jobCfg.ID = "list-test" + string(rune('a'+i))
		if err := sched.AddJob(jobCfg); err != nil {
			t.Fatalf("failed to add job: %v", err)
		}
	}

	jobs := sched.ListJobs()
	if len(jobs) != 5 {
		t.Errorf("expected 5 jobs, got %d", len(jobs))
	}
}

func TestSchedulerRunNow(t *testing.T) {
	cfg := config.SchedulerConfig{Timezone: "UTC"}
	pool := sharedPool()
	defer pool.Stop()

	sched := New(cfg, pool)

	jobCfg := config.JobConfig{
		ID:      "run-now-test",
		Name:    "Run Now Test",
		Cron:    "*/5 * * * *",
		Type:    "test",
		Handler: "noop",
		Enabled: true,
		Timeout: 60,
	}

	if err := sched.AddJob(jobCfg); err != nil {
		t.Fatalf("failed to add job: %v", err)
	}

	// Register a handler
	pool.RegisterHandler("noop", func(ctx context.Context, job worker.Job) (string, error) {
		return "done", nil
	})

	// Run now should not error
	err := sched.RunNow(context.Background(), "run-now-test")
	if err != nil {
		t.Errorf("RunNow failed: %v", err)
	}

	// Run for non-existent job should error
	err = sched.RunNow(context.Background(), "non-existent")
	if !errors.Is(err, ErrJobNotFound) {
		t.Errorf("expected ErrJobNotFound, got %v", err)
	}
}

func TestSchedulerEnableDisable(t *testing.T) {
	cfg := config.SchedulerConfig{Timezone: "UTC"}
	pool := sharedPool()
	defer pool.Stop()

	sched := New(cfg, pool)

	jobCfg := config.JobConfig{
		ID:      "enable-test",
		Name:    "Enable Test",
		Cron:    "*/5 * * * *",
		Type:    "test",
		Handler: "test-handler",
		Enabled: true,
		Timeout: 60,
	}

	if err := sched.AddJob(jobCfg); err != nil {
		t.Fatalf("failed to add job: %v", err)
	}

	// Disable job
	if err := sched.DisableJob(context.Background(), "enable-test"); err != nil {
		t.Fatalf("DisableJob failed: %v", err)
	}

	job, _ := sched.GetJob("enable-test")
	if job.Enabled {
		t.Error("expected job to be disabled")
	}

	// Enable job
	if err := sched.EnableJob(context.Background(), "enable-test"); err != nil {
		t.Fatalf("EnableJob failed: %v", err)
	}

	job, _ = sched.GetJob("enable-test")
	if !job.Enabled {
		t.Error("expected job to be enabled")
	}

	// Enable/Disable non-existent job
	err := sched.EnableJob(context.Background(), "non-existent")
	if !errors.Is(err, ErrJobNotFound) {
		t.Errorf("expected ErrJobNotFound, got %v", err)
	}

	err = sched.DisableJob(context.Background(), "non-existent")
	if !errors.Is(err, ErrJobNotFound) {
		t.Errorf("expected ErrJobNotFound, got %v", err)
	}
}

func TestSchedulerStartStop(t *testing.T) {
	cfg := config.SchedulerConfig{Timezone: "UTC"}
	pool := sharedPool()

	sched := New(cfg, pool)
	sched.Start()
	sched.Stop()
	pool.Stop()
}

func TestSchedulerStopIdempotent(t *testing.T) {
	cfg := config.SchedulerConfig{Timezone: "UTC"}
	pool := sharedPool()

	sched := New(cfg, pool)
	sched.Start()

	// Call Stop multiple times - should not panic
	for i := 0; i < 5; i++ {
		sched.Stop()
	}

	pool.Stop()
}

func TestSchedulerTimezone(t *testing.T) {
	// Test valid timezone
	cfg := config.SchedulerConfig{Timezone: "America/New_York"}
	pool := sharedPool()
	defer pool.Stop()

	sched := New(cfg, pool)
	if sched == nil {
		t.Fatal("scheduler should be created with valid timezone")
	}

	// Test invalid timezone (should default to UTC)
	cfg = config.SchedulerConfig{Timezone: "Invalid/Timezone"}
	pool2 := sharedPool()
	defer pool2.Stop()

	sched2 := New(cfg, pool2)
	if sched2 == nil {
		t.Fatal("scheduler should be created with invalid timezone (defaults to UTC)")
	}
}

func TestSchedulerRetryConfig(t *testing.T) {
	cfg := config.SchedulerConfig{Timezone: "UTC"}
	pool := sharedPool()
	defer pool.Stop()

	sched := New(cfg, pool)

	// Create a handler that counts invocations
	var count int
	var mu sync.Mutex

	pool.RegisterHandler("retry-test", func(ctx context.Context, job worker.Job) (string, error) {
		mu.Lock()
		count++
		c := count
		mu.Unlock()

		if c < 3 {
			return "", errors.New("fail")
		}
		return "success", nil
	})

	jobCfg := config.JobConfig{
		ID:       "retry-test",
		Name:     "Retry Test",
		Cron:     "*/5 * * * *",
		Type:     "test",
		Handler:  "retry-test",
		Enabled:  true,
		Timeout:  5,
		Retry:    config.RetryConfig{MaxAttempts: 5, InitialDelay: 10, MaxDelay: 100, Multiplier: 2},
	}

	if err := sched.AddJob(jobCfg); err != nil {
		t.Fatalf("failed to add job: %v", err)
	}

	// Verify retry config is stored
	job, _ := sched.GetJob("retry-test")
	if job.Retry.MaxAttempts != 5 {
		t.Errorf("expected MaxAttempts 5, got %d", job.Retry.MaxAttempts)
	}
}

func TestSchedulerConcurrency(t *testing.T) {
	cfg := config.SchedulerConfig{Timezone: "UTC"}
	pool := sharedPool()
	defer pool.Stop()

	sched := New(cfg, pool)

	var wg sync.WaitGroup

	// Concurrent AddJob calls
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			jobCfg := config.JobConfig{
				ID:       id,
				Name:     "Concurrent Job",
				Cron:     "*/5 * * * *",
				Type:     "test",
				Handler:  "test-handler",
				Enabled:  true,
				Timeout:  60,
			}
			sched.AddJob(jobCfg)
		}("concurrent-job-" + string(rune('0'+i)))
	}

	// Concurrent ListJobs calls
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sched.ListJobs()
		}()
	}

	// Concurrent GetJob calls
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			sched.GetJob(id)
		}("concurrent-job-0")
	}

	wg.Wait()

	// All jobs should be present
	jobs := sched.ListJobs()
	if len(jobs) != 10 {
		t.Errorf("expected 10 jobs, got %d", len(jobs))
	}
}