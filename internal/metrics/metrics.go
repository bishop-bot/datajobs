package metrics

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics for the application.
type Metrics struct {
	// Job execution metrics
	JobsExecutedTotal   *prometheus.CounterVec
	JobDurationSeconds  *prometheus.HistogramVec
	JobRetriesTotal     *prometheus.CounterVec
	JobsRunningGauge    prometheus.Gauge

	// Queue metrics
	QueueDepthGauge    prometheus.Gauge
	QueueCapacityGauge prometheus.Gauge
	QueueFullTotal     prometheus.Counter

	// HTTP metrics
	HTTPRequestsTotal   *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec

	// Worker metrics
	WorkerPoolSizeGauge prometheus.Gauge
	WorkerActiveGauge   prometheus.Gauge

	// Dead letter metrics
	DeadLetterTotal    *prometheus.CounterVec
	DeadLetterGauge    prometheus.Gauge
}

// New creates and registers all Prometheus metrics.
func New(namespace string) *Metrics {
	m := &Metrics{
		JobsExecutedTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "jobs_executed_total",
				Help:      "Total number of jobs executed",
			},
			[]string{"job_id", "status"}, // success, failure, timeout
		),

		JobDurationSeconds: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "job_duration_seconds",
				Help:      "Duration of job execution in seconds",
				Buckets:   []float64{0.1, 0.5, 1, 5, 10, 30, 60, 120, 300, 600},
			},
			[]string{"job_id"},
		),

		JobRetriesTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "job_retries_total",
				Help:      "Total number of job retries",
			},
			[]string{"job_id"},
		),

		JobsRunningGauge: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "jobs_running",
				Help:      "Number of currently running jobs",
			},
		),

		QueueDepthGauge: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "job_queue_depth",
				Help:      "Current number of jobs in queue",
			},
		),

		QueueCapacityGauge: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "job_queue_capacity",
				Help:      "Maximum job queue capacity",
			},
		),

		QueueFullTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "job_queue_full_total",
				Help:      "Total number of times the job queue was full and rejected a job",
			},
		),

		HTTPRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "http_requests_total",
				Help:      "Total number of HTTP requests",
			},
			[]string{"method", "path", "status"},
		),

		HTTPRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "http_request_duration_seconds",
				Help:      "HTTP request duration in seconds",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),

		WorkerPoolSizeGauge: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "worker_pool_size",
				Help:      "Configured worker pool size",
			},
		),

		WorkerActiveGauge: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "worker_active",
				Help:      "Number of active workers",
			},
		),

		DeadLetterTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "dead_letter_total",
				Help:      "Total number of jobs sent to dead letter queue",
			},
			[]string{"job_id", "reason"},
		),

		DeadLetterGauge: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "dead_letter_count",
				Help:      "Current number of jobs in dead letter queue",
			},
		),
	}

	return m
}

// RecordJobStart records the start of a job execution.
func (m *Metrics) RecordJobStart(ctx context.Context, jobID string) func() {
	m.JobsRunningGauge.Inc()
	m.WorkerActiveGauge.Inc()
	start := time.Now()

	return func() {
		duration := time.Since(start).Seconds()
		m.JobDurationSeconds.WithLabelValues(jobID).Observe(duration)
		m.JobsRunningGauge.Dec()
		m.WorkerActiveGauge.Dec()
	}
}

// RecordJobEnd records the completion of a job.
func (m *Metrics) RecordJobEnd(ctx context.Context, jobID, status string) {
	m.JobsExecutedTotal.WithLabelValues(jobID, status).Inc()
}

// RecordJobRetry records a job retry.
func (m *Metrics) RecordJobRetry(ctx context.Context, jobID string) {
	m.JobRetriesTotal.WithLabelValues(jobID).Inc()
}

// RecordDeadLetter records a job being sent to dead letter queue.
func (m *Metrics) RecordDeadLetter(ctx context.Context, jobID, reason string) {
	m.DeadLetterTotal.WithLabelValues(jobID, reason).Inc()
	m.DeadLetterGauge.Inc()
}

// SetQueueDepth sets the current queue depth.
func (m *Metrics) SetQueueDepth(depth int) {
	m.QueueDepthGauge.Set(float64(depth))
}

// SetQueueCapacity sets the queue capacity.
func (m *Metrics) SetQueueCapacity(capacity int) {
	m.QueueCapacityGauge.Set(float64(capacity))
}

// RecordQueueFull records when the queue is full.
func (m *Metrics) RecordQueueFull(ctx context.Context) {
	m.QueueFullTotal.Inc()
}

// RecordHTTPRequest records an HTTP request.
func (m *Metrics) RecordHTTPRequest(ctx context.Context, method, path, status string, duration time.Duration) {
	m.HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
	m.HTTPRequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
}

// SetWorkerPoolSize sets the configured worker pool size.
func (m *Metrics) SetWorkerPoolSize(size int) {
	m.WorkerPoolSizeGauge.Set(float64(size))
}