package server

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-agent-harness/internal/harness"
	"go-agent-harness/internal/store"
)

func TestCronRunStartCacheDeduplicatesOnlyInFlightStarts(t *testing.T) {
	cache := newCronRunStartCache()
	started := make(chan struct{})
	release := make(chan struct{})
	var startCalls atomic.Int32
	start := func() (harness.Run, error) {
		if startCalls.Add(1) == 1 {
			close(started)
			<-release
		}
		return harness.Run{ID: "run-1", Status: harness.RunStatusQueued}, nil
	}

	type result struct {
		run harness.Run
		err error
	}
	results := make(chan result, 2)
	go func() {
		run, err := cache.getOrStart(context.Background(), "tenant-a", "correlation-1", "fingerprint-1", start)
		results <- result{run: run, err: err}
	}()
	<-started
	secondReady := make(chan struct{})
	go func() {
		close(secondReady)
		run, err := cache.getOrStart(context.Background(), "tenant-a", "correlation-1", "fingerprint-1", start)
		results <- result{run: run, err: err}
	}()
	<-secondReady
	// Give the second call a bounded opportunity to join the in-flight entry.
	// It cannot complete until release closes.
	select {
	case got := <-results:
		t.Fatalf("concurrent delivery completed before release: run %q, err %v", got.run.ID, got.err)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)

	for i := 0; i < 2; i++ {
		got := <-results
		if got.err != nil || got.run.ID != "run-1" {
			t.Fatalf("result %d = run %q, err %v; want run-1 without error", i, got.run.ID, got.err)
		}
	}
	if got := startCalls.Load(); got != 1 {
		t.Fatalf("start callback calls = %d, want 1", got)
	}
	cache.mu.Lock()
	entriesAfterCompletion := len(cache.entries)
	cache.mu.Unlock()
	if entriesAfterCompletion != 0 {
		t.Fatalf("completed cache entries = %d, want 0", entriesAfterCompletion)
	}

	got, err := cache.getOrStart(context.Background(), "tenant-a", "correlation-1", "fingerprint-1", start)
	if err != nil || got.ID != "run-1" {
		t.Fatalf("sequential replay = run %q, err %v; want run-1 without error", got.ID, err)
	}
	if got := startCalls.Load(); got != 2 {
		t.Fatalf("start callback calls after sequential delivery = %d, want 2 after cache eviction", got)
	}

	if _, err := cache.getOrStart(context.Background(), "tenant-a", "correlation-1", "different-fingerprint", start); err != nil {
		t.Fatalf("completed cache entry still influenced later delivery: %v", err)
	}
}

func TestCronRunStartCacheDuplicateWaiterHonorsContextCancellation(t *testing.T) {
	cache := newCronRunStartCache()
	started := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_, _ = cache.getOrStart(context.Background(), "tenant-a", "correlation-cancel", "fingerprint", func() (harness.Run, error) {
			close(started)
			<-release
			return harness.Run{ID: "run-owner"}, nil
		})
	}()
	<-started

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := cache.getOrStart(ctx, "tenant-a", "correlation-cancel", "fingerprint", func() (harness.Run, error) {
		t.Fatal("cancelled duplicate must not become the starter")
		return harness.Run{}, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled duplicate error = %v, want context.Canceled", err)
	}
	close(release)
}

type blockingCronRunCreateStore struct {
	*store.MemoryStore
	entered chan struct{}
	once    sync.Once
}

func (s *blockingCronRunCreateStore) CreateRun(ctx context.Context, run *store.Run) error {
	s.once.Do(func() { close(s.entered) })
	<-ctx.Done()
	return ctx.Err()
}

