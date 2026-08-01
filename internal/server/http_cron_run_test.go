package server

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go-agent-harness/internal/forensics/audittrail"
	"go-agent-harness/internal/harness"
	"go-agent-harness/internal/store"
)

type failingCronRunCreateStore struct {
	*store.MemoryStore
	claimedRunID string
	markCalls    int
}

type shutdownAfterCronRunCreateStore struct {
	*store.MemoryStore
	afterCreate func()
	once        sync.Once
}

type failOnceCronRunAcceptedStore struct {
	*store.MemoryStore
	mu        sync.Mutex
	markCalls int
}

type shutdownBlockingCronProvider struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

type sharedCronDispatchProvider struct {
	mu      sync.Mutex
	calls   int
	started chan struct{}
	release chan struct{}
}

func (p *sharedCronDispatchProvider) Complete(ctx context.Context, _ harness.CompletionRequest) (harness.CompletionResult, error) {
	p.mu.Lock()
	p.calls++
	p.mu.Unlock()
	select {
	case p.started <- struct{}{}:
	default:
	}
	select {
	case <-p.release:
		return harness.CompletionResult{Content: "done"}, nil
	case <-ctx.Done():
		return harness.CompletionResult{}, ctx.Err()
	}
}

func (p *sharedCronDispatchProvider) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

type synchronizedCronRunStore struct {
	store.Store
	cronStarts store.CronRunStartStore

	claimArrived chan<- struct{}
	claimRelease <-chan struct{}
	getRunID     string
	getArrived   chan<- struct{}
	getRelease   <-chan struct{}
	getOnce      sync.Once
	renewed      chan<- struct{}
}

func (s *synchronizedCronRunStore) ClaimCronRunStart(ctx context.Context, start store.CronRunStart) (store.CronRunStart, bool, error) {
	persisted, claimed, err := s.cronStarts.ClaimCronRunStart(ctx, start)
	if s.claimArrived != nil {
		s.claimArrived <- struct{}{}
		<-s.claimRelease
	}
	return persisted, claimed, err
}

func (s *synchronizedCronRunStore) AcquireCronRunStartDispatchLease(ctx context.Context, tenantID, idempotencyKey, owner string, now, leaseUntil time.Time) (store.CronRunStart, bool, error) {
	return s.cronStarts.AcquireCronRunStartDispatchLease(ctx, tenantID, idempotencyKey, owner, now, leaseUntil)
}

func (s *synchronizedCronRunStore) RenewCronRunStartDispatchLease(ctx context.Context, tenantID, idempotencyKey, owner string, now, leaseUntil time.Time) (store.CronRunStart, bool, error) {
	persisted, renewed, err := s.cronStarts.RenewCronRunStartDispatchLease(ctx, tenantID, idempotencyKey, owner, now, leaseUntil)
	if renewed && s.renewed != nil {
		s.renewed <- struct{}{}
	}
	return persisted, renewed, err
}

func (s *synchronizedCronRunStore) MarkCronRunStartAccepted(ctx context.Context, tenantID, idempotencyKey, owner string) error {
	return s.cronStarts.MarkCronRunStartAccepted(ctx, tenantID, idempotencyKey, owner)
}

func (s *synchronizedCronRunStore) GetRun(ctx context.Context, runID string) (*store.Run, error) {
	run, err := s.Store.GetRun(ctx, runID)
	if runID == s.getRunID && s.getArrived != nil {
		s.getOnce.Do(func() {
			s.getArrived <- struct{}{}
			<-s.getRelease
		})
	}
	return run, err
}

func (p *shutdownBlockingCronProvider) Complete(ctx context.Context, _ harness.CompletionRequest) (harness.CompletionResult, error) {
	p.once.Do(func() { close(p.started) })
	select {
	case <-p.release:
		return harness.CompletionResult{Content: "released"}, nil
	case <-ctx.Done():
		return harness.CompletionResult{}, ctx.Err()
	}
}

func (s *failOnceCronRunAcceptedStore) MarkCronRunStartAccepted(ctx context.Context, tenantID, idempotencyKey, owner string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.markCalls++
	if s.markCalls == 1 {
		return errors.New("simulated transient accepted-binding failure")
	}
	return s.MemoryStore.MarkCronRunStartAccepted(ctx, tenantID, idempotencyKey, owner)
}

func (s *shutdownAfterCronRunCreateStore) CreateRun(ctx context.Context, run *store.Run) error {
	run.Model = "persisted-recovery-model"
	run.ProviderName = "persisted-recovery-provider"
	run.CreatedAt = time.Unix(1_700_000_000, 0).UTC()
	run.UpdatedAt = run.CreatedAt
	if err := s.MemoryStore.CreateRun(ctx, run); err != nil {
		return err
	}
	s.once.Do(func() {
		if s.afterCreate != nil {
			s.afterCreate()
		}
	})
	return nil
}

func (s *failingCronRunCreateStore) ClaimCronRunStart(ctx context.Context, start store.CronRunStart) (store.CronRunStart, bool, error) {
	s.claimedRunID = start.RunID
	return s.MemoryStore.ClaimCronRunStart(ctx, start)
}

func (s *failingCronRunCreateStore) MarkCronRunStartAccepted(ctx context.Context, tenantID, idempotencyKey, owner string) error {
	s.markCalls++
	return s.MemoryStore.MarkCronRunStartAccepted(ctx, tenantID, idempotencyKey, owner)
}

func (*failingCronRunCreateStore) CreateRun(context.Context, *store.Run) error {
	return errors.New("simulated durable CreateRun failure")
}

