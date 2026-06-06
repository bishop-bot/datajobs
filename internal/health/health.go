package health

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/bishop-bot/datajobs/internal/logging"
)

// Status represents the health status.
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
	StatusDegraded  Status = "degraded"
)

// State holds the health state.
type State struct {
	Status  Status              `json:"status"`
	Version string              `json:"version"`
	Uptime  string              `json:"uptime"`
	Checks  map[string]CheckResult `json:"checks"`
}

// CheckResult holds the result of a health check.
type CheckResult struct {
	Status   Status `json:"status"`
	Message  string `json:"message,omitempty"`
	Duration string `json:"duration,omitempty"`
}

// Checker is the interface for health checks.
type Checker interface {
	Name() string
	Check() (Status, string, error)
}

// Server holds health check state and handlers.
type Server struct {
	version   string
	startTime time.Time
	checkers  []Checker
	ready     atomic.Bool
}

// New creates a new health server.
func New(version string) *Server {
	return &Server{
		version:   version,
		startTime: time.Now(),
		ready:     atomic.Bool{},
	}
}

// AddChecker adds a health checker.
func (s *Server) AddChecker(checker Checker) {
	s.checkers = append(s.checkers, checker)
}

// SetReady sets the ready state.
func (s *Server) SetReady(ready bool) {
	s.ready.Store(ready)
}

// IsReady returns the ready state.
func (s *Server) IsReady() bool {
	return s.ready.Load()
}

// LivenessHandler handles liveness probes.
func (s *Server) LivenessHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// ReadinessHandler handles readiness probes.
func (s *Server) ReadinessHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	state := s.getState()

	status := http.StatusOK
	if state.Status != StatusHealthy {
		status = http.StatusServiceUnavailable
	}

	data, err := json.Marshal(state)
	if err != nil {
		logging.Warn("failed to marshal health state", "error", err)
		http.Error(w, `{"status":"unhealthy","error":"failed to marshal state"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(status)
	w.Write(data)
}

// StatusHandler returns the full health status.
func (s *Server) StatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	data, err := json.Marshal(s.getState())
	if err != nil {
		logging.Warn("failed to marshal health state", "error", err)
		http.Error(w, `{"status":"unhealthy","error":"failed to marshal state"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (s *Server) getState() State {
	checks := make(map[string]CheckResult)
	overallStatus := StatusHealthy

	for _, checker := range s.checkers {
		start := time.Now()
		status, message, err := checker.Check()
		duration := time.Since(start)

		result := CheckResult{
			Status:   status,
			Duration: duration.String(),
		}
		if message != "" {
			result.Message = message
		}
		if err != nil {
			result.Message = err.Error()
		}

		checks[checker.Name()] = result

		if status == StatusUnhealthy {
			overallStatus = StatusUnhealthy
		} else if status == StatusDegraded && overallStatus == StatusHealthy {
			overallStatus = StatusDegraded
		}
	}

	return State{
		Status:  overallStatus,
		Version: s.version,
		Uptime:  time.Since(s.startTime).String(),
		Checks:  checks,
	}
}

// DummyChecker is a placeholder that always returns healthy.
type DummyChecker struct{}

func (d *DummyChecker) Name() string { return "dummy" }
func (d *DummyChecker) Check() (Status, string, error) {
	return StatusHealthy, "", nil
}