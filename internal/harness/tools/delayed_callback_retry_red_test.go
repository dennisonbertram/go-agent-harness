package tools

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestCallbackRetryableAdmissionFailureRetainsReservedRunID is the #1006
// pre-link crash boundary: a callback must reserve its run identity at Set and
// a retryable admission error must not consume that identity as "fired".
func TestCallbackRetryableAdmissionFailureRetainsReservedRunID(t *testing.T) {
	store := &failingCallbackStore{}
	starter := &retryableCallbackStarter{err: errors.New("temporary unavailable")}
	mgr := NewCallbackManager(starter, WithCallbackStore(store))
	defer mgr.Shutdown()

	info, err := mgr.Set(setReq("conv", MinCallbackDelay, "continue"))
	if err != nil {
		t.Fatal(err)
	}
	if info.RunID == "" {
		t.Fatal("Set did not durably reserve a callback run ID")
	}
	mgr.fire(info.ID)

	got, err := store.Get(context.Background(), info.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != CallbackStateRetryWait {
		t.Fatalf("state = %q, want retry_wait", got.State)
	}
	if got.RunID != info.RunID || got.Attempt != 1 || !got.NextAttemptAt.After(time.Time{}) {
		t.Fatalf("retry checkpoint = %#v", got)
	}
}

func TestCallbackManagerRetryThenStartedUsesOneReservedRunID(t *testing.T) {
	store := &failingCallbackStore{}
	calls := 0
	starter := &mockRunStarter{startFn: func(string, string, string, string) error {
		calls++
		if calls == 1 {
			return errors.New("temporary unavailable")
		}
		return nil
	}}
	mgr := NewCallbackManager(starter, WithCallbackStore(store))
	defer mgr.Shutdown()
	info, err := mgr.Set(setReq("conv", MinCallbackDelay, "continue"))
	if err != nil {
		t.Fatal(err)
	}
	mgr.fire(info.ID)
	first, err := store.Get(context.Background(), info.ID)
	if err != nil {
		t.Fatal(err)
	}
	if first.State != CallbackStateRetryWait {
		t.Fatalf("first=%#v", first)
	}
	mgr.fire(info.ID)
	second, err := store.Get(context.Background(), info.ID)
	if err != nil {
		t.Fatal(err)
	}
	if second.State != CallbackStateStarted || second.RunID != first.RunID || calls != 2 {
		t.Fatalf("second=%#v calls=%d", second, calls)
	}
}

type retryableCallbackStarter struct{ err error }

func (s *retryableCallbackStarter) StartRun(string, string, string, string) error { return s.err }

func (s *retryableCallbackStarter) StartCallback(context.Context, CallbackInfo) (string, error) {
	return "", s.err
}

type callbackAdmissionStarter struct {
	mu      sync.Mutex
	ids     []string
	entered chan struct{}
	release chan struct{}
	errs    []error
}

// transientLeaseStore models a pooled SQLite connection that returns a
// transient busy error while the existing dispatch lease is still valid.
// It deliberately delegates all durable state to the real store so the test
// exercises two real managers competing for the same callback row.
type transientLeaseStore struct {
	CallbackStore
	mu        sync.Mutex
	failCount int
}

func (s *transientLeaseStore) ExtendLease(ctx context.Context, id, token string, now, until time.Time) (bool, error) {
	s.mu.Lock()
	if s.failCount != 0 {
		if s.failCount > 0 {
			s.failCount--
		}
		s.mu.Unlock()
		return false, errors.New("database is locked")
	}
	s.mu.Unlock()
	return s.CallbackStore.ExtendLease(ctx, id, token, now, until)
}

type cancellationAwareCallbackStarter struct {
	entered  chan struct{}
	canceled chan time.Time
	once     sync.Once
}

type transientClaimStore struct {
	CallbackStore
	mu        sync.Mutex
	failCount int
	calls     int
}

// blockingLeaseStore models a SQLite renewal held in a busy wait until the
// context deadline. It is intentionally not an immediate-error seam: #1106
// must cancel admission at that deadline before another manager can reclaim.
type blockingLeaseStore struct {
	CallbackStore
	once            sync.Once
	deadlineOnce    sync.Once
	releaseOnce     sync.Once
	entered         chan struct{}
	deadlineReached chan struct{}
	release         chan struct{}
}

func (s *blockingLeaseStore) ExtendLease(ctx context.Context, _ string, _ string, _ time.Time, _ time.Time) (bool, error) {
	s.once.Do(func() { close(s.entered) })
	<-ctx.Done()
	s.deadlineOnce.Do(func() { close(s.deadlineReached) })
	<-s.release
	return false, ctx.Err()
}

func (s *blockingLeaseStore) unblock() { s.releaseOnce.Do(func() { close(s.release) }) }

func (s *transientClaimStore) ClaimDue(ctx context.Context, id, token string, now, until time.Time) (CallbackInfo, bool, error) {
	s.mu.Lock()
	s.calls++
	if s.failCount > 0 {
		s.failCount--
		s.mu.Unlock()
		return CallbackInfo{}, false, errors.New("database is locked")
	}
	s.mu.Unlock()
	return s.CallbackStore.ClaimDue(ctx, id, token, now, until)
}

func (s *transientClaimStore) claimCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func (*cancellationAwareCallbackStarter) StartRun(string, string, string, string) error { return nil }

func (s *cancellationAwareCallbackStarter) StartCallback(ctx context.Context, _ CallbackInfo) (string, error) {
	s.once.Do(func() { close(s.entered) })
	<-ctx.Done()
	s.canceled <- time.Now()
	return "", ctx.Err()
}

type leaseStealingCallbackStarter struct {
	store *SQLiteCallbackStore
}

func (s *leaseStealingCallbackStarter) StartRun(string, string, string, string) error { return nil }

func (s *leaseStealingCallbackStarter) StartCallback(_ context.Context, info CallbackInfo) (string, error) {
	now := time.Now().UTC().Add(time.Hour)
	if _, won, err := s.store.ReclaimExpired(context.Background(), info.ID, "new-owner", now, now.Add(time.Hour)); err != nil || !won {
		return "", fmt.Errorf("steal lease won=%v: %w", won, err)
	}
	return info.RunID, nil
}

func (s *callbackAdmissionStarter) StartRun(string, string, string, string) error { return nil }

func (s *callbackAdmissionStarter) StartCallback(ctx context.Context, info CallbackInfo) (string, error) {
	s.mu.Lock()
	call := len(s.ids)
	s.ids = append(s.ids, info.RunID)
	entered := s.entered
	release := s.release
	var err error
	if call < len(s.errs) {
		err = s.errs[call]
	}
	s.mu.Unlock()
	if entered != nil && call == 0 {
		close(entered)
	}
	if release != nil && call == 0 {
		select {
		case <-release:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return info.RunID, err
}

func (s *callbackAdmissionStarter) calls() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.ids...)
}

func newRetrySQLiteStore(t *testing.T, path string) *SQLiteCallbackStore {
	t.Helper()
	store, err := NewSQLiteCallbackStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		store.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func waitForCallbackState(t *testing.T, store CallbackStore, id string, want CallbackState) CallbackInfo {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		info, err := store.Get(context.Background(), id)
		if err == nil && info.State == want {
			return info
		}
		time.Sleep(time.Millisecond)
	}
	info, err := store.Get(context.Background(), id)
	t.Fatalf("callback %s state = %q err=%v, want %q", id, info.State, err, want)
	return CallbackInfo{}
}

func waitForCallbackAdmission(t *testing.T, entered <-chan struct{}) {
	t.Helper()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("callback admission did not start")
	}
}