func TestCronRunEndpointRequiresAuthAndPreservesScope(t *testing.T) {
	ms := store.NewMemoryStore()
	token, key := cronTestAPIKey(t, "tenant-remote", "remote cron", []string{store.ScopeRunsWrite})
	if err := ms.CreateAPIKey(context.Background(), key); err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
	runner := testRunnerForCronStore(ms)
	h := NewWithOptions(ServerOptions{Runner: runner, Store: ms})

	unauthenticated := httptestRequest(t, http.MethodPost, "/v1/cron/runs", `{"prompt":"hello"}`, "")
	unauthenticatedRecorder := httptestRecorder(h, unauthenticated)
	if unauthenticatedRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, body=%s", unauthenticatedRecorder.Code, unauthenticatedRecorder.Body.String())
	}

	payload := `{"prompt":"remote prompt","tenant_id":"tenant-remote","agent_id":"agent-remote","conversation_id":"conversation-remote","job_id":"job-remote","execution_id":"execution-remote","correlation_key":"cron/job-remote/execution-remote"}`
	req := httptestRequest(t, http.MethodPost, "/v1/cron/runs", payload, token)
	req.Header.Set("Idempotency-Key", "cron/job-remote/execution-remote")
	res := httptestRecorder(h, req)
	if res.Code != http.StatusAccepted {
		t.Fatalf("cron run status = %d, body=%s", res.Code, res.Body.String())
	}
	var response struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil || response.RunID == "" {
		t.Fatalf("response = %s, err=%v", res.Body.String(), err)
	}
	originalRunID := response.RunID
	run, ok := runner.GetRun(response.RunID)
	if !ok {
		t.Fatalf("runner missing run %q", response.RunID)
	}
	if run.TenantID != "tenant-remote" || run.AgentID != "agent-remote" || run.ConversationID != "conversation-remote" {
		t.Fatalf("run scope = tenant %q agent %q conversation %q", run.TenantID, run.AgentID, run.ConversationID)
	}

	duplicate := httptestRequest(t, http.MethodPost, "/v1/cron/runs", payload, token)
	duplicate.Header.Set("Idempotency-Key", "cron/job-remote/execution-remote")
	duplicateRes := httptestRecorder(h, duplicate)
	if duplicateRes.Code != http.StatusAccepted {
		t.Fatalf("duplicate cron run status = %d, body=%s", duplicateRes.Code, duplicateRes.Body.String())
	}
	var duplicateBody struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(duplicateRes.Body.Bytes(), &duplicateBody); err != nil {
		t.Fatalf("duplicate response = %s, err=%v", duplicateRes.Body.String(), err)
	}
	if duplicateBody.RunID != originalRunID {
		t.Fatalf("duplicate run_id = %q, want original run %q", duplicateBody.RunID, originalRunID)
	}

	conflictingPayload := strings.Replace(payload, "remote prompt", "different prompt", 1)
	conflicting := httptestRequest(t, http.MethodPost, "/v1/cron/runs", conflictingPayload, token)
	conflicting.Header.Set("Idempotency-Key", "cron/job-remote/execution-remote")
	conflictingRes := httptestRecorder(h, conflicting)
	if conflictingRes.Code != http.StatusConflict {
		t.Fatalf("conflicting replay status = %d, body=%s", conflictingRes.Code, conflictingRes.Body.String())
	}

	wrongTenantPayload := strings.Replace(payload, "tenant-remote", "tenant-other", 1)
	wrongTenant := httptestRequest(t, http.MethodPost, "/v1/cron/runs", wrongTenantPayload, token)
	wrongTenant.Header.Set("Idempotency-Key", "cron/job-remote/execution-remote")
	wrongTenantRes := httptestRecorder(h, wrongTenant)
	if wrongTenantRes.Code != http.StatusBadRequest {
		t.Fatalf("tenant mismatch status = %d, body=%s", wrongTenantRes.Code, wrongTenantRes.Body.String())
	}
}

func TestCronRunEndpointBoundsRequestBody(t *testing.T) {
	ms := store.NewMemoryStore()
	token, key := cronTestAPIKey(t, "tenant-bounds", "remote cron bounds", []string{store.ScopeRunsWrite})
	if err := ms.CreateAPIKey(context.Background(), key); err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
	h := NewWithOptions(ServerOptions{Runner: testRunnerForCron(t), Store: ms})
	payload := `{"prompt":"` + strings.Repeat("x", int(defaultMaxRequestBodyBytes)) + `"}`
	req := httptestRequest(t, http.MethodPost, "/v1/cron/runs", payload, token)
	req.Header.Set("Idempotency-Key", "cron/job-bounds/execution-bounds")
	res := httptestRecorder(h, req)
	if res.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized request status = %d, body=%s", res.Code, res.Body.String())
	}
}

func TestCronRunEndpointDeduplicatesConcurrentDelivery(t *testing.T) {
	ms := store.NewMemoryStore()
	token, key := cronTestAPIKey(t, "tenant-concurrent", "remote cron concurrent", []string{store.ScopeRunsWrite})
	if err := ms.CreateAPIKey(context.Background(), key); err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
	runner := testRunnerForCronStore(ms)
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })
	h := NewWithOptions(ServerOptions{Runner: runner, Store: ms})
	payload := `{"prompt":"concurrent prompt","tenant_id":"tenant-concurrent","agent_id":"agent-concurrent","conversation_id":"conversation-concurrent","job_id":"job-concurrent","execution_id":"execution-concurrent","correlation_key":"cron/job-concurrent/execution-concurrent"}`

	const deliveries = 16
	runIDs := make(chan string, deliveries)
	errs := make(chan string, deliveries)
	var ready sync.WaitGroup
	ready.Add(deliveries)
	start := make(chan struct{})
	var done sync.WaitGroup
	done.Add(deliveries)
	for i := 0; i < deliveries; i++ {
		go func() {
			defer done.Done()
			req := httptestRequest(t, http.MethodPost, "/v1/cron/runs", payload, token)
			req.Header.Set("Idempotency-Key", "cron/job-concurrent/execution-concurrent")
			ready.Done()
			<-start
			res := httptestRecorder(h, req)
			if res.Code != http.StatusAccepted {
				errs <- res.Body.String()
				return
			}
			var response struct {
				RunID string `json:"run_id"`
			}
			if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil || response.RunID == "" {
				errs <- res.Body.String()
				return
			}
			runIDs <- response.RunID
		}()
	}
	ready.Wait()
	close(start)
	done.Wait()
	close(errs)
	for failure := range errs {
		t.Fatalf("concurrent delivery failed: %s", failure)
	}
	close(runIDs)
	var acceptedRunID string
	for runID := range runIDs {
		if acceptedRunID == "" {
			acceptedRunID = runID
		}
		if runID != acceptedRunID {
			t.Fatalf("concurrent run_id = %q, want %q", runID, acceptedRunID)
		}
	}
	runs, err := ms.ListRuns(context.Background(), store.RunFilter{TenantID: "tenant-concurrent"})
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 1 || runs[0].ID != acceptedRunID {
		t.Fatalf("persisted runs = %+v, want one accepted run %q", runs, acceptedRunID)
	}
}