func TestCronRunLeaseLossCancelsBlockedPreAdmissionBeforeTakeoverDispatch(t *testing.T) {
	base := store.NewMemoryStore()
	blocked := &blockingCronRunCreateStore{MemoryStore: base, entered: make(chan struct{})}
	runner := testRunnerForCronStore(blocked)
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })
	now := time.Now().UTC()
	var nowMu sync.Mutex
	currentNow := now
	nowFn := func() time.Time { nowMu.Lock(); defer nowMu.Unlock(); return currentNow }
	ticks := make(chan time.Time, 1)
	s := &Server{
		runner: runner, runStore: blocked, timeNow: nowFn,
		cronRunDispatchLeaseDuration: time.Second, cronRunDispatchHeartbeatTicks: ticks,
	}
	req := cronRunRequest{Prompt: "must not double dispatch", TenantID: "tenant-lease", AgentID: "agent", ConversationID: "conversation", JobID: "job", ExecutionID: "execution", CorrelationKey: "cron/job/execution"}
	result := make(chan error, 1)
	go func() {
		_, err := s.getOrStartCronRun(context.Background(), req, req.CorrelationKey, harness.RunRequest{Prompt: req.Prompt, TenantID: req.TenantID, AgentID: req.AgentID, ConversationID: req.ConversationID})
		result <- err
	}()
	select {
	case <-blocked.entered:
	case <-time.After(time.Second):
		t.Fatal("reserved run did not enter cancellable pre-admission persistence")
	}
	nowMu.Lock()
	currentNow = now.Add(2 * time.Second)
	nowMu.Unlock()
	binding, acquired, err := base.AcquireCronRunStartDispatchLease(context.Background(), req.TenantID, req.CorrelationKey, "owner-b", currentNow, currentNow.Add(time.Second))
	if err != nil || !acquired || binding.DispatchOwner != "owner-b" {
		t.Fatalf("takeover = %+v acquired=%t err=%v", binding, acquired, err)
	}
	ticks <- currentNow
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("lease-losing pre-admission unexpectedly succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("lease loss did not cancel blocked pre-admission")
	}
	if _, exists := runner.GetRun(binding.RunID); exists {
		t.Fatal("lease-losing owner published a local run after takeover")
	}
	if _, err := base.GetRun(context.Background(), binding.RunID); !store.IsNotFound(err) {
		t.Fatalf("lease-losing owner persisted run = %v, want not found", err)
	}
}

func TestCronRunDispatchLeaseHeartbeatStopsWhenRunIsAbsent(t *testing.T) {
	now := time.Now().UTC()
	ms := store.NewMemoryStore()
	binding := seedCronRunDispatchLease(t, ms, "run_absent_heartbeat", "owner-absent", now)
	renewed := make(chan struct{}, 1)
	wrapped := &synchronizedCronRunStore{Store: ms, cronStarts: ms, renewed: renewed}
	runner := testRunnerForCronStore(wrapped)
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })
	ticks := make(chan time.Time, 1)
	stopped := make(chan string, 1)
	s := &Server{runner: runner, runStore: wrapped, timeNow: func() time.Time { return now }, cronRunDispatchHeartbeatTicks: ticks, cronRunLeaseHeartbeatStopped: stopped}
	s.ensureCronRunDispatchLeaseHeartbeat(wrapped, binding, "owner-absent")
	ticks <- now
	awaitCronRunLeaseHeartbeatStop(t, stopped)
	select {
	case <-renewed:
		t.Fatal("absent run renewed its dispatch lease")
	default:
	}
}

func TestCronRunDispatchLeaseHeartbeatStopsOnTerminalRun(t *testing.T) {
	now := time.Now().UTC()
	ms := store.NewMemoryStore()
	binding := seedCronRunDispatchLease(t, ms, "run_terminal_heartbeat", "owner-terminal", now)
	renewed := make(chan struct{}, 1)
	wrapped := &synchronizedCronRunStore{Store: ms, cronStarts: ms, renewed: renewed}
	provider := &shutdownBlockingCronProvider{started: make(chan struct{}), release: make(chan struct{})}
	runner := harness.NewRunner(provider, harness.NewRegistry(), harness.RunnerConfig{DefaultModel: "gpt-4.1-mini", MaxSteps: 1, Store: wrapped})
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })
	run, err := runner.StartRunWithID(harness.RunRequest{Prompt: "terminal heartbeat"}, binding.RunID)
	if err != nil {
		t.Fatalf("StartRunWithID: %v", err)
	}
	select {
	case <-provider.started:
	case <-time.After(time.Second):
		t.Fatal("provider did not start")
	}
	_, events, cancel, err := runner.Subscribe(run.ID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer cancel()
	ticks := make(chan time.Time, 1)
	stopped := make(chan string, 1)
	s := &Server{runner: runner, runStore: wrapped, timeNow: func() time.Time { return now }, cronRunDispatchHeartbeatTicks: ticks, cronRunLeaseHeartbeatStopped: stopped}
	s.ensureCronRunDispatchLeaseHeartbeat(wrapped, binding, "owner-terminal")
	close(provider.release)
	awaitCronRunTerminalEvent(t, events)
	ticks <- now
	awaitCronRunLeaseHeartbeatStop(t, stopped)
	select {
	case <-renewed:
		t.Fatal("terminal run renewed its dispatch lease")
	default:
	}
}

