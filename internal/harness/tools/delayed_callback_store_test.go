package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

type failingCallbackStore struct {
	updates int
	fail    int
	rows    map[string]CallbackInfo
}

func (s *failingCallbackStore) Migrate(context.Context) error { return nil }
func (s *failingCallbackStore) Close() error                  { return nil }
func (s *failingCallbackStore) Create(_ context.Context, i CallbackInfo) error {
	if s.rows == nil {
		s.rows = map[string]CallbackInfo{}
	}
	s.rows[i.ID] = i
	return nil
}
func (s *failingCallbackStore) Get(_ context.Context, id string) (CallbackInfo, error) {
	i, ok := s.rows[id]
	if !ok {
		return CallbackInfo{}, fmt.Errorf("missing")
	}
	return i, nil
}
func (s *failingCallbackStore) Update(_ context.Context, i CallbackInfo) error {
	s.updates++
	if s.updates <= s.fail {
		return fmt.Errorf("injected update failure")
	}
	s.rows[i.ID] = i
	return nil
}
func (s *failingCallbackStore) ListPending(context.Context) ([]CallbackInfo, error) { return nil, nil }

func TestCallbackSQLiteStoreRoundTripAndScope(t *testing.T) {
	store, err := NewSQLiteCallbackStore(filepath.Join(t.TempDir(), "callbacks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	want := CallbackInfo{ID: "cb-1", ConversationID: "conv", TenantID: "tenant", AgentID: "agent", Prompt: "hello", Delay: "5s", State: CallbackStatePending, FiresAt: time.Now().Add(time.Minute), CreatedAt: time.Now()}
	if err := store.Create(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(ctx, want.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.TenantID != want.TenantID || got.AgentID != want.AgentID || got.ConversationID != want.ConversationID || got.Prompt != want.Prompt {
		t.Fatalf("round trip = %#v", got)
	}
}

func TestCallbackManagerShutdownRestartPreservesPendingAndOverdueFires(t *testing.T) {
	store, err := NewSQLiteCallbackStore(filepath.Join(t.TempDir(), "callbacks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	first := NewCallbackManager(&mockRunStarter{}, WithCallbackStore(store))
	info, err := first.Set(SetRequest{ConversationID: "conv", TenantID: "tenant", AgentID: "agent", Prompt: "hello", Delay: MinCallbackDelay})
	if err != nil {
		t.Fatal(err)
	}
	first.Shutdown()
	got, err := store.Get(context.Background(), info.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != CallbackStatePending {
		t.Fatalf("shutdown persisted %s, want pending", got.State)
	}
	if err := store.Update(context.Background(), CallbackInfo{ID: got.ID, ConversationID: got.ConversationID, TenantID: got.TenantID, AgentID: got.AgentID, Prompt: got.Prompt, Delay: got.Delay, State: CallbackStatePending, FiresAt: time.Now().Add(-time.Second), CreatedAt: got.CreatedAt}); err != nil {
		t.Fatal(err)
	}
	starter := &mockRunStarter{}
	second := NewCallbackManager(starter, WithCallbackStore(store))
	defer second.Shutdown()
	if err := second.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(time.Second)
	for len(starter.getCalls()) == 0 {
		select {
		case <-deadline:
			t.Fatal("overdue callback did not fire after recovery")
		case <-time.After(time.Millisecond):
		}
	}
}

func TestCallbackManagerRecoverySkipsCanceledAndFired(t *testing.T) {
	store, err := NewSQLiteCallbackStore(filepath.Join(t.TempDir(), "callbacks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	for _, state := range []CallbackState{CallbackStateCanceled, CallbackStateFired} {
		i := CallbackInfo{ID: string(state), ConversationID: "conv", Prompt: "must not run", Delay: "5s", State: state, FiresAt: time.Now().Add(-time.Second), CreatedAt: time.Now()}
		if err := store.Create(ctx, i); err != nil {
			t.Fatal(err)
		}
	}
	starter := &mockRunStarter{}
	mgr := NewCallbackManager(starter, WithCallbackStore(store))
	defer mgr.Shutdown()
	if err := mgr.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	if got := starter.getCalls(); len(got) != 0 {
		t.Fatalf("terminal callbacks fired: %#v", got)
	}
}

func TestCallbackManagerCancelPersistsTerminalState(t *testing.T) {
	store, err := NewSQLiteCallbackStore(filepath.Join(t.TempDir(), "callbacks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	mgr := NewCallbackManager(&mockRunStarter{}, WithCallbackStore(store))
	defer mgr.Shutdown()
	i, err := mgr.Set(setReq("conv", MinCallbackDelay, "later"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.Cancel(i.ID); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(context.Background(), i.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != CallbackStateCanceled {
		t.Fatalf("state=%s want canceled", got.State)
	}
}

func TestCallbackManagerCancelStoreFailureLeavesTimerArmed(t *testing.T) {
	store := &failingCallbackStore{fail: 1}
	starter := &mockRunStarter{}
	mgr := NewCallbackManager(starter, WithCallbackStore(store))
	defer mgr.Shutdown()
	i, err := mgr.Set(setReq("conv", MinCallbackDelay, "later"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.Cancel(i.ID); err == nil {
		t.Fatal("cancel unexpectedly succeeded")
	}
	mgr.fire(i.ID)
	if calls := starter.getCalls(); len(calls) != 1 {
		t.Fatalf("pending callback did not fire after failed cancellation: %#v", calls)
	}
}

func TestCallbackManagerFireStoreFailureRetriesOnceBeforeDispatch(t *testing.T) {
	store := &failingCallbackStore{fail: 1}
	starter := &mockRunStarter{}
	mgr := NewCallbackManager(starter, WithCallbackStore(store))
	defer mgr.Shutdown()
	i, err := mgr.Set(setReq("conv", MinCallbackDelay, "later"))
	if err != nil {
		t.Fatal(err)
	}
	mgr.fire(i.ID)
	deadline := time.After(time.Second)
	for len(starter.getCalls()) == 0 {
		select {
		case <-deadline:
			t.Fatal("persistence retry did not dispatch")
		case <-time.After(time.Millisecond):
		}
	}
	if store.updates != 2 {
		t.Fatalf("updates=%d want one retry", store.updates)
	}
}

func TestCallbackManagerShutdownWaitsForCommittedDispatch(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	starter := &mockRunStarter{startFn: func(string, string, string, string) error { close(entered); <-release; return nil }}
	m := NewCallbackManager(starter)
	i := CallbackInfo{ID: "commit-wins", ConversationID: "c", Prompt: "p", State: CallbackStatePending}
	m.callbacks[i.ID] = &pendingCallback{info: i, timer: time.AfterFunc(time.Hour, func() {})}
	m.byConv[i.ConversationID] = []string{i.ID}
	go m.fire(i.ID)
	<-entered
	done := make(chan struct{})
	go func() { m.Shutdown(); close(done) }()
	select {
	case <-done:
		t.Fatal("shutdown returned before committed StartRun completed")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	<-done
}