func TestCronRunStartLeaseDeduplicatesInitialDeliveryAcrossServers(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "runs.db")
	firstStore, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore first: %v", err)
	}
	t.Cleanup(func() { _ = firstStore.Close() })
	if err := firstStore.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate first: %v", err)
	}
	secondStore, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore second: %v", err)
	}
	t.Cleanup(func() { _ = secondStore.Close() })
	if err := secondStore.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate second: %v", err)
	}

	claimArrived := make(chan struct{}, 2)
	claimRelease := make(chan struct{})
	firstWrapped := &synchronizedCronRunStore{Store: firstStore, cronStarts: firstStore, claimArrived: claimArrived, claimRelease: claimRelease}
	secondWrapped := &synchronizedCronRunStore{Store: secondStore, cronStarts: secondStore, claimArrived: claimArrived, claimRelease: claimRelease}
	provider := &sharedCronDispatchProvider{started: make(chan struct{}, 2), release: make(chan struct{})}
	firstRunner := testRunnerWithCronProvider(firstWrapped, provider)
	secondRunner := testRunnerWithCronProvider(secondWrapped, provider)
	t.Cleanup(func() { _ = firstRunner.Shutdown(context.Background()) })
	t.Cleanup(func() { _ = secondRunner.Shutdown(context.Background()) })
	t.Cleanup(func() { close(provider.release) })

	req := cronRunRequest{
		Prompt:         "shared initial delivery",
		TenantID:       "tenant-shared-initial",
		AgentID:        "agent-shared-initial",
		ConversationID: "conversation-shared-initial",
		JobID:          "job-shared-initial",
		ExecutionID:    "execution-shared-initial",
		CorrelationKey: "cron/job-shared-initial/execution-shared-initial",
	}
	runReq := harness.RunRequest{Prompt: req.Prompt, TenantID: req.TenantID, AgentID: req.AgentID, ConversationID: req.ConversationID}
	type result struct {
		run harness.Run
		err error
	}
	results := make(chan result, 2)
	go func() {
		run, startErr := (&Server{runner: firstRunner, runStore: firstWrapped, timeNow: time.Now}).getOrStartCronRun(context.Background(), req, req.CorrelationKey, runReq)
		results <- result{run: run, err: startErr}
	}()
	go func() {
		run, startErr := (&Server{runner: secondRunner, runStore: secondWrapped, timeNow: time.Now}).getOrStartCronRun(context.Background(), req, req.CorrelationKey, runReq)
		results <- result{run: run, err: startErr}
	}()
	<-claimArrived
	<-claimArrived
	close(claimRelease)

	first := <-results
	second := <-results
	if first.err != nil || second.err != nil {
		t.Fatalf("shared initial delivery errors = %v, %v", first.err, second.err)
	}
	if first.run.ID == "" || first.run.ID != second.run.ID {
		t.Fatalf("shared initial run IDs = %q, %q", first.run.ID, second.run.ID)
	}
	assertSingleCronRunnerAdmission(t, first.run.ID, firstRunner, secondRunner)
	assertSingleCronProviderDispatch(t, provider)
}

func TestCronRunStartLeaseDeduplicatesQueuedRecoveryAcrossServers(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "runs.db")
	firstStore, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore first: %v", err)
	}
	t.Cleanup(func() { _ = firstStore.Close() })
	if err := firstStore.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate first: %v", err)
	}
	secondStore, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore second: %v", err)
	}
	t.Cleanup(func() { _ = secondStore.Close() })
	if err := secondStore.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate second: %v", err)
	}

	req := cronRunRequest{
		Prompt:         "shared queued recovery",
		TenantID:       "tenant-shared-recovery",
		AgentID:        "agent-shared-recovery",
		ConversationID: "conversation-shared-recovery",
		JobID:          "job-shared-recovery",
		ExecutionID:    "execution-shared-recovery",
		CorrelationKey: "cron/job-shared-recovery/execution-shared-recovery",
	}
	const reservedRunID = "run_shared_queued_recovery"
	now := time.Now().UTC().Truncate(time.Second)
	if _, claimed, err := firstStore.ClaimCronRunStart(context.Background(), store.CronRunStart{
		TenantID:       req.TenantID,
		IdempotencyKey: req.CorrelationKey,
		Fingerprint:    cronRunRequestFingerprint(req),
		RunID:          reservedRunID,
		Accepted:       true,
		CreatedAt:      now,
	}); err != nil || !claimed {
		t.Fatalf("ClaimCronRunStart seed: claimed=%t err=%v", claimed, err)
	}
	if _, acquired, err := firstStore.AcquireCronRunStartDispatchLease(context.Background(), req.TenantID, req.CorrelationKey, "crashed-harnessd", now, now.Add(10*time.Second)); err != nil || !acquired {
		t.Fatalf("AcquireCronRunStartDispatchLease seed: acquired=%t err=%v", acquired, err)
	}
	if err := firstStore.MarkCronRunStartAccepted(context.Background(), req.TenantID, req.CorrelationKey, "crashed-harnessd"); err != nil {
		t.Fatalf("MarkCronRunStartAccepted seed: %v", err)
	}
	if err := firstStore.CreateRun(context.Background(), &store.Run{
		ID:             reservedRunID,
		Prompt:         req.Prompt,
		Model:          "gpt-4.1-mini",
		Status:         store.RunStatusQueued,
		TenantID:       req.TenantID,
		AgentID:        req.AgentID,
		ConversationID: req.ConversationID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}); err != nil {
		t.Fatalf("CreateRun seed: %v", err)
	}
	leaseDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open lease expiry DB: %v", err)
	}
	t.Cleanup(func() { _ = leaseDB.Close() })
	expireCronRunLease(t, leaseDB, req.TenantID, req.CorrelationKey)

	getArrived := make(chan struct{}, 2)
	getRelease := make(chan struct{})
	firstWrapped := &synchronizedCronRunStore{Store: firstStore, cronStarts: firstStore, getRunID: reservedRunID, getArrived: getArrived, getRelease: getRelease}
	secondWrapped := &synchronizedCronRunStore{Store: secondStore, cronStarts: secondStore, getRunID: reservedRunID, getArrived: getArrived, getRelease: getRelease}
	provider := &sharedCronDispatchProvider{started: make(chan struct{}, 2), release: make(chan struct{})}
	firstRunner := testRunnerWithCronProvider(firstWrapped, provider)
	secondRunner := testRunnerWithCronProvider(secondWrapped, provider)
	t.Cleanup(func() { _ = firstRunner.Shutdown(context.Background()) })
	t.Cleanup(func() { _ = secondRunner.Shutdown(context.Background()) })
	t.Cleanup(func() { close(provider.release) })

	runReq := harness.RunRequest{Prompt: req.Prompt, TenantID: req.TenantID, AgentID: req.AgentID, ConversationID: req.ConversationID}
	type result struct {
		run harness.Run
		err error
	}
	results := make(chan result, 2)
	go func() {
		run, startErr := (&Server{runner: firstRunner, runStore: firstWrapped, timeNow: func() time.Time { return now.Add(11 * time.Second) }}).getOrStartCronRun(context.Background(), req, req.CorrelationKey, runReq)
		results <- result{run: run, err: startErr}
	}()
	go func() {
		run, startErr := (&Server{runner: secondRunner, runStore: secondWrapped, timeNow: func() time.Time { return now.Add(11 * time.Second) }}).getOrStartCronRun(context.Background(), req, req.CorrelationKey, runReq)
		results <- result{run: run, err: startErr}
	}()
	<-getArrived
	<-getArrived
	close(getRelease)

	first := <-results
	second := <-results
	if first.err != nil || second.err != nil {
		t.Fatalf("shared queued recovery errors = %v, %v", first.err, second.err)
	}
	if first.run.ID != reservedRunID || second.run.ID != reservedRunID {
		t.Fatalf("shared recovery run IDs = %q, %q; want %q", first.run.ID, second.run.ID, reservedRunID)
	}
	assertSingleCronRunnerAdmission(t, reservedRunID, firstRunner, secondRunner)
	assertSingleCronProviderDispatch(t, provider)
}

