package audit

import (
	"testing"
)

func TestJobRunStatus(t *testing.T) {
	tests := []struct {
		status   JobRunStatus
		expected string
	}{
		{StatusRunning, "running"},
		{StatusSuccess, "success"},
		{StatusFailure, "failure"},
		{StatusCancelled, "cancelled"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.expected {
			t.Errorf("JobRunStatus = %q, want %q", tt.status, tt.expected)
		}
	}
}

func TestDefaultJobRunResult(t *testing.T) {
	result := &DefaultJobRunResult{
		Data: map[string]interface{}{
			"processed": 100,
			"upserted":  50,
		},
	}

	mp := result.ToMap()
	if mp["processed"] != 100 {
		t.Errorf("ToMap()[processed] = %v, want 100", mp["processed"])
	}
	if mp["upserted"] != 50 {
		t.Errorf("ToMap()[upserted] = %v, want 50", mp["upserted"])
	}
}

func TestDefaultJobRunResultNilData(t *testing.T) {
	result := &DefaultJobRunResult{}
	mp := result.ToMap()
	if mp == nil {
		t.Error("ToMap() should return empty map, not nil")
	}
}

func TestNewStats(t *testing.T) {
	stats := NewStats()
	if stats.ByStatus == nil {
		t.Error("NewStats().ByStatus should not be nil")
	}
}

func TestJobRunQuery(t *testing.T) {
	query := JobRunQuery{
		JobID:  "test-job",
		Status: StatusSuccess,
		Limit:  10,
		Offset: 0,
	}

	if query.JobID != "test-job" {
		t.Errorf("JobRunQuery.JobID = %q, want %q", query.JobID, "test-job")
	}
	if query.Status != StatusSuccess {
		t.Errorf("JobRunQuery.Status = %q, want %q", query.Status, StatusSuccess)
	}
	if query.Limit != 10 {
		t.Errorf("JobRunQuery.Limit = %d, want %d", query.Limit, 10)
	}
}
