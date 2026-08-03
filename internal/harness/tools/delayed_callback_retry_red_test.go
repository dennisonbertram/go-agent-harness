package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
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

func (s *transientLeaseStore) AcquireCallbackRecoveryAuthority(ctx context.Context) (func(), error) {
	if authority, ok := s.CallbackStore.(callbackRecoveryAuthority); ok {
		return authority.AcquireCallbackRecoveryAuthority(ctx)
	}
	return nil, errors.New("callback recovery authority unavailable")
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

// deadlineThenSuccessStarter blocks its first admission until the manager's
// lease-deadline context cancellation, then admits the retry using the same
// reserved run identity.
type deadlineThenSuccessStarter struct {
	mu      sync.Mutex
	ids     []string
	entered chan struct{}
	once    sync.Once
}

func (*deadlineThenSuccessStarter) StartRun(string, string, string, string) error { return nil }

func (s *deadlineThenSuccessStarter) StartCallback(ctx context.Context, info CallbackInfo) (string, error) {
	s.mu.Lock()
	call := len(s.ids)
	s.ids = append(s.ids, info.RunID)
	s.mu.Unlock()
	if call == 0 {
		s.once.Do(func() { close(s.entered) })
		<-ctx.Done()
		return "", ctx.Err()
	}
	return info.RunID, nil
}

func (s *deadlineThenSuccessStarter) calls() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.ids...)
}

type orderedTakeoverStarter struct {
	oldCanceled <-chan time.Time
	entered     chan struct{}
	release     chan struct{}
	mu          sync.Mutex
	ids         []string
	premature   bool
}

func (*orderedTakeoverStarter) StartRun(string, string, string, string) error { return nil }

func (s *orderedTakeoverStarter) StartCallback(ctx context.Context, info CallbackInfo) (string, error) {
	s.mu.Lock()
	select {
	case <-s.oldCanceled:
	default:
		s.premature = true
	}
	s.ids = append(s.ids, info.RunID)
	s.mu.Unlock()
	close(s.entered)
	select {
	case <-s.release:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	return info.RunID, nil
}

func (s *orderedTakeoverStarter) snapshot() ([]string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.ids...), s.premature
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

func (s *blockingLeaseStore) AcquireCallbackRecoveryAuthority(ctx context.Context) (func(), error) {
	if authority, ok := s.CallbackStore.(callbackRecoveryAuthority); ok {
		return authority.AcquireCallbackRecoveryAuthority(ctx)
	}
	return nil, errors.New("callback recovery authority unavailable")
}

func (s *blockingLeaseStore) ExtendLease(ctx context.Context, _ string, _ string, _ time.Time, _ time.Time) (bool, error) {
	s.once.Do(func() { close(s.entered) })
	<-ctx.Done()
	s.deadlineOnce.Do(func() { close(s.deadlineReached) })
	<-s.release
	return false, ctx.Err()
}

func (s *blockingLeaseStore) unblock() { s.releaseOnce.Do(func() { close(s.release) }) }

// releaseObservingStore makes the durable handoff observable to the contender
// test.  The contender is armed before the old lease expires: seeing the old
// context cancel is insufficient, because its StartCallback can still be
// unwinding.  A replacement may only admit after the old token has durably
// released the row.
type releaseObservingStore struct {
	CallbackStore
	once     sync.Once
	released chan struct{}
}

func (s *releaseObservingStore) AcquireCallbackRecoveryAuthority(ctx context.Context) (func(), error) {
	if authority, ok := s.CallbackStore.(callbackRecoveryAuthority); ok {
		return authority.AcquireCallbackRecoveryAuthority(ctx)
	}
	return nil, errors.New("callback recovery authority unavailable")
}

func (s *releaseObservingStore) ReleaseLease(ctx context.Context, id, token string, next time.Time) error {
	err := s.CallbackStore.ReleaseLease(ctx, id, token, next)
	if err == nil {
		s.once.Do(func() { close(s.released) })
	}
	return err
}

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
	if err := second.Recover(context.Background()); err == nil {
		t.Fatal("second manager unexpectedly recovered a live workspace")
	}
	// Wait beyond the original lease. The first manager's heartbeat must keep
	// the claim live; the second bootstrap cannot obtain recovery authority.
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
	first.retryBase = time.Millisecond
	defer first.Shutdown()
	second := NewCallbackManager(starter, WithCallbackStore(storeB))
	second.leaseTime = 90 * time.Millisecond
	defer second.Shutdown()
	if err := first.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitForCallbackAdmission(t, starter.entered)
	if err := second.Recover(context.Background()); err == nil {
		t.Fatal("second manager unexpectedly recovered a live workspace")
	}
	// This extends beyond the original lease. The first heartbeat initially
	// reports busy; it must retry before the last confirmed deadline rather
	// than cancel and leave a reclaimable duplicate run.
	time.Sleep(150 * time.Millisecond)
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
	// Stop the former owner before testing recovery takeover. Its canceled
	// admission durably releases into retry_wait, so a new manager claims the
	// next token rather than stealing a live dispatching row.
	first.Shutdown()
	released := waitForCallbackState(t, baseA, info.ID, CallbackStateRetryWait)
	claimNow := released.NextAttemptAt.Add(time.Millisecond)
	claimed, won, err := storeB.ClaimDue(context.Background(), info.ID, "takeover", claimNow, claimNow.Add(time.Minute))
	if err != nil || !won || claimed.DispatchToken != "takeover" {
		t.Fatalf("post-deadline takeover callback=%#v won=%v err=%v", claimed, won, err)
	}
	if err := storeB.MarkStarted(context.Background(), info.ID, "takeover", info.RunID); err != nil {
		t.Fatalf("takeover completion: %v", err)
	}
}