func TestCronRunDispatchLeaseHeartbeatStopsOnOwnershipLoss(t *testing.T) {
	now := time.Now().UTC()
	ms := store.NewMemoryStore()
	binding := seedCronRunDispatchLease(t, ms, "run_lost_heartbeat", "owner-a", now)
	renewed := make(chan struct{}, 1)
	wrapped := &synchronizedCronRunStore{Store: ms, cronStarts: ms, renewed: renewed}
	provider := &shutdownBlockingCronProvider{started: make(chan struct{}), release: make(chan struct{})}
	runner := harness.NewRunner(provider, harness.NewRegistry(), harness.RunnerConfig{DefaultModel: "gpt-4.1-mini", MaxSteps: 1, Store: wrapped})
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })
	t.Cleanup(func() { close(provider.release) })
	if _, err := runner.StartRunWithID(harness.RunRequest{Prompt: "ownership-loss heartbeat"}, binding.RunID); err != nil {
		t.Fatalf("StartRunWithID: %v", err)
	}
	select {
	case <-provider.started:
	case <-time.After(time.Second):
		t.Fatal("provider did not start")
	}
	if got, acquired, err := ms.AcquireCronRunStartDispatchLease(context.Background(), binding.TenantID, binding.IdempotencyKey, "owner-b", now.Add(2*time.Minute), now.Add(3*time.Minute)); err != nil || !acquired || got.DispatchOwner != "owner-b" {
		t.Fatalf("owner-b takeover = %+v acquired=%t err=%v", got, acquired, err)
	}
	ticks := make(chan time.Time, 1)
	stopped := make(chan string, 1)
	s := &Server{runner: runner, runStore: wrapped, timeNow: func() time.Time { return now.Add(2 * time.Minute) }, cronRunDispatchHeartbeatTicks: ticks, cronRunLeaseHeartbeatStopped: stopped}
	s.ensureCronRunDispatchLeaseHeartbeat(wrapped, binding, "owner-a")
	ticks <- now
	awaitCronRunLeaseHeartbeatStop(t, stopped)
	select {
	case <-renewed:
		t.Fatal("owner-lost run renewed its dispatch lease")
	default:
	}
}

func seedCronRunDispatchLease(t *testing.T, ms *store.MemoryStore, runID, owner string, now time.Time) store.CronRunStart {
	t.Helper()
	binding := store.CronRunStart{TenantID: "tenant-heartbeat", IdempotencyKey: "cron/" + runID, Fingerprint: "fingerprint-" + runID, RunID: runID, CreatedAt: now}
	if _, claimed, err := ms.ClaimCronRunStart(context.Background(), binding); err != nil || !claimed {
		t.Fatalf("ClaimCronRunStart: claimed=%t err=%v", claimed, err)
	}
	got, acquired, err := ms.AcquireCronRunStartDispatchLease(context.Background(), binding.TenantID, binding.IdempotencyKey, owner, now, now.Add(time.Minute))
	if err != nil || !acquired {
		t.Fatalf("AcquireCronRunStartDispatchLease: acquired=%t err=%v", acquired, err)
	}
	return got
}

func awaitCronRunLeaseHeartbeatStop(t *testing.T, stopped <-chan string) {
	t.Helper()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("cron run dispatch lease heartbeat did not stop")
	}
}

func awaitCronRunTerminalEvent(t *testing.T, events <-chan harness.Event) {
	t.Helper()
	for {
		select {
		case event, ok := <-events:
			if !ok {
				t.Fatal("run event stream closed before terminal event")
			}
			if event.Type == harness.EventRunCompleted || event.Type == harness.EventRunFailed || event.Type == harness.EventRunCancelled {
				return
			}
		case <-time.After(time.Second):
			t.Fatal("run did not reach a terminal event")
		}
	}
}
