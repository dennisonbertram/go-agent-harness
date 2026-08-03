package harness

import (
	"context"
	"errors"
	"sync"
	"testing"

	"go-agent-harness/internal/store"
)

type barrierRunStore struct {
	*store.MemoryStore
	once        sync.Once
	entered     chan struct{}
	release     <-chan struct{}
	blockCreate bool
}

type racingCreateRunStore struct {
	*store.MemoryStore
	mu            sync.Mutex
	createCount   int
	firstEntered  chan struct{}
	secondEntered chan struct{}
	releaseFirst  chan struct{}
	firstCreated  chan struct{}
}

func (s *racingCreateRunStore) CreateRun(ctx context.Context, run *store.Run) error {
	s.mu.Lock()
	s.createCount++
	call := s.createCount
	s.mu.Unlock()
	switch call {
	case 1:
		close(s.firstEntered)
		<-s.releaseFirst
		err := s.MemoryStore.CreateRun(ctx, run)
		close(s.firstCreated)
		return err
	case 2:
		close(s.secondEntered)
		<-s.firstCreated
		return s.MemoryStore.CreateRun(ctx, run)
	default:
		return s.MemoryStore.CreateRun(ctx, run)
	}
}

func (s *barrierRunStore) CreateRun(ctx context.Context, r *store.Run) error {
	if s.blockCreate {
		s.once.Do(func() { close(s.entered) })
		<-s.release
	}
	return s.MemoryStore.CreateRun(ctx, r)
}
func (s *barrierRunStore) GetRun(ctx context.Context, id string) (*store.Run, error) {
	if !s.blockCreate {
		s.once.Do(func() { close(s.entered) })
		<-s.release
	}
	return s.MemoryStore.GetRun(ctx, id)
}

// TestEnsureRunWithIDContextReturnsPersistedQueuedIdentity is the #1006
// post-admission/pre-callback-link crash boundary. The second owner must resume
// the reserved queued run, never allocate a second conversation turn.
func TestEnsureRunWithIDContextReturnsPersistedQueuedIdentity(t *testing.T) {
	st := store.NewMemoryStore()
	runner := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{Store: st, DefaultModel: "test"})
	defer runner.Shutdown(context.Background())
	req := RunRequest{Prompt: "continue", ConversationID: "conv", TenantID: "tenant", AgentID: "agent"}
	first, err := runner.EnsureRunWithIDContext(context.Background(), req, "run_callback_reserved")
	if err != nil {
		t.Fatalf("first admission: %v", err)
	}
	second, err := runner.EnsureRunWithIDContext(context.Background(), req, "run_callback_reserved")
	if err != nil {
		t.Fatalf("reconcile same ID: %v", err)
	}
	if first.ID != second.ID || second.ID != "run_callback_reserved" {
		t.Fatalf("ids = %q / %q, want one reserved identity", first.ID, second.ID)
	}
}

func TestEnsureRunWithIDContextNormalizesDefaultScopeAndRejectsMismatch(t *testing.T) {
	st := store.NewMemoryStore()
	runner := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{Store: st, DefaultModel: "test"})
	defer runner.Shutdown(context.Background())
	req := RunRequest{Prompt: "continue", ConversationID: "conv"}
	if _, err := runner.EnsureRunWithIDContext(context.Background(), req, "run_callback_default_scope"); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.EnsureRunWithIDContext(context.Background(), req, "run_callback_default_scope"); err != nil {
		t.Fatalf("default scope did not reconcile: %v", err)
	}
	if _, err := runner.EnsureRunWithIDContext(context.Background(), RunRequest{Prompt: "different", ConversationID: "conv"}, "run_callback_default_scope"); err == nil {
		t.Fatal("mismatched prompt reused reserved identity")
	}
}

