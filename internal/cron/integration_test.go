//go:build !short

package cron

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestIntegrationCronAPI(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test-cronsd.db")

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	executor := &ShellExecutor{}
	clock := RealClock{}
	scheduler := NewScheduler(store, executor, clock, SchedulerConfig{MaxConcurrent: 2})
	handler := NewServer(store, scheduler, clock)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := NewClient(srv.URL)

	// Health check.
	if err := client.Health(ctx); err != nil {
		t.Fatalf("health: %v", err)
	}

	// Create job.
	job, err := client.CreateJob(ctx, CreateJobRequest{
		Name:       "echo-hello",
		Schedule:   "* * * * *",
		ExecType:   ExecTypeShell,
		ExecConfig: `{"command":"echo hello"}`,
		TimeoutSec: 10,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if job.ID == "" {
		t.Fatalf("expected non-empty job ID")
	}
	if job.Status != StatusActive {
		t.Fatalf("expected active, got %q", job.Status)
	}

	// List jobs.
	jobs, err := client.ListJobs(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}

	// Get job by ID.
	got, err := client.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got.Name != "echo-hello" {
		t.Fatalf("expected echo-hello, got %q", got.Name)
	}

	// Get job by name.
	got, err = client.GetJob(ctx, "echo-hello")
	if err != nil {
		t.Fatalf("get by name: %v", err)
	}
	if got.ID != job.ID {
		t.Fatalf("expected ID %s, got %s", job.ID, got.ID)
	}

	// Pause job.
	status := StatusPaused
	paused, err := client.UpdateJob(ctx, job.ID, UpdateJobRequest{Status: &status})
	if err != nil {
		t.Fatalf("pause: %v", err)
	}
	if paused.Status != StatusPaused {
		t.Fatalf("expected paused, got %q", paused.Status)
	}

	// Resume job.
	status = StatusActive
	resumed, err := client.UpdateJob(ctx, job.ID, UpdateJobRequest{Status: &status})
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if resumed.Status != StatusActive {
		t.Fatalf("expected active, got %q", resumed.Status)
	}

	// List executions (should be empty).
	execs, err := client.ListExecutions(ctx, job.ID, 10, 0)
	if err != nil {
		t.Fatalf("list executions: %v", err)
	}
	if len(execs) != 0 {
		t.Fatalf("expected 0 executions, got %d", len(execs))
	}

	// Delete job.
	if err := client.DeleteJob(ctx, job.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// List should be empty.
	jobs, err = client.ListJobs(ctx)
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("expected 0 jobs after delete, got %d", len(jobs))
	}
}