// TestCallbackManagerDeadlineReleaseRearmsSingleOwner is the liveness half of
// the live-owner handoff: when a deadline-cancelled admission releases its
// token into retry_wait, the same (and only) manager must schedule that retry.
// Requiring a second daemon merely to re-arm a callback strands ordinary
// harnessd deployments forever.
func TestCallbackManagerDeadlineReleaseRearmsSingleOwner(t *testing.T) {
	store := newRetrySQLiteStore(t, filepath.Join(t.TempDir(), "callbacks.db"))
	now := time.Now().UTC()
	info := CallbackInfo{ID: "single-owner-release", ConversationID: "conv", Prompt: "continue", Delay: "5s", State: CallbackStatePending, FiresAt: now.Add(-time.Second), CreatedAt: now, RunID: "run_callback_single-owner-release"}
	if err := store.Create(context.Background(), info); err != nil {
		t.Fatal(err)
	}
	starter := &deadlineThenSuccessStarter{entered: make(chan struct{})}
	blocking := &blockingLeaseStore{CallbackStore: store, entered: make(chan struct{}), deadlineReached: make(chan struct{}), release: make(chan struct{})}
	mgr := NewCallbackManager(starter, WithCallbackStore(blocking))
	mgr.leaseTime = 40 * time.Millisecond
	mgr.retryBase = time.Millisecond
	t.Cleanup(blocking.unblock)
	t.Cleanup(mgr.Shutdown)
	if err := mgr.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitForCallbackAdmission(t, starter.entered)
	select {
	case <-blocking.deadlineReached:
	case <-time.After(time.Second):
		t.Fatal("blocked renewal never reached its lease deadline")
	}
	// Let the deadline-cancelled first call return. The manager itself must
	// re-arm retry_wait; no second manager is constructed in this regression.
	blocking.unblock()
	started := waitForCallbackState(t, store, info.ID, CallbackStateStarted)
	if got := starter.calls(); len(got) != 2 || got[0] != info.RunID || got[1] != info.RunID || started.Attempt != 2 {
		t.Fatalf("single-manager handoff calls=%#v state=%#v", got, started)
	}
}

