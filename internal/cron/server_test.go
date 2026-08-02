package cron

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	robfigcron "github.com/robfig/cron/v3"
)

func newTestServer(t *testing.T) (http.Handler, *mockStore) {
	t.Helper()
	store := &mockStore{
		GetJobFunc: func(_ context.Context, id string) (Job, error) {
			job := testJob("default-job")
			job.ID = id
			return job, nil
		},
	}
	clock := newMockClock(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	executor := &mockExecutor{}
	scheduler := NewScheduler(store, executor, clock, SchedulerConfig{MaxConcurrent: 1})
	handler := authenticatedTestHandler(NewServer(testTenantClaimStore{Store: store}, scheduler, clock, testIngressAuthConfig()))
	return handler, store
}

func testIngressAuthConfig() IngressAuthConfig {
	return IngressAuthConfig{APIKey: testIngressKey, TenantID: testIngressTenant}
}

func authenticatedTestHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" && r.Header.Get("Authorization") == "" {
			r.Header.Set("Authorization", "Bearer "+testIngressKey)
		}
		next.ServeHTTP(w, r)
	})
}

func authenticatedServer(store Store, scheduler *Scheduler, clock Clock) http.Handler {
	return authenticatedTestHandler(NewServer(store, scheduler, clock, testIngressAuthConfig()))
}

type testTenantClaimStore struct {
	Store
}

func (s testTenantClaimStore) ClaimJobTenant(ctx context.Context, jobID, tenantID string) (Job, bool, error) {
	job, err := s.GetJob(ctx, jobID)
	if err != nil {
		return Job{}, false, err
	}
	if job.TenantID == tenantID {
		return job, true, nil
	}
	if job.TenantID != "" || job.ExecType != ExecTypeShell {
		return job, false, nil
	}
	job.TenantID = tenantID
	if err := s.UpdateJob(ctx, job); err != nil {
		return Job{}, false, err
	}
	return job, true, nil
}

func TestServerHealth(t *testing.T) {
	handler, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected ok, got %q", body["status"])
	}
}

func TestServerListJobsFallbackFiltersCompleteScope(t *testing.T) {
	scope := Scope{TenantID: "tenant-a", ConversationID: "conversation-a", AgentID: "agent-a"}
	owned := testJob("owned")
	owned.TenantID, owned.ConversationID, owned.AgentID = scope.TenantID, scope.ConversationID, scope.AgentID
	otherTenant := testJob("other-tenant")
	otherTenant.TenantID, otherTenant.ConversationID, otherTenant.AgentID = "tenant-b", scope.ConversationID, scope.AgentID
	otherConversation := testJob("other-conversation")
	otherConversation.TenantID, otherConversation.ConversationID, otherConversation.AgentID = scope.TenantID, "conversation-b", scope.AgentID
	otherAgent := testJob("other-agent")
	otherAgent.TenantID, otherAgent.ConversationID, otherAgent.AgentID = scope.TenantID, scope.ConversationID, "agent-b"

	// mockStore deliberately implements Store but not ScopedStore. This proves
	// the compatibility fallback applies the complete ownership tuple instead
	// of returning an unfiltered cross-conversation list.
	store := &mockStore{ListJobsFunc: func(context.Context) ([]Job, error) {
		return []Job{owned, otherTenant, otherConversation, otherAgent}, nil
	}}
	clock := newMockClock(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	handler := authenticatedServer(store, NewScheduler(store, &mockExecutor{}, clock, SchedulerConfig{}), clock)
	req := httptest.NewRequest(http.MethodGet, "/v1/jobs", nil)
	req.Header.Set("X-Cron-Tenant-ID", scope.TenantID)
	req.Header.Set("X-Cron-Conversation-ID", scope.ConversationID)
	req.Header.Set("X-Cron-Agent-ID", scope.AgentID)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("scoped fallback list status = %d, body=%s", w.Code, w.Body.String())
	}
	var payload struct {
		Jobs []Job `json:"jobs"`
	}
	if err := json.NewDecoder(w.Body).Decode(&payload); err != nil {
		t.Fatalf("decode scoped fallback list: %v", err)
	}
	if len(payload.Jobs) != 1 || payload.Jobs[0].ID != owned.ID {
		t.Fatalf("scoped fallback list = %#v, want only %s", payload.Jobs, owned.ID)
	}
}