func TestCronRunStartLeaseKeepsLiveBackloggedOwnerAcrossServers(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "runs.db")
	firstStore, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore first: %v", err)
	}
	t.Cleanup(func() { _ = firstStore.Close() })
	if err := firstStore.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate first: %v", err)
	}
	secondStore, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore second: %v", err)
	}
	t.Cleanup(func() { _ = secondStore.Close() })
	if err := secondStore.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate second: %v", err)
	}
	leaseDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open lease inspection DB: %v", err)
	}
	t.Cleanup(func() { _ = leaseDB.Close() })
	if _, err := leaseDB.Exec(`PRAGMA busy_timeout=5000;`); err != nil {
		t.Fatalf("set lease inspection busy timeout: %v", err)
	}

	blockerStarted := make(chan struct{})
	releaseBlocker := make(chan struct{})
	var releaseOnce sync.Once
	releaseFirst := func() { releaseOnce.Do(func() { close(releaseBlocker) }) }
	firstProvider := &blockingExternalProvider{
		results: []harness.CompletionResult{{Content: "blocker done"}, {Content: "unexpected cron dispatch"}},
		beforeCall: func(idx int) {
			if idx == 0 {
				close(blockerStarted)
				<-releaseBlocker
			}
		},
	}
	heartbeatRenewed := make(chan struct{}, 1)
	firstWrapped := &synchronizedCronRunStore{Store: firstStore, cronStarts: firstStore, renewed: heartbeatRenewed}
	firstRunner := harness.NewRunner(firstProvider, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "gpt-4.1-mini", MaxSteps: 1, WorkerPoolSize: 1, Store: firstWrapped,
	})
	t.Cleanup(func() { _ = firstRunner.Shutdown(context.Background()) })
	t.Cleanup(releaseFirst)
	if _, err := firstRunner.StartRun(harness.RunRequest{Prompt: "occupy first worker"}); err != nil {
		t.Fatalf("StartRun blocker: %v", err)
	}
	select {
	case <-blockerStarted:
	case <-time.After(time.Second):
		t.Fatal("blocker did not occupy first runner worker")
	}

	secondProvider := &sharedCronDispatchProvider{started: make(chan struct{}, 1), release: make(chan struct{})}
	secondRunner := testRunnerWithCronProvider(secondStore, secondProvider)
	t.Cleanup(func() { _ = secondRunner.Shutdown(context.Background()) })
	t.Cleanup(func() { close(secondProvider.release) })

	leaseNow := time.Now().UTC()
	req := cronRunRequest{
		Prompt:         "live backlogged cron",
		TenantID:       "tenant-live-backlog",
		AgentID:        "agent-live-backlog",
		ConversationID: "conversation-live-backlog",
		JobID:          "job-live-backlog",
		ExecutionID:    "execution-live-backlog",
		CorrelationKey: "cron/job-live-backlog/execution-live-backlog",
	}
	runReq := harness.RunRequest{Prompt: req.Prompt, TenantID: req.TenantID, AgentID: req.AgentID, ConversationID: req.ConversationID}
	heartbeatTicks := make(chan time.Time, 1)
	heartbeatStopped := make(chan string, 1)
	firstServer := &Server{
		runner:                        firstRunner,
		runStore:                      firstWrapped,
		timeNow:                       func() time.Time { return leaseNow },
		cronRunDispatchHeartbeatTicks: heartbeatTicks,
		cronRunLeaseHeartbeatStopped:  heartbeatStopped,
	}
	accepted, err := firstServer.getOrStartCronRun(context.Background(), req, req.CorrelationKey, runReq)
	if err != nil {
		t.Fatalf("first getOrStartCronRun: %v", err)
	}
	if active, exists := firstRunner.GetRun(accepted.ID); !exists || active.Status != harness.RunStatusQueued {
		t.Fatalf("first runner active run = %+v exists=%t, want queued", active, exists)
	}
	nearExpiry := setCronRunLeaseNearExpiry(t, leaseDB, req.TenantID, req.CorrelationKey)
	heartbeatTicks <- time.Now()
	select {
	case <-heartbeatRenewed:
	case <-time.After(time.Second):
		t.Fatal("live queued run did not renew its dispatch lease")
	}
	renewedExpiry := readCronRunLeaseExpiry(t, leaseDB, req.TenantID, req.CorrelationKey)
	if renewedExpiry <= nearExpiry {
		t.Fatalf("renewed lease expiry = %d, want greater than forced near-expiry %d", renewedExpiry, nearExpiry)
	}

	secondServer := &Server{runner: secondRunner, runStore: secondStore, timeNow: func() time.Time { return leaseNow.Add(defaultCronRunDispatchLeaseDuration + time.Second) }}
	replayed, err := secondServer.getOrStartCronRun(context.Background(), req, req.CorrelationKey, runReq)
	if err != nil {
		t.Fatalf("second getOrStartCronRun: %v", err)
	}
	if replayed.ID != accepted.ID {
		t.Fatalf("replayed run ID = %q, want %q", replayed.ID, accepted.ID)
	}
	if _, exists := secondRunner.GetRun(accepted.ID); exists {
		t.Fatalf("second runner dispatched live owner's backlogged run %q", accepted.ID)
	}
	if calls := secondProvider.callCount(); calls != 0 {
		t.Fatalf("second provider calls = %d, want 0 while first owner is live", calls)
	}

	releaseFirst()
	if err := firstRunner.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown first runner: %v", err)
	}
	select {
	case <-heartbeatStopped:
	case <-time.After(time.Second):
		t.Fatal("dispatch lease heartbeat did not stop with its runner")
	}
	expireCronRunLease(t, leaseDB, req.TenantID, req.CorrelationKey)
	recovered, err := secondServer.getOrStartCronRun(context.Background(), req, req.CorrelationKey, runReq)
	if err != nil {
		t.Fatalf("recovery getOrStartCronRun: %v", err)
	}
	if recovered.ID != accepted.ID {
		t.Fatalf("recovered run ID = %q, want %q", recovered.ID, accepted.ID)
	}
	if _, exists := secondRunner.GetRun(accepted.ID); !exists {
		t.Fatalf("second runner did not recover stopped owner's run %q", accepted.ID)
	}
	assertSingleCronProviderDispatch(t, secondProvider)
}

