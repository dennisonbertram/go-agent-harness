package cron

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestServer(t *testing.T) (http.Handler, *mockStore) {
	t.Helper()
	store := &mockStore{}
	clock := newMockClock(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	executor := &mockExecutor{}
	scheduler := NewScheduler(store, executor, clock, SchedulerConfig{MaxConcurrent: 1})
	handler := NewServer(store, scheduler, clock)
	return handler, store
}

func TestServerHealth(t *testing.T) {
	handler, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected ok, got %q", body["status"])
	}
}

func TestServerCreateJob(t *testing.T) {
	handler, store := newTestServer(t)
	store.CreateJobFunc = func(_ context.Context, job Job) (Job, error) {
		return job, nil
	}

	payload := `{"name":"test-job","schedule":"*/5 * * * *","execution_type":"shell","execution_config":"{\"command\":\"echo hi\"}"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs", strings.NewReader(payload))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var job Job
	if err := json.NewDecoder(w.Body).Decode(&job); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if job.Name != "test-job" {
		t.Fatalf("expected test-job, got %q", job.Name)
	}
	if job.Status != StatusActive {
		t.Fatalf("expected active, got %q", job.Status)
	}
	if job.ID == "" {
		t.Fatalf("expected non-empty ID")
	}
}

func TestServerCreateJobValidation(t *testing.T) {
	handler, _ := newTestServer(t)

	tests := []struct {
		name    string
		payload string
		errMsg  string
	}{
		{"missing name", `{"schedule":"* * * * *","execution_type":"shell"}`, "name is required"},
		{"missing schedule", `{"name":"x","execution_type":"shell"}`, "schedule is required"},
		{"bad schedule", `{"name":"x","schedule":"bad","execution_type":"shell"}`, "invalid schedule"},
		{"bad exec type", `{"name":"x","schedule":"* * * * *","execution_type":"bad"}`, "execution_type"},
		{"invalid json", `{bad`, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/jobs", strings.NewReader(tt.payload))
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
			if tt.errMsg != "" && !strings.Contains(w.Body.String(), tt.errMsg) {
				t.Fatalf("expected error containing %q, got %s", tt.errMsg, w.Body.String())
			}
		})
	}
}

func TestServerListJobs(t *testing.T) {
	handler, store := newTestServer(t)
	j := testJob("list-test")
	store.ListJobsFunc = func(_ context.Context) ([]Job, error) {
		return []Job{j}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/jobs", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var result struct {
		Jobs []Job `json:"jobs"`
	}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(result.Jobs))
	}
}

func TestServerListJobsEmpty(t *testing.T) {
	handler, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/jobs", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var result struct {
		Jobs []Job `json:"jobs"`
	}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result.Jobs) != 0 {
		t.Fatalf("expected 0 jobs, got %d", len(result.Jobs))
	}
}

func TestServerGetJobByID(t *testing.T) {
	handler, store := newTestServer(t)
	j := testJob("get-test")
	store.GetJobFunc = func(_ context.Context, id string) (Job, error) {
		if id == j.ID {
			return j, nil
		}
		return Job{}, sql.ErrNoRows
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/jobs/"+j.ID, nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var got Job
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Name != "get-test" {
		t.Fatalf("expected get-test, got %q", got.Name)
	}
}

func TestServerGetJobByName(t *testing.T) {
	handler, store := newTestServer(t)
	j := testJob("named-job")
	store.GetJobFunc = func(_ context.Context, id string) (Job, error) {
		return Job{}, sql.ErrNoRows
	}
	store.GetJobByNameFunc = func(_ context.Context, name string) (Job, error) {
		if name == "named-job" {
			return j, nil
		}
		return Job{}, sql.ErrNoRows
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/jobs/named-job", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var got Job
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Name != "named-job" {
		t.Fatalf("expected named-job, got %q", got.Name)
	}
}

func TestServerGetJobNotFound(t *testing.T) {
	handler, store := newTestServer(t)
	store.GetJobFunc = func(_ context.Context, id string) (Job, error) {
		return Job{}, sql.ErrNoRows
	}
	store.GetJobByNameFunc = func(_ context.Context, name string) (Job, error) {
		return Job{}, sql.ErrNoRows
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/jobs/nonexistent", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestServerUpdateJobSchedule(t *testing.T) {
	handler, store := newTestServer(t)
	j := testJob("update-test")
	store.GetJobFunc = func(_ context.Context, id string) (Job, error) {
		if id == j.ID {
			return j, nil
		}
		return Job{}, sql.ErrNoRows
	}
	var updated Job
	store.UpdateJobFunc = func(_ context.Context, job Job) error {
		updated = job
		return nil
	}

	newSchedule := "0 * * * *"
	payload := fmt.Sprintf(`{"schedule":"%s"}`, newSchedule)
	req := httptest.NewRequest(http.MethodPatch, "/v1/jobs/"+j.ID, strings.NewReader(payload))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if updated.Schedule != newSchedule {
		t.Fatalf("expected schedule %q, got %q", newSchedule, updated.Schedule)
	}
}

func TestServerUpdateJobPause(t *testing.T) {
	handler, store := newTestServer(t)
	j := testJob("pause-test")
	store.GetJobFunc = func(_ context.Context, id string) (Job, error) {
		return j, nil
	}
	var updated Job
	store.UpdateJobFunc = func(_ context.Context, job Job) error {
		updated = job
		return nil
	}

	payload := `{"status":"paused"}`
	req := httptest.NewRequest(http.MethodPatch, "/v1/jobs/"+j.ID, strings.NewReader(payload))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if updated.Status != StatusPaused {
		t.Fatalf("expected paused, got %q", updated.Status)
	}
}

func TestServerUpdateJobResume(t *testing.T) {
	handler, store := newTestServer(t)
	j := testJob("resume-test")
	j.Status = StatusPaused
	store.GetJobFunc = func(_ context.Context, id string) (Job, error) {
		return j, nil
	}
	store.UpdateJobFunc = func(_ context.Context, job Job) error {
		return nil
	}

	payload := `{"status":"active"}`
	req := httptest.NewRequest(http.MethodPatch, "/v1/jobs/"+j.ID, strings.NewReader(payload))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestServerUpdateJobNotFound(t *testing.T) {
	handler, store := newTestServer(t)
	store.GetJobFunc = func(_ context.Context, id string) (Job, error) {
		return Job{}, sql.ErrNoRows
	}

	req := httptest.NewRequest(http.MethodPatch, "/v1/jobs/missing", strings.NewReader(`{"status":"paused"}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestServerUpdateJobInvalidJSON(t *testing.T) {
	handler, store := newTestServer(t)
	j := testJob("bad-json")
	store.GetJobFunc = func(_ context.Context, id string) (Job, error) {
		return j, nil
	}

	req := httptest.NewRequest(http.MethodPatch, "/v1/jobs/"+j.ID, strings.NewReader(`{bad`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestServerDeleteJob(t *testing.T) {
	handler, store := newTestServer(t)
	deleted := false
	store.DeleteJobFunc = func(_ context.Context, id string) error {
		deleted = true
		return nil
	}

	req := httptest.NewRequest(http.MethodDelete, "/v1/jobs/some-id", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	if !deleted {
		t.Fatalf("expected delete to be called")
	}
}

func TestServerHistory(t *testing.T) {
	handler, store := newTestServer(t)
	exec := Execution{
		ID:     "exec-1",
		JobID:  "job-1",
		Status: ExecStatusSuccess,
	}
	store.ListExecutionsFunc = func(_ context.Context, jobID string, limit, offset int) ([]Execution, error) {
		return []Execution{exec}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/jobs/job-1/history?limit=10&offset=0", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var result struct {
		Executions []Execution `json:"executions"`
	}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result.Executions) != 1 {
		t.Fatalf("expected 1 execution, got %d", len(result.Executions))
	}
}

func TestServerHistoryEmpty(t *testing.T) {
	handler, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/jobs/job-1/history", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var result struct {
		Executions []Execution `json:"executions"`
	}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result.Executions) != 0 {
		t.Fatalf("expected 0 executions, got %d", len(result.Executions))
	}
}

func TestServerMethodNotAllowed(t *testing.T) {
	handler, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/v1/jobs", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestServerJobByIDMethodNotAllowed(t *testing.T) {
	handler, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/v1/jobs/some-id", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestServerHistoryMethodNotAllowed(t *testing.T) {
	handler, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/v1/jobs/some-id/history", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestServerJobByIDNotFound(t *testing.T) {
	handler, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/jobs/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestServerJobByIDUnknownSubpath(t *testing.T) {
	handler, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/jobs/some-id/unknown", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestNextRunTime(t *testing.T) {
	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	next, err := nextRunTime("*/5 * * * *", from)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Date(2025, 1, 1, 0, 5, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, next)
	}

	_, err = nextRunTime("bad-schedule", from)
	if err == nil {
		t.Fatalf("expected error for bad schedule")
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"key": "value"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %q", ct)
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusBadRequest, "test_error", "test message")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	errObj, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object")
	}
	if errObj["code"] != "test_error" {
		t.Fatalf("expected test_error, got %v", errObj["code"])
	}
}

func TestWriteMethodNotAllowed(t *testing.T) {
	w := httptest.NewRecorder()
	writeMethodNotAllowed(w, "GET, POST")
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
	if allow := w.Header().Get("Allow"); allow != "GET, POST" {
		t.Fatalf("expected Allow: GET, POST, got %q", allow)
	}
}

func TestServerCreateJobStoreError(t *testing.T) {
	handler, store := newTestServer(t)
	store.CreateJobFunc = func(_ context.Context, job Job) (Job, error) {
		return Job{}, fmt.Errorf("store failure")
	}

	payload := `{"name":"err-job","schedule":"* * * * *","execution_type":"shell"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs", strings.NewReader(payload))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestServerUpdateJobInvalidSchedule(t *testing.T) {
	handler, store := newTestServer(t)
	j := testJob("bad-sched")
	store.GetJobFunc = func(_ context.Context, id string) (Job, error) {
		return j, nil
	}

	payload := `{"schedule":"bad"}`
	req := httptest.NewRequest(http.MethodPatch, "/v1/jobs/"+j.ID, strings.NewReader(payload))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestServerCreateJobDefaultTimeout(t *testing.T) {
	handler, store := newTestServer(t)
	var created Job
	store.CreateJobFunc = func(_ context.Context, job Job) (Job, error) {
		created = job
		return job, nil
	}

	payload := `{"name":"default-timeout","schedule":"* * * * *","execution_type":"shell"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs", strings.NewReader(payload))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if created.TimeoutSec != 30 {
		t.Fatalf("expected default timeout 30, got %d", created.TimeoutSec)
	}
}

func TestServerListJobsStoreError(t *testing.T) {
	handler, store := newTestServer(t)
	store.ListJobsFunc = func(_ context.Context) ([]Job, error) {
		return nil, fmt.Errorf("db error")
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/jobs", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// Verify we are using bytes.Buffer correctly by testing JSON encoding round-trip
func TestServerCreateJobRoundTrip(t *testing.T) {
	handler, store := newTestServer(t)
	store.CreateJobFunc = func(_ context.Context, job Job) (Job, error) {
		return job, nil
	}

	input := CreateJobRequest{
		Name:       "round-trip",
		Schedule:   "0 0 * * *",
		ExecType:   ExecTypeShell,
		ExecConfig: `{"command":"echo test"}`,
		TimeoutSec: 60,
		Tags:       "test,ci",
	}
	body, _ := json.Marshal(input)
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var job Job
	if err := json.NewDecoder(w.Body).Decode(&job); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if job.TimeoutSec != 60 {
		t.Fatalf("expected 60 timeout, got %d", job.TimeoutSec)
	}
	if job.Tags != "test,ci" {
		t.Fatalf("expected tags 'test,ci', got %q", job.Tags)
	}
}