// TestCallbackManagerRecoveryRequiresExclusiveWorkspaceAuthority proves that
// wall-clock expiry is not crash evidence. A live manager keeps the workspace
// recovery lock even after a callback lease expires; a second bootstrap must
// fail closed instead of converting the row into retry_wait and admitting a
// duplicate continuation.
func TestCallbackManagerRecoveryRequiresExclusiveWorkspaceAuthority(t *testing.T) {
	path := filepath.Join(t.TempDir(), "callbacks.db")
	firstStore := newRetrySQLiteStore(t, path)
	secondStore := newRetrySQLiteStore(t, path)
	now := time.Now().UTC()
	info := CallbackInfo{ID: "live-owner-recovery", ConversationID: "conv", Prompt: "continue", Delay: "5s", State: CallbackStatePending, FiresAt: now.Add(-time.Second), CreatedAt: now, RunID: "run_callback_live-owner-recovery"}
	if err := firstStore.Create(context.Background(), info); err != nil {
		t.Fatal(err)
	}
	firstStarter := &cancellationAwareCallbackStarter{entered: make(chan struct{}), canceled: make(chan time.Time, 1)}
	first := NewCallbackManager(firstStarter, WithCallbackStore(firstStore))
	first.leaseTime = 40 * time.Millisecond
	t.Cleanup(first.Shutdown)
	if err := first.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitForCallbackAdmission(t, firstStarter.entered)
	// Wait beyond the live callback lease. This remains a live process: its
	// StartCallback has not returned and it still owns the workspace fence.
	time.Sleep(60 * time.Millisecond)
	secondStarter := &callbackAdmissionStarter{}
	second := NewCallbackManager(secondStarter, WithCallbackStore(secondStore))
	t.Cleanup(second.Shutdown)
	if err := second.Recover(context.Background()); err == nil {
		t.Fatal("second live bootstrap recovered an expired row without workspace authority")
	}
	select {
	case got := <-secondStarter.entered:
		t.Fatalf("second bootstrap admitted callback %v", got)
	case <-time.After(30 * time.Millisecond):
	}
	if calls := secondStarter.calls(); len(calls) != 0 {
		t.Fatalf("second bootstrap admitted callback %#v", calls)
	}
}

// TestCallbackManagerRecoveryReclaimsLegacyNullLease keeps rows created by
// pre-lease migrations recoverable. NULL means an abandoned dispatch with no
// heartbeat metadata, not an eternal invisible lease.
func TestCallbackManagerRecoveryReclaimsLegacyNullLease(t *testing.T) {
	store := newRetrySQLiteStore(t, filepath.Join(t.TempDir(), "callbacks.db"))
	now := time.Now().UTC()
	info := CallbackInfo{ID: "legacy-null-lease", ConversationID: "conv", Prompt: "continue", Delay: "5s", State: CallbackStateDispatching, FiresAt: now.Add(-time.Minute), CreatedAt: now.Add(-time.Minute), RunID: "run_callback_legacy-null-lease", Attempt: 1, DispatchToken: "legacy"}
	if err := store.Create(context.Background(), info); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(context.Background(), `UPDATE delayed_callbacks SET dispatch_token=?,dispatch_lease_until=NULL WHERE id=?`, info.DispatchToken, info.ID); err != nil {
		t.Fatal(err)
	}
	starter := &callbackAdmissionStarter{}
	mgr := NewCallbackManager(starter, WithCallbackStore(store))
	defer mgr.Shutdown()
	if err := mgr.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	started := waitForCallbackState(t, store, info.ID, CallbackStateStarted)
	if calls := starter.calls(); len(calls) != 1 || calls[0] != info.RunID || started.Attempt != 2 {
		t.Fatalf("legacy NULL recovery calls=%#v state=%#v", calls, started)
	}
}

// TestCallbackManagerRecoveryReclaimsAfterFutureLeaseExpiry proves the crash
// path does not become a permanent dispatching poll. Bootstrap may see a
// future lease left by a dead process; when its timer reaches that deadline,
// the already-authorized manager must atomically turn it into retry work.
func TestCallbackManagerRecoveryReclaimsAfterFutureLeaseExpiry(t *testing.T) {
	store := newRetrySQLiteStore(t, filepath.Join(t.TempDir(), "callbacks.db"))
	now := time.Now().UTC()
	info := CallbackInfo{ID: "future-crash-lease", ConversationID: "conv", Prompt: "continue", Delay: "5s", State: CallbackStateDispatching, FiresAt: now.Add(-time.Minute), CreatedAt: now.Add(-time.Minute), RunID: "run_callback_future-crash-lease", Attempt: 1, DispatchToken: "crashed-owner", DispatchLeaseUntil: now.Add(40 * time.Millisecond)}
	if err := store.Create(context.Background(), info); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(context.Background(), `UPDATE delayed_callbacks SET dispatch_token=?,dispatch_lease_until=? WHERE id=?`, info.DispatchToken, info.DispatchLeaseUntil, info.ID); err != nil {
		t.Fatal(err)
	}
	starter := &callbackAdmissionStarter{}
	mgr := NewCallbackManager(starter, WithCallbackStore(store))
	defer mgr.Shutdown()
	if err := mgr.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	started := waitForCallbackState(t, store, info.ID, CallbackStateStarted)
	if calls := starter.calls(); len(calls) != 1 || calls[0] != info.RunID || started.Attempt != 2 {
		t.Fatalf("future expired lease was not recovered once: calls=%#v state=%#v", calls, started)
	}
}

