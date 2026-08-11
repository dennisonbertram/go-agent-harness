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

func TestCronRunRequestFingerprintIncludesRoutingPolicy(t *testing.T) {
	base := cronRunRequest{
		Prompt: "continue", TenantID: "tenant", AgentID: "agent",
		ConversationID: "conversation", JobID: "job", ExecutionID: "execution",
		CorrelationKey: "cron/job/execution", Model: "fixture-model",
		ProviderName: "missing-primary", AllowFallback: true,
		FallbackProviders: []string{"secondary", "tertiary"},
	}
	wantDifferent := []cronRunRequest{
		func() cronRunRequest { changed := base; changed.Model = "other-model"; return changed }(),
		func() cronRunRequest { changed := base; changed.ProviderName = "other-provider"; return changed }(),
		func() cronRunRequest { changed := base; changed.AllowFallback = false; return changed }(),
		func() cronRunRequest {
			changed := base
			changed.FallbackProviders = []string{"tertiary", "secondary"}
			return changed
		}(),
	}
	want := cronRunRequestFingerprint(base)
	for _, changed := range wantDifferent {
		if got := cronRunRequestFingerprint(changed); got == want {
			t.Fatalf("routing change did not alter fingerprint: %#v", changed)
		}
	}
}

// scriptedCronRunDispatchStore forces the otherwise timing-dependent
// cross-server lease-contention branch while retaining the real durable
// reservation and runner persistence behavior.
type scriptedCronRunDispatchStore struct {
	*store.MemoryStore
	foreignFirstAcquire bool
	acquireCalls        atomic.Int32
	createCalls         atomic.Int32
	mu                  sync.Mutex
	claimedRunID        string
}

func (s *scriptedCronRunDispatchStore) ClaimCronRunStart(ctx context.Context, start store.CronRunStart) (store.CronRunStart, bool, error) {
	binding, claimed, err := s.MemoryStore.ClaimCronRunStart(ctx, start)
	if err == nil {
		s.mu.Lock()
		s.claimedRunID = binding.RunID
		s.mu.Unlock()
	}
	return binding, claimed, err
}

func (s *scriptedCronRunDispatchStore) AcquireCronRunStartDispatchLease(ctx context.Context, tenantID, idempotencyKey, owner string, now, leaseUntil time.Time) (store.CronRunStart, bool, error) {
	if s.acquireCalls.Add(1) == 1 && s.foreignFirstAcquire {
		binding, _, err := s.MemoryStore.ClaimCronRunStart(ctx, store.CronRunStart{
			TenantID: tenantID, IdempotencyKey: idempotencyKey, Fingerprint: "unused", RunID: "unused", CreatedAt: now,
		})
		if err != nil {
			return store.CronRunStart{}, false, err
		}
		binding.DispatchOwner = "foreign-harnessd"
		binding.DispatchLeaseUntil = now.Add(time.Minute)
		return binding, false, nil
	}
	return s.MemoryStore.AcquireCronRunStartDispatchLease(ctx, tenantID, idempotencyKey, owner, now, leaseUntil)
}

func (s *scriptedCronRunDispatchStore) CreateRun(ctx context.Context, run *store.Run) error {
	s.createCalls.Add(1)
	return s.MemoryStore.CreateRun(ctx, run)
}

func (s *scriptedCronRunDispatchStore) reservedRunID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.claimedRunID
}

type countingCronDispatchProvider struct{ calls atomic.Int32 }

func (p *countingCronDispatchProvider) Complete(context.Context, harness.CompletionRequest) (harness.CompletionResult, error) {
	p.calls.Add(1)
	return harness.CompletionResult{Content: "cron dispatched"}, nil
}

