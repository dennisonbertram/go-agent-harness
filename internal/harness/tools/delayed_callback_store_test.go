package tools

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

type failingCallbackStore struct {
	mu      sync.Mutex
	updates int
	fail    int
	rows    map[string]CallbackInfo
}

func (s *failingCallbackStore) Migrate(context.Context) error { return nil }
func (s *failingCallbackStore) Close() error                  { return nil }
func (s *failingCallbackStore) Create(_ context.Context, i CallbackInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rows == nil {
		s.rows = map[string]CallbackInfo{}
	}
	s.rows[i.ID] = i
	return nil
}
func (s *failingCallbackStore) Get(_ context.Context, id string) (CallbackInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getLocked(id)
}
func (s *failingCallbackStore) getLocked(id string) (CallbackInfo, error) {
	i, ok := s.rows[id]
	if !ok {
		return CallbackInfo{}, fmt.Errorf("missing")
	}
	return i, nil
}
func (s *failingCallbackStore) Update(_ context.Context, i CallbackInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.updateLocked(i)
}
func (s *failingCallbackStore) updateLocked(i CallbackInfo) error {
	s.updates++
	if s.updates <= s.fail {
		return fmt.Errorf("injected update failure")
	}
	s.rows[i.ID] = i
	return nil
}
func (s *failingCallbackStore) ListPending(context.Context) ([]CallbackInfo, error) {
	return s.listStates(CallbackStatePending), nil
}
func (s *failingCallbackStore) ListAll(context.Context) ([]CallbackInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]CallbackInfo, 0, len(s.rows))
	for _, info := range s.rows {
		info.DispatchToken = ""
		info.DispatchLeaseUntil = time.Time{}
		out = append(out, info)
	}
	return out, nil
}
func (s *failingCallbackStore) listStates(states ...CallbackState) []CallbackInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	wanted := map[CallbackState]bool{}
	for _, state := range states {
		wanted[state] = true
	}
	var out []CallbackInfo
	for _, info := range s.rows {
		if wanted[info.State] {
			out = append(out, info)
		}
	}
	return out
}
func (s *failingCallbackStore) ClaimDue(_ context.Context, id, token string, now, until time.Time) (CallbackInfo, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	info, err := s.getLocked(id)
	if err != nil {
		return CallbackInfo{}, false, err
	}
	if info.State != CallbackStatePending && info.State != CallbackStateRetryWait {
		return info, false, nil
	}
	info.State = callbackStateDispatchingFenced
	info.NextAttemptAt = time.Time{}
	info.DispatchToken = token
	info.DispatchLeaseUntil = until
	info.Attempt++
	if err := s.updateLocked(info); err != nil {
		return CallbackInfo{}, false, err
	}
	return info, true, nil
}
func (s *failingCallbackStore) ReclaimExpired(_ context.Context, id, expectedToken, token string, now, until time.Time) (CallbackInfo, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	info, err := s.getLocked(id)
	if err != nil {
		return CallbackInfo{}, false, err
	}
	if info.State != callbackStateDispatchingFenced || info.DispatchToken != expectedToken || info.DispatchLeaseUntil.After(now) {
		return info, false, nil
	}
	info.DispatchToken = token
	info.DispatchLeaseUntil = until
	info.Attempt++
	if err := s.updateLocked(info); err != nil {
		return CallbackInfo{}, false, err
	}
	return info, true, nil
}
func (s *failingCallbackStore) ExtendLease(_ context.Context, id, token string, _ time.Time, until time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	info, err := s.getLocked(id)
	if err != nil || info.State != callbackStateDispatchingFenced || info.DispatchToken != token {
		return false, err
	}
	info.DispatchLeaseUntil = until
	s.rows[id] = info
	return true, nil
}
func (s *failingCallbackStore) ReleaseLease(_ context.Context, id, token string, next time.Time, summary string) error {
	return s.finishDispatch(id, token, CallbackStateRetryWait, "", next, "")
}
func (s *failingCallbackStore) RecoverExpiredLease(_ context.Context, id, expectedToken string, now time.Time) (CallbackInfo, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	info, err := s.getLocked(id)
	if err != nil {
		return CallbackInfo{}, false, err
	}
	if info.State != callbackStateDispatchingFenced || info.DispatchToken != expectedToken || info.DispatchLeaseUntil.After(now) {
		return info, false, nil
	}
	info.State = CallbackStateRetryWait
	info.NextAttemptAt = now
	info.DispatchToken = ""
	info.DispatchLeaseUntil = time.Time{}
	s.rows[id] = info
	return info, true, nil
}
func (s *failingCallbackStore) MarkStarted(_ context.Context, id, token, runID string) error {
	return s.finishDispatch(id, token, CallbackStateStarted, runID, time.Time{}, "")
}
func (s *failingCallbackStore) MarkRetry(_ context.Context, id, token string, next time.Time, summary string) error {
	return s.finishDispatch(id, token, CallbackStateRetryWait, "", next, SafeCallbackErrorSummary(summary))
}
func (s *failingCallbackStore) MarkFailed(_ context.Context, id, token, summary string) error {
	return s.finishDispatch(id, token, CallbackStateFailed, "", time.Time{}, SafeCallbackErrorSummary(summary))
}
func (s *failingCallbackStore) finishDispatch(id, token string, state CallbackState, runID string, next time.Time, summary string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	info, err := s.getLocked(id)
	if err != nil {
		return err
	}
	if info.State != callbackStateDispatchingFenced || info.DispatchToken != token {
		return fmt.Errorf("callback %s dispatch lease lost", id)
	}
	info.State = state
	if runID != "" {
		info.RunID = runID
	}
	info.NextAttemptAt = next
	info.LastError = summary
	info.DispatchToken = ""
	info.DispatchLeaseUntil = time.Time{}
	s.rows[id] = info
	return nil
}
func (s *failingCallbackStore) CancelPending(_ context.Context, id string) (CallbackInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	info, err := s.getLocked(id)
	if err != nil {
		return CallbackInfo{}, err
	}
	if info.State != CallbackStatePending && info.State != CallbackStateRetryWait {
		return CallbackInfo{}, fmt.Errorf("callback %s cannot be canceled", id)
	}
	info.State = CallbackStateCanceled
	if err := s.updateLocked(info); err != nil {
		return CallbackInfo{}, err
	}
	return info, nil
}

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
	want := CallbackInfo{ID: "cb-1", ConversationID: "conv", TenantID: "tenant", AgentID: "agent", Prompt: "hello", Delay: "5s", State: CallbackStatePending, FiresAt: time.Now().Add(time.Minute), CreatedAt: time.Now(), Model: "fixture-model", ProviderName: "missing-primary", AllowFallback: true, FallbackProviders: []string{"secondary", "tertiary"}}
	if err := store.Create(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(ctx, want.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.TenantID != want.TenantID || got.AgentID != want.AgentID || got.ConversationID != want.ConversationID || got.Prompt != want.Prompt || got.Model != want.Model || got.ProviderName != want.ProviderName || got.AllowFallback != want.AllowFallback || !slices.Equal(got.FallbackProviders, want.FallbackProviders) {
		t.Fatalf("round trip = %#v", got)
	}
	pending, err := store.ListPending(ctx)
	if err != nil || len(pending) != 1 || pending[0].ID != want.ID {
		t.Fatalf("pending = %#v, err=%v", pending, err)
	}
}

func TestCallbackSQLiteStoreClaimFencesDuplicateAndStaleToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "callbacks.db")
	first, err := NewSQLiteCallbackStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := NewSQLiteCallbackStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	ctx := context.Background()
	if err := first.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := first.Create(ctx, CallbackInfo{ID: "claim", ConversationID: "c", Prompt: "p", Delay: "5s", State: CallbackStatePending, FiresAt: now.Add(-time.Second), CreatedAt: now, RunID: "run_callback_claim"}); err != nil {
		t.Fatal(err)
	}
	a, wonA, err := first.ClaimDue(ctx, "claim", "owner-a", now, now.Add(time.Minute))
	if err != nil || !wonA {
		t.Fatalf("first claim=%#v won=%v err=%v", a, wonA, err)
	}
	if a.DispatchToken != "owner-a" {
		t.Fatalf("first claim returned unverified token %q", a.DispatchToken)
	}
	b, wonB, err := second.ClaimDue(ctx, "claim", "owner-b", now, now.Add(time.Minute))
	if err != nil || wonB || b.DispatchToken != "owner-a" {
		t.Fatalf("duplicate callback=%#v won=%v err=%v", b, wonB, err)
	}
	if err := first.MarkStarted(ctx, "claim", a.DispatchToken, "run_callback_claim"); err != nil {
		t.Fatal(err)
	}
	if err := second.MarkStarted(ctx, "claim", "stale-token", "run_callback_claim"); err == nil {
		t.Fatal("stale token committed started")
	}
}

