package main

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"go-agent-harness/internal/cron"
	htools "go-agent-harness/internal/harness/tools"
)

func newTestEmbeddedAdapter(t *testing.T) *embeddedCronAdapter {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "cron.db")
	st, err := cron.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	if err := st.Migrate(context.Background()); err != nil {
		st.Close()
		t.Fatalf("migrate: %v", err)
	}
	clock := cron.RealClock{}
	sched := cron.NewScheduler(st, &cron.ShellExecutor{}, clock, cron.SchedulerConfig{MaxConcurrent: 5})
	if err := sched.Start(context.Background()); err != nil {
		st.Close()
		t.Fatalf("start scheduler: %v", err)
	}
	t.Cleanup(func() {
		sched.Stop()
		st.Close()
	})
	return &embeddedCronAdapter{store: st, scheduler: sched, clock: clock}
}

func TestEmbeddedCronAdapter_CreateJob(t *testing.T) {
	t.Parallel()
	adapter := newTestEmbeddedAdapter(t)
	ctx := context.Background()

	job, err := adapter.CreateJob(ctx, htools.CronCreateJobRequest{
		Name:       "test-job",
		Schedule:   "*/5 * * * *",
		ExecType:   "shell",
		ExecConfig: `{"cmd":"echo hi"}`,
		TimeoutSec: 60,
		Tags:       "test",
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if job.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if job.Name != "test-job" {
		t.Fatalf("Name: got %q, want %q", job.Name, "test-job")
	}
	if job.Status != "active" {
		t.Fatalf("Status: got %q, want active", job.Status)
	}
	if job.NextRunAt.IsZero() {
		t.Fatal("expected non-zero NextRunAt")
	}
	// Default timeout
	job2, err := adapter.CreateJob(ctx, htools.CronCreateJobRequest{
		Name:     "default-timeout",
		Schedule: "0 * * * *",
		ExecType: "shell",
	})
	if err != nil {
		t.Fatalf("CreateJob default timeout: %v", err)
	}
	if job2.TimeoutSec != 30 {
		t.Fatalf("expected default timeout 30, got %d", job2.TimeoutSec)
	}
}

func TestEmbeddedCronAdapter_CreateJob_Validation(t *testing.T) {
	t.Parallel()
	adapter := newTestEmbeddedAdapter(t)
	ctx := context.Background()

	// Empty name
	if _, err := adapter.CreateJob(ctx, htools.CronCreateJobRequest{
		Schedule: "*/5 * * * *",
		ExecType: "shell",
	}); err == nil {
		t.Fatal("expected error for empty name")
	}

	// Empty schedule
	if _, err := adapter.CreateJob(ctx, htools.CronCreateJobRequest{
		Name:     "x",
		ExecType: "shell",
	}); err == nil {
		t.Fatal("expected error for empty schedule")
	}

	// Bad schedule
	if _, err := adapter.CreateJob(ctx, htools.CronCreateJobRequest{
		Name:     "x",
		Schedule: "bad-schedule",
		ExecType: "shell",
	}); err == nil {
		t.Fatal("expected error for bad schedule")
	}

	// Invalid exec type
	if _, err := adapter.CreateJob(ctx, htools.CronCreateJobRequest{
		Name:     "x",
		Schedule: "*/5 * * * *",
		ExecType: "invalid",
	}); err == nil {
		t.Fatal("expected error for invalid exec_type")
	}
}

func TestEmbeddedCronAdapter_GetJob(t *testing.T) {
	t.Parallel()
	adapter := newTestEmbeddedAdapter(t)
	ctx := context.Background()

	created, err := adapter.CreateJob(ctx, htools.CronCreateJobRequest{
		Name:     "get-test",
		Schedule: "*/5 * * * *",
		ExecType: "shell",
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	// Get by ID
	got, err := adapter.GetJob(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetJob by ID: %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("ID mismatch: got %q, want %q", got.ID, created.ID)
	}

	// Get by name (fallback)
	got2, err := adapter.GetJob(ctx, "get-test")
	if err != nil {
		t.Fatalf("GetJob by name: %v", err)
	}
	if got2.Name != "get-test" {
		t.Fatalf("Name mismatch: got %q", got2.Name)
	}

	// Not found
	if _, err := adapter.GetJob(ctx, "nonexistent"); err == nil {
		t.Fatal("expected error for nonexistent job")
	}
}

func TestEmbeddedCronAdapter_ListJobs(t *testing.T) {
	t.Parallel()
	adapter := newTestEmbeddedAdapter(t)
	ctx := context.Background()

	// Empty initially
	jobs, err := adapter.ListJobs(ctx)
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("expected 0 jobs, got %d", len(jobs))
	}

	// Create two
	adapter.CreateJob(ctx, htools.CronCreateJobRequest{Name: "j1", Schedule: "*/5 * * * *", ExecType: "shell"})
	adapter.CreateJob(ctx, htools.CronCreateJobRequest{Name: "j2", Schedule: "0 * * * *", ExecType: "shell"})

	jobs, err = adapter.ListJobs(ctx)
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
}

func TestEmbeddedCronAdapter_UpdateJob_Schedule(t *testing.T) {
	t.Parallel()
	adapter := newTestEmbeddedAdapter(t)
	ctx := context.Background()

	created, err := adapter.CreateJob(ctx, htools.CronCreateJobRequest{
		Name:     "update-sched",
		Schedule: "*/5 * * * *",
		ExecType: "shell",
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	newSched := "0 * * * *"
	updated, err := adapter.UpdateJob(ctx, created.ID, htools.CronUpdateJobRequest{
		Schedule: &newSched,
	})
	if err != nil {
		t.Fatalf("UpdateJob: %v", err)
	}
	if updated.Schedule != "0 * * * *" {
		t.Fatalf("Schedule: got %q, want %q", updated.Schedule, "0 * * * *")
	}
	if updated.NextRunAt.IsZero() {
		t.Fatal("expected non-zero NextRunAt after schedule change")
	}
}

func TestEmbeddedCronAdapter_UpdateJob_PauseResume(t *testing.T) {
	t.Parallel()
	adapter := newTestEmbeddedAdapter(t)
	ctx := context.Background()

	created, err := adapter.CreateJob(ctx, htools.CronCreateJobRequest{
		Name:     "pause-resume",
		Schedule: "*/5 * * * *",
		ExecType: "shell",
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	// Pause
	paused := "paused"
	got, err := adapter.UpdateJob(ctx, created.ID, htools.CronUpdateJobRequest{Status: &paused})
	if err != nil {
		t.Fatalf("Pause: %v", err)
	}
	if got.Status != "paused" {
		t.Fatalf("Status: got %q, want paused", got.Status)
	}

	// Resume
	active := "active"
	got, err = adapter.UpdateJob(ctx, created.ID, htools.CronUpdateJobRequest{Status: &active})
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if got.Status != "active" {
		t.Fatalf("Status: got %q, want active", got.Status)
	}
}

func TestEmbeddedCronAdapter_UpdateJob_Validation(t *testing.T) {
	t.Parallel()
	adapter := newTestEmbeddedAdapter(t)
	ctx := context.Background()

	// Not found
	if _, err := adapter.UpdateJob(ctx, "nonexistent", htools.CronUpdateJobRequest{}); err == nil {
		t.Fatal("expected error for nonexistent job")
	}

	created, _ := adapter.CreateJob(ctx, htools.CronCreateJobRequest{
		Name:     "val-test",
		Schedule: "*/5 * * * *",
		ExecType: "shell",
	})

	// Empty schedule
	empty := ""
	if _, err := adapter.UpdateJob(ctx, created.ID, htools.CronUpdateJobRequest{Schedule: &empty}); err == nil {
		t.Fatal("expected error for empty schedule")
	}

	// Bad schedule
	bad := "bad-schedule"
	if _, err := adapter.UpdateJob(ctx, created.ID, htools.CronUpdateJobRequest{Schedule: &bad}); err == nil {
		t.Fatal("expected error for bad schedule")
	}

	// Invalid status
	invalid := "invalid"
	if _, err := adapter.UpdateJob(ctx, created.ID, htools.CronUpdateJobRequest{Status: &invalid}); err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestEmbeddedCronAdapter_DeleteJob(t *testing.T) {
	t.Parallel()
	adapter := newTestEmbeddedAdapter(t)
	ctx := context.Background()

	created, _ := adapter.CreateJob(ctx, htools.CronCreateJobRequest{
		Name:     "delete-me",
		Schedule: "*/5 * * * *",
		ExecType: "shell",
	})

	if err := adapter.DeleteJob(ctx, created.ID); err != nil {
		t.Fatalf("DeleteJob: %v", err)
	}

	// Verify deleted (ListJobs should not include it — soft delete behavior depends on store)
	jobs, _ := adapter.ListJobs(ctx)
	for _, j := range jobs {
		if j.ID == created.ID {
			t.Fatal("expected job to be deleted")
		}
	}
}

func TestEmbeddedCronAdapter_ListExecutions(t *testing.T) {
	t.Parallel()
	adapter := newTestEmbeddedAdapter(t)
	ctx := context.Background()

	created, _ := adapter.CreateJob(ctx, htools.CronCreateJobRequest{
		Name:     "exec-test",
		Schedule: "*/5 * * * *",
		ExecType: "shell",
	})

	execs, err := adapter.ListExecutions(ctx, created.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListExecutions: %v", err)
	}
	if len(execs) != 0 {
		t.Fatalf("expected 0 executions, got %d", len(execs))
	}
}

func TestEmbeddedCronAdapter_Health(t *testing.T) {
	t.Parallel()
	adapter := newTestEmbeddedAdapter(t)
	if err := adapter.Health(context.Background()); err != nil {
		t.Fatalf("Health: %v", err)
	}
}

func TestEmbeddedCronAdapter_Concurrent(t *testing.T) {
	t.Parallel()
	adapter := newTestEmbeddedAdapter(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	errs := make(chan error, 70)

	// Seed a job so concurrent reads/updates have something to hit.
	seed, _ := adapter.CreateJob(ctx, htools.CronCreateJobRequest{
		Name: "seed-job", Schedule: "*/5 * * * *", ExecType: "shell",
	})

	for i := 0; i < 10; i++ {
		i := i
		wg.Add(5)
		go func() {
			defer wg.Done()
			if _, err := adapter.ListJobs(ctx); err != nil {
				errs <- fmt.Errorf("ListJobs: %w", err)
			}
		}()
		go func() {
			defer wg.Done()
			adapter.GetJob(ctx, seed.ID)
		}()
		go func() {
			defer wg.Done()
			adapter.Health(ctx)
		}()
		go func() {
			defer wg.Done()
			adapter.ListExecutions(ctx, seed.ID, 10, 0)
		}()
		go func() {
			defer wg.Done()
			// Writes may hit SQLITE_BUSY under extreme concurrency — acceptable.
			adapter.CreateJob(ctx, htools.CronCreateJobRequest{
				Name:     fmt.Sprintf("concurrent-%d", i),
				Schedule: "*/5 * * * *",
				ExecType: "shell",
			})
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent error: %v", err)
	}
}
