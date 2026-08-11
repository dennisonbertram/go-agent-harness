package checkpoints

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type recordBlockingCheckpointStore struct {
	Store
	blockID string
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (s *recordBlockingCheckpointStore) Update(ctx context.Context, record *Record) error {
	if record.ID == s.blockID {
		s.once.Do(func() { close(s.started) })
		select {
		case <-s.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return s.Store.Update(ctx, record)
}

func (s *recordBlockingCheckpointStore) ResolvePending(
	ctx context.Context,
	id string,
	status Status,
	resumePayload string,
	updatedAt time.Time,
) (*Record, bool, error) {
	if id == s.blockID {
		s.once.Do(func() { close(s.started) })
		select {
		case <-s.release:
		case <-ctx.Done():
			return nil, false, ctx.Err()
		}
	}
	return s.Store.ResolvePending(ctx, id, status, resumePayload, updatedAt)
}

type racingConditionalCheckpointStore struct {
	*MemoryStore
	entered chan struct{}
	release chan struct{}
}

type transientPollGetStore struct {
	Store
	mu         sync.Mutex
	getCalls   int
	pollFailed chan struct{}
}

func (s *transientPollGetStore) Get(ctx context.Context, id string) (*Record, error) {
	s.mu.Lock()
	s.getCalls++
	call := s.getCalls
	s.mu.Unlock()
	if call == 3 {
		close(s.pollFailed)
		return nil, errors.New("transient checkpoint read outage")
	}
	return s.Store.Get(ctx, id)
}

func newRacingConditionalCheckpointStore() *racingConditionalCheckpointStore {
	return &racingConditionalCheckpointStore{
		MemoryStore: NewMemoryStore(),
		entered:     make(chan struct{}, 2),
		release:     make(chan struct{}),
	}
}

func (s *racingConditionalCheckpointStore) waitForRace(ctx context.Context) error {
	select {
	case s.entered <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-s.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *racingConditionalCheckpointStore) Update(ctx context.Context, record *Record) error {
	if err := s.waitForRace(ctx); err != nil {
		return err
	}
	return s.MemoryStore.Update(ctx, record)
}

// ResolvePending is the atomic store contract exercised by the repair. It is
// intentionally an extra method until the production Store interface adopts
// it; the pre-fix Service continues through non-conditional Update above.
func (s *racingConditionalCheckpointStore) ResolvePending(
	ctx context.Context,
	id string,
	status Status,
	resumePayload string,
	updatedAt time.Time,
) (*Record, bool, error) {
	if err := s.waitForRace(ctx); err != nil {
		return nil, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.records[id]
	if !ok {
		return nil, false, &NotFoundError{ID: id}
	}
	if current.Status != StatusPending {
		return cloneRecord(current), false, nil
	}
	current.Status = status
	current.ResumePayload = resumePayload
	current.UpdatedAt = updatedAt
	return cloneRecord(current), true, nil
}

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

func TestMemoryStoreUpdateCopiesReplacementRecord(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	record := &Record{
		ID:            "checkpoint-memory-update",
		Kind:          KindExternalResume,
		Status:        StatusPending,
		WorkflowRunID: "workflow-memory-update",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := store.Create(context.Background(), record); err != nil {
		t.Fatalf("Create: %v", err)
	}

	replacement := cloneRecord(record)
	replacement.Status = StatusResumed
	replacement.ResumePayload = `{"answer":"persisted"}`
	replacement.UpdatedAt = now.Add(time.Minute)
	if err := store.Update(context.Background(), replacement); err != nil {
		t.Fatalf("Update: %v", err)
	}
	replacement.Status = StatusDenied
	replacement.ResumePayload = `{"answer":"mutated-after-update"}`

	loaded, err := store.Get(context.Background(), record.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if loaded.Status != StatusResumed {
		t.Fatalf("status = %q, want %q", loaded.Status, StatusResumed)
	}
	if loaded.ResumePayload != `{"answer":"persisted"}` {
		t.Fatalf("resume payload = %q, want copied replacement payload", loaded.ResumePayload)
	}
	if !loaded.UpdatedAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("updated_at = %s, want %s", loaded.UpdatedAt, now.Add(time.Minute))
	}
}

func TestCheckpointStoresResolvePendingAtomically(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		open func(t *testing.T) Store
	}{
		{
			name: "memory",
			open: func(t *testing.T) Store {
				t.Helper()
				return NewMemoryStore()
			},
		},
		{
			name: "sqlite",
			open: func(t *testing.T) Store {
				t.Helper()
				store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "checkpoints.db"))
				if err != nil {
					t.Fatalf("NewSQLiteStore: %v", err)
				}
				if err := store.Migrate(context.Background()); err != nil {
					t.Fatalf("Migrate: %v", err)
				}
				return store
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			store := test.open(t)
			t.Cleanup(func() { _ = store.Close() })
			now := time.Now().UTC()
			record := &Record{
				ID:        "checkpoint-atomic-" + test.name,
				Kind:      KindUserInput,
				Status:    StatusPending,
				RunID:     "run-atomic-" + test.name,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := store.Create(context.Background(), record); err != nil {
				t.Fatalf("Create: %v", err)
			}
			resumed, won, err := store.ResolvePending(
				context.Background(),
				record.ID,
				StatusResumed,
				`{"answer":"yes"}`,
				now.Add(time.Second),
			)
			if err != nil {
				t.Fatalf("ResolvePending first: %v", err)
			}
			if !won || resumed.Status != StatusResumed {
				t.Fatalf("first resolution = (%+v, %t), want resumed winner", resumed, won)
			}
			current, won, err := store.ResolvePending(
				context.Background(),
				record.ID,
				StatusExpired,
				"",
				now.Add(2*time.Second),
			)
			if err != nil {
				t.Fatalf("ResolvePending second: %v", err)
			}
			if won {
				t.Fatal("second terminal transition unexpectedly won")
			}
			if current.Status != StatusResumed || current.ResumePayload != `{"answer":"yes"}` {
				t.Fatalf("current record = %+v, want original resumed payload", current)
			}
		})
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

func TestServiceReportsWhenResolutionAlreadyLostToExpiry(t *testing.T) {
	t.Parallel()

	svc := NewService(NewMemoryStore(), time.Now)
	record, err := svc.Create(context.Background(), CreateRequest{
		Kind:       KindUserInput,
		RunID:      "run-resolution-race",
		DeadlineAt: time.Now().Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	expired, err := svc.ExpirePending(context.Background(), record.ID)
	if err != nil {
		t.Fatalf("ExpirePending: %v", err)
	}
	if !expired {
		t.Fatal("ExpirePending did not resolve pending checkpoint")
	}
	if err := svc.Resume(context.Background(), record.ID, map[string]any{"answer": "late"}); !errors.Is(err, ErrAlreadyResolved) {
		t.Fatalf("Resume error = %v, want ErrAlreadyResolved", err)
	}
}

func TestServiceResolutionDoesNotSerializeUnrelatedRecords(t *testing.T) {
	t.Parallel()

	base := NewMemoryStore()
	store := &recordBlockingCheckpointStore{
		Store:   base,
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	released := false
	t.Cleanup(func() {
		if !released {
			close(store.release)
		}
	})
	svc := NewService(store, time.Now)
	first, err := svc.Create(context.Background(), CreateRequest{Kind: KindUserInput, RunID: "run-a"})
	if err != nil {
		t.Fatalf("Create first: %v", err)
	}
	second, err := svc.Create(context.Background(), CreateRequest{Kind: KindUserInput, RunID: "run-b"})
	if err != nil {
		t.Fatalf("Create second: %v", err)
	}
	store.blockID = first.ID
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- svc.Resume(context.Background(), first.ID, map[string]any{"answer": "a"})
	}()
	select {
	case <-store.started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first record resolution")
	}

	secondDone := make(chan error, 1)
	go func() {
		secondDone <- svc.Resume(context.Background(), second.ID, map[string]any{"answer": "b"})
	}()
	select {
	case err := <-secondDone:
		if err != nil {
			t.Fatalf("unrelated Resume: %v", err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("unrelated checkpoint resolution was blocked by service-wide serialization")
	}
	close(store.release)
	released = true
	if err := <-firstDone; err != nil {
		t.Fatalf("first Resume: %v", err)
	}
}

func TestServiceResolutionLockHonorsWaitingContext(t *testing.T) {
	t.Parallel()

	base := NewMemoryStore()
	store := &recordBlockingCheckpointStore{
		Store:   base,
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	released := false
	t.Cleanup(func() {
		if !released {
			close(store.release)
		}
	})
	svc := NewService(store, time.Now)
	record, err := svc.Create(context.Background(), CreateRequest{Kind: KindUserInput, RunID: "run-context"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	store.blockID = record.ID
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- svc.Resume(context.Background(), record.ID, map[string]any{"answer": "first"})
	}()
	select {
	case <-store.started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first resolution")
	}

	waitCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- svc.Resume(waitCtx, record.ID, map[string]any{"answer": "second"})
	}()
	select {
	case err := <-secondDone:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("waiting Resume error = %v, want context deadline", err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("checkpoint resolution lock ignored the waiting context")
	}
	close(store.release)
	released = true
	if err := <-firstDone; err != nil {
		t.Fatalf("first Resume: %v", err)
	}
}

func TestServiceConditionalResolutionHasOneWinnerAcrossServices(t *testing.T) {
	t.Parallel()

	store := newRacingConditionalCheckpointStore()
	svcA := NewService(store, time.Now)
	svcB := NewService(store, time.Now)
	record, err := svcA.Create(context.Background(), CreateRequest{Kind: KindUserInput, RunID: "run-shared"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	resumeDone := make(chan error, 1)
	expireDone := make(chan struct {
		expired bool
		err     error
	}, 1)
	go func() {
		resumeDone <- svcA.Resume(context.Background(), record.ID, map[string]any{"answer": "accepted"})
	}()
	go func() {
		expired, err := svcB.ExpirePending(context.Background(), record.ID)
		expireDone <- struct {
			expired bool
			err     error
		}{expired: expired, err: err}
	}()
	for range 2 {
		select {
		case <-store.entered:
		case <-time.After(time.Second):
			t.Fatal("timed out staging cross-service resolution race")
		}
	}
	close(store.release)
	resumeErr := <-resumeDone
	expireResult := <-expireDone
	if expireResult.err != nil {
		t.Fatalf("ExpirePending: %v", expireResult.err)
	}
	resumeWon := resumeErr == nil
	if resumeErr != nil && !errors.Is(resumeErr, ErrAlreadyResolved) {
		t.Fatalf("Resume error = %v, want nil or ErrAlreadyResolved", resumeErr)
	}
	if resumeWon == expireResult.expired {
		t.Fatalf("resolution winners: resume=%t expire=%t, want exactly one", resumeWon, expireResult.expired)
	}
	loaded, err := svcA.Get(context.Background(), record.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if resumeWon && loaded.Status != StatusResumed {
		t.Fatalf("stored status = %q, want resumed winner", loaded.Status)
	}
	if expireResult.expired && loaded.Status != StatusExpired {
		t.Fatalf("stored status = %q, want expired winner", loaded.Status)
	}
}

func TestServiceWaitObservesResolutionFromAnotherService(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	waitingService := NewService(store, time.Now)
	resolvingService := NewService(store, time.Now)
	record, err := waitingService.Create(context.Background(), CreateRequest{Kind: KindUserInput, RunID: "run-remote"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	waitCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	waitDone := make(chan waitResult, 1)
	go func() {
		result, err := waitingService.Wait(waitCtx, record.ID)
		waitDone <- waitResult{result: result, err: err}
	}()

	deadline := time.Now().Add(time.Second)
	for {
		waitingService.mu.Lock()
		registered := len(waitingService.waiters[record.ID]) == 1
		waitingService.mu.Unlock()
		if registered {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for waiter registration")
		}
		time.Sleep(time.Millisecond)
	}
	if err := resolvingService.Resume(context.Background(), record.ID, map[string]any{"answer": "remote"}); err != nil {
		t.Fatalf("remote Resume: %v", err)
	}
	select {
	case outcome := <-waitDone:
		if outcome.err != nil {
			t.Fatalf("Wait: %v", outcome.err)
		}
		if outcome.result.Status != StatusResumed {
			t.Fatalf("Wait status = %q, want resumed", outcome.result.Status)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("Wait did not observe resolution persisted by another Service")
	}
}

func TestServiceWaitSurvivesTransientPollingReadFailure(t *testing.T) {
	t.Parallel()

	baseStore := NewMemoryStore()
	store := &transientPollGetStore{
		Store:      baseStore,
		pollFailed: make(chan struct{}),
	}
	waitingService := NewService(store, time.Now)
	resolvingService := NewService(baseStore, time.Now)
	record, err := waitingService.Create(context.Background(), CreateRequest{Kind: KindUserInput, RunID: "run-transient-poll"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	waitCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	waitDone := make(chan waitResult, 1)
	go func() {
		result, err := waitingService.Wait(waitCtx, record.ID)
		waitDone <- waitResult{result: result, err: err}
	}()

	select {
	case <-store.pollFailed:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for transient polling failure")
	}
	if err := resolvingService.Resume(context.Background(), record.ID, map[string]any{"answer": "after outage"}); err != nil {
		t.Fatalf("remote Resume: %v", err)
	}
	select {
	case outcome := <-waitDone:
		if outcome.err != nil {
			t.Fatalf("Wait returned transient polling error: %v", outcome.err)
		}
		if outcome.result.Status != StatusResumed || outcome.result.Payload["answer"] != "after outage" {
			t.Fatalf("Wait result = %+v, want resumed remote payload", outcome.result)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Wait did not remain registered after transient polling failure")
	}
}

func TestServiceStoreDenyExpireAndWaitCancellation(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	store := NewMemoryStore()
	if err := store.Close(); err != nil {
		t.Fatalf("MemoryStore Close: %v", err)
	}
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
		t.Fatalf("Create denied record: %v", err)
	}
	if err := svc.Deny(context.Background(), denied.ID); err != nil {
		t.Fatalf("Deny: %v", err)
	}
	result, err := svc.Wait(context.Background(), denied.ID)
	if err != nil {
		t.Fatalf("Wait denied: %v", err)
	}
	if result.Status != StatusDenied {
		t.Fatalf("Wait denied status = %q, want %q", result.Status, StatusDenied)
	}

	expired, err := svc.Create(context.Background(), CreateRequest{
		Kind:       KindExternalResume,
		RunID:      "run-expire",
		DeadlineAt: now.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("Create expired record: %v", err)
	}
	if err := svc.Expire(context.Background(), expired.ID); err != nil {
		t.Fatalf("Expire: %v", err)
	}
	result, err = svc.Wait(context.Background(), expired.ID)
	if err != nil {
		t.Fatalf("Wait expired: %v", err)
	}
	if result.Status != StatusExpired {
		t.Fatalf("Wait expired status = %q, want %q", result.Status, StatusExpired)
	}

	cancelled, err := svc.Create(context.Background(), CreateRequest{
		Kind:       KindUserInput,
		RunID:      "run-cancel-wait",
		DeadlineAt: now.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("Create cancellation record: %v", err)
	}
	waitCtx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := svc.Wait(waitCtx, cancelled.ID)
		errCh <- err
	}()

	deadline := time.Now().Add(2 * time.Second)
	for {
		svc.mu.Lock()
		waiterCount := len(svc.waiters[cancelled.ID])
		svc.mu.Unlock()
		if waiterCount == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for service waiter registration")
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Wait cancellation error = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for cancelled waiter")
	}
	svc.mu.Lock()
	_, stillRegistered := svc.waiters[cancelled.ID]
	svc.mu.Unlock()
	if stillRegistered {
		t.Fatal("cancelled waiter was not unregistered")
	}
}

func TestSQLiteStoreUpdatePendingByWorkflowRunAndNotFound(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "checkpoints.db")
	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 4, 5, 12, 0, 0, 123, time.UTC)
	older := &Record{
		ID:            "checkpoint-old",
		Kind:          KindExternalResume,
		Status:        StatusPending,
		WorkflowRunID: "workflow-1",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	newer := &Record{
		ID:            "checkpoint-new",
		Kind:          KindExternalResume,
		Status:        StatusPending,
		WorkflowRunID: "workflow-1",
		CreatedAt:     now,
		UpdatedAt:     now.Add(time.Minute),
	}
	if err := store.Create(context.Background(), older); err != nil {
		t.Fatalf("Create older: %v", err)
	}
	if err := store.Create(context.Background(), newer); err != nil {
		t.Fatalf("Create newer: %v", err)
	}

	pending, err := store.PendingByWorkflowRun(context.Background(), "workflow-1")
	if err != nil {
		t.Fatalf("PendingByWorkflowRun: %v", err)
	}
	if pending == nil || pending.ID != newer.ID {
		t.Fatalf("pending workflow checkpoint = %+v, want %s", pending, newer.ID)
	}

	newer.Status = StatusResumed
	newer.ResumePayload = `{"approved":true}`
	newer.UpdatedAt = now.Add(2 * time.Minute)
	if err := store.Update(context.Background(), newer); err != nil {
		t.Fatalf("Update: %v", err)
	}
	loaded, err := store.Get(context.Background(), newer.ID)
	if err != nil {
		t.Fatalf("Get updated: %v", err)
	}
	if loaded.Status != StatusResumed {
		t.Fatalf("updated status = %q, want %q", loaded.Status, StatusResumed)
	}
	if loaded.ResumePayload != newer.ResumePayload {
		t.Fatalf("resume payload = %q, want %q", loaded.ResumePayload, newer.ResumePayload)
	}
	if !loaded.UpdatedAt.Equal(newer.UpdatedAt) {
		t.Fatalf("updated_at = %s, want %s", loaded.UpdatedAt, newer.UpdatedAt)
	}

	pending, err = store.PendingByWorkflowRun(context.Background(), "workflow-1")
	if err != nil {
		t.Fatalf("PendingByWorkflowRun after update: %v", err)
	}
	if pending == nil || pending.ID != older.ID {
		t.Fatalf("pending workflow checkpoint after update = %+v, want %s", pending, older.ID)
	}

	_, err = store.Get(context.Background(), "missing")
	if !IsNotFound(err) {
		t.Fatalf("Get missing error = %v, want NotFoundError", err)
	}
	if !strings.Contains(err.Error(), "checkpoint not found: missing") {
		t.Fatalf("NotFoundError text = %q", err.Error())
	}
	if IsNotFound(context.Canceled) {
		t.Fatal("IsNotFound returned true for unrelated error")
	}
}

// TestServiceApproveWithPayloadWakesWaiterWithPayload covers the
// approve-with-payload path used by plan-exit approach options: the waiter
// observes an approved status and the operator's selected option.
func TestServiceApproveWithPayloadWakesWaiterWithPayload(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	svc := NewService(NewMemoryStore(), func() time.Time { return now })
	record, err := svc.Create(context.Background(), CreateRequest{
		Kind:       KindApproval,
		RunID:      "run-1",
		CallID:     "plan_exit",
		Tool:       "plan_exit",
		DeadlineAt: now.Add(5 * time.Minute),
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

	if err := svc.ApproveWithPayload(context.Background(), record.ID, map[string]any{"option": "b"}); err != nil {
		t.Fatalf("ApproveWithPayload: %v", err)
	}

	select {
	case err := <-errCh:
		t.Fatalf("Wait error: %v", err)
	case result := <-waitCh:
		if result.Status != StatusApproved {
			t.Fatalf("wait status = %q, want %q", result.Status, StatusApproved)
		}
		if got := result.Payload["option"]; got != "b" {
			t.Fatalf("payload option = %v, want b", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for approve")
	}

	loaded, err := svc.Get(context.Background(), record.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if loaded.Status != StatusApproved {
		t.Fatalf("stored status = %q, want %q", loaded.Status, StatusApproved)
	}
}