// TestCallbackManagerDeadlineReleaseHonorsAttemptBound prevents the safe
// same-manager rearm from becoming an unbounded retry loop. A cancellation at
// the configured maximum is terminal and retains the same safe failure state.
func TestCallbackManagerDeadlineReleaseHonorsAttemptBound(t *testing.T) {
	store := newRetrySQLiteStore(t, filepath.Join(t.TempDir(), "callbacks.db"))
	now := time.Now().UTC()
	info := CallbackInfo{ID: "deadline-attempt-bound", ConversationID: "conv", Prompt: "continue", Delay: "5s", State: CallbackStatePending, FiresAt: now.Add(-time.Second), CreatedAt: now, RunID: "run_callback_deadline-attempt-bound"}
	if err := store.Create(context.Background(), info); err != nil {
		t.Fatal(err)
	}
	starter := &cancellationAwareCallbackStarter{entered: make(chan struct{}), canceled: make(chan time.Time, 1)}
	blocking := &blockingLeaseStore{CallbackStore: store, entered: make(chan struct{}), deadlineReached: make(chan struct{}), release: make(chan struct{})}
	mgr := NewCallbackManager(starter, WithCallbackStore(blocking))
	mgr.leaseTime = 40 * time.Millisecond
	mgr.maxAttempts = 1
	t.Cleanup(mgr.Shutdown)
	t.Cleanup(blocking.unblock)
	if err := mgr.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitForCallbackAdmission(t, starter.entered)
	select {
	case <-blocking.deadlineReached:
	case <-time.After(time.Second):
		t.Fatal("deadline did not cancel bounded admission")
	}
	blocking.unblock()
	failed := waitForCallbackState(t, store, info.ID, CallbackStateFailed)
	if !failed.NextAttemptAt.IsZero() || failed.Attempt != 1 || failed.LastError != "callback admission unavailable" {
		t.Fatalf("bounded deadline state=%#v", failed)
	}
}

func TestCallbackManagerRecoverRejectsStoreWithoutWorkspaceAuthority(t *testing.T) {
	store := &failingCallbackStore{}
	mgr := NewCallbackManager(&retryableCallbackStarter{}, WithCallbackStore(store))
	defer mgr.Shutdown()
	if err := mgr.Recover(context.Background()); !errors.Is(err, ErrCallbackRecoveryAuthorityRequired) {
		t.Fatalf("Recover error=%v, want workspace authority requirement", err)
	}
}

func TestCallbackManagerRecoverRejectsInMemorySQLiteWithoutFilesystemFence(t *testing.T) {
	store, err := NewSQLiteCallbackStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	mgr := NewCallbackManager(&callbackAdmissionStarter{}, WithCallbackStore(store))
	defer mgr.Shutdown()
	if err := mgr.Recover(context.Background()); !errors.Is(err, ErrCallbackRecoveryAuthorityRequired) {
		t.Fatalf("Recover error=%v, want filesystem authority requirement", err)
	}
}