func TestCronRunEndpointDoesNotAcceptBindingWhenReservedRunPersistenceFails(t *testing.T) {
	runStore := &failingCronRunCreateStore{MemoryStore: store.NewMemoryStore()}
	token, key := cronTestAPIKey(t, "tenant-persist-failure", "remote cron persistence failure", []string{store.ScopeRunsWrite})
	if err := runStore.CreateAPIKey(context.Background(), key); err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
	runner := testRunnerForCronStore(runStore)
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })
	h := NewWithOptions(ServerOptions{Runner: runner, Store: runStore})
	payload := `{"prompt":"must persist before dispatch","tenant_id":"tenant-persist-failure","job_id":"job-persist-failure","execution_id":"execution-persist-failure","correlation_key":"cron/job-persist-failure/execution-persist-failure"}`
	req := httptestRequest(t, http.MethodPost, "/v1/cron/runs", payload, token)
	req.Header.Set("Idempotency-Key", "cron/job-persist-failure/execution-persist-failure")

	res := httptestRecorder(h, req)
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("cron run status = %d, body=%s; want 503", res.Code, res.Body.String())
	}
	if runStore.markCalls != 0 {
		t.Fatalf("accepted-binding marks = %d, want 0", runStore.markCalls)
	}
	if runStore.claimedRunID == "" {
		t.Fatal("expected a reserved run ID")
	}
	if _, ok := runner.GetRun(runStore.claimedRunID); ok {
		t.Fatalf("runner retained or dispatched unpersisted reserved run %q", runStore.claimedRunID)
	}
}

func TestCronRunEndpointResumesPersistedQueuedReservationAfterDispatchFailure(t *testing.T) {
	runStore := &shutdownAfterCronRunCreateStore{MemoryStore: store.NewMemoryStore()}
	token, key := cronTestAPIKey(t, "tenant-dispatch-recovery", "remote cron dispatch recovery", []string{store.ScopeRunsWrite})
	if err := runStore.CreateAPIKey(context.Background(), key); err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
	firstRunner := testRunnerForCronStore(runStore)
	runStore.afterCreate = func() {
		if err := firstRunner.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown first runner: %v", err)
		}
	}
	leaseNow := time.Now().UTC()
	firstHandler := cronTestHandlerAt(firstRunner, runStore, leaseNow)
	payload := `{"prompt":"resume queued reservation","tenant_id":"tenant-dispatch-recovery","job_id":"job-dispatch-recovery","execution_id":"execution-dispatch-recovery","correlation_key":"cron/job-dispatch-recovery/execution-dispatch-recovery"}`
	firstRequest := httptestRequest(t, http.MethodPost, "/v1/cron/runs", payload, token)
	firstRequest.Header.Set("Idempotency-Key", "cron/job-dispatch-recovery/execution-dispatch-recovery")

	firstResponse := httptestRecorder(firstHandler, firstRequest)
	if firstResponse.Code == http.StatusAccepted {
		t.Fatalf("first dispatch unexpectedly accepted: %s", firstResponse.Body.String())
	}
	persisted, err := runStore.ListRuns(context.Background(), store.RunFilter{TenantID: "tenant-dispatch-recovery"})
	if err != nil {
		t.Fatalf("ListRuns after dispatch failure: %v", err)
	}
	if len(persisted) != 1 || persisted[0].Status != store.RunStatusQueued {
		t.Fatalf("persisted runs after dispatch failure = %+v, want one queued reservation", persisted)
	}
	reservedRunID := persisted[0].ID

	blockerStarted := make(chan struct{})
	releaseBlocker := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseBlocker) }) }
	replacementProvider := &blockingExternalProvider{
		results: []harness.CompletionResult{
			{Content: "blocker done"},
			{Content: "resumed done"},
		},
		beforeCall: func(idx int) {
			if idx == 0 {
				close(blockerStarted)
				<-releaseBlocker
			}
		},
	}
	replacementRunner := harness.NewRunner(
		replacementProvider,
		harness.NewRegistry(),
		harness.RunnerConfig{
			DefaultModel:        "gpt-4.1-mini",
			DefaultSystemPrompt: "You are helpful.",
			MaxSteps:            1,
			WorkerPoolSize:      1,
			Store:               runStore,
		},
	)
	t.Cleanup(func() { _ = replacementRunner.Shutdown(context.Background()) })
	t.Cleanup(release)
	if _, err := replacementRunner.StartRun(harness.RunRequest{Prompt: "occupy worker"}); err != nil {
		t.Fatalf("StartRun blocker: %v", err)
	}
	select {
	case <-blockerStarted:
	case <-time.After(time.Second):
		t.Fatal("blocker did not occupy replacement runner worker")
	}
	replacementHandler := cronTestHandlerAt(replacementRunner, runStore, leaseNow.Add(defaultCronRunDispatchLeaseDuration+time.Second))
	replay := httptestRequest(t, http.MethodPost, "/v1/cron/runs", payload, token)
	replay.Header.Set("Idempotency-Key", "cron/job-dispatch-recovery/execution-dispatch-recovery")
	replayResponse := httptestRecorder(replacementHandler, replay)
	if replayResponse.Code != http.StatusAccepted {
		t.Fatalf("replay status = %d, body=%s", replayResponse.Code, replayResponse.Body.String())
	}
	var replayed struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(replayResponse.Body.Bytes(), &replayed); err != nil {
		t.Fatalf("decode replay response: %v", err)
	}
	if replayed.RunID != reservedRunID {
		t.Fatalf("replayed run ID = %q, want reserved %q", replayed.RunID, reservedRunID)
	}
	resumed, ok := replacementRunner.GetRun(reservedRunID)
	if !ok {
		t.Fatalf("replacement runner did not resume queued reserved run %q", reservedRunID)
	}
	if resumed.Model != persisted[0].Model ||
		resumed.ProviderName != persisted[0].ProviderName ||
		resumed.Status != harness.RunStatusQueued ||
		!resumed.CreatedAt.Equal(persisted[0].CreatedAt) ||
		!resumed.UpdatedAt.Equal(persisted[0].UpdatedAt) ||
		resumed.Prompt != persisted[0].Prompt ||
		resumed.TenantID != persisted[0].TenantID ||
		resumed.AgentID != persisted[0].AgentID ||
		resumed.ConversationID != persisted[0].ConversationID {
		t.Fatalf("resumed run = %+v, want exact persisted identity/state %+v", resumed, persisted[0])
	}

	secondReplay := httptestRequest(t, http.MethodPost, "/v1/cron/runs", payload, token)
	secondReplay.Header.Set("Idempotency-Key", "cron/job-dispatch-recovery/execution-dispatch-recovery")
	secondReplayResponse := httptestRecorder(replacementHandler, secondReplay)
	if secondReplayResponse.Code != http.StatusAccepted {
		t.Fatalf("second replay status = %d, body=%s", secondReplayResponse.Code, secondReplayResponse.Body.String())
	}
	afterReplay, err := runStore.ListRuns(context.Background(), store.RunFilter{TenantID: "tenant-dispatch-recovery"})
	if err != nil {
		t.Fatalf("ListRuns after replay: %v", err)
	}
	if len(afterReplay) != 1 || afterReplay[0].ID != reservedRunID {
		t.Fatalf("runs after replay = %+v, want only reserved run %q", afterReplay, reservedRunID)
	}
	release()
}