// TestSQLiteCallbackStoreRecoveryRejectsStaleObservedToken is the recovery
// half of token atomicity. A recovery pass that observed owner A must not clear
// a later owner B between its read and mutation.
func TestSQLiteCallbackStoreRecoveryRejectsStaleObservedToken(t *testing.T) {
	store, err := NewSQLiteCallbackStore(filepath.Join(t.TempDir(), "callbacks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	info := CallbackInfo{ID: "recovery-cas", ConversationID: "conv", Prompt: "continue", Delay: "5s", State: callbackStateDispatchingFenced, FiresAt: now.Add(-time.Minute), CreatedAt: now.Add(-time.Minute), RunID: "run_callback_recovery-cas", Attempt: 1, DispatchToken: "observed-owner", DispatchLeaseUntil: now.Add(-time.Second)}
	if err := store.Create(ctx, info); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE delayed_callbacks SET state='dispatching_fenced',dispatch_token=?,dispatch_lease_until=? WHERE id=?`, info.DispatchToken, info.DispatchLeaseUntil, info.ID); err != nil {
		t.Fatal(err)
	}
	observed, err := store.Get(ctx, info.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE delayed_callbacks SET dispatch_token='replacement-owner' WHERE id=?`, info.ID); err != nil {
		t.Fatal(err)
	}
	_, released, err := store.RecoverExpiredLease(ctx, info.ID, observed.DispatchToken, now)
	if err != nil {
		t.Fatal(err)
	}
	if released {
		t.Fatalf("stale recovery for %q cleared a replacement owner", observed.DispatchToken)
	}
	got, err := store.Get(ctx, info.ID)
	if err != nil || got.State != callbackStateDispatchingFenced || got.DispatchToken != "replacement-owner" {
		t.Fatalf("replacement owner changed: %#v err=%v", got, err)
	}
}

func TestCallbackManagerListNormalizesPrivateDispatchState(t *testing.T) {
	store, err := NewSQLiteCallbackStore(filepath.Join(t.TempDir(), "callbacks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	info := CallbackInfo{ID: "public-dispatch", ConversationID: "conv", Prompt: "continue", Delay: "5s", State: CallbackStatePending, FiresAt: now.Add(-time.Second), CreatedAt: now, RunID: "run_callback_public-dispatch"}
	if err := store.Create(ctx, info); err != nil {
		t.Fatal(err)
	}
	internal, won, err := store.ClaimDue(ctx, info.ID, "private-owner", now, now.Add(time.Minute))
	if err != nil || !won || internal.State != callbackStateDispatchingFenced {
		t.Fatalf("internal claim=%#v won=%v err=%v", internal, won, err)
	}
	mgr := NewCallbackManager(&callbackAdmissionStarter{}, WithCallbackStore(store))
	defer mgr.Shutdown()
	listed, err := mgr.ListAllCallbacks(ctx)
	if err != nil || len(listed) != 1 {
		t.Fatalf("listed=%#v err=%v", listed, err)
	}
	if listed[0].State != CallbackStateDispatching || listed[0].DispatchToken != "" || !listed[0].DispatchLeaseUntil.IsZero() {
		t.Fatalf("private dispatch ownership escaped API: %#v", listed[0])
	}
}

// TestCallbackSQLiteStoreConfiguresEveryPooledConnection guards #1106's
// connection-local SQLite setup. Holding the first connection open forces the
// second PRAGMA read through a different physical pooled connection.
func TestCallbackSQLiteStoreConfiguresEveryPooledConnection(t *testing.T) {
	s, err := NewSQLiteCallbackStore(filepath.Join(t.TempDir(), "callbacks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.db.SetMaxOpenConns(2)
	ctx := context.Background()
	first, err := s.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := s.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	for name, conn := range map[string]*sql.Conn{"first": first, "second": second} {
		var busy int
		if err := conn.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busy); err != nil || busy != 5000 {
			t.Fatalf("%s busy_timeout=%d err=%v, want 5000", name, busy, err)
		}
		var journal string
		if err := conn.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journal); err != nil || strings.ToLower(journal) != "wal" {
			t.Fatalf("%s journal_mode=%q err=%v, want wal", name, journal, err)
		}
	}
}

// TestCallbackSQLiteStorePreservesQuestionMarkPath ensures pragma parameters
// are encoded as SQLite URI query values rather than turning a literal '?' in
// a workspace database filename into an accidental DSN delimiter.
func TestCallbackSQLiteStorePreservesQuestionMarkPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "callbacks?workspace.db")
	s, err := NewSQLiteCallbackStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	info := CallbackInfo{ID: "literal-question", ConversationID: "c", Prompt: "p", Delay: "5s", State: CallbackStatePending, FiresAt: time.Now().Add(time.Minute), CreatedAt: time.Now(), RunID: "run_callback_literal-question"}
	if err := s.Create(context.Background(), info); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("literal question-mark database path was not created: %v", err)
	}
	got, err := s.Get(context.Background(), info.ID)
	if err != nil || got.ID != info.ID {
		t.Fatalf("round trip from literal question-mark path=%#v err=%v", got, err)
	}
}

