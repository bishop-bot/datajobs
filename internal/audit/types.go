package audit

import "time"

// JobRunStatus represents the status of a job run.
type JobRunStatus string

const (
	StatusRunning   JobRunStatus = "running"
	StatusSuccess  JobRunStatus = "success"
	StatusFailure  JobRunStatus = "failure"
	StatusCancelled JobRunStatus = "cancelled"
)

// JobRun represents a single job execution record.
type JobRun struct {
	ID           int64        `json:"id"`
	JobID        string       `json:"job_id"`
	StartedAt    time.Time    `json:"started_at"`
	CompletedAt  *time.Time   `json:"completed_at,omitempty"`
	Status       JobRunStatus `json:"status"`
	ErrorMessage string       `json:"error_message,omitempty"`
	DurationMs   int64        `json:"duration_ms,omitempty"`
	Parameters   string       `json:"parameters,omitempty"`   // JSON string
	Results      string       `json:"results,omitempty"`      // JSON string
	Attempt      int          `json:"attempt"`
	Handler      string       `json:"handler,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
}

// JobRunQuery represents query parameters for fetching job runs.
type JobRunQuery struct {
	JobID    string       // Filter by job ID
	Status   JobRunStatus // Filter by status
	Limit    int          // Max records to return (default: 50)
	Offset   int          // Offset for pagination
	StartDate *time.Time  // Filter by start date
	EndDate   *time.Time  // Filter by end date
}

// JobRunResult is the interface for structured job results.
type JobRunResult interface {
	// ToMap converts the result to a map for JSON serialization.
	ToMap() map[string]interface{}
}

// DefaultJobRunResult is a generic implementation of JobRunResult.
type DefaultJobRunResult struct {
	Data map[string]interface{} `json:"data,omitempty"`
}

// ToMap implements JobRunResult.
func (r *DefaultJobRunResult) ToMap() map[string]interface{} {
	if r.Data == nil {
		return make(map[string]interface{})
	}
	return r.Data
}
