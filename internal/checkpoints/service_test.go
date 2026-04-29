package checkpoints

import (
	"context"
	"path/filepath"
	"strings"
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

func TestServiceWaitContextCancelUnregistersWaiter(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	svc := NewService(NewMemoryStore(), func() time.Time { return now })
	record, err := svc.Create(context.Background(), CreateRequest{
		Kind:       KindApproval,
		RunID:      "run-cancel-wait",
		DeadlineAt: now.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, waitErr := svc.Wait(ctx, record.ID)
		errCh <- waitErr
	}()

	deadline := time.Now().Add(2 * time.Second)
	for {
		svc.mu.Lock()
		waiterCount := len(svc.waiters[record.ID])
		svc.mu.Unlock()
		if waiterCount == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for checkpoint waiter registration")
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != context.Canceled {
			t.Fatalf("Wait error = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Wait cancellation")
	}

	svc.mu.Lock()
	_, stillRegistered := svc.waiters[record.ID]
	svc.mu.Unlock()
	if stillRegistered {
		t.Fatal("expected cancelled waiter to be unregistered")
	}
}

func TestServiceResolutionHelpersAndStores(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	store := NewMemoryStore()
	svc := NewService(store, func() time.Time { return now })
	if svc.Store() != store {
		t.Fatal("Store did not return configured store")
	}

	denied, err := svc.Create(context.Background(), CreateRequest{
		Kind:       KindApproval,
		RunID:      "run-deny",
		DeadlineAt: now.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("Create denied: %v", err)
	}
	if err := svc.Deny(context.Background(), denied.ID); err != nil {
		t.Fatalf("Deny: %v", err)
	}
	loaded, err := svc.Get(context.Background(), denied.ID)
	if err != nil {
		t.Fatalf("Get denied: %v", err)
	}
	if loaded.Status != StatusDenied {
		t.Fatalf("status = %q, want %q", loaded.Status, StatusDenied)
	}

	expired, err := svc.Create(context.Background(), CreateRequest{
		Kind:          KindExternalResume,
		WorkflowRunID: "wf-expire",
		DeadlineAt:    now.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("Create expired: %v", err)
	}
	if err := svc.Expire(context.Background(), expired.ID); err != nil {
		t.Fatalf("Expire: %v", err)
	}
	pending, ok, err := svc.PendingByWorkflowRun(context.Background(), "wf-expire")
	if err != nil {
		t.Fatalf("PendingByWorkflowRun: %v", err)
	}
	if ok {
		t.Fatalf("expected expired checkpoint to stop being pending, got %s", pending.ID)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	_, err = svc.Get(context.Background(), "missing")
	if !IsNotFound(err) {
		t.Fatalf("expected NotFoundError, got %v", err)
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Fatalf("not found error = %q, want checkpoint id", err.Error())
	}
}

func TestSQLiteStoreUpdatesAndQueriesWorkflowPending(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "checkpoints.db")
	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	record := &Record{
		ID:            "checkpoint-sqlite-update",
		Kind:          KindExternalResume,
		Status:        StatusPending,
		WorkflowRunID: "wf-sqlite",
		DeadlineAt:    now.Add(time.Minute),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := store.Create(context.Background(), record); err != nil {
		t.Fatalf("Create: %v", err)
	}
	pending, err := store.PendingByWorkflowRun(context.Background(), "wf-sqlite")
	if err != nil {
		t.Fatalf("PendingByWorkflowRun: %v", err)
	}
	if pending == nil || pending.ID != record.ID {
		t.Fatalf("pending = %+v, want %s", pending, record.ID)
	}

	record.Status = StatusResumed
	record.ResumePayload = `{"ok":true}`
	record.UpdatedAt = now.Add(time.Second)
	if err := store.Update(context.Background(), record); err != nil {
		t.Fatalf("Update: %v", err)
	}
	loaded, err := store.Get(context.Background(), record.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if loaded.Status != StatusResumed || loaded.ResumePayload == "" {
		t.Fatalf("loaded = %+v, want resumed with payload", loaded)
	}
	pending, err = store.PendingByWorkflowRun(context.Background(), "wf-sqlite")
	if err != nil {
		t.Fatalf("PendingByWorkflowRun after update: %v", err)
	}
	if pending != nil {
		t.Fatalf("expected no pending checkpoint after resume, got %+v", pending)
	}
}