func TestEnsureRunWithIDContextReconcilesCreateRaceAfterOldLeaseCancellation(t *testing.T) {
	st := &racingCreateRunStore{
		MemoryStore:   store.NewMemoryStore(),
		firstEntered:  make(chan struct{}),
		secondEntered: make(chan struct{}),
		releaseFirst:  make(chan struct{}),
		firstCreated:  make(chan struct{}),
	}
	oldRunner := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{Store: st, DefaultModel: "test"})
	defer oldRunner.Shutdown(context.Background())
	newRunner := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{Store: st, DefaultModel: "test"})
	defer newRunner.Shutdown(context.Background())
	req := RunRequest{Prompt: "continue", ConversationID: "conv"}
	oldCtx, cancelOld := context.WithCancel(context.Background())
	oldDone := make(chan error, 1)
	go func() {
		_, err := oldRunner.EnsureRunWithIDContext(oldCtx, req, "run_callback_race")
		oldDone <- err
	}()
	<-st.firstEntered
	newDone := make(chan struct {
		run Run
		err error
	}, 1)
	go func() {
		run, err := newRunner.EnsureRunWithIDContext(context.Background(), req, "run_callback_race")
		newDone <- struct {
			run Run
			err error
		}{run: run, err: err}
	}()
	<-st.secondEntered
	cancelOld()
	close(st.releaseFirst)
	if err := <-oldDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("old owner error = %v, want canceled", err)
	}
	got := <-newDone
	if got.err != nil || got.run.ID != "run_callback_race" {
		t.Fatalf("new owner run=%#v err=%v", got.run, got.err)
	}
}

func TestEnsureRunWithIDContextConcurrentQueuedReconcileReturnsOneIdentity(t *testing.T) {
	st := store.NewMemoryStore()
	const runID = "run_callback_concurrent_queued"
	if err := st.CreateRun(context.Background(), &store.Run{ID: runID, Prompt: "continue", ConversationID: "conv", TenantID: "default", AgentID: "default", Model: "test", Status: store.RunStatusQueued}); err != nil {
		t.Fatal(err)
	}
	runner := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{Store: st, DefaultModel: "test"})
	defer runner.Shutdown(context.Background())
	start := make(chan struct{})
	errs := make(chan error, 20)
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			run, err := runner.EnsureRunWithIDContext(context.Background(), RunRequest{Prompt: "continue", ConversationID: "conv"}, runID)
			if err == nil && run.ID != runID {
				err = errors.New("returned a different run identity")
			}
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestStartRunWithIDContext_CanceledAfterDurableCreateRetainsQueuedIdentity(t *testing.T) {
	release := make(chan struct{})
	st := &barrierRunStore{MemoryStore: store.NewMemoryStore(), entered: make(chan struct{}), release: release, blockCreate: true}
	r := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{Store: st, DefaultModel: "test"})
	defer r.Shutdown(context.Background())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, e := r.StartRunWithIDContext(ctx, RunRequest{Prompt: "p", ConversationID: "c", TenantID: "t", AgentID: "a"}, "run_cancel_create")
		done <- e
	}()
	<-st.entered
	cancel()
	close(release)
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	if _, ok := r.GetRun("run_cancel_create"); ok {
		t.Fatal("locally published canceled run")
	}
	got, err := st.MemoryStore.GetRun(context.Background(), "run_cancel_create")
	if err != nil || got.Status != store.RunStatusQueued {
		t.Fatalf("durable=%#v err=%v", got, err)
	}
}

func TestResumeRunWithIDContext_CanceledAfterQueuedPreflightRetainsIdentity(t *testing.T) {
	base := store.NewMemoryStore()
	if err := base.CreateRun(context.Background(), &store.Run{ID: "run_cancel_resume", Prompt: "p", ConversationID: "c", TenantID: "t", AgentID: "a", Model: "test", Status: store.RunStatusQueued}); err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	st := &barrierRunStore{MemoryStore: base, entered: make(chan struct{}), release: release}
	r := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{Store: st, DefaultModel: "test"})
	defer r.Shutdown(context.Background())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, e := r.ResumeRunWithIDContext(ctx, RunRequest{Prompt: "p", ConversationID: "c", TenantID: "t", AgentID: "a"}, "run_cancel_resume")
		done <- e
	}()
	<-st.entered
	cancel()
	close(release)
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	if _, ok := r.GetRun("run_cancel_resume"); ok {
		t.Fatal("locally published canceled resumed run")
	}
	got, err := base.GetRun(context.Background(), "run_cancel_resume")
	if err != nil || got.Status != store.RunStatusQueued {
		t.Fatalf("durable=%#v err=%v", got, err)
	}
}
