package checkpoints

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteStorePersistsCheckpointAcrossReopen(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "checkpoints.db")
	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	svc := NewService(store, func() time.Time { return now })
	record, err := svc.Create(context.Background(), CreateRequest{
		Kind:       KindApproval,
		RunID:      "run-1",
		CallID:     "call-1",
		Tool:       "write",
		Args:       `{"path":"README.md"}`,
		DeadlineAt: now.Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore reopen: %v", err)
	}
	if err := reopened.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate reopen: %v", err)
	}
	defer reopened.Close()

	svc = NewService(reopened, func() time.Time { return now })
	loaded, err := svc.Get(context.Background(), record.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if loaded.Status != StatusPending {
		t.Fatalf("status = %q, want %q", loaded.Status, StatusPending)
	}
	if loaded.RunID != "run-1" {
		t.Fatalf("run_id = %q, want run-1", loaded.RunID)
	}
	if loaded.Tool != "write" {
		t.Fatalf("tool = %q, want write", loaded.Tool)
	}

	pending, ok, err := svc.PendingByRun(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("PendingByRun: %v", err)
	}
	if !ok {
		t.Fatal("expected pending checkpoint for run")
	}
	if pending.ID != record.ID {
		t.Fatalf("pending id = %q, want %q", pending.ID, record.ID)
	}
}

func TestServiceResumeWakesWaiterAndPersistsPayload(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	svc := NewService(NewMemoryStore(), func() time.Time { return now })
	record, err := svc.Create(context.Background(), CreateRequest{
		Kind:           KindExternalResume,
		WorkflowRunID:  "wf-1",
		RunID:          "run-1",
		SuspendPayload: `{"prompt":"Need human confirmation"}`,
		ResumeSchema:   `{"type":"object"}`,
		DeadlineAt:     now.Add(5 * time.Minute),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	waitCh := make(chan WaitResult, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := svc.Wait(context.Background(), record.ID)
		if err != nil {
			errCh <- err
			return
		}
		waitCh <- result
	}()

	if err := svc.Resume(context.Background(), record.ID, map[string]any{"decision": "approved"}); err != nil {
		t.Fatalf("Resume: %v", err)
	}

	select {
	case err := <-errCh:
		t.Fatalf("Wait error: %v", err)
	case result := <-waitCh:
		if result.Status != StatusResumed {
			t.Fatalf("wait status = %q, want %q", result.Status, StatusResumed)
		}
		if got := result.Payload["decision"]; got != "approved" {
			t.Fatalf("payload decision = %v, want approved", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for resume")
	}

	loaded, err := svc.Get(context.Background(), record.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if loaded.Status != StatusResumed {
		t.Fatalf("stored status = %q, want %q", loaded.Status, StatusResumed)
	}
	if loaded.ResumePayload == "" {
		t.Fatal("expected persisted resume payload")
	}
}