func TestCallbackManagerDuplicateManagersClaimOneDispatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "callbacks.db")
	storeA := newRetrySQLiteStore(t, path)
	storeB := newRetrySQLiteStore(t, path)
	now := time.Now().UTC()
	info := CallbackInfo{ID: "one-claim", ConversationID: "conv", Prompt: "continue", Delay: "5s", State: CallbackStatePending, FiresAt: now.Add(-time.Second), CreatedAt: now, RunID: "run_callback_one-claim"}
	if err := storeA.Create(context.Background(), info); err != nil {
		t.Fatal(err)
	}
	starter := &callbackAdmissionStarter{entered: make(chan struct{}), release: make(chan struct{})}
	first := NewCallbackManager(starter, WithCallbackStore(storeA))
	first.leaseTime = 30 * time.Millisecond
	defer first.Shutdown()
	second := NewCallbackManager(starter, WithCallbackStore(storeB))
	second.leaseTime = 30 * time.Millisecond
	defer second.Shutdown()
	if err := first.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitForCallbackAdmission(t, starter.entered)
	if err := second.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	second.fire(info.ID)
	// Wait beyond the original lease. The first manager's heartbeat must keep
	// the claim live, so the second manager's recovery timer cannot take over.
	time.Sleep(100 * time.Millisecond)
	if got := starter.calls(); len(got) != 1 || got[0] != info.RunID {
		t.Fatalf("duplicate dispatch calls = %#v", got)
	}
	close(starter.release)
	started := waitForCallbackState(t, storeA, info.ID, CallbackStateStarted)
	if started.Attempt != 1 {
		t.Fatalf("attempts = %d, want 1", started.Attempt)
	}
}