func TestCallbackSQLiteStoreFilesystemPathsRoundTripAndPoolConfig(t *testing.T) {
	root := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	for _, path := range []string{
		filepath.Join("relative", "callbacks.db"),
		filepath.Join(root, "absolute.db"),
		filepath.Join(root, "literal?question.db"),
		`C:\callbacks?windows-like.db`,
	} {
		t.Run(path, func(t *testing.T) {
			expected, err := filepath.Abs(path)
			if err != nil {
				t.Fatal(err)
			}
			s, err := NewSQLiteCallbackStore(path)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			if err := s.Migrate(context.Background()); err != nil {
				t.Fatal(err)
			}
			info := CallbackInfo{ID: "path-" + strings.ReplaceAll(path, "/", "-"), ConversationID: "c", Prompt: "p", Delay: "5s", State: CallbackStatePending, FiresAt: time.Now().Add(time.Minute), CreatedAt: time.Now(), RunID: "run_callback_path"}
			if err := s.Create(context.Background(), info); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(expected); err != nil {
				t.Fatalf("physical database %q: %v", expected, err)
			}
			if got, err := s.Get(context.Background(), info.ID); err != nil || got.ID != info.ID {
				t.Fatalf("round trip=%#v err=%v", got, err)
			}
			s.db.SetMaxOpenConns(2)
			first, err := s.db.Conn(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer first.Close()
			second, err := s.db.Conn(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer second.Close()
			for _, conn := range []*sql.Conn{first, second} {
				var busy int
				if err := conn.QueryRowContext(context.Background(), "PRAGMA busy_timeout").Scan(&busy); err != nil || busy != 5000 {
					t.Fatalf("busy_timeout=%d err=%v", busy, err)
				}
			}
		})
	}
}

func TestCallbackSQLiteStorePendingDoesNotClaimBeforeFiresAt(t *testing.T) {
	s := newRetrySQLiteStore(t, filepath.Join(t.TempDir(), "callbacks.db"))
	now := time.Now().UTC()
	info := CallbackInfo{ID: "future", ConversationID: "c", Prompt: "p", Delay: "5s", State: CallbackStatePending, FiresAt: now.Add(time.Minute), CreatedAt: now, RunID: "run_callback_future"}
	if err := s.Create(context.Background(), info); err != nil {
		t.Fatal(err)
	}
	got, won, err := s.ClaimDue(context.Background(), info.ID, "early", now, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if won || got.State != CallbackStatePending {
		t.Fatalf("early claim won=%v callback=%#v", won, got)
	}
}

func TestCallbackSQLiteStoreRetryFailureAndCancelStateFences(t *testing.T) {
	s, err := NewSQLiteCallbackStore(filepath.Join(t.TempDir(), "callbacks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := s.Create(ctx, CallbackInfo{ID: "retry", ConversationID: "c", Prompt: "p", Delay: "5s", State: CallbackStatePending, FiresAt: now.Add(-time.Second), CreatedAt: now, RunID: "run_callback_retry"}); err != nil {
		t.Fatal(err)
	}
	claimed, won, err := s.ClaimDue(ctx, "retry", "token", now, now.Add(time.Second))
	if err != nil || !won {
		t.Fatal(err)
	}
	if err := s.MarkRetry(ctx, "retry", claimed.DispatchToken, now.Add(time.Minute), "temporary"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get(ctx, "retry"); got.State != CallbackStateRetryWait || got.Attempt != 1 || got.LastError != "callback admission failed" {
		t.Fatalf("retry=%#v", got)
	}
	if _, err := s.CancelPending(ctx, "retry"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get(ctx, "retry"); got.State != CallbackStateCanceled {
		t.Fatalf("cancel=%#v", got)
	}
}

func TestCallbackSQLiteStoreExpiredLeaseTakeoverRejectsOldCompletion(t *testing.T) {
	s, err := NewSQLiteCallbackStore(filepath.Join(t.TempDir(), "callbacks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := s.Create(ctx, CallbackInfo{ID: "lease", ConversationID: "c", Prompt: "p", Delay: "5s", State: CallbackStatePending, FiresAt: now.Add(-time.Second), CreatedAt: now, RunID: "run_callback_lease"}); err != nil {
		t.Fatal(err)
	}
	first, won, err := s.ClaimDue(ctx, "lease", "old", now, now.Add(time.Second))
	if err != nil || !won {
		t.Fatal(err)
	}
	stale, won, err := s.ReclaimExpired(ctx, "lease", "stale-observation", "stale-winner", now.Add(2*time.Second), now.Add(3*time.Second))
	if err != nil || won || stale.DispatchToken != first.DispatchToken {
		t.Fatalf("stale expected-token reclaim=%#v won=%v err=%v", stale, won, err)
	}
	second, won, err := s.ReclaimExpired(ctx, "lease", first.DispatchToken, "new", now.Add(2*time.Second), now.Add(3*time.Second))
	if err != nil || !won {
		t.Fatalf("reclaim=%#v won=%v err=%v", second, won, err)
	}
	if err := s.MarkStarted(ctx, "lease", first.DispatchToken, "run_callback_lease"); err == nil {
		t.Fatal("stale owner completed")
	}
}

func TestCallbackSQLiteStoreMarkFailedBoundsSummary(t *testing.T) {
	s, err := NewSQLiteCallbackStore(filepath.Join(t.TempDir(), "callbacks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := s.Create(ctx, CallbackInfo{ID: "failed", ConversationID: "c", Prompt: "p", Delay: "5s", State: CallbackStatePending, FiresAt: now.Add(-time.Second), CreatedAt: now, RunID: "run_callback_failed"}); err != nil {
		t.Fatal(err)
	}
	claimed, won, err := s.ClaimDue(ctx, "failed", "owner", now, now.Add(time.Minute))
	if err != nil || !won {
		t.Fatal(err)
	}
	if err := s.MarkFailed(ctx, "failed", claimed.DispatchToken, "provider secret=abc "+string(make([]byte, 2048))); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, "failed")
	if err != nil {
		t.Fatal(err)
	}
	if got.State != CallbackStateFailed || got.LastError != "callback admission failed" || strings.Contains(got.LastError, "secret") {
		t.Fatalf("failed=%#v", got)
	}
}

func TestCallbackSQLiteStoreMarkRetryBoundsSummary(t *testing.T) {
	s, err := NewSQLiteCallbackStore(filepath.Join(t.TempDir(), "callbacks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := s.Create(ctx, CallbackInfo{ID: "retry-bounded", ConversationID: "c", Prompt: "p", Delay: "5s", State: CallbackStatePending, FiresAt: now.Add(-time.Second), CreatedAt: now, RunID: "run_callback_retry-bounded"}); err != nil {
		t.Fatal(err)
	}
	claimed, won, err := s.ClaimDue(ctx, "retry-bounded", "owner", now, now.Add(time.Minute))
	if err != nil || !won {
		t.Fatalf("claim won=%v err=%v", won, err)
	}
	if err := s.MarkRetry(ctx, claimed.ID, claimed.DispatchToken, now.Add(time.Minute), "Authorization: Bearer sk-live-secret "+strings.Repeat("x", 300)); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, claimed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.LastError != "callback admission failed" || strings.Contains(got.LastError, "sk-live-secret") {
		t.Fatalf("retry summary = %q, want safe fallback", got.LastError)
	}
}

func TestCallbackSQLiteStoreMigrates1005RowWithReservedRunID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "callbacks.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE delayed_callbacks (id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL DEFAULT '', agent_id TEXT NOT NULL DEFAULT '', conversation_id TEXT NOT NULL, prompt TEXT NOT NULL, delay TEXT NOT NULL, fires_at TIMESTAMP NOT NULL, state TEXT NOT NULL, created_at TIMESTAMP NOT NULL, updated_at TIMESTAMP NOT NULL, attempt INTEGER NOT NULL DEFAULT 0, run_id TEXT NOT NULL DEFAULT '')`)
	if err != nil {
		t.Fatal(err)
	}
	// #1005 wrote local-zone timestamps. A raw +02 value compares after the
	// equivalent UTC timestamp lexically, so #1006 must normalize legacy rows
	// before its atomic due-time predicates run.
	now := time.Now().UTC()
	legacyDue := now.Add(-time.Minute).In(time.Local)
	if _, err = db.Exec(`INSERT INTO delayed_callbacks(id,conversation_id,prompt,delay,fires_at,state,created_at,updated_at) VALUES('legacy','c','p','5s',?,'pending',?,?)`, legacyDue, legacyDue, legacyDue); err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err := NewSQLiteCallbackStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(context.Background(), "legacy")
	if err != nil {
		t.Fatal(err)
	}
	if got.RunID != "run_callback_legacy" || got.State != CallbackStatePending {
		t.Fatalf("migrated=%#v", got)
	}
	if got.Model != "" || got.ProviderName != "" || got.AllowFallback || len(got.FallbackProviders) != 0 {
		t.Fatalf("legacy routing defaults = %#v", got)
	}
	claimed, won, err := s.ClaimDue(context.Background(), got.ID, "owner", now, now.Add(time.Minute))
	if err != nil || !won || claimed.State != callbackStateDispatchingFenced {
		t.Fatalf("legacy due claim won=%v callback=%#v err=%v", won, claimed, err)
	}
}

func TestParseStoredCallbackTimeSupportsLegacyDriverValues(t *testing.T) {
	want := time.Date(2026, time.August, 3, 1, 2, 3, 456000000, time.UTC)
	for name, value := range map[string]any{
		"time":   want,
		"string": "2026-08-03 01:02:03.456 +0000 UTC",
		"bytes":  []byte("2026-08-03 01:02:03.456"),
	} {
		t.Run(name, func(t *testing.T) {
			got, err := parseStoredCallbackTime(value)
			if err != nil || !got.Equal(want) {
				t.Fatalf("parse = %v, %v; want %v", got, err, want)
			}
		})
	}
	if _, err := parseStoredCallbackTime(nil); err == nil {
		t.Fatal("required NULL timestamp was accepted")
	}
	if got, err := parseOptionalStoredCallbackTime(nil); err != nil || got != nil {
		t.Fatalf("optional NULL = %v, %v", got, err)
	}
	if _, err := parseStoredCallbackTime(42); err == nil {
		t.Fatal("unsupported timestamp type was accepted")
	}
	if _, err := parseStoredCallbackTime("not-a-time"); err == nil {
		t.Fatal("invalid timestamp text was accepted")
	}
}

func TestCallbackSQLiteStoreListAllReturnsSafeRetryStatus(t *testing.T) {
	s, err := NewSQLiteCallbackStore(filepath.Join(t.TempDir(), "callbacks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := s.Create(ctx, CallbackInfo{ID: "visible", ConversationID: "c", Prompt: "p", Delay: "5s", State: CallbackStateRetryWait, FiresAt: now, CreatedAt: now, RunID: "run_callback_visible", Attempt: 2, NextAttemptAt: now.Add(time.Minute), LastError: "temporary"}); err != nil {
		t.Fatal(err)
	}
	items, err := s.ListAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].State != CallbackStateRetryWait || items[0].RunID == "" || items[0].DispatchToken != "" || items[0].LastError != "callback admission failed" {
		t.Fatalf("items=%#v", items)
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