func TestCronRunEndpointReusesActiveQueuedRunAfterAcceptedMarkFailure(t *testing.T) {
	runStore := &failOnceCronRunAcceptedStore{MemoryStore: store.NewMemoryStore()}
	token, key := cronTestAPIKey(t, "tenant-mark-retry", "remote cron accepted mark retry", []string{store.ScopeRunsWrite})
	if err := runStore.CreateAPIKey(context.Background(), key); err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
	blockerStarted := make(chan struct{})
	releaseBlocker := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseBlocker) }) }
	provider := &blockingExternalProvider{
		results: []harness.CompletionResult{
			{Content: "blocker done"},
			{Content: "cron done"},
			{Content: "unexpected duplicate"},
		},
		beforeCall: func(idx int) {
			if idx == 0 {
				close(blockerStarted)
				<-releaseBlocker
			}
		},
	}
	runner := harness.NewRunner(
		provider,
		harness.NewRegistry(),
		harness.RunnerConfig{
			DefaultModel:        "gpt-4.1-mini",
			DefaultSystemPrompt: "You are helpful.",
			MaxSteps:            1,
			WorkerPoolSize:      1,
			Store:               runStore,
		},
	)
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })
	t.Cleanup(release)
	if _, err := runner.StartRun(harness.RunRequest{Prompt: "occupy worker"}); err != nil {
		t.Fatalf("StartRun blocker: %v", err)
	}
	select {
	case <-blockerStarted:
	case <-time.After(time.Second):
		t.Fatal("blocker did not occupy runner worker")
	}

	handler := NewWithOptions(ServerOptions{Runner: runner, Store: runStore})
	payload := `{"prompt":"retry accepted mark","tenant_id":"tenant-mark-retry","job_id":"job-mark-retry","execution_id":"execution-mark-retry","correlation_key":"cron/job-mark-retry/execution-mark-retry"}`
	first := httptestRequest(t, http.MethodPost, "/v1/cron/runs", payload, token)
	first.Header.Set("Idempotency-Key", "cron/job-mark-retry/execution-mark-retry")
	firstResponse := httptestRecorder(handler, first)
	if firstResponse.Code != http.StatusServiceUnavailable {
		t.Fatalf("first status = %d, body=%s; want 503", firstResponse.Code, firstResponse.Body.String())
	}
	persisted, err := runStore.ListRuns(context.Background(), store.RunFilter{TenantID: "tenant-mark-retry"})
	if err != nil {
		t.Fatalf("ListRuns after mark failure: %v", err)
	}
	if len(persisted) != 1 || persisted[0].Status != store.RunStatusQueued {
		t.Fatalf("persisted runs after mark failure = %+v, want one queued run", persisted)
	}

	retry := httptestRequest(t, http.MethodPost, "/v1/cron/runs", payload, token)
	retry.Header.Set("Idempotency-Key", "cron/job-mark-retry/execution-mark-retry")
	retryResponse := httptestRecorder(handler, retry)
	if retryResponse.Code != http.StatusAccepted {
		t.Fatalf("retry status = %d, body=%s; want 202", retryResponse.Code, retryResponse.Body.String())
	}
	var replayed struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(retryResponse.Body.Bytes(), &replayed); err != nil {
		t.Fatalf("decode retry response: %v", err)
	}
	if replayed.RunID != persisted[0].ID {
		t.Fatalf("retry run ID = %q, want active %q", replayed.RunID, persisted[0].ID)
	}
	if _, ok := runner.GetRun(persisted[0].ID); !ok {
		t.Fatalf("active queued run %q disappeared", persisted[0].ID)
	}
	runStore.mu.Lock()
	markCalls := runStore.markCalls
	runStore.mu.Unlock()
	if markCalls != 2 {
		t.Fatalf("accepted-binding calls = %d, want failed attempt plus successful retry", markCalls)
	}

	release()
	deadline := time.Now().Add(time.Second)
	for {
		provider.mu.Lock()
		calls := provider.calls
		provider.mu.Unlock()
		if calls >= 2 {
			if calls != 2 {
				t.Fatalf("provider calls = %d, want blocker plus one cron execution", calls)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("provider calls never reached 2; got %d", calls)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestCronRunEndpointResumesAcceptedQueuedRunDrainedByShutdown(t *testing.T) {
	runStore := store.NewMemoryStore()
	token, key := cronTestAPIKey(t, "tenant-accepted-queued", "remote cron accepted queued recovery", []string{store.ScopeRunsWrite})
	if err := runStore.CreateAPIKey(context.Background(), key); err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
	provider := &shutdownBlockingCronProvider{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	firstRunner := harness.NewRunner(
		provider,
		harness.NewRegistry(),
		harness.RunnerConfig{
			DefaultModel:        "gpt-4.1-mini",
			DefaultSystemPrompt: "You are helpful.",
			MaxSteps:            1,
			WorkerPoolSize:      1,
			Store:               runStore,
		},
	)
	if _, err := firstRunner.StartRun(harness.RunRequest{Prompt: "occupy worker"}); err != nil {
		t.Fatalf("StartRun blocker: %v", err)
	}
	select {
	case <-provider.started:
	case <-time.After(time.Second):
		t.Fatal("blocker did not occupy first runner worker")
	}

	leaseNow := time.Now().UTC()
	firstHandler := cronTestHandlerAt(firstRunner, runStore, leaseNow)
	payload := `{"prompt":"resume accepted queued run","tenant_id":"tenant-accepted-queued","job_id":"job-accepted-queued","execution_id":"execution-accepted-queued","correlation_key":"cron/job-accepted-queued/execution-accepted-queued"}`
	firstRequest := httptestRequest(t, http.MethodPost, "/v1/cron/runs", payload, token)
	firstRequest.Header.Set("Idempotency-Key", "cron/job-accepted-queued/execution-accepted-queued")
	firstResponse := httptestRecorder(firstHandler, firstRequest)
	if firstResponse.Code != http.StatusAccepted {
		t.Fatalf("first status = %d, body=%s; want 202", firstResponse.Code, firstResponse.Body.String())
	}
	var firstAccepted struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(firstResponse.Body.Bytes(), &firstAccepted); err != nil {
		t.Fatalf("decode first response: %v", err)
	}
	shutdownDone := make(chan error, 1)
	go func() { shutdownDone <- firstRunner.Shutdown(context.Background()) }()
	time.Sleep(10 * time.Millisecond)
	close(provider.release)
	select {
	case err := <-shutdownDone:
		if err != nil {
			t.Fatalf("Shutdown first runner: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("first runner shutdown did not drain queued work")
	}
	persisted, err := runStore.GetRun(context.Background(), firstAccepted.RunID)
	if err != nil {
		t.Fatalf("GetRun after shutdown: %v", err)
	}
	if persisted.Status != store.RunStatusQueued {
		t.Fatalf("persisted status after queued drain = %q, want queued", persisted.Status)
	}

	replacementRunner := testRunnerForCronStore(runStore)
	t.Cleanup(func() { _ = replacementRunner.Shutdown(context.Background()) })
	replacementHandler := cronTestHandlerAt(replacementRunner, runStore, leaseNow.Add(defaultCronRunDispatchLeaseDuration+time.Second))
	replay := httptestRequest(t, http.MethodPost, "/v1/cron/runs", payload, token)
	replay.Header.Set("Idempotency-Key", "cron/job-accepted-queued/execution-accepted-queued")
	replayResponse := httptestRecorder(replacementHandler, replay)
	if replayResponse.Code != http.StatusAccepted {
		t.Fatalf("replay status = %d, body=%s; want 202", replayResponse.Code, replayResponse.Body.String())
	}
	var replayed struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(replayResponse.Body.Bytes(), &replayed); err != nil {
		t.Fatalf("decode replay response: %v", err)
	}
	if replayed.RunID != firstAccepted.RunID {
		t.Fatalf("replayed run ID = %q, want accepted queued %q", replayed.RunID, firstAccepted.RunID)
	}
	if _, ok := replacementRunner.GetRun(firstAccepted.RunID); !ok {
		t.Fatalf("replacement runner did not resume accepted queued run %q", firstAccepted.RunID)
	}
}

func TestCronRunEndpointDurablyDeduplicatesAfterRestart(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "runs.db")
	firstStore, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore first: %v", err)
	}
	if err := firstStore.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate first: %v", err)
	}
	if err := firstStore.MigrateAPIKeys(context.Background()); err != nil {
		t.Fatalf("MigrateAPIKeys first: %v", err)
	}
	token, key := cronTestAPIKey(t, "tenant-restart", "remote cron restart", []string{store.ScopeRunsWrite})
	if err := firstStore.CreateAPIKey(context.Background(), key); err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}

	firstRunner := testRunnerForCronStore(firstStore)
	firstHandler := NewWithOptions(ServerOptions{Runner: firstRunner, Store: firstStore})
	payload := `{"prompt":"restart-safe prompt","tenant_id":"tenant-restart","agent_id":"agent-restart","conversation_id":"conversation-restart","job_id":"job-restart","execution_id":"execution-restart","correlation_key":"cron/job-restart/execution-restart"}`
	firstRequest := httptestRequest(t, http.MethodPost, "/v1/cron/runs", payload, token)
	firstRequest.Header.Set("Idempotency-Key", "cron/job-restart/execution-restart")
	firstResponse := httptestRecorder(firstHandler, firstRequest)
	if firstResponse.Code != http.StatusAccepted {
		t.Fatalf("first cron run status = %d, body=%s", firstResponse.Code, firstResponse.Body.String())
	}
	var accepted struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(firstResponse.Body.Bytes(), &accepted); err != nil || accepted.RunID == "" {
		t.Fatalf("first response = %s, err=%v", firstResponse.Body.String(), err)
	}
	if err := firstRunner.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown first runner: %v", err)
	}
	if err := firstStore.Close(); err != nil {
		t.Fatalf("Close first store: %v", err)
	}

	reopenedStore, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore reopened: %v", err)
	}
	t.Cleanup(func() { _ = reopenedStore.Close() })
	if err := reopenedStore.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate reopened: %v", err)
	}
	if err := reopenedStore.MigrateAPIKeys(context.Background()); err != nil {
		t.Fatalf("MigrateAPIKeys reopened: %v", err)
	}
	replacementRunner := testRunnerForCronStore(reopenedStore)
	t.Cleanup(func() { _ = replacementRunner.Shutdown(context.Background()) })
	replacementHandler := NewWithOptions(ServerOptions{Runner: replacementRunner, Store: reopenedStore})

	replayedRequest := httptestRequest(t, http.MethodPost, "/v1/cron/runs", payload, token)
	replayedRequest.Header.Set("Idempotency-Key", "cron/job-restart/execution-restart")
	replayedResponse := httptestRecorder(replacementHandler, replayedRequest)
	if replayedResponse.Code != http.StatusAccepted {
		t.Fatalf("replayed cron run status = %d, body=%s", replayedResponse.Code, replayedResponse.Body.String())
	}
	var replayed struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(replayedResponse.Body.Bytes(), &replayed); err != nil {
		t.Fatalf("replayed response = %s, err=%v", replayedResponse.Body.String(), err)
	}
	if replayed.RunID != accepted.RunID {
		t.Fatalf("replayed run_id = %q, want accepted run %q", replayed.RunID, accepted.RunID)
	}
	if _, startedAgain := replacementRunner.GetRun(accepted.RunID); startedAgain {
		t.Fatalf("replacement runner started accepted run %q again", accepted.RunID)
	}
}

