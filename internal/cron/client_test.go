package cron

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientHealth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	if err := client.Health(context.Background()); err != nil {
		t.Fatalf("health: %v", err)
	}
}

func TestClientHealthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	if err := client.Health(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
}

func TestClientCreateJob(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/jobs" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var req CreateJobRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		job := Job{
			ID:       "test-id",
			Name:     req.Name,
			Schedule: req.Schedule,
			ExecType: req.ExecType,
			Status:   StatusActive,
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(job)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	job, err := client.CreateJob(context.Background(), CreateJobRequest{
		Name:     "my-job",
		Schedule: "* * * * *",
		ExecType: ExecTypeShell,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if job.Name != "my-job" {
		t.Fatalf("expected my-job, got %q", job.Name)
	}
}

func TestClientCreateJobError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"code": "validation_error", "message": "name is required"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	_, err := client.CreateJob(context.Background(), CreateJobRequest{})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestClientListJobs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/jobs" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jobs": []Job{{ID: "j1", Name: "job1"}},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	jobs, err := client.ListJobs(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
}

func TestClientListJobsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	_, err := client.ListJobs(context.Background())
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestClientGetJob(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/jobs/job-123" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(Job{ID: "job-123", Name: "my-job"})
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	job, err := client.GetJob(context.Background(), "job-123")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if job.ID != "job-123" {
		t.Fatalf("expected job-123, got %q", job.ID)
	}
}

func TestClientGetJobNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"code": "not_found", "message": "job not found"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	_, err := client.GetJob(context.Background(), "missing")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestClientUpdateJob(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		_ = json.NewEncoder(w).Encode(Job{ID: "job-1", Status: StatusPaused})
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	status := StatusPaused
	job, err := client.UpdateJob(context.Background(), "job-1", UpdateJobRequest{Status: &status})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if job.Status != StatusPaused {
		t.Fatalf("expected paused, got %q", job.Status)
	}
}

func TestClientDeleteJob(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	if err := client.DeleteJob(context.Background(), "job-1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestClientDeleteJobError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("fail"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	if err := client.DeleteJob(context.Background(), "job-1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestClientListExecutions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/jobs/job-1/history" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("limit") != "10" {
			t.Fatalf("expected limit=10, got %s", r.URL.Query().Get("limit"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"executions": []Execution{{ID: "exec-1", JobID: "job-1"}},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	execs, err := client.ListExecutions(context.Background(), "job-1", 10, 0)
	if err != nil {
		t.Fatalf("list executions: %v", err)
	}
	if len(execs) != 1 {
		t.Fatalf("expected 1 execution, got %d", len(execs))
	}
}

func TestClientUpdateJobError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"job not found"}}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	_, err := client.UpdateJob(context.Background(), "missing", UpdateJobRequest{})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestClientConnectionError(t *testing.T) {
	client := NewClient("http://127.0.0.1:1") // port 1 should refuse
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Health(ctx); err == nil {
		t.Fatalf("expected connection error")
	}
}