// TestCallbackManagerTransientHeartbeatBusyRetainsClaim is the #1106
// regression: a transient SQLite error before the last confirmed lease
// deadline must not cancel the active admission. A competing manager must
// therefore never reclaim and start the same reserved run ID.
func TestCallbackManagerTransientHeartbeatBusyRetainsClaim(t *testing.T) {
	path := filepath.Join(t.TempDir(), "callbacks.db")
	baseA := newRetrySQLiteStore(t, path)
	storeB := newRetrySQLiteStore(t, path)
	now := time.Now().UTC()
	info := CallbackInfo{ID: "transient-heartbeat", ConversationID: "conv", Prompt: "continue", Delay: "5s", State: CallbackStatePending, FiresAt: now.Add(-time.Second), CreatedAt: now, RunID: "run_callback_transient-heartbeat"}
	if err := baseA.Create(context.Background(), info); err != nil {
		t.Fatal(err)
	}
	storeA := &transientLeaseStore{CallbackStore: baseA, failCount: 1}
	starter := &callbackAdmissionStarter{entered: make(chan struct{}), release: make(chan struct{})}
	first := NewCallbackManager(starter, WithCallbackStore(storeA))
	first.leaseTime = 90 * time.Millisecond
	defer first.Shutdown()
	second := NewCallbackManager(starter, WithCallbackStore(storeB))
	second.leaseTime = 90 * time.Millisecond
	defer second.Shutdown()
	if err := first.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitForCallbackAdmission(t, starter.entered)
	if err := second.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	// This extends beyond the original lease. The first heartbeat initially
	// reports busy; it must retry before the last confirmed deadline rather
	// than cancel and leave a reclaimable duplicate run.
	time.Sleep(150 * time.Millisecond)
	second.fire(info.ID)
	if got := starter.calls(); len(got) != 1 || got[0] != info.RunID {
		t.Fatalf("transient heartbeat allowed duplicate dispatch: %#v", got)
	}
	close(starter.release)
	started := waitForCallbackState(t, baseA, info.ID, CallbackStateStarted)
	if started.Attempt != 1 || started.RunID != info.RunID {
		t.Fatalf("durable callback = %#v", started)
	}
}

