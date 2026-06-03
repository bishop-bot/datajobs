package worker

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bishop-bot/datajobs/internal/config"
	"github.com/bishop-bot/datajobs/internal/metrics"
)

// sharedMetrics ensures we don't re-register Prometheus metrics across tests
var (
	sharedMetrics     *metrics.Metrics
	sharedMetricsOnce sync.Once
)

func getMetrics() *metrics.Metrics {
	sharedMetricsOnce.Do(func() {
		sharedMetrics = metrics.New("test")
	})
	return sharedMetrics
}

func TestPoolCreation(t *testing.T) {
	cfg := config.WorkerConfig{
		PoolSize:        5,
		QueueCapacity:   10,
		ShutdownTimeout: 30,
	}
	m := getMetrics()
	pool := NewPool(cfg, m)
	defer pool.Stop()

	if pool == nil {
		t.Fatal("expected non-nil pool")
	}

	if pool.GetQueueDepth() != 0 {
		t.Errorf("expected initial queue depth 0, got %d", pool.GetQueueDepth())
	}
}

func TestPoolRegisterHandler(t *testing.T) {
	cfg := config.WorkerConfig{PoolSize: 5, QueueCapacity: 10}
	m := getMetrics()
	pool := NewPool(cfg, m)
	defer pool.Stop()

	var executed bool
	// Register a handler that sets executed to true
	pool.RegisterHandler("test-register", func(ctx context.Context, job Job) (string, error) {
		executed = true
		return "registered", nil
	})

	job := Job{
		ID:      "register-test",
		Handler: "test-register",
		Retry:   config.RetryConfig{MaxAttempts: 1},
		Timeout: 100 * time.Millisecond,
	}

	err := pool.Submit(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wait for execution
	time.Sleep(50 * time.Millisecond)

	if !executed {
		t.Error("expected handler to be executed")
	}
}

func TestPoolSubmitJob(t *testing.T) {
	cfg := config.WorkerConfig{
		PoolSize:        1,
		QueueCapacity:   10,
		ShutdownTimeout: 30,
	}
	m := getMetrics()
	pool := NewPool(cfg, m)
	defer pool.Stop()

	var executed bool
	pool.RegisterHandler("test-submit", func(ctx context.Context, job Job) (string, error) {
		executed = true
		return "success", nil
	})

	job := Job{
		ID:      "test-job",
		Type:    "test",
		Handler: "test-submit",
		Retry:   config.RetryConfig{MaxAttempts: 1},
		Timeout: 10 * time.Second,
	}

	err := pool.Submit(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wait for execution
	time.Sleep(100 * time.Millisecond)

	if !executed {
		t.Error("expected job to be executed")
	}
}

func TestPoolQueueFull(t *testing.T) {
	// Test that submit fails when the queue is full
	cfg := config.WorkerConfig{
		PoolSize:      1, // Only 1 worker
		QueueCapacity: 1, // Only 1 job in queue
	}
	m := getMetrics()
	pool := NewPool(cfg, m)
	defer pool.Stop()

	pool.RegisterHandler("blocking", func(ctx context.Context, job Job) (string, error) {
		// Processing time doesn't matter for this test
		time.Sleep(50 * time.Millisecond)
		return "done", nil
	})

	// Submit first job - goes to queue (worker picks it up asynchronously)
	job1 := Job{
		ID:      "job1",
		Handler: "blocking",
		Retry:   config.RetryConfig{MaxAttempts: 1},
		Timeout: 5 * time.Second,
	}
	err := pool.Submit(context.Background(), job1)
	if err != nil {
		t.Fatalf("first submit should succeed: %v", err)
	}

	// Immediately submit second job - the queue is at capacity
	// because job1 is still in the queue (worker hasn't picked it up yet)
	job2 := Job{
		ID:      "job2",
		Handler: "blocking",
		Retry:   config.RetryConfig{MaxAttempts: 1},
		Timeout: 5 * time.Second,
	}
	err = pool.Submit(context.Background(), job2)
	if err != ErrQueueFull {
		t.Errorf("expected ErrQueueFull, got %v", err)
	}

	// Wait for job1 to complete and be removed from queue
	time.Sleep(100 * time.Millisecond)

	// Now job2 can be submitted (queue has space)
	job3 := Job{
		ID:      "job3",
		Handler: "blocking",
		Retry:   config.RetryConfig{MaxAttempts: 1},
		Timeout: 5 * time.Second,
	}
	err = pool.Submit(context.Background(), job3)
	if err != nil {
		t.Errorf("third submit should succeed after space opens: %v", err)
	}
}

func TestCalculateBackoff(t *testing.T) {
	retry := config.RetryConfig{
		InitialDelay: 1000,
		MaxDelay:     10000,
		Multiplier:   2.0,
	}

	tests := []struct {
		attempt     int
		expectedMs  int // expected in milliseconds
	}{
		{0, 1000},  // 1000 * 2^0 = 1000ms (1s)
		{1, 2000},  // 1000 * 2^1 = 2000ms (2s)
		{2, 4000},  // 1000 * 2^2 = 4000ms (4s)
		{3, 8000},  // 1000 * 2^3 = 8000ms (8s)
		{4, 10000}, // 1000 * 2^4 = 16000ms capped at maxDelay (10s)
	}

	for _, tt := range tests {
		backoff := CalculateBackoff(retry, tt.attempt)
		expected := time.Duration(tt.expectedMs) * time.Millisecond
		if backoff != expected {
			t.Errorf("attempt %d: expected backoff %v, got %v", tt.attempt, expected, backoff)
		}
	}
}

func TestDeadLetterQueue(t *testing.T) {
	cfg := config.WorkerConfig{
		PoolSize:        1,
		QueueCapacity:   10,
		ShutdownTimeout: 30,
	}
	m := getMetrics()
	pool := NewPool(cfg, m)
	defer pool.Stop()

	pool.RegisterHandler("always-fail", func(ctx context.Context, job Job) (string, error) {
		return "", &testError{"always fails"}
	})

	job := Job{
		ID:      "dl-test",
		Handler: "always-fail",
		Retry:   config.RetryConfig{MaxAttempts: 1},
		Timeout: 5 * time.Second,
	}

	pool.Submit(context.Background(), job)

	// Wait for job to fail and go to DLQ
	time.Sleep(500 * time.Millisecond)

	dlCount := pool.GetDeadLetterCount()
	if dlCount < 1 {
		t.Errorf("expected at least 1 job in DLQ, got %d", dlCount)
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestJobMetadata(t *testing.T) {
	cfg := config.WorkerConfig{PoolSize: 1, QueueCapacity: 10}
	m := getMetrics()
	pool := NewPool(cfg, m)
	defer pool.Stop()

	var capturedJob Job
	pool.RegisterHandler("capture-metadata", func(ctx context.Context, job Job) (string, error) {
		capturedJob = job
		return "ok", nil
	})

	job := Job{
		ID:      "meta-test",
		Type:    "test",
		Handler: "capture-metadata",
		Metadata: map[string]interface{}{
			"key1": "value1",
			"count": 42,
			"nested": map[string]interface{}{
				"inner": "data",
			},
		},
		Retry:   config.RetryConfig{MaxAttempts: 1},
		Timeout: 5 * time.Second,
	}

	pool.Submit(context.Background(), job)
	time.Sleep(100 * time.Millisecond)

	if capturedJob.ID != "meta-test" {
		t.Errorf("expected job ID meta-test, got %s", capturedJob.ID)
	}

	val, ok := capturedJob.Metadata["key1"]
	if !ok || val != "value1" {
		t.Errorf("expected metadata key1=value1, got %v", capturedJob.Metadata)
	}
}

func TestWorkerPoolConcurrency(t *testing.T) {
	cfg := config.WorkerConfig{
		PoolSize:        5,
		QueueCapacity:   100,
		ShutdownTimeout: 30,
	}
	m := getMetrics()
	pool := NewPool(cfg, m)
	defer pool.Stop()

	var counter atomic.Int64
	pool.RegisterHandler("counter-concurrent", func(ctx context.Context, job Job) (string, error) {
		counter.Add(1)
		return "done", nil
	})

	// Submit 20 jobs
	for i := 0; i < 20; i++ {
		job := Job{
			ID:      "concurrent",
			Handler: "counter-concurrent",
			Retry:   config.RetryConfig{MaxAttempts: 1},
			Timeout: 5 * time.Second,
		}
		pool.Submit(context.Background(), job)
	}

	// Wait for all to complete
	time.Sleep(500 * time.Millisecond)

	if counter.Load() != 20 {
		t.Errorf("expected 20 executions, got %d", counter.Load())
	}
}

func TestPoolQueueDepth(t *testing.T) {
	cfg := config.WorkerConfig{
		PoolSize:        2,
		QueueCapacity:   5,
		ShutdownTimeout: 30,
	}
	m := getMetrics()
	pool := NewPool(cfg, m)
	defer pool.Stop()

	pool.RegisterHandler("slow", func(ctx context.Context, job Job) (string, error) {
		time.Sleep(200 * time.Millisecond)
		return "done", nil
	})

	// Submit jobs that will queue up
	for i := 0; i < 3; i++ {
		job := Job{
			ID:      "depth-test",
			Handler: "slow",
			Retry:   config.RetryConfig{MaxAttempts: 1},
			Timeout: 5 * time.Second,
		}
		pool.Submit(context.Background(), job)
	}

	// Check queue depth (may be 0-3 depending on timing)
	depth := pool.GetQueueDepth()
	if depth < 0 || depth > 3 {
		t.Errorf("unexpected queue depth: %d", depth)
	}

	// Wait for completion
	time.Sleep(500 * time.Millisecond)
}

func TestPoolGetDeadLetterCount(t *testing.T) {
	cfg := config.WorkerConfig{
		PoolSize:        1,
		QueueCapacity:   10,
		ShutdownTimeout: 30,
	}
	m := getMetrics()
	pool := NewPool(cfg, m)

	// Initial count should be 0
	count := pool.GetDeadLetterCount()
	if count != 0 {
		t.Errorf("expected initial DLQ count 0, got %d", count)
	}
}

func TestPoolBoundedConcurrency(t *testing.T) {
	// Test that PoolSize actually limits concurrent execution
	cfg := config.WorkerConfig{
		PoolSize:      2, // Only 2 workers
		QueueCapacity: 100,
	}
	m := getMetrics()
	pool := NewPool(cfg, m)
	defer pool.Stop()

	var maxConcurrent atomic.Int32
	var currentConcurrent atomic.Int32

	pool.RegisterHandler("concurrent-limit", func(ctx context.Context, job Job) (string, error) {
		inc := currentConcurrent.Add(1)
		// Track max concurrent
		for {
			old := maxConcurrent.Load()
			if inc <= old || maxConcurrent.CompareAndSwap(old, inc) {
				break
			}
		}

		// Simulate work
		time.Sleep(100 * time.Millisecond)

		currentConcurrent.Add(-1)
		return "done", nil
	})

	// Submit 20 jobs rapidly
	for i := 0; i < 20; i++ {
		job := Job{
			ID:      "bounded",
			Handler: "concurrent-limit",
			Retry:   config.RetryConfig{MaxAttempts: 1},
			Timeout: 5 * time.Second,
		}
		if err := pool.Submit(context.Background(), job); err != nil {
			t.Fatalf("submit failed: %v", err)
		}
	}

	// Wait for all jobs to complete
	time.Sleep(2 * time.Second)

	// Verify max concurrent never exceeded PoolSize
	maxConcurrentJobs := maxConcurrent.Load()
	if maxConcurrentJobs > int32(cfg.PoolSize) {
		t.Errorf("max concurrent %d exceeded PoolSize %d", maxConcurrentJobs, cfg.PoolSize)
	}
}

func TestPoolGracefulShutdown(t *testing.T) {
	cfg := config.WorkerConfig{
		PoolSize:      2,
		QueueCapacity: 10,
	}
	m := getMetrics()
	pool := NewPool(cfg, m)

	var completed int32
	pool.RegisterHandler("shutdown-test", func(ctx context.Context, job Job) (string, error) {
		time.Sleep(100 * time.Millisecond)
		atomic.AddInt32(&completed, 1)
		return "done", nil
	})

	// Submit jobs
	for i := 0; i < 5; i++ {
		job := Job{
			ID:      "shutdown",
			Handler: "shutdown-test",
			Retry:   config.RetryConfig{MaxAttempts: 1},
			Timeout: 5 * time.Second,
		}
		pool.Submit(context.Background(), job)
	}

	// Give some time for jobs to start processing
	time.Sleep(50 * time.Millisecond)

	// Stop should wait for in-progress jobs
	pool.Stop()

	// At least some jobs should have completed (may be 2-5 depending on timing)
	if completed < 2 {
		t.Errorf("expected at least 2 completed, got %d", completed)
	}
}

func TestPoolStopIdempotent(t *testing.T) {
	cfg := config.WorkerConfig{
		PoolSize:      1,
		QueueCapacity: 10,
	}
	m := getMetrics()
	pool := NewPool(cfg, m)

	// Stop multiple times should not panic
	pool.Stop()
	pool.Stop()
}

func TestPoolSubmitAfterStop(t *testing.T) {
	cfg := config.WorkerConfig{
		PoolSize:      1,
		QueueCapacity: 10,
	}
	m := getMetrics()
	pool := NewPool(cfg, m)
	pool.Stop()

	// Submit after stop should fail
	job := Job{
		ID:      "after-stop",
		Handler: "noop",
		Retry:   config.RetryConfig{MaxAttempts: 1},
		Timeout: 5 * time.Second,
	}
	err := pool.Submit(context.Background(), job)
	if err == nil {
		t.Error("expected error submitting after stop")
	}
}