// TestCallbackRecoveryAuthorityReleasedOnProcessDeath proves the lock is not
// merely released by a graceful Shutdown. The child is killed while holding
// the flock; the kernel must then let a replacement process acquire it.
func TestCallbackRecoveryAuthorityReleasedOnProcessDeath(t *testing.T) {
	if os.Getenv("GO_WANT_CALLBACK_LOCK_HELPER") == "1" {
		store, err := NewSQLiteCallbackStore(os.Getenv("CALLBACK_LOCK_PATH"))
		if err != nil {
			os.Exit(2)
		}
		defer store.Close()
		release, err := store.AcquireCallbackRecoveryAuthority(context.Background())
		if err != nil {
			os.Exit(3)
		}
		defer release()
		if err := os.WriteFile(os.Getenv("CALLBACK_LOCK_READY"), []byte("locked"), 0600); err != nil {
			os.Exit(4)
		}
		select {}
	}
	path := filepath.Join(t.TempDir(), "callbacks.db")
	ready := filepath.Join(t.TempDir(), "callback-lock-ready")
	child := exec.Command(os.Args[0], "-test.run=^TestCallbackRecoveryAuthorityReleasedOnProcessDeath$", "-test.v")
	child.Env = append(os.Environ(), "GO_WANT_CALLBACK_LOCK_HELPER=1", "CALLBACK_LOCK_PATH="+path, "CALLBACK_LOCK_READY="+ready)
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = child.Process.Kill(); _ = child.Wait() })
	deadline := time.Now().Add(time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("child never acquired callback recovery authority")
		}
		time.Sleep(time.Millisecond)
	}
	if err := child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := child.Wait(); err == nil {
		t.Fatal("callback lock helper unexpectedly exited cleanly")
	}
	replacement, err := NewSQLiteCallbackStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer replacement.Close()
	release, err := replacement.AcquireCallbackRecoveryAuthority(context.Background())
	if err != nil {
		t.Fatalf("kernel did not release dead process callback lock: %v", err)
	}
	release()
}

// TestCallbackManagerRecoveryReleasesOnlyAbandonedExpiredDispatch preserves
// restart recovery without letting a normally armed manager steal a live
// dispatch. Recover is the harness bootstrap boundary, after the prior process
// has been confirmed gone; it first converts an expired abandoned token into a
// regular retry claim and then admits the reserved run once.
func TestCallbackManagerRecoveryReleasesOnlyAbandonedExpiredDispatch(t *testing.T) {
	store := newRetrySQLiteStore(t, filepath.Join(t.TempDir(), "callbacks.db"))
	now := time.Now().UTC()
	info := CallbackInfo{ID: "recovery-handoff", ConversationID: "conv", Prompt: "continue", Delay: "5s", State: CallbackStateDispatching, FiresAt: now.Add(-time.Minute), CreatedAt: now.Add(-time.Minute), RunID: "run_callback_recovery-handoff", Attempt: 1, DispatchToken: "abandoned", DispatchLeaseUntil: now.Add(-time.Second)}
	if err := store.Create(context.Background(), info); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(context.Background(), `UPDATE delayed_callbacks SET dispatch_token=?,dispatch_lease_until=? WHERE id=?`, info.DispatchToken, info.DispatchLeaseUntil, info.ID); err != nil {
		t.Fatal(err)
	}
	starter := &callbackAdmissionStarter{entered: make(chan struct{}), release: make(chan struct{})}
	mgr := NewCallbackManager(starter, WithCallbackStore(store))
	defer mgr.Shutdown()
	if err := mgr.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitForCallbackAdmission(t, starter.entered)
	close(starter.release)
	got := waitForCallbackState(t, store, info.ID, CallbackStateStarted)
	if got.Attempt != 2 || got.RunID != info.RunID {
		t.Fatalf("recovered callback = %#v", got)
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
	storeA := &releaseObservingStore{CallbackStore: blocking, released: make(chan struct{})}
	first := NewCallbackManager(oldStarter, WithCallbackStore(storeA))
	first.leaseTime = 90 * time.Millisecond
	t.Cleanup(first.Shutdown)
	// Cleanups run LIFO: unblock the held renewal before Shutdown waits for its
	// dispatch goroutine when this intentionally-red test calls Fatal.
	t.Cleanup(blocking.unblock)
	if err := first.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitForCallbackAdmission(t, oldStarter.entered)
	// A second bootstrap while the original owner is still live must fail at the
	// workspace fence, even if its callback lease eventually expires.
	newStarter := &orderedTakeoverStarter{oldCanceled: oldStarter.canceled, entered: make(chan struct{}), release: make(chan struct{})}
	second := NewCallbackManager(newStarter, WithCallbackStore(storeB))
	second.leaseTime = 90 * time.Millisecond
	defer second.Shutdown()
	if err := second.Recover(context.Background()); err == nil {
		t.Fatal("second manager unexpectedly recovered a live workspace")
	}
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
	case <-storeA.released:
	case <-time.After(time.Second):
		t.Fatal("old owner did not durably release its token after canceled admission")
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