// TestCallbackManagerPersistentHeartbeatBusyWaitsForConfirmedDeadline proves
// that repeated transient storage errors only surrender ownership after the
// last successfully persisted lease expires. At that point a new owner can
// reclaim the durable row, while the old admission has already been canceled.
func TestCallbackManagerPersistentHeartbeatBusyWaitsForConfirmedDeadline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "callbacks.db")
	baseA := newRetrySQLiteStore(t, path)
	storeB := newRetrySQLiteStore(t, path)
	now := time.Now().UTC()
	info := CallbackInfo{ID: "heartbeat-deadline", ConversationID: "conv", Prompt: "continue", Delay: "5s", State: CallbackStatePending, FiresAt: now.Add(-time.Second), CreatedAt: now, RunID: "run_callback_heartbeat-deadline"}
	if err := baseA.Create(context.Background(), info); err != nil {
		t.Fatal(err)
	}
	storeA := &transientLeaseStore{CallbackStore: baseA, failCount: -1}
	starter := &cancellationAwareCallbackStarter{entered: make(chan struct{}), canceled: make(chan time.Time, 1)}
	first := NewCallbackManager(starter, WithCallbackStore(storeA))
	first.leaseTime = 90 * time.Millisecond
	defer first.Shutdown()
	startedAt := time.Now()
	if err := first.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitForCallbackAdmission(t, starter.entered)
	var canceledAt time.Time
	select {
	case canceledAt = <-starter.canceled:
	case <-time.After(time.Second):
		t.Fatal("persistent heartbeat busy never canceled admission at lease deadline")
	}
	if elapsed := canceledAt.Sub(startedAt); elapsed < 75*time.Millisecond {
		t.Fatalf("admission canceled after %v, before last confirmed lease deadline", elapsed)
	}
	// Stop the former owner before testing recovery takeover. Its durable row is
	// intentionally left dispatching, so after that final lease expires another
	// process can own the same reserved run identity safely.
	first.Shutdown()
	time.Sleep(95 * time.Millisecond)
	claimed, won, err := storeB.ReclaimExpired(context.Background(), info.ID, "takeover", time.Now().UTC(), time.Now().UTC().Add(time.Minute))
	if err != nil || !won || claimed.DispatchToken != "takeover" {
		t.Fatalf("post-deadline takeover callback=%#v won=%v err=%v", claimed, won, err)
	}
	if err := storeB.MarkStarted(context.Background(), info.ID, "takeover", info.RunID); err != nil {
		t.Fatalf("takeover completion: %v", err)
	}
}

// TestCallbackManagerRetriesTransientClaimContention confirms that contention
// before ownership is retried in one bounded dispatch rather than relying on
// the historical one-shot timer retry.
func TestCallbackManagerRetriesTransientClaimContention(t *testing.T) {
	base := newRetrySQLiteStore(t, filepath.Join(t.TempDir(), "callbacks.db"))
	store := &transientClaimStore{CallbackStore: base, failCount: 2}
	starter := &callbackAdmissionStarter{}
	mgr := NewCallbackManager(starter, WithCallbackStore(store))
	mgr.claimRetries = 3
	mgr.claimBackoff = time.Millisecond
	defer mgr.Shutdown()
	info, err := mgr.Set(setReq("conv", MinCallbackDelay, "continue"))
	if err != nil {
		t.Fatal(err)
	}
	if err := base.Update(context.Background(), CallbackInfo{ID: info.ID, State: CallbackStatePending, FiresAt: time.Now().Add(-time.Second)}); err != nil {
		t.Fatal(err)
	}
	mgr.fire(info.ID)
	started := waitForCallbackState(t, base, info.ID, CallbackStateStarted)
	if got := store.claimCalls(); got != 3 || started.RunID != info.RunID {
		t.Fatalf("claim calls=%d started=%#v", got, started)
	}
}

