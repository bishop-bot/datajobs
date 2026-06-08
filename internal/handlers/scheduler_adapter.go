package handlers

import (
	"context"

	"github.com/bishop-bot/datajobs/internal/scheduler"
)

// schedulerAdapter wraps scheduler.Scheduler to implement SchedulerRunner interface.
type schedulerAdapter struct {
	s *scheduler.Scheduler
}

// NewSchedulerAdapter creates an adapter for the scheduler.Scheduler.
func NewSchedulerAdapter(s *scheduler.Scheduler) SchedulerRunner {
	return &schedulerAdapter{s: s}
}

func (a *schedulerAdapter) RunNow(ctx context.Context, jobID string, metadata map[string]interface{}) error {
	return a.s.RunNow(ctx, jobID, metadata)
}

func (a *schedulerAdapter) ListJobs() []JobInfo {
	jobs := a.s.ListJobs()
	result := make([]JobInfo, len(jobs))
	for i, job := range jobs {
		result[i] = JobInfo{ID: job.ID, Name: job.Name}
	}
	return result
}

func (a *schedulerAdapter) GetJob(jobID string) (JobInfo, bool) {
	job, ok := a.s.GetJob(jobID)
	if !ok {
		return JobInfo{}, false
	}
	return JobInfo{ID: job.ID, Name: job.Name}, true
}