func testRunnerForCronStore(runStore store.Store) *harness.Runner {
	return testRunnerWithCronProvider(runStore, &staticProvider{result: harness.CompletionResult{Content: "done"}})
}

func cronTestHandlerAt(runner *harness.Runner, runStore store.Store, now time.Time) http.Handler {
	s := &Server{
		runner:       runner,
		runStore:     runStore,
		timeNow:      func() time.Time { return now },
		authDisabled: authDisabledFromEnv(),
		mcpServers:   make(map[string]connectedMCPServer),
	}
	return s.buildMux()
}

func testRunnerWithCronProvider(runStore store.Store, provider harness.Provider) *harness.Runner {
	return harness.NewRunner(
		provider,
		harness.NewRegistry(),
		harness.RunnerConfig{
			DefaultModel:        "gpt-4.1-mini",
			DefaultSystemPrompt: "You are helpful.",
			MaxSteps:            1,
			Store:               runStore,
		},
	)
}

func assertSingleCronProviderDispatch(t *testing.T, provider *sharedCronDispatchProvider) {
	t.Helper()
	select {
	case <-provider.started:
	case <-time.After(time.Second):
		t.Fatal("cron provider was not dispatched")
	}
	deadline := time.Now().Add(100 * time.Millisecond)
	for time.Now().Before(deadline) {
		if calls := provider.callCount(); calls > 1 {
			t.Fatalf("cron provider dispatches = %d, want exactly 1", calls)
		}
		time.Sleep(time.Millisecond)
	}
	if calls := provider.callCount(); calls != 1 {
		t.Fatalf("cron provider dispatches = %d, want exactly 1", calls)
	}
}