// TestCallbackManagerBlockingHeartbeatCancelsBeforeTakeover proves the
// deadline guard owns cancellation independently of a blocked ExtendLease.
// The replacement manager is not allowed to admit its reserved run until the
// original admission has observed cancellation.
func TestCallbackManagerBlockingHeartbeatCancelsBeforeTakeover(t *testing.T) {
	path := filepath.Join(t.TempDir(), "callbacks.db")
	baseA := newRetrySQLiteStore(t, path)
	storeB := newRetrySQLiteStore(t, path)
	now := time.Now().UTC()
	info := CallbackInfo{ID: "blocked-heartbeat", ConversationID: "conv", Prompt: "continue", Delay: "5s", State: CallbackStatePending, FiresAt: now.Add(-time.Second), CreatedAt: now, RunID: "run_callback_blocked-heartbeat"}
	if err := baseA.Create(context.Background(), info); err != nil {
		t.Fatal(err)
	}
	oldStarter := &cancellationAwareCallbackStarter{entered: make(chan struct{}), canceled: make(chan time.Time, 1)}
	blocking := &blockingLeaseStore{CallbackStore: baseA, entered: make(chan struct{}), deadlineReached: make(chan struct{}), release: make(chan struct{})}
	first := NewCallbackManager(oldStarter, WithCallbackStore(blocking))
	first.leaseTime = 90 * time.Millisecond
	t.Cleanup(first.Shutdown)
	// Cleanups run LIFO: unblock the held renewal before Shutdown waits for its
	// dispatch goroutine when this intentionally-red test calls Fatal.
	t.Cleanup(blocking.unblock)
	if err := first.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitForCallbackAdmission(t, oldStarter.entered)
	select {
	case <-blocking.entered:
	case <-time.After(time.Second):
		t.Fatal("heartbeat renewal did not enter blocking lease store")
	}
	select {
	case <-blocking.deadlineReached:
	case <-time.After(time.Second):
		t.Fatal("blocking lease store did not reach its renewal deadline")
	}
	select {
	case <-oldStarter.canceled:
	case <-time.After(time.Second):
		t.Fatal("blocked heartbeat left old dispatch alive after lease deadline")
	}

	newStarter := &callbackAdmissionStarter{entered: make(chan struct{}), release: make(chan struct{})}
	second := NewCallbackManager(newStarter, WithCallbackStore(storeB))
	second.leaseTime = 90 * time.Millisecond
	defer second.Shutdown()
	if err := second.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	go second.fire(info.ID)
	waitForCallbackAdmission(t, newStarter.entered)
	if got := newStarter.calls(); len(got) != 1 || got[0] != info.RunID {
		t.Fatalf("replacement admission=%#v, want one fenced run", got)
	}
	close(newStarter.release)
	started := waitForCallbackState(t, storeB, info.ID, CallbackStateStarted)
	if started.Attempt != 2 || started.RunID != info.RunID {
		t.Fatalf("replacement durable state=%#v", started)
	}
	blocking.unblock()
}