func TestCronRunDispatchPollRetriesForeignLeaseAndAdmitsReservedRun(t *testing.T) {
	durable := &scriptedCronRunDispatchStore{MemoryStore: store.NewMemoryStore(), foreignFirstAcquire: true}
	provider := &countingCronDispatchProvider{}
	runner := testRunnerWithCronProvider(durable, provider)
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })

	req := cronRunRequest{
		Prompt: "retry foreign cron dispatch lease", TenantID: "tenant-dispatch-poll", AgentID: "agent-dispatch-poll",
		ConversationID: "conversation-dispatch-poll", JobID: "job-dispatch-poll", ExecutionID: "execution-dispatch-poll",
		CorrelationKey: "cron/job-dispatch-poll/execution-dispatch-poll",
	}
	s := &Server{runner: runner, runStore: durable, cronRunDispatchPollInterval: time.Microsecond}
	run, err := s.getOrStartCronRun(context.Background(), req, req.CorrelationKey, harness.RunRequest{
		Prompt: req.Prompt, TenantID: req.TenantID, AgentID: req.AgentID, ConversationID: req.ConversationID,
	})
	if err != nil {
		t.Fatalf("getOrStartCronRun: %v", err)
	}
	if reservedRunID := durable.reservedRunID(); reservedRunID == "" || run.ID != reservedRunID {
		t.Fatalf("returned run ID = %q, want the same durable reserved run ID %q", run.ID, reservedRunID)
	}
	if got := durable.acquireCalls.Load(); got < 2 {
		t.Fatalf("dispatch lease acquisitions = %d, want at least 2 after foreign contention", got)
	}
	if got := durable.createCalls.Load(); got != 1 {
		t.Fatalf("runner admissions = %d, want exactly 1", got)
	}
	if _, exists := runner.GetRun(run.ID); !exists {
		t.Fatalf("runner missing admitted reserved run %q", run.ID)
	}
	deadline := time.Now().Add(time.Second)
	for provider.calls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := provider.calls.Load(); got != 1 {
		t.Fatalf("provider dispatches = %d, want exactly 1", got)
	}
}

func TestCronRunDispatchPollCancellationPreventsAdmission(t *testing.T) {
	durable := &scriptedCronRunDispatchStore{MemoryStore: store.NewMemoryStore()}
	provider := &countingCronDispatchProvider{}
	runner := testRunnerWithCronProvider(durable, provider)
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })

	req := cronRunRequest{
		Prompt: "cancel foreign cron dispatch lease", TenantID: "tenant-dispatch-cancel", AgentID: "agent-dispatch-cancel",
		ConversationID: "conversation-dispatch-cancel", JobID: "job-dispatch-cancel", ExecutionID: "execution-dispatch-cancel",
		CorrelationKey: "cron/job-dispatch-cancel/execution-dispatch-cancel",
	}
	reservedRunID := "run_dispatch_poll_cancel"
	now := time.Now().UTC()
	if _, claimed, err := durable.ClaimCronRunStart(context.Background(), store.CronRunStart{
		TenantID: req.TenantID, IdempotencyKey: req.CorrelationKey, Fingerprint: cronRunRequestFingerprint(req),
		RunID: reservedRunID, CreatedAt: now,
	}); err != nil || !claimed {
		t.Fatalf("ClaimCronRunStart: claimed=%t err=%v", claimed, err)
	}
	if _, acquired, err := durable.MemoryStore.AcquireCronRunStartDispatchLease(context.Background(), req.TenantID, req.CorrelationKey, "foreign-harnessd", now, now.Add(time.Hour)); err != nil || !acquired {
		t.Fatalf("AcquireCronRunStartDispatchLease seed: acquired=%t err=%v", acquired, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := &Server{runner: runner, runStore: durable, cronRunDispatchPollInterval: time.Hour}
	started := time.Now()
	_, err := s.getOrStartCronRun(ctx, req, req.CorrelationKey, harness.RunRequest{
		Prompt: req.Prompt, TenantID: req.TenantID, AgentID: req.AgentID, ConversationID: req.ConversationID,
	})
	if !errors.Is(err, errCronRunIdempotencyUnavailable) {
		t.Fatalf("cancelled lease wait error = %v, want errCronRunIdempotencyUnavailable", err)
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("cancelled lease wait took %s, want prompt return without polling interval", elapsed)
	}
	if got := durable.createCalls.Load(); got != 0 {
		t.Fatalf("runner admissions after cancellation = %d, want 0", got)
	}
	if _, exists := runner.GetRun(reservedRunID); exists {
		t.Fatalf("runner admitted cancelled reserved run %q", reservedRunID)
	}
	if got := provider.calls.Load(); got != 0 {
		t.Fatalf("provider dispatches after cancellation = %d, want 0", got)
	}
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