func assertSingleCronRunnerAdmission(t *testing.T, runID string, runners ...*harness.Runner) {
	t.Helper()
	admissions := 0
	for _, runner := range runners {
		if _, exists := runner.GetRun(runID); exists {
			admissions++
		}
	}
	if admissions != 1 {
		t.Fatalf("runner admissions for %q = %d, want exactly 1", runID, admissions)
	}
}

func setCronRunLeaseNearExpiry(t *testing.T, db *sql.DB, tenantID, idempotencyKey string) int64 {
	t.Helper()
	var expiry int64
	err := db.QueryRow(`
UPDATE cron_run_starts
SET dispatch_lease_until = CAST((julianday('now') - 2440587.5) * 86400000000000 AS INTEGER) + 1000000000
WHERE tenant_id = ? AND idempotency_key = ?
RETURNING dispatch_lease_until
`, tenantID, idempotencyKey).Scan(&expiry)
	if err != nil {
		t.Fatalf("set cron run lease near expiry: %v", err)
	}
	return expiry
}

func readCronRunLeaseExpiry(t *testing.T, db *sql.DB, tenantID, idempotencyKey string) int64 {
	t.Helper()
	var expiry int64
	if err := db.QueryRow(`
SELECT dispatch_lease_until
FROM cron_run_starts
WHERE tenant_id = ? AND idempotency_key = ?
`, tenantID, idempotencyKey).Scan(&expiry); err != nil {
		t.Fatalf("read cron run lease expiry: %v", err)
	}
	return expiry
}

func expireCronRunLease(t *testing.T, db *sql.DB, tenantID, idempotencyKey string) {
	t.Helper()
	result, err := db.Exec(`
UPDATE cron_run_starts
SET dispatch_lease_until = 0
WHERE tenant_id = ? AND idempotency_key = ?
`, tenantID, idempotencyKey)
	if err != nil {
		t.Fatalf("expire cron run lease: %v", err)
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		t.Fatalf("expire cron run lease rows = %d err=%v, want 1", rows, err)
	}
}

func TestCronRunEndpointPreservesInitiatorAPIKeyPrefix(t *testing.T) {
	ms := store.NewMemoryStore()
	token, key := cronTestAPIKey(t, "tenant-remote", "remote cron", []string{store.ScopeRunsWrite})
	if err := ms.CreateAPIKey(context.Background(), key); err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}

	rolloutDir := t.TempDir()
	runner := harness.NewRunner(
		&staticProvider{result: harness.CompletionResult{Content: "done"}},
		harness.NewRegistry(),
		harness.RunnerConfig{
			DefaultModel:        "gpt-4.1-mini",
			DefaultSystemPrompt: "You are helpful.",
			MaxSteps:            1,
			RolloutDir:          rolloutDir,
			AuditTrailEnabled:   true,
			Store:               ms,
		},
	)
	h := NewWithOptions(ServerOptions{Runner: runner, Store: ms})
	body := `{"prompt":"audit remote prompt","tenant_id":"tenant-remote","job_id":"job-audit","execution_id":"execution-audit","correlation_key":"cron/job-audit/execution-audit"}`
	req := httptestRequest(t, http.MethodPost, "/v1/cron/runs", body, token)
	req.Header.Set("Idempotency-Key", "cron/job-audit/execution-audit")
	res := httptestRecorder(h, req)
	if res.Code != http.StatusAccepted {
		t.Fatalf("cron run status = %d, body=%s", res.Code, res.Body.String())
	}
	var response struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("response = %s, err=%v", res.Body.String(), err)
	}
	waitForAuditInitiatorPrefix(t, rolloutDir, response.RunID, token[:8])
}

func waitForAuditInitiatorPrefix(t *testing.T, rolloutDir, runID, want string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		found, err := auditInitiatorPrefix(rolloutDir, runID)
		if err == nil && found != "" {
			if found != want {
				t.Fatalf("audit initiator_api_key_prefix = %q, want %q", found, want)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for run.started audit entry for %q", runID)
}

func auditInitiatorPrefix(rolloutDir, runID string) (string, error) {
	var result string
	err := filepath.WalkDir(rolloutDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || entry.Name() != "audit.jsonl" {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			var auditEntry audittrail.AuditEntry
			if err := json.Unmarshal(scanner.Bytes(), &auditEntry); err != nil {
				return err
			}
			if auditEntry.RunID == runID && auditEntry.EventType == "run.started" {
				if value, ok := auditEntry.Payload["initiator_api_key_prefix"].(string); ok {
					result = value
					return filepath.SkipAll
				}
			}
		}
		return scanner.Err()
	})
	return result, err
}

func httptestRequest(t *testing.T, method, path, body, token string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(method, path, bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

func httptestRecorder(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, req)
	return recorder
}