func TestCallbackManagerCancelLosesAfterDispatchClaim(t *testing.T) {
	store := newRetrySQLiteStore(t, filepath.Join(t.TempDir(), "callbacks.db"))
	starter := &callbackAdmissionStarter{entered: make(chan struct{}), release: make(chan struct{})}
	mgr := NewCallbackManager(starter, WithCallbackStore(store))
	defer mgr.Shutdown()
	info, err := mgr.Set(setReq("conv", MinCallbackDelay, "continue"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Update(context.Background(), CallbackInfo{ID: info.ID, State: CallbackStatePending, FiresAt: time.Now().Add(-time.Second)}); err != nil {
		t.Fatal(err)
	}
	go mgr.fire(info.ID)
	waitForCallbackAdmission(t, starter.entered)
	if _, err := mgr.Cancel(info.ID); !errors.Is(err, ErrCallbackCancelConflict) {
		t.Fatalf("cancel error = %v, want dispatch conflict", err)
	}
	close(starter.release)
	waitForCallbackState(t, store, info.ID, CallbackStateStarted)
}

func TestCallbackManagerRecoveryHonorsRetryWaitAndReusesIdentity(t *testing.T) {
	store := newRetrySQLiteStore(t, filepath.Join(t.TempDir(), "callbacks.db"))
	now := time.Now().UTC()
	next := now.Add(60 * time.Millisecond)
	info := CallbackInfo{ID: "retry-recover", ConversationID: "conv", Prompt: "continue", Delay: "5s", State: CallbackStateRetryWait, FiresAt: now.Add(-time.Minute), CreatedAt: now, RunID: "run_callback_retry-recover", Attempt: 1, NextAttemptAt: next, LastError: "callback admission unavailable"}
	if err := store.Create(context.Background(), info); err != nil {
		t.Fatal(err)
	}
	starter := &callbackAdmissionStarter{}
	mgr := NewCallbackManager(starter, WithCallbackStore(store))
	defer mgr.Shutdown()
	if err := mgr.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	time.Sleep(15 * time.Millisecond)
	if got := starter.calls(); len(got) != 0 {
		t.Fatalf("retry dispatched before next_attempt_at: %#v", got)
	}
	started := waitForCallbackState(t, store, info.ID, CallbackStateStarted)
	if got := starter.calls(); len(got) != 1 || got[0] != info.RunID || started.RunID != info.RunID {
		t.Fatalf("recovered identity started=%#v calls=%#v", started, got)
	}
}

func TestCallbackManagerAdmissionErrorRetriesSameReservedIdentity(t *testing.T) {
	store := newRetrySQLiteStore(t, filepath.Join(t.TempDir(), "callbacks.db"))
	starter := &callbackAdmissionStarter{errs: []error{errors.New("lost after admission")}}
	mgr := NewCallbackManager(starter, WithCallbackStore(store))
	mgr.retryBase = 5 * time.Millisecond
	defer mgr.Shutdown()
	info, err := mgr.Set(setReq("conv", MinCallbackDelay, "continue"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Update(context.Background(), CallbackInfo{ID: info.ID, State: CallbackStatePending, FiresAt: time.Now().Add(-time.Second)}); err != nil {
		t.Fatal(err)
	}
	mgr.fire(info.ID)
	waitForCallbackState(t, store, info.ID, CallbackStateStarted)
	ids := starter.calls()
	if len(ids) != 2 || ids[0] != info.RunID || ids[1] != info.RunID {
		t.Fatalf("admission IDs = %#v, want reserved identity twice", ids)
	}
}

func TestCallbackManagerPermanentAdmissionFailureIsTerminalAndSafe(t *testing.T) {
	store := newRetrySQLiteStore(t, filepath.Join(t.TempDir(), "callbacks.db"))
	starter := &callbackAdmissionStarter{errs: []error{&CallbackStartError{Err: errors.New("secret=do-not-store"), Retry: false, Summary: "invalid callback scope"}}}
	mgr := NewCallbackManager(starter, WithCallbackStore(store))
	defer mgr.Shutdown()
	info, err := mgr.Set(setReq("conv", MinCallbackDelay, "continue"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Update(context.Background(), CallbackInfo{ID: info.ID, State: CallbackStatePending, FiresAt: time.Now().Add(-time.Second)}); err != nil {
		t.Fatal(err)
	}
	mgr.fire(info.ID)
	failed := waitForCallbackState(t, store, info.ID, CallbackStateFailed)
	if failed.LastError != "invalid callback scope" || len(starter.calls()) != 1 {
		t.Fatalf("failed = %#v calls=%#v", failed, starter.calls())
	}
}

func TestCallbackManagerRejectsUntrustedClassifiedSummary(t *testing.T) {
	store := newRetrySQLiteStore(t, filepath.Join(t.TempDir(), "callbacks.db"))
	starter := &callbackAdmissionStarter{errs: []error{&CallbackStartError{
		Err: errors.New("upstream rejected api_key=sk-secret"), Retry: false,
		Summary: "Authorization: Bearer sk-secret password=hunter2",
	}}}
	mgr := NewCallbackManager(starter, WithCallbackStore(store))
	defer mgr.Shutdown()
	info, err := mgr.Set(setReq("conv", MinCallbackDelay, "continue"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Update(context.Background(), CallbackInfo{ID: info.ID, State: CallbackStatePending, FiresAt: time.Now().Add(-time.Second)}); err != nil {
		t.Fatal(err)
	}
	mgr.fire(info.ID)
	failed := waitForCallbackState(t, store, info.ID, CallbackStateFailed)
	if failed.LastError != "callback admission failed" || strings.Contains(failed.LastError, "secret") || strings.Contains(failed.LastError, "hunter2") {
		t.Fatalf("unsafe classified summary persisted: %#v", failed)
	}
}

func TestCallbackManagerRetryExhaustionIsTerminal(t *testing.T) {
	store := newRetrySQLiteStore(t, filepath.Join(t.TempDir(), "callbacks.db"))
	starter := &callbackAdmissionStarter{errs: []error{errors.New("one"), errors.New("two"), errors.New("three")}}
	mgr := NewCallbackManager(starter, WithCallbackStore(store))
	mgr.retryBase = time.Millisecond
	defer mgr.Shutdown()
	info, err := mgr.Set(setReq("conv", MinCallbackDelay, "continue"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Update(context.Background(), CallbackInfo{ID: info.ID, State: CallbackStatePending, FiresAt: time.Now().Add(-time.Second)}); err != nil {
		t.Fatal(err)
	}
	mgr.fire(info.ID)
	failed := waitForCallbackState(t, store, info.ID, CallbackStateFailed)
	if failed.Attempt != 3 || failed.LastError != "callback admission unavailable" || len(starter.calls()) != 3 {
		t.Fatalf("exhausted = %#v calls=%#v", failed, starter.calls())
	}
}

func TestCallbackManagerRefreshesDurableStateAfterLeaseLoss(t *testing.T) {
	store := newRetrySQLiteStore(t, filepath.Join(t.TempDir(), "callbacks.db"))
	starter := &leaseStealingCallbackStarter{store: store}
	mgr := NewCallbackManager(starter, WithCallbackStore(store))
	defer mgr.Shutdown()
	info, err := mgr.Set(setReq("conv", MinCallbackDelay, "continue"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Update(context.Background(), CallbackInfo{ID: info.ID, State: CallbackStatePending, FiresAt: time.Now().Add(-time.Second)}); err != nil {
		t.Fatal(err)
	}
	mgr.fire(info.ID)
	got, err := store.Get(context.Background(), info.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != CallbackStateDispatching || got.DispatchToken != "new-owner" || got.Attempt != 2 {
		t.Fatalf("lease winner = %#v", got)
	}
	mgr.mu.Lock()
	local := mgr.callbacks[info.ID].info
	mgr.mu.Unlock()
	if local.DispatchToken != "new-owner" || local.State != CallbackStateDispatching {
		t.Fatalf("local state was not refreshed: %#v", local)
	}
}

func TestCallbackStartErrorContract(t *testing.T) {
	base := errors.New("base")
	err := &CallbackStartError{Err: base, Retry: false, Summary: "safe"}
	if err.Error() != "base" || !errors.Is(err, base) {
		t.Fatalf("callback start error contract: %v", err)
	}
	if got := (&CallbackStartError{}).Error(); got != "callback admission failed" {
		t.Fatalf("empty callback error = %q", got)
	}
}

func TestCallbackManagerLifecycleEventsNeverExposeLeaseToken(t *testing.T) {
	store := newRetrySQLiteStore(t, filepath.Join(t.TempDir(), "callbacks.db"))
	starter := &callbackAdmissionStarter{errs: []error{errors.New("temporary")}}
	sink := &capturingSink{}
	mgr := NewCallbackManager(starter, WithCallbackStore(store), WithEventSink(sink))
	mgr.retryBase = 5 * time.Millisecond
	defer mgr.Shutdown()
	info, err := mgr.Set(setReq("conv", MinCallbackDelay, "continue"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Update(context.Background(), CallbackInfo{ID: info.ID, State: CallbackStatePending, FiresAt: time.Now().Add(-time.Second)}); err != nil {
		t.Fatal(err)
	}
	mgr.fire(info.ID)
	waitForCallbackState(t, store, info.ID, CallbackStateStarted)
	deadline := time.Now().Add(time.Second)
	for len(sink.byType(eventCallbackStarted)) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	var sawRetry, sawStarted bool
	for _, event := range sink.snapshot() {
		if event.Info.DispatchToken != "" || !event.Info.DispatchLeaseUntil.IsZero() {
			t.Fatalf("event exposed lease metadata: %#v", event)
		}
		if event.Event == eventCallbackRetryWait {
			sawRetry = true
		}
		if event.Event == eventCallbackStarted && event.Info.RunID == info.RunID {
			sawStarted = true
		}
	}
	if !sawRetry || !sawStarted {
		t.Fatalf("lifecycle events retry=%v started=%v events=%#v", sawRetry, sawStarted, sink.snapshot())
	}
}
