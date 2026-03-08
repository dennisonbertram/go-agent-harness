package cron

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test_cron.db")
	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestNewSQLiteStore_EmptyPath(t *testing.T) {
	_, err := NewSQLiteStore("")
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestNewSQLiteStore_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "deep", "test.db")
	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	store.Close()
	if _, err := os.Stat(filepath.Dir(path)); os.IsNotExist(err) {
		t.Fatal("expected directory to be created")
	}
}

func TestMigrate_Idempotent(t *testing.T) {
	store := newTestStore(t)
	// Migrate again should not error.
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
}

func TestCreateJob_GetJob(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	job := testJob("test-create")

	created, err := store.CreateJob(ctx, job)
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if created.ID != job.ID {
		t.Fatalf("expected ID %s, got %s", job.ID, created.ID)
	}

	got, err := store.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if got.Name != job.Name {
		t.Fatalf("expected name %s, got %s", job.Name, got.Name)
	}
	if got.Schedule != job.Schedule {
		t.Fatalf("expected schedule %s, got %s", job.Schedule, got.Schedule)
	}
	if got.ExecType != job.ExecType {
		t.Fatalf("expected exec_type %s, got %s", job.ExecType, got.ExecType)
	}
	if got.Status != StatusActive {
		t.Fatalf("expected status %s, got %s", StatusActive, got.Status)
	}
}

func TestGetJobByName(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	job := testJob("find-by-name")

	if _, err := store.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	got, err := store.GetJobByName(ctx, "find-by-name")
	if err != nil {
		t.Fatalf("GetJobByName: %v", err)
	}
	if got.ID != job.ID {
		t.Fatalf("expected ID %s, got %s", job.ID, got.ID)
	}
}

func TestCreateJob_UniqueNameConstraint(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	job1 := testJob("unique-name")
	if _, err := store.CreateJob(ctx, job1); err != nil {
		t.Fatalf("CreateJob first: %v", err)
	}

	job2 := testJob("unique-name")
	_, err := store.CreateJob(ctx, job2)
	if err == nil {
		t.Fatal("expected error for duplicate name")
	}
}

func TestListJobs(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		job := testJob("list-" + uuid.New().String()[:8])
		if _, err := store.CreateJob(ctx, job); err != nil {
			t.Fatalf("CreateJob: %v", err)
		}
	}

	jobs, err := store.ListJobs(ctx)
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(jobs))
	}
}

func TestUpdateJob(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	job := testJob("update-me")

	if _, err := store.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	job.Schedule = "0 * * * *"
	job.Tags = "updated"
	job.UpdatedAt = time.Now().UTC()
	if err := store.UpdateJob(ctx, job); err != nil {
		t.Fatalf("UpdateJob: %v", err)
	}

	got, err := store.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if got.Schedule != "0 * * * *" {
		t.Fatalf("expected updated schedule, got %s", got.Schedule)
	}
	if got.Tags != "updated" {
		t.Fatalf("expected updated tags, got %s", got.Tags)
	}
}

func TestDeleteJob_SoftDelete(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	job := testJob("delete-me")

	if _, err := store.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	if err := store.DeleteJob(ctx, job.ID); err != nil {
		t.Fatalf("DeleteJob: %v", err)
	}

	// GetJob should not find deleted jobs.
	_, err := store.GetJob(ctx, job.ID)
	if err == nil {
		t.Fatal("expected error for deleted job")
	}

	// ListJobs should not include deleted jobs.
	jobs, err := store.ListJobs(ctx)
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	for _, j := range jobs {
		if j.ID == job.ID {
			t.Fatal("deleted job should not appear in ListJobs")
		}
	}
}

func TestGetJobByName_ExcludesDeleted(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	job := testJob("deleted-name")

	if _, err := store.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if err := store.DeleteJob(ctx, job.ID); err != nil {
		t.Fatalf("DeleteJob: %v", err)
	}

	_, err := store.GetJobByName(ctx, "deleted-name")
	if err == nil {
		t.Fatal("expected error for deleted job by name")
	}
}

func TestCreateExecution_ListExecutions(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	job := testJob("exec-job")

	if _, err := store.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		exec := Execution{
			ID:        uuid.New().String(),
			JobID:     job.ID,
			StartedAt: now.Add(time.Duration(i) * time.Minute),
			Status:    ExecStatusSuccess,
		}
		if _, err := store.CreateExecution(ctx, exec); err != nil {
			t.Fatalf("CreateExecution: %v", err)
		}
	}

	execs, err := store.ListExecutions(ctx, job.ID, 3, 0)
	if err != nil {
		t.Fatalf("ListExecutions: %v", err)
	}
	if len(execs) != 3 {
		t.Fatalf("expected 3 executions, got %d", len(execs))
	}
	// Should be ordered by started_at DESC.
	if !execs[0].StartedAt.After(execs[1].StartedAt) {
		t.Fatal("expected descending order by started_at")
	}
}

func TestUpdateExecution(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	job := testJob("update-exec-job")

	if _, err := store.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	now := time.Now().UTC()
	exec := Execution{
		ID:        uuid.New().String(),
		JobID:     job.ID,
		StartedAt: now,
		Status:    ExecStatusRunning,
	}
	if _, err := store.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("CreateExecution: %v", err)
	}

	exec.Status = ExecStatusSuccess
	exec.FinishedAt = now.Add(10 * time.Second)
	exec.DurationMs = 10000
	exec.OutputSummary = "done"
	if err := store.UpdateExecution(ctx, exec); err != nil {
		t.Fatalf("UpdateExecution: %v", err)
	}

	execs, err := store.ListExecutions(ctx, job.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListExecutions: %v", err)
	}
	if len(execs) != 1 {
		t.Fatalf("expected 1 execution, got %d", len(execs))
	}
	if execs[0].Status != ExecStatusSuccess {
		t.Fatalf("expected status %s, got %s", ExecStatusSuccess, execs[0].Status)
	}
	if execs[0].DurationMs != 10000 {
		t.Fatalf("expected duration_ms 10000, got %d", execs[0].DurationMs)
	}
}

func TestListExecutions_Offset(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	job := testJob("offset-job")

	if _, err := store.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		exec := Execution{
			ID:        uuid.New().String(),
			JobID:     job.ID,
			StartedAt: now.Add(time.Duration(i) * time.Minute),
			Status:    ExecStatusSuccess,
		}
		if _, err := store.CreateExecution(ctx, exec); err != nil {
			t.Fatalf("CreateExecution: %v", err)
		}
	}

	execs, err := store.ListExecutions(ctx, job.ID, 2, 2)
	if err != nil {
		t.Fatalf("ListExecutions: %v", err)
	}
	if len(execs) != 2 {
		t.Fatalf("expected 2 executions with offset, got %d", len(execs))
	}
}

func TestGetJob_NotFound(t *testing.T) {
	store := newTestStore(t)
	_, err := store.GetJob(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent job")
	}
	if err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestClose_Nil(t *testing.T) {
	var s *SQLiteStore
	if err := s.Close(); err != nil {
		t.Fatalf("Close on nil: %v", err)
	}
}

func TestListExecutions_DefaultLimit(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	job := testJob("default-limit")

	if _, err := store.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	// With limit 0, should default to 20.
	execs, err := store.ListExecutions(ctx, job.ID, 0, 0)
	if err != nil {
		t.Fatalf("ListExecutions: %v", err)
	}
	if execs != nil {
		t.Fatalf("expected nil executions, got %v", execs)
	}
}