func TestRemoteClient_ScopeIsolatesCRUDAndHistory(t *testing.T) {
	store := newTestStore(t)
	clock := newMockClock(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	scheduler := NewScheduler(store, &mockExecutor{}, clock, SchedulerConfig{})
	ts := httptest.NewServer(authenticatedServer(store, scheduler, clock))
	t.Cleanup(ts.Close)
	client := NewClient(ts.URL)
	scopeA := Scope{TenantID: "tenant-a", ConversationID: "conversation", AgentID: "agent"}
	scopeB := Scope{TenantID: testIngressTenant, ConversationID: "conversation-b", AgentID: "agent-b"}
	create := func(scope Scope) Job {
		job, err := client.CreateJob(WithScope(context.Background(), scope), CreateJobRequest{TenantID: scope.TenantID, ConversationID: scope.ConversationID, AgentID: scope.AgentID, Name: "same-name", Schedule: "*/5 * * * *", ExecType: ExecTypeShell, ExecConfig: `{"command":"echo ok"}`})
		if err != nil {
			t.Fatalf("create %s: %v", scope.TenantID, err)
		}
		return job
	}
	jobA, jobB := create(scopeA), create(scopeB)
	if jobs, err := client.ListJobs(WithScope(context.Background(), scopeA)); err != nil || len(jobs) != 1 || jobs[0].ID != jobA.ID {
		t.Fatalf("scoped list = %#v, %v", jobs, err)
	}
	if _, err := client.GetJob(WithScope(context.Background(), scopeA), jobB.ID); !IsJobNotFound(err) {
		t.Fatalf("cross-scope get error = %v, want not found", err)
	}
	tags := "blocked"
	if _, err := client.UpdateJob(WithScope(context.Background(), scopeA), jobB.ID, UpdateJobRequest{Tags: &tags}); !IsJobNotFound(err) {
		t.Fatalf("cross-scope update error = %v, want not found", err)
	}
	if err := client.DeleteJob(WithScope(context.Background(), scopeA), jobB.ID); !IsJobNotFound(err) {
		t.Fatalf("cross-scope delete error = %v, want not found", err)
	}
	if _, err := store.CreateExecution(context.Background(), Execution{ID: "exec-b", JobID: jobB.ID, StartedAt: clock.Now(), Status: ExecStatusSuccess}); err != nil {
		t.Fatalf("create execution: %v", err)
	}
	if _, err := client.ListExecutions(WithScope(context.Background(), scopeA), jobB.ID, 10, 0); !IsJobNotFound(err) {
		t.Fatalf("cross-scope history error = %v, want not found", err)
	}
	if _, err := client.GetJob(WithScope(context.Background(), scopeB), "same-name"); !IsJobNotFound(err) {
		t.Fatalf("ID-only route accepted a name: %v", err)
	}
	if got, err := client.GetJobByName(WithScope(context.Background(), scopeB), "same-name"); err != nil || got.ID != jobB.ID {
		t.Fatalf("scoped operator name lookup = %#v, %v", got, err)
	}
	if _, err := client.GetJobByName(context.Background(), "same-name"); !IsJobAmbiguous(err) {
		t.Fatalf("global operator name lookup error = %v, want ambiguity", err)
	}

	ctxA := WithScope(context.Background(), scopeA)
	gotA, err := client.GetJob(ctxA, jobA.ID)
	if err != nil || gotA.ID != jobA.ID {
		t.Fatalf("owned get = %#v, %v", gotA, err)
	}
	if _, err := store.CreateExecution(context.Background(), Execution{ID: "exec-a", JobID: jobA.ID, StartedAt: clock.Now(), Status: ExecStatusSuccess, RunID: "run-a"}); err != nil {
		t.Fatalf("create owned execution: %v", err)
	}
	ownedHistory, err := client.ListExecutions(ctxA, jobA.ID, 10, 0)
	if err != nil || len(ownedHistory) != 1 || ownedHistory[0].RunID != "run-a" {
		t.Fatalf("owned history = %#v, %v", ownedHistory, err)
	}
	newSchedule, ownedTags := "15 * * * *", "owned-update"
	updated, err := client.UpdateJob(ctxA, jobA.ID, UpdateJobRequest{Schedule: &newSchedule, Tags: &ownedTags, ExpectedUpdatedAt: &gotA.UpdatedAt})
	if err != nil || updated.Schedule != newSchedule || updated.Tags != ownedTags || updated.ID != jobA.ID {
		t.Fatalf("owned update = %#v, %v", updated, err)
	}
	paused := StatusPaused
	pausedJob, err := client.UpdateJob(ctxA, jobA.ID, UpdateJobRequest{Status: &paused, ExpectedUpdatedAt: &updated.UpdatedAt})
	if err != nil || pausedJob.Status != StatusPaused || scheduler.HasEntry(jobA.ID) {
		t.Fatalf("owned pause = %#v, entry=%v, err=%v", pausedJob, scheduler.HasEntry(jobA.ID), err)
	}
	active := StatusActive
	resumed, err := client.UpdateJob(ctxA, jobA.ID, UpdateJobRequest{Status: &active, ExpectedUpdatedAt: &pausedJob.UpdatedAt})
	if err != nil || resumed.Status != StatusActive || !scheduler.HasEntry(jobA.ID) {
		t.Fatalf("owned resume = %#v, entry=%v, err=%v", resumed, scheduler.HasEntry(jobA.ID), err)
	}
	activeAgain, err := client.UpdateJob(ctxA, jobA.ID, UpdateJobRequest{Status: &active, ExpectedUpdatedAt: &resumed.UpdatedAt})
	if err != nil || activeAgain.Status != StatusActive || len(scheduler.cron.Entries()) != 2 {
		t.Fatalf("active resume duplicated live entries: job=%#v entries=%d err=%v", activeAgain, len(scheduler.cron.Entries()), err)
	}
	if err := client.DeleteJob(ctxA, jobA.ID); err != nil {
		t.Fatalf("owned delete: %v", err)
	}
	if scheduler.HasEntry(jobA.ID) {
		t.Fatal("owned delete left a live scheduler entry")
	}
	if _, err := client.GetJob(ctxA, jobA.ID); !IsJobNotFound(err) {
		t.Fatalf("owned deleted get error = %v, want not found", err)
	}
	if gotB, err := client.GetJob(WithScope(context.Background(), scopeB), jobB.ID); err != nil || gotB.ID != jobB.ID {
		t.Fatalf("scope B was affected by scope A lifecycle: %#v, %v", gotB, err)
	}
	if len(scheduler.cron.Entries()) != 1 {
		t.Fatalf("live entries after owned delete = %d, want only scope B", len(scheduler.cron.Entries()))
	}
}

func TestRemoteClient_ConcurrentUpdateDeleteNeverRearmsDeletedJob(t *testing.T) {
	store := newTestStore(t)
	clock := newMockClock(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	scheduler := NewScheduler(store, &mockExecutor{}, clock, SchedulerConfig{})
	ts := httptest.NewServer(authenticatedServer(store, scheduler, clock))
	t.Cleanup(ts.Close)
	client := NewClient(ts.URL)
	scope := Scope{TenantID: testIngressTenant, ConversationID: "conversation", AgentID: "agent"}
	ctx := WithScope(context.Background(), scope)
	job, err := client.CreateJob(ctx, CreateJobRequest{TenantID: scope.TenantID, ConversationID: scope.ConversationID, AgentID: scope.AgentID, Name: "concurrent", Schedule: "*/5 * * * *", ExecType: ExecTypeShell, ExecConfig: `{"command":"echo ok"}`})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	var updateErr, deleteErr error
	go func() {
		defer wg.Done()
		<-start
		schedule := "15 * * * *"
		_, updateErr = client.UpdateJob(ctx, job.ID, UpdateJobRequest{Schedule: &schedule, ExpectedUpdatedAt: &job.UpdatedAt})
	}()
	go func() { defer wg.Done(); <-start; deleteErr = client.DeleteJob(ctx, job.ID) }()
	close(start)
	wg.Wait()
	if deleteErr != nil {
		t.Fatalf("delete: %v", deleteErr)
	}
	if updateErr != nil && !IsJobNotFound(updateErr) && !IsJobConflict(updateErr) {
		t.Fatalf("update: %v", updateErr)
	}
	if _, err := client.GetJob(ctx, job.ID); !IsJobNotFound(err) {
		t.Fatalf("post-delete get = %v, want not found", err)
	}
	if scheduler.HasEntry(job.ID) || len(scheduler.cron.Entries()) != 0 {
		t.Fatalf("deleted job rearmed: has=%v entries=%d", scheduler.HasEntry(job.ID), len(scheduler.cron.Entries()))
	}
}

func TestServerUpdate_SchedulerReplacementFailureRollsBackPersistence(t *testing.T) {
	store := newTestStore(t)
	clock := newMockClock(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	scheduler := NewScheduler(store, &mockExecutor{}, clock, SchedulerConfig{})
	job := testJob("rollback-scheduler")
	job.ID, job.Schedule = "rollback-scheduler", "*/5 * * * *"
	if _, err := store.CreateJob(context.Background(), job); err != nil {
		t.Fatalf("create job: %v", err)
	}
	if err := scheduler.AddJob(job); err != nil {
		t.Fatalf("arm old job: %v", err)
	}
	scheduler.addFunc = func(string, func()) (robfigcron.EntryID, error) { return 0, fmt.Errorf("injected add failure") }
	handler := authenticatedServer(store, scheduler, clock)
	req := httptest.NewRequest(http.MethodPatch, "/v1/jobs/"+job.ID, strings.NewReader(`{"schedule":"0 * * * *"}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	persisted, err := store.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("load after failed replacement: %v", err)
	}
	if persisted != job {
		t.Fatalf("persisted after failed replacement = %#v, want exact pre-update %#v", persisted, job)
	}
	if !scheduler.HasEntry(job.ID) || len(scheduler.cron.Entries()) != 1 {
		t.Fatalf("failed replacement must preserve old live entry: has=%v entries=%d", scheduler.HasEntry(job.ID), len(scheduler.cron.Entries()))
	}
}

type updateCASFailingStore struct{ Store }

func (updateCASFailingStore) UpdateJobCAS(context.Context, Job, time.Time) error {
	return fmt.Errorf("injected update CAS failure")
}

func TestServerUpdate_PreparedReplacementCASFailureAbortsAndPreservesOldJob(t *testing.T) {
	baseStore := newTestStore(t)
	clock := newMockClock(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	scheduler := NewScheduler(baseStore, &mockExecutor{}, clock, SchedulerConfig{})
	job := testJob("prepared-cas-failure")
	job.ID, job.Schedule = "prepared-cas-failure", "*/5 * * * *"
	created, err := baseStore.CreateJob(context.Background(), job)
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	if err := scheduler.AddJob(created); err != nil {
		t.Fatalf("arm old job: %v", err)
	}
	handler := authenticatedServer(updateCASFailingStore{Store: baseStore}, scheduler, clock)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodPatch, "/v1/jobs/"+created.ID, strings.NewReader(`{"schedule":"0 * * * *"}`)))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	persisted, err := baseStore.GetJob(context.Background(), created.ID)
	if err != nil || persisted != created {
		t.Fatalf("persisted after CAS failure = %#v, %v; want exact old %#v", persisted, err, created)
	}
	if !scheduler.HasEntry(created.ID) || len(scheduler.cron.Entries()) != 1 {
		t.Fatalf("CAS failure must abort prepared entry and preserve old live entry: has=%v entries=%d", scheduler.HasEntry(created.ID), len(scheduler.cron.Entries()))
	}
}

func TestServerUpdate_RedundantActiveStatusDoesNotReregister(t *testing.T) {
	store := newTestStore(t)
	clock := newMockClock(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	scheduler := NewScheduler(store, &mockExecutor{}, clock, SchedulerConfig{})
	job := testJob("redundant-active")
	created, err := store.CreateJob(context.Background(), job)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := scheduler.AddJob(created); err != nil {
		t.Fatalf("arm: %v", err)
	}
	adds := 0
	scheduler.addFunc = func(string, func()) (robfigcron.EntryID, error) { adds++; return 0, fmt.Errorf("unexpected add") }
	handler := authenticatedServer(store, scheduler, clock)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodPatch, "/v1/jobs/"+created.ID, strings.NewReader(`{"status":"active","tags":"changed"}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	if adds != 0 || !scheduler.HasEntry(created.ID) || len(scheduler.cron.Entries()) != 1 {
		t.Fatalf("redundant active status mutated scheduler: adds=%d has=%v entries=%d", adds, scheduler.HasEntry(created.ID), len(scheduler.cron.Entries()))
	}
	persisted, err := store.GetJob(context.Background(), created.ID)
	if err != nil || persisted.Tags != "changed" || persisted.Status != StatusActive {
		t.Fatalf("persisted redundant-active update = %#v, %v", persisted, err)
	}
}

func TestServerUpdate_SchedulerFailureAndRollbackConflictConvergesFailClosed(t *testing.T) {
	store := newTestStore(t)
	clock := newMockClock(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	scheduler := NewScheduler(store, &mockExecutor{}, clock, SchedulerConfig{})
	job := testJob("rollback-conflict")
	job.ID, job.Schedule = "rollback-conflict", "*/5 * * * *"
	created, err := store.CreateJob(context.Background(), job)
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	if err := scheduler.AddJob(created); err != nil {
		t.Fatalf("arm old job: %v", err)
	}

	var touchOnce sync.Once
	scheduler.addFunc = func(string, func()) (robfigcron.EntryID, error) {
		touchOnce.Do(func() {
			current, getErr := store.GetJob(context.Background(), created.ID)
			if getErr != nil {
				t.Fatalf("load job before concurrent touch: %v", getErr)
			}
			if touchErr := store.TouchJobRun(
				context.Background(), current.ID, clock.Now(), current.NextRunAt, current.UpdatedAt.Add(time.Second),
			); touchErr != nil {
				t.Fatalf("concurrent touch: %v", touchErr)
			}
		})
		return 0, fmt.Errorf("injected persistent add failure")
	}

	handler := authenticatedServer(store, scheduler, clock)
	req := httptest.NewRequest(http.MethodPatch, "/v1/jobs/"+created.ID, strings.NewReader(`{"schedule":"0 * * * *"}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	persisted, err := store.GetJob(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("load after failed replacement: %v", err)
	}
	if persisted.Status != StatusActive || persisted.Schedule != created.Schedule || persisted.ExecConfig != created.ExecConfig {
		t.Fatalf("persisted after prepare failure = %#v, want original active config %#v", persisted, created)
	}
	if persisted.LastRunAt.IsZero() {
		t.Fatal("fail-closed convergence overwrote the concurrent TouchJobRun")
	}
	if !scheduler.HasEntry(created.ID) {
		t.Fatal("prepare failure removed the old runnable scheduler entry")
	}
}

func TestServerUpdate_AddFailureAndRollbackConflictConvergesFailClosed(t *testing.T) {
	store := newTestStore(t)
	clock := newMockClock(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	scheduler := NewScheduler(store, &mockExecutor{}, clock, SchedulerConfig{})
	job := testJob("resume-rollback-conflict")
	job.ID, job.Status = "resume-rollback-conflict", StatusPaused
	created, err := store.CreateJob(context.Background(), job)
	if err != nil {
		t.Fatalf("create paused job: %v", err)
	}

	var touchOnce sync.Once
	scheduler.addFunc = func(string, func()) (robfigcron.EntryID, error) {
		touchOnce.Do(func() {
			current, getErr := store.GetJob(context.Background(), created.ID)
			if getErr != nil {
				t.Fatalf("load job before concurrent touch: %v", getErr)
			}
			if touchErr := store.TouchJobRun(
				context.Background(), current.ID, clock.Now(), current.NextRunAt, current.UpdatedAt.Add(time.Second),
			); touchErr != nil {
				t.Fatalf("concurrent touch: %v", touchErr)
			}
		})
		return 0, fmt.Errorf("injected persistent add failure")
	}

	handler := authenticatedServer(store, scheduler, clock)
	req := httptest.NewRequest(http.MethodPatch, "/v1/jobs/"+created.ID, strings.NewReader(`{"status":"active"}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	persisted, err := store.GetJob(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("load after failed resume: %v", err)
	}
	if persisted.Status != StatusPaused {
		t.Fatalf("persisted status = %q, want fail-closed paused", persisted.Status)
	}
	if persisted.LastRunAt.IsZero() {
		t.Fatal("fail-closed convergence overwrote the concurrent TouchJobRun")
	}
	if scheduler.HasEntry(created.ID) {
		t.Fatal("fail-closed convergence left a runnable scheduler entry")
	}
}

func TestServerUpdate_RollbackConflictReloadsAndRestoresAuthoritativeActiveJob(t *testing.T) {
	store := newTestStore(t)
	clock := newMockClock(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	scheduler := NewScheduler(store, &mockExecutor{}, clock, SchedulerConfig{})
	job := testJob("reload-active")
	job.ID, job.Schedule = "reload-active", "*/5 * * * *"
	created, err := store.CreateJob(context.Background(), job)
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	if err := scheduler.AddJob(created); err != nil {
		t.Fatalf("arm old job: %v", err)
	}

	realAdd := scheduler.addFunc
	addAttempts := 0
	scheduler.addFunc = func(spec string, callback func()) (robfigcron.EntryID, error) {
		addAttempts++
		if addAttempts == 1 {
			current, getErr := store.GetJob(context.Background(), created.ID)
			if getErr != nil {
				t.Fatalf("load job before concurrent touch: %v", getErr)
			}
			if touchErr := store.TouchJobRun(
				context.Background(), current.ID, clock.Now(), current.NextRunAt, current.UpdatedAt.Add(time.Second),
			); touchErr != nil {
				t.Fatalf("concurrent touch: %v", touchErr)
			}
			return 0, fmt.Errorf("injected one-shot replacement failure")
		}
		return realAdd(spec, callback)
	}

	handler := authenticatedServer(store, scheduler, clock)
	req := httptest.NewRequest(http.MethodPatch, "/v1/jobs/"+created.ID, strings.NewReader(`{"schedule":"0 * * * *"}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	persisted, err := store.GetJob(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("load after recovered replacement: %v", err)
	}
	if persisted.Status != StatusActive || persisted.Schedule != created.Schedule || persisted.ExecConfig != created.ExecConfig {
		t.Fatalf("persisted job = %#v, want original active config %#v", persisted, created)
	}
	if persisted.LastRunAt.IsZero() {
		t.Fatal("fail-closed staging overwrote the concurrent TouchJobRun")
	}
	if !scheduler.HasEntry(created.ID) || len(scheduler.cron.Entries()) != 1 {
		t.Fatalf("prepare failure lost old live entry: has=%v entries=%d", scheduler.HasEntry(created.ID), len(scheduler.cron.Entries()))
	}
	if addAttempts != 1 {
		t.Fatalf("add attempts = %d, want one failed prepared registration", addAttempts)
	}
}

type deleteFailingStore struct {
	Store
	err error
}

func (s *deleteFailingStore) DeleteJob(context.Context, string) error     { return s.err }
func (s *deleteFailingStore) DeactivateJob(context.Context, string) error { return s.err }

func TestServerCreate_SchedulerAndDeleteFailureLeavesDurablyInactiveJob(t *testing.T) {
	baseStore := newTestStore(t)
	store := &deleteFailingStore{Store: baseStore, err: fmt.Errorf("injected delete failure")}
	clock := newMockClock(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	scheduler := NewScheduler(store, &mockExecutor{}, clock, SchedulerConfig{})
	scheduler.addFunc = func(string, func()) (robfigcron.EntryID, error) {
		return 0, fmt.Errorf("injected add failure")
	}
	handler := authenticatedServer(store, scheduler, clock)
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs", strings.NewReader(
		`{"name":"create-fail-closed","schedule":"*/5 * * * *","execution_type":"shell","execution_config":"{\"command\":\"echo ok\"}"}`,
	))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	jobs, err := baseStore.ListJobs(context.Background())
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("jobs = %d, want one retained fail-closed record", len(jobs))
	}
	if jobs[0].Status != StatusPaused {
		t.Fatalf("retained status = %q, want paused", jobs[0].Status)
	}
	if scheduler.HasEntry(jobs[0].ID) {
		t.Fatal("failed create left a runnable scheduler entry")
	}
}

type activationFailingStore struct{ Store }

func (activationFailingStore) UpdateJobCAS(context.Context, Job, time.Time) error {
	return fmt.Errorf("injected activation failure")
}

func TestServerCreate_ActivationFailureNeverRestartRearmsPausedJob(t *testing.T) {
	baseStore := newTestStore(t)
	clock := newMockClock(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	store := activationFailingStore{Store: baseStore}
	scheduler := NewScheduler(store, &mockExecutor{}, clock, SchedulerConfig{})
	handler := authenticatedServer(store, scheduler, clock)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/jobs", strings.NewReader(`{"name":"activation-fail","schedule":"*/5 * * * *","execution_type":"shell","execution_config":"{\"command\":\"echo ok\"}"}`)))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	jobs, err := baseStore.ListJobs(context.Background())
	if err != nil || len(jobs) != 1 || jobs[0].Status != StatusPaused {
		t.Fatalf("durable create after activation failure = %#v, %v", jobs, err)
	}
	if scheduler.HasEntry(jobs[0].ID) {
		t.Fatal("activation failure left a live entry")
	}
	restarted := NewScheduler(baseStore, &mockExecutor{}, clock, SchedulerConfig{})
	if err := restarted.Start(context.Background()); err != nil {
		t.Fatalf("restart: %v", err)
	}
	t.Cleanup(restarted.Stop)
	if restarted.HasEntry(jobs[0].ID) {
		t.Fatal("paused failed create rearmed after restart")
	}
}

func TestServerCreateJob(t *testing.T) {
	handler, store := newTestServer(t)
	store.CreateJobFunc = func(_ context.Context, job Job) (Job, error) {
		return job, nil
	}

	payload := `{"name":"test-job","schedule":"*/5 * * * *","execution_type":"shell","execution_config":"{\"command\":\"echo hi\"}"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs", strings.NewReader(payload))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var job Job
	if err := json.NewDecoder(w.Body).Decode(&job); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if job.Name != "test-job" {
		t.Fatalf("expected test-job, got %q", job.Name)
	}
	if job.Status != StatusActive {
		t.Fatalf("expected active, got %q", job.Status)
	}
	if job.ID == "" {
		t.Fatalf("expected non-empty ID")
	}
}

func TestServerCreateJobValidation(t *testing.T) {
	handler, _ := newTestServer(t)

	tests := []struct {
		name    string
		payload string
		errMsg  string
	}{
		{"missing name", `{"schedule":"* * * * *","execution_type":"shell"}`, "name is required"},
		{"missing schedule", `{"name":"x","execution_type":"shell"}`, "schedule is required"},
		{"bad schedule", `{"name":"x","schedule":"bad","execution_type":"shell"}`, "invalid schedule"},
		{"bad exec type", `{"name":"x","schedule":"* * * * *","execution_type":"bad"}`, "execution_type"},
		{"invalid json", `{bad`, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/jobs", strings.NewReader(tt.payload))
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
			if tt.errMsg != "" && !strings.Contains(w.Body.String(), tt.errMsg) {
				t.Fatalf("expected error containing %q, got %s", tt.errMsg, w.Body.String())
			}
		})
	}
}

func TestServerCreateJobRejectsUnsafeExecutionConfigAndTimeout(t *testing.T) {
	handler, _ := newTestServer(t)
	tests := []struct {
		name    string
		payload string
		errMsg  string
	}{
		{"empty shell command", `{"name":"x","schedule":"* * * * *","execution_type":"shell","execution_config":"{\"command\":\"\"}"}`, "non-empty command"},
		{"incomplete harness prompt", `{"name":"x","schedule":"* * * * *","execution_type":"harness","execution_config":"{\"prompt\":\" \"}"}`, "non-empty prompt"},
		{"zero timeout", `{"name":"x","schedule":"* * * * *","execution_type":"shell","execution_config":"{\"command\":\"echo hi\"}","timeout_seconds":0}`, "timeout_seconds must be positive"},
		{"negative timeout", `{"name":"x","schedule":"* * * * *","execution_type":"shell","execution_config":"{\"command\":\"echo hi\"}","timeout_seconds":-1}`, "timeout_seconds must be positive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/jobs", strings.NewReader(tt.payload))
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tt.errMsg) {
				t.Fatalf("expected error containing %q, got %s", tt.errMsg, w.Body.String())
			}
		})
	}
}

func TestServerListJobs(t *testing.T) {
	handler, store := newTestServer(t)
	j := testJob("list-test")
	store.ListJobsFunc = func(_ context.Context) ([]Job, error) {
		return []Job{j}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/jobs", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var result struct {
		Jobs []Job `json:"jobs"`
	}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(result.Jobs))
	}
}

func TestServerListJobsEmpty(t *testing.T) {
	handler, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/jobs", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var result struct {
		Jobs []Job `json:"jobs"`
	}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result.Jobs) != 0 {
		t.Fatalf("expected 0 jobs, got %d", len(result.Jobs))
	}
}

func TestServerGetJobByID(t *testing.T) {
	handler, store := newTestServer(t)
	j := testJob("get-test")
	store.GetJobFunc = func(_ context.Context, id string) (Job, error) {
		if id == j.ID {
			return j, nil
		}
		return Job{}, sql.ErrNoRows
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/jobs/"+j.ID, nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var got Job
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Name != "get-test" {
		t.Fatalf("expected get-test, got %q", got.Name)
	}
}

func TestServerOperatorGetJobByNameUsesDistinctRoute(t *testing.T) {
	handler, store := newTestServer(t)
	j := testJob("named-job")
	j.TenantID = testIngressTenant
	store.GetJobFunc = func(_ context.Context, id string) (Job, error) {
		return Job{}, sql.ErrNoRows
	}
	store.GetJobByNameFunc = func(_ context.Context, name string) (Job, error) {
		if name == "named-job" {
			return j, nil
		}
		return Job{}, sql.ErrNoRows
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/jobs/by-name?name=named-job", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var got Job
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Name != "named-job" {
		t.Fatalf("expected named-job, got %q", got.Name)
	}
}

func TestOperatorNameLookupQueryRoundTripsArbitraryNonEmptyNames(t *testing.T) {
	store := newTestStore(t)
	clock := newMockClock(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	scheduler := NewScheduler(store, &mockExecutor{}, clock, SchedulerConfig{})
	ts := httptest.NewServer(authenticatedServer(store, scheduler, clock))
	t.Cleanup(ts.Close)
	client := NewClient(ts.URL)
	for i, name := range []string{"folder/nightly", "nightly report", "100% ready", "日本語-☃"} {
		job, err := client.CreateJob(context.Background(), CreateJobRequest{Name: name, Schedule: "0 0 * * *", ExecType: ExecTypeShell, ExecConfig: `{"command":"echo ok"}`})
		if err != nil {
			t.Fatalf("create %q: %v", name, err)
		}
		got, err := client.GetJobByName(context.Background(), name)
		if err != nil {
			t.Fatalf("GetJobByName(%q): %v", name, err)
		}
		if got.ID != job.ID || got.Name != name {
			t.Fatalf("GetJobByName(%q) = %#v, want ID %q", name, got, job.ID)
		}
		if i == 0 {
			if _, err := client.GetJob(context.Background(), name); err == nil {
				t.Fatal("ID route accepted slash name")
			}
		}
	}
}

func TestOperatorNameLookupQueryRejectsEmptyAndNonGET(t *testing.T) {
	handler, _ := newTestServer(t)
	for _, tt := range []struct {
		method, target string
		status         int
		allow          string
	}{
		{http.MethodGet, "/v1/jobs/by-name", http.StatusBadRequest, ""},
		{http.MethodGet, "/v1/jobs/by-name?name=", http.StatusBadRequest, ""},
		{http.MethodPost, "/v1/jobs/by-name?name=job", http.StatusMethodNotAllowed, "GET"},
	} {
		req := httptest.NewRequest(tt.method, tt.target, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != tt.status {
			t.Fatalf("%s %s status=%d body=%s, want %d", tt.method, tt.target, w.Code, w.Body.String(), tt.status)
		}
		if tt.allow != "" && w.Header().Get("Allow") != tt.allow {
			t.Fatalf("%s %s Allow=%q, want %q", tt.method, tt.target, w.Header().Get("Allow"), tt.allow)
		}
	}
	client := NewClient("http://127.0.0.1:1")
	if _, err := client.GetJobByName(context.Background(), ""); err == nil || !strings.Contains(err.Error(), "name is required") {
		t.Fatalf("empty client lookup error=%v, want name is required", err)
	}
}

func TestServerJobIDRouteNeverFallsBackToName(t *testing.T) {
	handler, store := newTestServer(t)
	store.GetJobFunc = func(context.Context, string) (Job, error) { return Job{}, sql.ErrNoRows }
	nameLookups := 0
	store.GetJobByNameFunc = func(context.Context, string) (Job, error) { nameLookups++; return testJob("named-job"), nil }
	req := httptest.NewRequest(http.MethodGet, "/v1/jobs/named-job", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	if nameLookups != 0 {
		t.Fatalf("ID route performed %d name lookups", nameLookups)
	}
}

func TestServerGetJobNotFound(t *testing.T) {
	handler, store := newTestServer(t)
	store.GetJobFunc = func(_ context.Context, id string) (Job, error) {
		return Job{}, sql.ErrNoRows
	}
	store.GetJobByNameFunc = func(_ context.Context, name string) (Job, error) {
		return Job{}, sql.ErrNoRows
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/jobs/nonexistent", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestServerUpdateJobSchedule(t *testing.T) {
	handler, store := newTestServer(t)
	j := testJob("update-test")
	store.GetJobFunc = func(_ context.Context, id string) (Job, error) {
		if id == j.ID {
			return j, nil
		}
		return Job{}, sql.ErrNoRows
	}
	var updated Job
	store.UpdateJobFunc = func(_ context.Context, job Job) error {
		updated = job
		return nil
	}

	newSchedule := "0 * * * *"
	payload := fmt.Sprintf(`{"schedule":"%s"}`, newSchedule)
	req := httptest.NewRequest(http.MethodPatch, "/v1/jobs/"+j.ID, strings.NewReader(payload))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if updated.Schedule != newSchedule {
		t.Fatalf("expected schedule %q, got %q", newSchedule, updated.Schedule)
	}
}

func TestServerUpdateJobRejectsNonpositiveTimeoutWithoutMutationOrDispatch(t *testing.T) {
	for _, timeout := range []int{0, -1} {
		t.Run(fmt.Sprintf("timeout_%d", timeout), func(t *testing.T) {
			job := testJob("patch-timeout")
			job.ExecType = ExecTypeHarness
			job.TenantID = testIngressTenant
			job.ExecConfig = `{"prompt":"keep validated timeout"}`
			job.TimeoutSec = 60
			var updates atomic.Int32
			var dispatches atomic.Int32
			runStore := &mockStore{
				GetJobFunc: func(_ context.Context, id string) (Job, error) {
					if id != job.ID {
						return Job{}, sql.ErrNoRows
					}
					return job, nil
				},
				UpdateJobCASFunc: func(context.Context, Job, time.Time) error {
					updates.Add(1)
					return nil
				},
			}
			clock := newMockClock(time.Now().UTC())
			scheduler := NewScheduler(runStore, &mockExecutor{ExecuteFunc: func(context.Context, Job) (string, error) {
				dispatches.Add(1)
				return "unexpected", nil
			}}, clock, SchedulerConfig{MaxConcurrent: 1})
			handler := authenticatedTestHandler(NewServer(testTenantClaimStore{Store: runStore}, scheduler, clock, testIngressAuthConfig()))

			body := fmt.Sprintf(`{"timeout_seconds":%d}`, timeout)
			request := httptest.NewRequest(http.MethodPatch, "/v1/jobs/"+job.ID, strings.NewReader(body))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "timeout_seconds must be positive") {
				t.Fatalf("PATCH status = %d, body=%s; want actionable 400", response.Code, response.Body.String())
			}
			if updates.Load() != 0 || dispatches.Load() != 0 {
				t.Fatalf("rejected PATCH updates=%d dispatches=%d, want zero", updates.Load(), dispatches.Load())
			}
		})
	}
}

// TestServerUpdateJobSchedule_PausedJobNotReArmed (BT-005, P2) reproduces
// BUG 4: PATCHing only the schedule of a PAUSED job re-arms it in the live
// scheduler. The re-arm condition in handleUpdateJob (~line 239) is
// `req.Schedule != nil && (req.Status == nil || *req.Status == StatusActive)`.
// A schedule-only PATCH leaves req.Status nil, so a paused job is added
// back to the live scheduler even though its stored status remains
// "paused" — a paused job starts firing (or, after the BUG-2 fix, at least
// sits incorrectly registered in the scheduler's live entry set).
//
// This test fails before the fix (the job ends up in scheduler.entries)
// and passes after the fix (gating on the job's effective post-update
// status rather than the request's status field).
func TestServerUpdateJobSchedule_PausedJobNotReArmed(t *testing.T) {
	store := &mockStore{}
	clock := newMockClock(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	executor := &mockExecutor{}
	scheduler := NewScheduler(store, executor, clock, SchedulerConfig{MaxConcurrent: 1})
	handler := authenticatedTestHandler(NewServer(testTenantClaimStore{Store: store}, scheduler, clock, testIngressAuthConfig()))

	// j is already paused and, per the real job lifecycle, was removed
	// from the live scheduler when it was paused — it is NOT in
	// scheduler.entries at the start of this test.
	j := testJob("patch-paused-schedule")
	j.Status = StatusPaused

	store.GetJobFunc = func(_ context.Context, id string) (Job, error) {
		return j, nil
	}
	store.UpdateJobFunc = func(_ context.Context, job Job) error { return nil }

	// PATCH only the schedule — no "status" field in the request body.
	newSchedule := "0 * * * *"
	payload := fmt.Sprintf(`{"schedule":"%s"}`, newSchedule)
	req := httptest.NewRequest(http.MethodPatch, "/v1/jobs/"+j.ID, strings.NewReader(payload))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	scheduler.mu.Lock()
	_, scheduled := scheduler.entries[j.ID]
	scheduler.mu.Unlock()
	if scheduled {
		t.Fatal("expected a schedule-only PATCH on a paused job to NOT re-arm it in the live scheduler, but it was added to scheduler.entries")
	}
}

func TestServerUpdateJobPause(t *testing.T) {
	handler, store := newTestServer(t)
	j := testJob("pause-test")
	store.GetJobFunc = func(_ context.Context, id string) (Job, error) {
		return j, nil
	}
	var updated Job
	store.UpdateJobFunc = func(_ context.Context, job Job) error {
		updated = job
		return nil
	}

	payload := `{"status":"paused"}`
	req := httptest.NewRequest(http.MethodPatch, "/v1/jobs/"+j.ID, strings.NewReader(payload))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if updated.Status != StatusPaused {
		t.Fatalf("expected paused, got %q", updated.Status)
	}
}

func TestServerUpdateJobResume(t *testing.T) {
	handler, store := newTestServer(t)
	j := testJob("resume-test")
	j.Status = StatusPaused
	store.GetJobFunc = func(_ context.Context, id string) (Job, error) {
		return j, nil
	}
	store.UpdateJobFunc = func(_ context.Context, job Job) error {
		return nil
	}

	payload := `{"status":"active"}`
	req := httptest.NewRequest(http.MethodPatch, "/v1/jobs/"+j.ID, strings.NewReader(payload))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestServerUpdateJobResumeAndSchedule_ReArms (regression for BUG 4) is the
// positive counterpart of TestServerUpdateJobSchedule_PausedJobNotReArmed:
// a PATCH that resumes AND reschedules a paused job in the same request
// must still re-arm it in the live scheduler. This guards against
// overcorrecting the BUG-4 fix into never re-arming (e.g. accidentally
// checking req.Status instead of the freshly-set job.Status, or checking
// the OLD status instead of the effective one).
func TestServerUpdateJobResumeAndSchedule_ReArms(t *testing.T) {
	store := &mockStore{}
	clock := newMockClock(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	executor := &mockExecutor{}
	scheduler := NewScheduler(store, executor, clock, SchedulerConfig{MaxConcurrent: 1})
	handler := authenticatedTestHandler(NewServer(testTenantClaimStore{Store: store}, scheduler, clock, testIngressAuthConfig()))

	j := testJob("resume-and-schedule")
	j.Status = StatusPaused

	store.GetJobFunc = func(_ context.Context, id string) (Job, error) {
		return j, nil
	}
	store.UpdateJobFunc = func(_ context.Context, job Job) error { return nil }

	payload := `{"status":"active","schedule":"0 * * * *"}`
	req := httptest.NewRequest(http.MethodPatch, "/v1/jobs/"+j.ID, strings.NewReader(payload))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	scheduler.mu.Lock()
	_, scheduled := scheduler.entries[j.ID]
	scheduler.mu.Unlock()
	if !scheduled {
		t.Fatal("expected a resume+schedule PATCH to re-arm the job in the live scheduler")
	}
}

func TestServerUpdateJobNotFound(t *testing.T) {
	handler, store := newTestServer(t)
	store.GetJobFunc = func(_ context.Context, id string) (Job, error) {
		return Job{}, sql.ErrNoRows
	}

	req := httptest.NewRequest(http.MethodPatch, "/v1/jobs/missing", strings.NewReader(`{"status":"paused"}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestServerUpdateJobInvalidJSON(t *testing.T) {
	handler, store := newTestServer(t)
	j := testJob("bad-json")
	store.GetJobFunc = func(_ context.Context, id string) (Job, error) {
		return j, nil
	}

	req := httptest.NewRequest(http.MethodPatch, "/v1/jobs/"+j.ID, strings.NewReader(`{bad`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestServerUpdateJobRejectsStaleExpectedUpdatedAt(t *testing.T) {
	handler, store := newTestServer(t)
	job := testJob("stale-update")
	job.UpdatedAt = time.Date(2026, 7, 31, 1, 0, 0, 0, time.UTC)
	store.GetJobFunc = func(_ context.Context, id string) (Job, error) {
		if id == job.ID {
			return job, nil
		}
		return Job{}, sql.ErrNoRows
	}
	store.UpdateJobCASFunc = func(_ context.Context, _ Job, expected time.Time) error {
		if !expected.Equal(job.UpdatedAt) {
			return ErrJobConflict
		}
		return nil
	}

	req := httptest.NewRequest(http.MethodPatch, "/v1/jobs/"+job.ID, strings.NewReader(`{"tags":"stale","expected_updated_at":"2026-07-31T00:00:00Z"}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 conflict, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestServerUpdateJobRejectsUnsafeExecutionConfig(t *testing.T) {
	handler, store := newTestServer(t)
	job := testJob("unsafe-update")
	store.GetJobFunc = func(_ context.Context, id string) (Job, error) {
		if id == job.ID {
			return job, nil
		}
		return Job{}, sql.ErrNoRows
	}
	store.UpdateJobCASFunc = func(context.Context, Job, time.Time) error { return nil }

	tests := []struct {
		name    string
		payload string
		errMsg  string
	}{
		{"empty shell command", `{"execution_config":"{\"command\":\"\"}"}`, "non-empty command"},
		{"incomplete harness prompt", `{"execution_config":"{\"prompt\":\"\"}"}`, "non-empty command"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current := job
			if tt.name == "incomplete harness prompt" {
				current.ExecType = ExecTypeHarness
				current.ExecConfig = `{"prompt":"valid"}`
				store.GetJobFunc = func(_ context.Context, id string) (Job, error) {
					if id == current.ID {
						return current, nil
					}
					return Job{}, sql.ErrNoRows
				}
				tt.errMsg = "non-empty prompt"
			}
			req := httptest.NewRequest(http.MethodPatch, "/v1/jobs/"+current.ID, strings.NewReader(tt.payload))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), tt.errMsg) {
				t.Fatalf("response = %d %s, want actionable 400 containing %q", rec.Code, rec.Body.String(), tt.errMsg)
			}
		})
	}
}

func TestServerDeleteJob(t *testing.T) {
	handler, store := newTestServer(t)
	deleted := false
	store.DeleteJobFunc = func(_ context.Context, id string) error {
		deleted = true
		return nil
	}

	req := httptest.NewRequest(http.MethodDelete, "/v1/jobs/some-id", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	if !deleted {
		t.Fatalf("expected delete to be called")
	}
}

func TestServerHistory(t *testing.T) {
	handler, store := newTestServer(t)
	exec := Execution{
		ID:     "exec-1",
		JobID:  "job-1",
		Status: ExecStatusSkipped,
		RunID:  "run-linked",
		Error:  ErrExecutionSkippedOverlap.Error(),
	}
	store.ListExecutionsFunc = func(_ context.Context, jobID string, limit, offset int) ([]Execution, error) {
		return []Execution{exec}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/jobs/job-1/history?limit=10&offset=0", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var result struct {
		Executions []Execution `json:"executions"`
	}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result.Executions) != 1 {
		t.Fatalf("expected 1 execution, got %d", len(result.Executions))
	}
	if got := result.Executions[0]; got.Status != ExecStatusSkipped || got.RunID != "run-linked" || got.Error != ErrExecutionSkippedOverlap.Error() {
		t.Fatalf("history lifecycle fields = %#v", got)
	}
}

func TestServerHistoryEmpty(t *testing.T) {
	handler, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/jobs/job-1/history", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var result struct {
		Executions []Execution `json:"executions"`
	}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result.Executions) != 0 {
		t.Fatalf("expected 0 executions, got %d", len(result.Executions))
	}
}

func TestServerMethodNotAllowed(t *testing.T) {
	handler, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/v1/jobs", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestServerJobByIDMethodNotAllowed(t *testing.T) {
	handler, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/v1/jobs/some-id", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestServerHistoryMethodNotAllowed(t *testing.T) {
	handler, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/v1/jobs/some-id/history", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestServerJobByIDNotFound(t *testing.T) {
	handler, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/jobs/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestServerJobByIDUnknownSubpath(t *testing.T) {
	handler, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/jobs/some-id/unknown", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestNextRunTime(t *testing.T) {
	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	next, err := NextRunTime("*/5 * * * *", from)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Date(2025, 1, 1, 0, 5, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, next)
	}

	_, err = NextRunTime("bad-schedule", from)
	if err == nil {
		t.Fatalf("expected error for bad schedule")
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"key": "value"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %q", ct)
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusBadRequest, "test_error", "test message")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	errObj, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object")
	}
	if errObj["code"] != "test_error" {
		t.Fatalf("expected test_error, got %v", errObj["code"])
	}
}

func TestWriteMethodNotAllowed(t *testing.T) {
	w := httptest.NewRecorder()
	writeMethodNotAllowed(w, "GET, POST")
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
	if allow := w.Header().Get("Allow"); allow != "GET, POST" {
		t.Fatalf("expected Allow: GET, POST, got %q", allow)
	}
}

func TestServerPatchInvalidStatus(t *testing.T) {
	handler, store := newTestServer(t)
	j := testJob("invalid-status-test")
	store.GetJobFunc = func(_ context.Context, id string) (Job, error) {
		return j, nil
	}

	payload := `{"status":"banana"}`
	req := httptest.NewRequest(http.MethodPatch, "/v1/jobs/"+j.ID, strings.NewReader(payload))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid status, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "status must be") {
		t.Fatalf("expected status validation error, got %s", w.Body.String())
	}
}

func TestServerLargeRequestBody(t *testing.T) {
	handler, _ := newTestServer(t)

	// Create a body larger than 1MB.
	bigBody := strings.Repeat("a", 1<<20+1)
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs", strings.NewReader(bigBody))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for oversized body, got %d: %s", w.Code, w.Body.String())
	}
}

func TestServerPatchEmptySchedule(t *testing.T) {
	handler, store := newTestServer(t)
	j := testJob("empty-sched")
	store.GetJobFunc = func(_ context.Context, id string) (Job, error) {
		return j, nil
	}

	payload := `{"schedule":""}`
	req := httptest.NewRequest(http.MethodPatch, "/v1/jobs/"+j.ID, strings.NewReader(payload))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty schedule, got %d: %s", w.Code, w.Body.String())
	}
}

func TestServerPatchWhitespaceSchedule(t *testing.T) {
	handler, store := newTestServer(t)
	j := testJob("ws-sched")
	store.GetJobFunc = func(_ context.Context, id string) (Job, error) {
		return j, nil
	}

	payload := `{"schedule":"  "}`
	req := httptest.NewRequest(http.MethodPatch, "/v1/jobs/"+j.ID, strings.NewReader(payload))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for whitespace schedule, got %d: %s", w.Code, w.Body.String())
	}
}

func TestServer_ConcurrentRequests(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "concurrent-server.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	clock := RealClock{}
	scheduler := NewScheduler(store, &ShellExecutor{}, clock, SchedulerConfig{MaxConcurrent: 2})
	handler := authenticatedTestHandler(NewServer(testTenantClaimStore{Store: store}, scheduler, clock, testIngressAuthConfig()))

	// First create 20 jobs sequentially so they all exist.
	var jobIDs []string
	for g := 0; g < 20; g++ {
		name := fmt.Sprintf("concurrent-server-%d", g)
		payload := fmt.Sprintf(`{"name":"%s","schedule":"*/5 * * * *","execution_type":"shell","execution_config":"{\"command\":\"echo hi\"}"}`, name)
		req := httptest.NewRequest(http.MethodPost, "/v1/jobs", strings.NewReader(payload))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("setup create %d: expected 201, got %d: %s", g, w.Code, w.Body.String())
		}
		var job Job
		if err := json.NewDecoder(w.Body).Decode(&job); err != nil {
			t.Fatalf("setup decode %d: %v", g, err)
		}
		jobIDs = append(jobIDs, job.ID)
	}

	// Now do concurrent reads, gets, and deletes. The goal is race detection.
	var wg sync.WaitGroup
	var panicCount int32

	for g := 0; g < 20; g++ {
		wg.Add(1)
		go func(gID int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt32(&panicCount, 1)
				}
			}()

			// List jobs.
			req := httptest.NewRequest(http.MethodGet, "/v1/jobs", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			// Get a job.
			req = httptest.NewRequest(http.MethodGet, "/v1/jobs/"+jobIDs[gID], nil)
			w = httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			// Delete the job (may get SQLITE_BUSY — that's OK for race detection).
			req = httptest.NewRequest(http.MethodDelete, "/v1/jobs/"+jobIDs[gID], nil)
			w = httptest.NewRecorder()
			handler.ServeHTTP(w, req)
		}(g)
	}

	wg.Wait()

	if atomic.LoadInt32(&panicCount) > 0 {
		t.Fatalf("concurrent requests caused %d panics", panicCount)
	}
}

func TestServerCreateJobStoreError(t *testing.T) {
	handler, store := newTestServer(t)
	store.CreateJobFunc = func(_ context.Context, job Job) (Job, error) {
		return Job{}, fmt.Errorf("store failure")
	}

	payload := `{"name":"err-job","schedule":"* * * * *","execution_type":"shell","execution_config":"{\"command\":\"echo hi\"}"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs", strings.NewReader(payload))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestServerUpdateJobInvalidSchedule(t *testing.T) {
	handler, store := newTestServer(t)
	j := testJob("bad-sched")
	store.GetJobFunc = func(_ context.Context, id string) (Job, error) {
		return j, nil
	}

	payload := `{"schedule":"bad"}`
	req := httptest.NewRequest(http.MethodPatch, "/v1/jobs/"+j.ID, strings.NewReader(payload))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestServerCreateJobDefaultTimeout(t *testing.T) {
	handler, store := newTestServer(t)
	var created Job
	store.CreateJobFunc = func(_ context.Context, job Job) (Job, error) {
		created = job
		return job, nil
	}

	payload := `{"name":"default-timeout","schedule":"* * * * *","execution_type":"shell","execution_config":"{\"command\":\"echo hi\"}"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs", strings.NewReader(payload))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if created.TimeoutSec != 30 {
		t.Fatalf("expected default timeout 30, got %d", created.TimeoutSec)
	}
}

func TestServerListJobsStoreError(t *testing.T) {
	handler, store := newTestServer(t)
	store.ListJobsFunc = func(_ context.Context) ([]Job, error) {
		return nil, fmt.Errorf("db error")
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/jobs", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// Verify we are using bytes.Buffer correctly by testing JSON encoding round-trip
func TestServerCreateJobRoundTrip(t *testing.T) {
	handler, store := newTestServer(t)
	store.CreateJobFunc = func(_ context.Context, job Job) (Job, error) {
		return job, nil
	}

	input := CreateJobRequest{
		TenantID:       "tenant-a",
		ConversationID: "conversation-a",
		AgentID:        "agent-a",
		Name:           "round-trip",
		Schedule:       "0 0 * * *",
		ExecType:       ExecTypeHarness,
		ExecConfig:     `{"prompt":"continue"}`,
		TimeoutSec:     60,
		Tags:           "test,ci",
	}
	body, _ := json.Marshal(input)
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var job Job
	if err := json.NewDecoder(w.Body).Decode(&job); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if job.TimeoutSec != 60 {
		t.Fatalf("expected 60 timeout, got %d", job.TimeoutSec)
	}
	if job.Tags != "test,ci" {
		t.Fatalf("expected tags 'test,ci', got %q", job.Tags)
	}
	if job.TenantID != input.TenantID {
		t.Fatalf("tenant_id = %q, want %q", job.TenantID, input.TenantID)
	}
	if job.ConversationID != input.ConversationID {
		t.Fatalf("conversation_id = %q, want %q", job.ConversationID, input.ConversationID)
	}
	if job.AgentID != input.AgentID {
		t.Fatalf("agent_id = %q, want %q", job.AgentID, input.AgentID)
	}
}
