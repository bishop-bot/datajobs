package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// mockScheduler for testing RunJob
type mockScheduler struct {
	runNowCalled    bool
	receivedJobID   string
	receivedMeta    map[string]interface{}
	shouldError     bool
}

func (m *mockScheduler) RunNow(ctx context.Context, jobID string, metadata map[string]interface{}) error {
	m.runNowCalled = true
	m.receivedJobID = jobID
	m.receivedMeta = metadata
	if m.shouldError {
		return ErrJobNotFound
	}
	return nil
}

func (m *mockScheduler) ListJobs() []JobInfo {
	return nil
}

func (m *mockScheduler) GetJob(id string) (JobInfo, bool) {
	return JobInfo{}, false
}

// ErrJobNotFound is a simple error for testing
var ErrJobNotFound = &jobNotFoundError{}

type jobNotFoundError struct{}

func (e *jobNotFoundError) Error() string { return "job not found" }

// testSystemHandler sets up a system handler with mock scheduler and chi router
type testSystemHandler struct {
	handler *SystemHandler
	mock    *mockScheduler
	router  *chi.Mux
}

func newTestSystemHandler() *testSystemHandler {
	mock := &mockScheduler{}
	handler := &SystemHandler{scheduler: mock, pool: nil}
	router := chi.NewRouter()
	router.Post("/api/v1/jobs/{id}/run", handler.RunJob)
	return &testSystemHandler{
		handler: handler,
		mock:    mock,
		router:  router,
	}
}

// serveRequest creates a request and serves it through the router
func (ts *testSystemHandler) serveRequest(method, path string, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	ts.router.ServeHTTP(w, req)
	return w
}

func TestRunJob_NoMetadata(t *testing.T) {
	ts := newTestSystemHandler()

	w := ts.serveRequest("POST", "/api/v1/jobs/test-job/run", "")

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d: %s", w.Code, w.Body.String())
	}

	if !ts.mock.runNowCalled {
		t.Error("RunNow should have been called")
	}

	if ts.mock.receivedJobID != "test-job" {
		t.Errorf("jobID = %q, want %q", ts.mock.receivedJobID, "test-job")
	}

	// No metadata should result in empty map
	if ts.mock.receivedMeta == nil {
		t.Error("metadata should be empty map, not nil")
	}
	if len(ts.mock.receivedMeta) != 0 {
		t.Errorf("metadata should be empty, got %v", ts.mock.receivedMeta)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success=true, got false")
	}
}

func TestRunJob_WithMetadata(t *testing.T) {
	ts := newTestSystemHandler()

	w := ts.serveRequest("POST", "/api/v1/jobs/historical-data/run", `{"instruments": ["265598"], "period": "1d", "bar": "5mins"}`)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d: %s", w.Code, w.Body.String())
	}

	if ts.mock.receivedJobID != "historical-data" {
		t.Errorf("jobID = %q, want %q", ts.mock.receivedJobID, "historical-data")
	}

	if len(ts.mock.receivedMeta) != 3 {
		t.Errorf("metadata should have 3 keys, got %d: %v", len(ts.mock.receivedMeta), ts.mock.receivedMeta)
	}

	if ts.mock.receivedMeta["instruments"] == nil {
		t.Error("metadata should contain 'instruments'")
	}

	instruments, ok := ts.mock.receivedMeta["instruments"].([]interface{})
	if !ok {
		t.Fatalf("instruments should be []interface{}, got %T", ts.mock.receivedMeta["instruments"])
	}
	if len(instruments) != 1 || instruments[0] != "265598" {
		t.Errorf("instruments = %v, want [265598]", instruments)
	}

	if ts.mock.receivedMeta["period"] != "1d" {
		t.Errorf("period = %v, want 1d", ts.mock.receivedMeta["period"])
	}

	if ts.mock.receivedMeta["bar"] != "5mins" {
		t.Errorf("bar = %v, want 5mins", ts.mock.receivedMeta["bar"])
	}
}

func TestRunJob_WithPartialMetadata(t *testing.T) {
	ts := newTestSystemHandler()

	w := ts.serveRequest("POST", "/api/v1/jobs/historical-data/run", `{"period": "1y"}`)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d", w.Code)
	}

	if len(ts.mock.receivedMeta) != 1 {
		t.Errorf("metadata should have 1 key, got %d", len(ts.mock.receivedMeta))
	}

	if ts.mock.receivedMeta["period"] != "1y" {
		t.Errorf("period = %v, want 1y", ts.mock.receivedMeta["period"])
	}
}

func TestRunJob_JobNotFound(t *testing.T) {
	ts := newTestSystemHandler()
	ts.mock.shouldError = true

	w := ts.serveRequest("POST", "/api/v1/jobs/nonexistent/run", "")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Success {
		t.Error("expected success=false")
	}
	if resp.Error != "job not found" {
		t.Errorf("error = %q, want %q", resp.Error, "job not found")
	}
}

func TestRunJob_InvalidJSON(t *testing.T) {
	ts := newTestSystemHandler()

	w := ts.serveRequest("POST", "/api/v1/jobs/test/run", `{invalid json}`)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Success {
		t.Error("expected success=false")
	}
	if !strings.Contains(resp.Error, "invalid JSON body") {
		t.Errorf("error should contain 'invalid JSON body', got %q", resp.Error)
	}
}

func TestRunJob_ResponseIncludesMetadata(t *testing.T) {
	ts := newTestSystemHandler()

	w := ts.serveRequest("POST", "/api/v1/jobs/test/run", `{"instruments": ["123", "456"]}`)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Data should be map, got %T", resp.Data)
	}

	// Check metadata is echoed back in response
	meta, ok := data["metadata"].(map[string]interface{})
	if !ok {
		t.Fatal("metadata should be in response Data")
	}

	if meta["instruments"] == nil {
		t.Error("metadata.instruments should be in response")
	}
}

func TestRunJob_EmptyJSONObject(t *testing.T) {
	ts := newTestSystemHandler()

	w := ts.serveRequest("POST", "/api/v1/jobs/test/run", `{}`)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d", w.Code)
	}

	// Empty object should result in empty map
	if len(ts.mock.receivedMeta) != 0 {
		t.Errorf("metadata should be empty, got %v", ts.mock.receivedMeta)
	}
}

func TestRunJob_NestedMetadata(t *testing.T) {
	ts := newTestSystemHandler()

	w := ts.serveRequest("POST", "/api/v1/jobs/test/run", `{"config": {"timeout": 30, "retries": 3}}`)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d", w.Code)
	}

	config, ok := ts.mock.receivedMeta["config"].(map[string]interface{})
	if !ok {
		t.Fatalf("config should be map, got %T", ts.mock.receivedMeta["config"])
	}

	if config["timeout"] != float64(30) {
		t.Errorf("timeout = %v, want 30", config["timeout"])
	}
	if config["retries"] != float64(3) {
		t.Errorf("retries = %v, want 3", config["retries"])
	}
}