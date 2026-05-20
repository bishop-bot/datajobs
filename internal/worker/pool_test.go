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
	cfg := config.WorkerConfig{
		PoolSize:        1,
		QueueCapacity:   1,
		ShutdownTimeout: 30,
	}
	m := getMetrics()
	pool := NewPool(cfg, m)

	pool.RegisterHandler("blocking", func(ctx context.Context, job Job) (string, error) {
		time.Sleep(500 * time.Millisecond)
		return "done", nil
	})

	// Submit first job (will block)
	job1 := Job{
		ID:      "job1",
		Handler: "blocking",
		Retry:   config.RetryConfig{MaxAttempts: 1},
		Timeout: 5 * time.Second,
	}
	pool.Submit(context.Background(), job1)

	// Small delay to ensure first job is being processed
	time.Sleep(50 * time.Millisecond)

	// Try to submit second job (queue should be full)
	job2 := Job{
		ID:      "job2",
		Handler: "blocking",
		Retry:   config.RetryConfig{MaxAttempts: 1},
		Timeout: 5 * time.Second,
	}
	err := pool.Submit(context.Background(), job2)

	if err != ErrQueueFull {
		t.Errorf("expected ErrQueueFull, got %v", err)
	}

	// Wait for first job to complete
	time.Sleep(600 * time.Millisecond)
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