package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"go-agent-harness/internal/config"
	"go-agent-harness/internal/cron"
	"go-agent-harness/internal/fakeprovider"
	"go-agent-harness/internal/harness"
	htools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/harness/tools/deferred"
)

func newTestEmbeddedAdapter(t *testing.T) *embeddedCronAdapter {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "cron.db")
	st, err := cron.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	if err := st.Migrate(context.Background()); err != nil {
		st.Close()
		t.Fatalf("migrate: %v", err)
	}
	clock := cron.RealClock{}
	sched := cron.NewScheduler(st, &cron.ShellExecutor{}, clock, cron.SchedulerConfig{MaxConcurrent: 5})
	if err := sched.Start(context.Background()); err != nil {
		st.Close()
		t.Fatalf("start scheduler: %v", err)
	}
	t.Cleanup(func() {
		sched.Stop()
		st.Close()
	})
	return &embeddedCronAdapter{store: st, scheduler: sched, clock: clock}
}

type injectedEmbeddedCronScheduler struct {
	delegate         *cron.Scheduler
	addErr           error
	updateErr        error
	beforeAddFail    func()
	beforeUpdateFail func()
}

func (s *injectedEmbeddedCronScheduler) AddJob(job cron.Job) error {
	if s.addErr != nil {
		if s.beforeAddFail != nil {
			s.beforeAddFail()
		}
		return s.addErr
	}
	return s.delegate.AddJob(job)
}

func (s *injectedEmbeddedCronScheduler) PrepareJob(job cron.Job) (*cron.PreparedJob, error) {
	if s.delegate.HasEntry(job.ID) && s.updateErr != nil {
		if s.beforeUpdateFail != nil {
			s.beforeUpdateFail()
		}
		return nil, s.updateErr
	}
	if s.addErr != nil {
		if s.beforeAddFail != nil {
			s.beforeAddFail()
		}
		return nil, s.addErr
	}
	return s.delegate.PrepareJob(job)
}

func (s *injectedEmbeddedCronScheduler) CommitJob(prepared *cron.PreparedJob) {
	s.delegate.CommitJob(prepared)
}

func (s *injectedEmbeddedCronScheduler) AbortJob(prepared *cron.PreparedJob) {
	s.delegate.AbortJob(prepared)
}

func (s *injectedEmbeddedCronScheduler) UpdateJobSchedule(job cron.Job) error {
	if s.updateErr != nil {
		if s.beforeUpdateFail != nil {
			s.beforeUpdateFail()
		}
		return s.updateErr
	}
	return s.delegate.UpdateJobSchedule(job)
}

func (s *injectedEmbeddedCronScheduler) RemoveJob(id string) { s.delegate.RemoveJob(id) }
func (s *injectedEmbeddedCronScheduler) HasEntry(id string) bool {
	return s.delegate.HasEntry(id)
}

type deleteFailingEmbeddedCronStore struct {
	cron.Store
	err error
}

func (s *deleteFailingEmbeddedCronStore) DeleteJob(context.Context, string) error     { return s.err }
func (s *deleteFailingEmbeddedCronStore) DeactivateJob(context.Context, string) error { return s.err }

func TestEmbeddedCronCreate_SchedulerAndDeleteFailureLeavesDurablyInactiveJob(t *testing.T) {
	baseStore := newTestCronStore(t)
	store := &deleteFailingEmbeddedCronStore{Store: baseStore, err: errors.New("injected delete failure")}
	clock := testClock{t: time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)}
	realScheduler := cron.NewScheduler(store, &cron.ShellExecutor{}, clock, cron.SchedulerConfig{MaxConcurrent: 1})
	t.Cleanup(realScheduler.Stop)
	scheduler := &injectedEmbeddedCronScheduler{delegate: realScheduler, addErr: errors.New("injected add failure")}
	adapter := &embeddedCronAdapter{store: store, scheduler: scheduler, clock: clock}

	_, err := adapter.CreateJob(context.Background(), htools.CronCreateJobRequest{
		Name: "create-fail-closed", Schedule: "*/5 * * * *", ExecType: cron.ExecTypeShell, ExecConfig: `{"command":"echo ok"}`,
	})
	if err == nil {
		t.Fatal("CreateJob succeeded despite scheduler failure")
	}
	jobs, listErr := baseStore.ListJobs(context.Background())
	if listErr != nil {
		t.Fatalf("list jobs: %v", listErr)
	}
	if len(jobs) != 1 {
		t.Fatalf("jobs = %d, want one retained fail-closed record", len(jobs))
	}
	if jobs[0].Status != cron.StatusPaused {
		t.Fatalf("retained status = %q, want paused", jobs[0].Status)
	}
	if scheduler.HasEntry(jobs[0].ID) {
		t.Fatal("failed create left a runnable scheduler entry")
	}
}

type activationFailingEmbeddedCronStore struct{ cron.Store }

func (activationFailingEmbeddedCronStore) UpdateJobCAS(context.Context, cron.Job, time.Time) error {
	return errors.New("injected activation failure")
}

type updateCASFailingEmbeddedCronStore struct{ cron.Store }

func (updateCASFailingEmbeddedCronStore) UpdateJobCAS(context.Context, cron.Job, time.Time) error {
	return errors.New("injected update CAS failure")
}

func TestEmbeddedCronUpdate_PreparedReplacementCASFailureAbortsAndPreservesOldJob(t *testing.T) {
	store := newTestCronStore(t)
	clock := testClock{t: time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)}
	scheduler := cron.NewScheduler(store, &cron.ShellExecutor{}, clock, cron.SchedulerConfig{MaxConcurrent: 1})
	t.Cleanup(scheduler.Stop)
	adapter := &embeddedCronAdapter{store: store, scheduler: scheduler, clock: clock}
	created, err := adapter.CreateJob(context.Background(), htools.CronCreateJobRequest{Name: "prepared-cas-failure", Schedule: "*/5 * * * *", ExecType: cron.ExecTypeShell, ExecConfig: `{"command":"echo ok"}`})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	adapter.store = updateCASFailingEmbeddedCronStore{Store: store}
	newSchedule := "0 * * * *"
	if _, err := adapter.UpdateJob(context.Background(), created.ID, htools.CronUpdateJobRequest{Schedule: &newSchedule, ExpectedUpdatedAt: &created.UpdatedAt}); err == nil {
		t.Fatal("schedule update succeeded despite CAS failure")
	}
	persisted, err := store.GetJob(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("load persisted: %v", err)
	}
	if persisted.Schedule != created.Schedule || persisted.Status != created.Status || persisted.ExecConfig != created.ExecConfig || !persisted.UpdatedAt.Equal(created.UpdatedAt) {
		t.Fatalf("persisted after CAS failure = %#v, want unchanged created %#v", persisted, created)
	}
	if !scheduler.HasEntry(created.ID) {
		t.Fatal("CAS failure removed the old live scheduler entry")
	}
}

func TestEmbeddedCronUpdate_RedundantActiveStatusDoesNotReregister(t *testing.T) {
	store := newTestCronStore(t)
	clock := testClock{t: time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)}
	realScheduler := cron.NewScheduler(store, &cron.ShellExecutor{}, clock, cron.SchedulerConfig{MaxConcurrent: 1})
	t.Cleanup(realScheduler.Stop)
	adapter := &embeddedCronAdapter{store: store, scheduler: realScheduler, clock: clock}
	created, err := adapter.CreateJob(context.Background(), htools.CronCreateJobRequest{Name: "redundant-active", Schedule: "*/5 * * * *", ExecType: cron.ExecTypeShell, ExecConfig: `{"command":"echo ok"}`})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	injected := &injectedEmbeddedCronScheduler{delegate: realScheduler, addErr: errors.New("unexpected add")}
	adapter.scheduler = injected
	active, tags := cron.StatusActive, "changed"
	updated, err := adapter.UpdateJob(context.Background(), created.ID, htools.CronUpdateJobRequest{Status: &active, Tags: &tags, ExpectedUpdatedAt: &created.UpdatedAt})
	if err != nil {
		t.Fatalf("redundant active update: %v", err)
	}
	if updated.Tags != tags || updated.Status != cron.StatusActive || !realScheduler.HasEntry(created.ID) {
		t.Fatalf("redundant active result = %#v, entry=%v", updated, realScheduler.HasEntry(created.ID))
	}
}

func TestEmbeddedCronCreate_ActivationFailureNeverRestartRearmsPausedJob(t *testing.T) {
	baseStore := newTestCronStore(t)
	clock := testClock{t: time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)}
	store := activationFailingEmbeddedCronStore{Store: baseStore}
	scheduler := cron.NewScheduler(store, &cron.ShellExecutor{}, clock, cron.SchedulerConfig{MaxConcurrent: 1})
	t.Cleanup(scheduler.Stop)
	adapter := &embeddedCronAdapter{store: store, scheduler: scheduler, clock: clock}
	if _, err := adapter.CreateJob(context.Background(), htools.CronCreateJobRequest{Name: "activation-fail", Schedule: "*/5 * * * *", ExecType: cron.ExecTypeShell, ExecConfig: `{"command":"echo ok"}`}); err == nil {
		t.Fatal("CreateJob succeeded despite activation failure")
	}
	jobs, err := baseStore.ListJobs(context.Background())
	if err != nil || len(jobs) != 1 || jobs[0].Status != cron.StatusPaused {
		t.Fatalf("durable create after activation failure = %#v, %v", jobs, err)
	}
	if scheduler.HasEntry(jobs[0].ID) {
		t.Fatal("activation failure left a live entry")
	}
	restarted := cron.NewScheduler(baseStore, &cron.ShellExecutor{}, clock, cron.SchedulerConfig{MaxConcurrent: 1})
	t.Cleanup(restarted.Stop)
	if err := restarted.Start(context.Background()); err != nil {
		t.Fatalf("restart: %v", err)
	}
	if restarted.HasEntry(jobs[0].ID) {
		t.Fatal("paused failed create rearmed after restart")
	}
}

func TestEmbeddedCronUpdate_AddFailureAndRollbackConflictConvergesFailClosed(t *testing.T) {
	store := newTestCronStore(t)
	clock := testClock{t: time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)}
	realScheduler := cron.NewScheduler(store, &cron.ShellExecutor{}, clock, cron.SchedulerConfig{MaxConcurrent: 1})
	t.Cleanup(realScheduler.Stop)
	scheduler := &injectedEmbeddedCronScheduler{delegate: realScheduler}
	adapter := &embeddedCronAdapter{store: store, scheduler: scheduler, clock: clock}

	created, err := adapter.CreateJob(context.Background(), htools.CronCreateJobRequest{
		Name: "resume-fail-closed", Schedule: "*/5 * * * *", ExecType: cron.ExecTypeShell, ExecConfig: `{"command":"echo ok"}`,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	pausedStatus := cron.StatusPaused
	paused, err := adapter.UpdateJob(context.Background(), created.ID, htools.CronUpdateJobRequest{
		Status: &pausedStatus, ExpectedUpdatedAt: &created.UpdatedAt,
	})
	if err != nil {
		t.Fatalf("pause: %v", err)
	}
	if scheduler.HasEntry(created.ID) {
		t.Fatal("pause left a scheduler entry")
	}

	var touchOnce sync.Once
	scheduler.beforeAddFail = func() {
		touchOnce.Do(func() {
			current, getErr := store.GetJob(context.Background(), created.ID)
			if getErr != nil {
				t.Fatalf("load before concurrent touch: %v", getErr)
			}
			if touchErr := store.TouchJobRun(
				context.Background(), current.ID, clock.Now(), current.NextRunAt, current.UpdatedAt.Add(time.Second),
			); touchErr != nil {
				t.Fatalf("concurrent touch: %v", touchErr)
			}
		})
	}
	scheduler.addErr = errors.New("injected persistent add failure")
	scheduler.updateErr = errors.New("injected persistent replacement failure")
	activeStatus := cron.StatusActive
	if _, err := adapter.UpdateJob(context.Background(), created.ID, htools.CronUpdateJobRequest{
		Status: &activeStatus, ExpectedUpdatedAt: &paused.UpdatedAt,
	}); err == nil {
		t.Fatal("resume succeeded despite scheduler failure")
	}
	persisted, err := store.GetJob(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("load after failed resume: %v", err)
	}
	if persisted.Status != cron.StatusPaused {
		t.Fatalf("persisted status = %q, want fail-closed paused", persisted.Status)
	}
	if persisted.LastRunAt.IsZero() {
		t.Fatal("fail-closed convergence overwrote the concurrent TouchJobRun")
	}
	if scheduler.HasEntry(created.ID) {
		t.Fatal("fail-closed convergence left a runnable scheduler entry")
	}
}

func TestEmbeddedCronUpdate_ReplacementFailureAndRollbackConflictConvergesFailClosed(t *testing.T) {
	store := newTestCronStore(t)
	clock := testClock{t: time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)}
	realScheduler := cron.NewScheduler(store, &cron.ShellExecutor{}, clock, cron.SchedulerConfig{MaxConcurrent: 1})
	t.Cleanup(realScheduler.Stop)
	scheduler := &injectedEmbeddedCronScheduler{delegate: realScheduler}
	adapter := &embeddedCronAdapter{store: store, scheduler: scheduler, clock: clock}

	created, err := adapter.CreateJob(context.Background(), htools.CronCreateJobRequest{
		Name: "replacement-fail-closed", Schedule: "*/5 * * * *", ExecType: cron.ExecTypeShell, ExecConfig: `{"command":"echo ok"}`,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	var touchOnce sync.Once
	scheduler.beforeUpdateFail = func() {
		touchOnce.Do(func() {
			current, getErr := store.GetJob(context.Background(), created.ID)
			if getErr != nil {
				t.Fatalf("load before concurrent touch: %v", getErr)
			}
			if touchErr := store.TouchJobRun(
				context.Background(), current.ID, clock.Now(), current.NextRunAt, current.UpdatedAt.Add(time.Second),
			); touchErr != nil {
				t.Fatalf("concurrent touch: %v", touchErr)
			}
		})
	}
	scheduler.updateErr = errors.New("injected persistent replacement failure")
	newSchedule := "0 * * * *"
	if _, err := adapter.UpdateJob(context.Background(), created.ID, htools.CronUpdateJobRequest{
		Schedule: &newSchedule, ExpectedUpdatedAt: &created.UpdatedAt,
	}); err == nil {
		t.Fatal("schedule update succeeded despite scheduler failure")
	}
	persisted, err := store.GetJob(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("load after failed replacement: %v", err)
	}
	if persisted.ID != created.ID || persisted.Schedule != created.Schedule || persisted.Status != created.Status || persisted.ExecConfig != created.ExecConfig {
		t.Fatalf("persisted after failed replacement = %#v, want pre-update config %#v", persisted, created)
	}
	if persisted.LastRunAt.IsZero() {
		t.Fatal("fail-closed convergence overwrote the concurrent TouchJobRun")
	}
	if !scheduler.HasEntry(created.ID) {
		t.Fatal("failed replacement removed the old runnable scheduler entry")
	}
}

func cronToolScope(tenant, conversation, agent string) context.Context {
	return context.WithValue(context.Background(), htools.ContextKeyRunMetadata, htools.RunMetadata{
		TenantID: tenant, ConversationID: conversation, AgentID: agent,
	})
}

func decodeCronToolJob(t *testing.T, result string) htools.CronJob {
	t.Helper()
	var job htools.CronJob
	if err := json.Unmarshal([]byte(result), &job); err != nil {
		t.Fatalf("decode cron job: %v (%s)", err, result)
	}
	return job
}

func decodeCronGetToolJob(t *testing.T, result string) htools.CronJob {
	t.Helper()
	var payload struct {
		Job htools.CronJob `json:"job"`
	}
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("decode cron_get result: %v (%s)", err, result)
	}
	return payload.Job
}

func TestEmbeddedCronModelToolsFullScopedLifecycle(t *testing.T) {
	adapter := newTestEmbeddedAdapter(t)
	client := deferred.NewScopedCronClient(adapter)
	if err := client.Health(cronToolScope("tenant-a", "conversation-a", "agent-a")); err != nil {
		t.Fatalf("scoped cron health: %v", err)
	}
	create := deferred.CronCreateTool(client)
	list := deferred.CronListTool(client)
	get := deferred.CronGetTool(client)
	update := deferred.CronUpdateTool(client)
	pause := deferred.CronPauseTool(client)
	resume := deferred.CronResumeTool(client)
	delete := deferred.CronDeleteTool(client)

	ctxA := cronToolScope("tenant-a", "conversation-a", "agent-a")
	ctxB := cronToolScope("tenant-b", "conversation-b", "agent-b")
	createdA := decodeCronToolJob(t, mustToolCall(t, create, ctxA, `{"name":"shared-name","schedule":"0 0 * * *","command":"echo initial","timeout_seconds":30}`))
	createdB := decodeCronToolJob(t, mustToolCall(t, create, ctxB, `{"name":"shared-name","schedule":"0 0 * * *","command":"echo other"}`))
	if createdA.ID == createdB.ID {
		t.Fatal("scoped jobs must have distinct stable identities")
	}
	if _, err := get.Handler(ctxA, json.RawMessage(`{"id":"shared-name"}`)); err == nil {
		t.Fatal("model-facing cron_get must accept job IDs only, never names")
	}

	listedA := mustToolCall(t, list, ctxA, `{}`)
	if !strings.Contains(listedA, createdA.ID) || strings.Contains(listedA, createdB.ID) {
		t.Fatalf("scope A list leaked another conversation: %s", listedA)
	}
	if _, err := get.Handler(ctxB, json.RawMessage(fmt.Sprintf(`{"id":%q}`, createdA.ID))); err == nil {
		t.Fatal("scope B must not read scope A's job")
	}
	if _, err := update.Handler(ctxB, json.RawMessage(fmt.Sprintf(`{"id":%q,"tags":"stolen","expected_updated_at":%q}`, createdA.ID, createdA.UpdatedAt.Format(time.RFC3339Nano)))); err == nil {
		t.Fatal("scope B must not update scope A's job")
	}
	if _, err := pause.Handler(ctxB, json.RawMessage(fmt.Sprintf(`{"id":%q,"expected_updated_at":%q}`, createdA.ID, createdA.UpdatedAt.Format(time.RFC3339Nano)))); err == nil {
		t.Fatal("scope B must not pause scope A's job")
	}
	if _, err := delete.Handler(ctxB, json.RawMessage(fmt.Sprintf(`{"id":%q,"expected_updated_at":%q}`, createdA.ID, createdA.UpdatedAt.Format(time.RFC3339Nano)))); err == nil {
		t.Fatal("scope B must not delete scope A's job")
	}

	gotA := decodeCronGetToolJob(t, mustToolCall(t, get, ctxA, fmt.Sprintf(`{"id":%q}`, createdA.ID)))
	updated := decodeCronToolJob(t, mustToolCall(t, update, ctxA, fmt.Sprintf(`{"id":%q,"schedule":"15 * * * *","command":"echo updated","timeout_seconds":45,"tags":"updated","tenant_id":"spoofed","expected_updated_at":%q}`, createdA.ID, gotA.UpdatedAt.Format(time.RFC3339Nano))))
	if updated.ID != createdA.ID || updated.TenantID != "tenant-a" || updated.ConversationID != "conversation-a" || updated.AgentID != "agent-a" {
		t.Fatalf("update changed identity or scope: %+v", updated)
	}
	if updated.Schedule != "15 * * * *" || updated.ExecConfig != `{"command":"echo updated"}` || updated.TimeoutSec != 45 || updated.Tags != "updated" {
		t.Fatalf("updated values not applied: %+v", updated)
	}
	if _, err := update.Handler(ctxA, json.RawMessage(fmt.Sprintf(`{"id":%q,"timeout_seconds":0,"expected_updated_at":%q}`, createdA.ID, updated.UpdatedAt.Format(time.RFC3339Nano)))); err == nil {
		t.Fatal("unsafe timeout must fail through the model-facing tool path")
	}

	paused := decodeCronToolJob(t, mustToolCall(t, pause, ctxA, fmt.Sprintf(`{"id":%q,"expected_updated_at":%q}`, createdA.ID, updated.UpdatedAt.Format(time.RFC3339Nano))))
	if paused.Status != cron.StatusPaused || adapter.scheduler.HasEntry(createdA.ID) {
		t.Fatalf("pause state = %q, scheduler entry = %v", paused.Status, adapter.scheduler.HasEntry(createdA.ID))
	}
	resumed := decodeCronToolJob(t, mustToolCall(t, resume, ctxA, fmt.Sprintf(`{"id":%q,"expected_updated_at":%q}`, createdA.ID, paused.UpdatedAt.Format(time.RFC3339Nano))))
	if resumed.Status != cron.StatusActive || !adapter.scheduler.HasEntry(createdA.ID) {
		t.Fatalf("resume state = %q, scheduler entry = %v", resumed.Status, adapter.scheduler.HasEntry(createdA.ID))
	}

	if _, err := get.Handler(ctxB, json.RawMessage(fmt.Sprintf(`{"id":%q}`, createdA.ID))); err == nil {
		t.Fatal("scope B must not mutate or inspect scope A's job")
	}
	if _, err := delete.Handler(ctxA, json.RawMessage(fmt.Sprintf(`{"id":%q,"expected_updated_at":%q}`, createdA.ID, resumed.UpdatedAt.Format(time.RFC3339Nano)))); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if adapter.scheduler.HasEntry(createdA.ID) {
		t.Fatal("delete must remove the scheduler entry")
	}
	if _, err := get.Handler(ctxA, json.RawMessage(fmt.Sprintf(`{"id":%q}`, createdA.ID))); err == nil {
		t.Fatal("deleted job must be not found")
	}
	if strings.Contains(mustToolCall(t, list, ctxA, `{}`), createdA.ID) {
		t.Fatal("deleted job remained in scoped list")
	}
}

func TestEmbeddedCronAdapterUsesAuthoritativeStoreScopePredicates(t *testing.T) {
	adapter := newTestEmbeddedAdapter(t)
	create := func(tenant string) htools.CronJob {
		job, err := adapter.CreateJob(context.Background(), htools.CronCreateJobRequest{TenantID: tenant, ConversationID: "conversation", AgentID: "agent", Name: "same-name", Schedule: "0 0 * * *", ExecType: cron.ExecTypeShell, ExecConfig: `{"command":"echo ok"}`})
		if err != nil {
			t.Fatalf("create %s: %v", tenant, err)
		}
		return job
	}
	jobA, jobB := create("tenant-a"), create("tenant-b")
	if _, err := adapter.GetJobByName(context.Background(), "same-name"); !cron.IsJobAmbiguous(err) {
		t.Fatalf("global operator name lookup = %v, want ambiguity", err)
	}
	ctxA := cron.WithScope(context.Background(), cron.Scope{TenantID: "tenant-a", ConversationID: "conversation", AgentID: "agent"})
	jobs, err := adapter.ListJobs(ctxA)
	if err != nil || len(jobs) != 1 || jobs[0].ID != jobA.ID {
		t.Fatalf("scoped list = %#v, %v", jobs, err)
	}
	if _, err := adapter.GetJob(ctxA, jobB.ID); !errors.Is(err, htools.ErrCronJobNotFound) {
		t.Fatalf("cross-scope get = %v, want not found", err)
	}
	if _, err := adapter.UpdateJob(ctxA, jobB.ID, htools.CronUpdateJobRequest{}); !errors.Is(err, htools.ErrCronJobNotFound) {
		t.Fatalf("cross-scope update = %v, want not found", err)
	}
	if _, err := adapter.ListExecutions(ctxA, jobB.ID, 10, 0); !errors.Is(err, htools.ErrCronJobNotFound) {
		t.Fatalf("cross-scope history = %v, want not found", err)
	}
	if err := adapter.DeleteJob(ctxA, jobB.ID); !errors.Is(err, htools.ErrCronJobNotFound) {
		t.Fatalf("cross-scope delete = %v, want not found", err)
	}
}

func mustToolCall(t *testing.T, tool htools.Tool, ctx context.Context, args string) string {
	t.Helper()
	result, err := tool.Handler(ctx, json.RawMessage(args))
	if err != nil {
		t.Fatalf("%s: %v", tool.Definition.Name, err)
	}
	return result
}

func assertDefaultRegistryCronScopeAndVersions(t *testing.T, rawClient htools.CronClient) {
	t.Helper()
	registry := harness.NewDefaultRegistryWithOptions(t.TempDir(), harness.DefaultRegistryOptions{
		ApprovalMode: harness.ToolApprovalModeFullAuto,
		CronClient:   rawClient,
	})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := registry.Shutdown(ctx); err != nil {
			t.Logf("registry shutdown: %v", err)
		}
	})

	ctxA := cronToolScope("tenant-a", "conversation-a", "agent-a")
	ctxB := cronToolScope("tenant-b", "conversation-b", "agent-b")
	createdResult, err := registry.Execute(ctxA, "cron_create", json.RawMessage(`{"name":"registry-scoped","schedule":"0 0 * * *","command":"echo initial"}`))
	if err != nil {
		t.Fatalf("registry cron_create: %v", err)
	}
	created := decodeCronToolJob(t, createdResult)

	// Operator/server paths keep the raw adapter and therefore remain able to
	// inspect jobs without model RunMetadata.
	if got, err := rawClient.GetJob(context.Background(), created.ID); err != nil || got.ID != created.ID {
		t.Fatalf("raw operator get = %#v, %v", got, err)
	}
	if _, err := registry.Execute(ctxB, "cron_get", json.RawMessage(fmt.Sprintf(`{"id":%q}`, created.ID))); !errors.Is(err, htools.ErrCronJobNotFound) {
		t.Fatalf("cross-scope registry get = %v, want not found", err)
	}
	if _, err := registry.Execute(context.Background(), "cron_list", json.RawMessage(`{}`)); err == nil || !strings.Contains(err.Error(), "cron scope is required") {
		t.Fatalf("unscoped registry list = %v, want required scope", err)
	}

	updatedResult, err := registry.Execute(ctxA, "cron_update", json.RawMessage(fmt.Sprintf(`{"id":%q,"tags":"current","expected_updated_at":%q}`, created.ID, created.UpdatedAt.Format(time.RFC3339Nano))))
	if err != nil {
		t.Fatalf("registry cron_update: %v", err)
	}
	updated := decodeCronToolJob(t, updatedResult)
	if _, err := registry.Execute(ctxA, "cron_pause", json.RawMessage(fmt.Sprintf(`{"id":%q,"expected_updated_at":%q}`, created.ID, created.UpdatedAt.Format(time.RFC3339Nano)))); !errors.Is(err, htools.ErrCronJobConflict) {
		t.Fatalf("stale registry pause = %v, want conflict", err)
	}
	pausedResult, err := registry.Execute(ctxA, "cron_pause", json.RawMessage(fmt.Sprintf(`{"id":%q,"expected_updated_at":%q}`, created.ID, updated.UpdatedAt.Format(time.RFC3339Nano))))
	if err != nil {
		t.Fatalf("current registry pause: %v", err)
	}
	paused := decodeCronToolJob(t, pausedResult)
	if _, err := registry.Execute(ctxA, "cron_resume", json.RawMessage(fmt.Sprintf(`{"id":%q,"expected_updated_at":%q}`, created.ID, updated.UpdatedAt.Format(time.RFC3339Nano)))); !errors.Is(err, htools.ErrCronJobConflict) {
		t.Fatalf("stale registry resume = %v, want conflict", err)
	}
	resumedResult, err := registry.Execute(ctxA, "cron_resume", json.RawMessage(fmt.Sprintf(`{"id":%q,"expected_updated_at":%q}`, created.ID, paused.UpdatedAt.Format(time.RFC3339Nano))))
	if err != nil {
		t.Fatalf("current registry resume: %v", err)
	}
	resumed := decodeCronToolJob(t, resumedResult)
	if resumed.Status != cron.StatusActive {
		t.Fatalf("resumed status = %q, want active", resumed.Status)
	}
	if _, err := registry.Execute(ctxA, "cron_delete", json.RawMessage(fmt.Sprintf(`{"id":%q,"expected_updated_at":%q}`, created.ID, paused.UpdatedAt.Format(time.RFC3339Nano)))); !errors.Is(err, htools.ErrCronJobConflict) {
		t.Fatalf("stale registry delete = %v, want conflict", err)
	}
	if _, err := registry.Execute(ctxA, "cron_delete", json.RawMessage(fmt.Sprintf(`{"id":%q,"expected_updated_at":%q}`, created.ID, resumed.UpdatedAt.Format(time.RFC3339Nano)))); err != nil {
		t.Fatalf("current registry delete: %v", err)
	}
}

func TestDefaultModelRegistryScopesEmbeddedAndRemoteCronAdapters(t *testing.T) {
	t.Run("embedded", func(t *testing.T) {
		assertDefaultRegistryCronScopeAndVersions(t, newTestEmbeddedAdapter(t))
	})
	t.Run("remote", func(t *testing.T) {
		adapter := newTestEmbeddedAdapter(t)
		scheduler, ok := adapter.scheduler.(*cron.Scheduler)
		if !ok {
			t.Fatalf("test adapter scheduler = %T, want *cron.Scheduler", adapter.scheduler)
		}
		const apiKey = "embedded-cron-test-key"
		server := httptest.NewServer(cron.NewServer(adapter.store, scheduler, adapter.clock, cron.IngressAuthConfig{APIKey: apiKey, TenantID: "tenant-a"}))
		t.Cleanup(server.Close)
		assertDefaultRegistryCronScopeAndVersions(t, &cronClientAdapter{client: cron.NewClient(server.URL, cron.WithAPIKey(apiKey))})
	})
}

func TestBuildCronBootstrapRequiresRemoteIngressCredential(t *testing.T) {
	bootstrap, err := buildCronBootstrap(
		t.TempDir(),
		"http://localhost:9090",
		"",
		config.Defaults().Cron,
		func(string, ...any) {},
		&cronRunStarter{},
	)
	if err == nil || !strings.Contains(err.Error(), "HARNESS_CRON_API_KEY") {
		if bootstrap.store != nil {
			_ = bootstrap.store.Close()
		}
		t.Fatalf("error = %v", err)
	}
}

func TestEmbeddedCron_ScopedHarnessJobContinuesOwnedConversation(t *testing.T) {
	provider := fakeprovider.New(
		[]fakeprovider.Turn{{Content: "scheduled reply"}},
		fakeprovider.WithExhaustedBehavior(fakeprovider.ExhaustRepeatLast),
	)
	runner := harness.NewRunner(provider, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     1,
	})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := runner.Shutdown(ctx); err != nil {
			t.Logf("runner shutdown: %v", err)
		}
	})

	origin, err := runner.StartRun(harness.RunRequest{
		Prompt:         "origin",
		TenantID:       "tenant-a",
		ConversationID: "conversation-a",
		AgentID:        "agent-a",
	})
	if err != nil {
		t.Fatalf("start origin run: %v", err)
	}
	if final := waitForTerminalStatus(t, runner, origin.ID); final.Status != harness.RunStatusCompleted {
		t.Fatalf("origin run status = %s, want completed (error: %s)", final.Status, final.Error)
	}

	bootstrap, err := buildCronBootstrap(
		t.TempDir(),
		"",
		"",
		config.Defaults().Cron,
		func(string, ...any) {},
		&cronRunStarter{runner: runner},
	)
	if err != nil {
		t.Fatalf("build cron bootstrap: %v", err)
	}
	t.Cleanup(func() {
		bootstrap.scheduler.Stop()
		if err := bootstrap.store.Close(); err != nil {
			t.Logf("close cron store: %v", err)
		}
	})

	scopedClient := deferred.NewScopedCronClient(bootstrap.client)
	createTool := deferred.CronCreateTool(scopedClient)
	getTool := deferred.CronGetTool(scopedClient)
	updateTool := deferred.CronUpdateTool(scopedClient)
	createCtx := context.WithValue(context.Background(), htools.ContextKeyRunMetadata, htools.RunMetadata{
		TenantID: "tenant-a", ConversationID: "conversation-a", AgentID: "agent-a",
	})
	createdResult, err := createTool.Handler(createCtx, json.RawMessage(`{"name":"continue-owned-conversation","schedule":"0 0 * * *","execution_type":"harness","prompt":"scheduled follow-up"}`))
	if err != nil {
		t.Fatalf("create harness cron job through tool: %v", err)
	}
	var job htools.CronJob
	if err := json.Unmarshal([]byte(createdResult), &job); err != nil {
		t.Fatalf("decode created cron job: %v", err)
	}
	if job.ExecType != string(cron.ExecTypeHarness) || job.ConversationID != "conversation-a" {
		t.Fatalf("created harness job = %+v", job)
	}
	current := decodeCronGetToolJob(t, mustToolCall(t, getTool, createCtx, fmt.Sprintf(`{"id":%q}`, job.ID)))
	job = decodeCronToolJob(t, mustToolCall(t, updateTool, createCtx, fmt.Sprintf(
		`{"id":%q,"prompt":"updated scheduled follow-up","expected_updated_at":%q}`,
		job.ID,
		current.UpdatedAt.Format(time.RFC3339Nano),
	)))

	if err := bootstrap.scheduler.TriggerJob(context.Background(), job.ID); err != nil {
		t.Fatalf("trigger cron job: %v", err)
	}
	bootstrap.scheduler.Stop()

	executions, err := bootstrap.store.ListExecutions(context.Background(), job.ID, 1, 0)
	if err != nil {
		t.Fatalf("list executions: %v", err)
	}
	if len(executions) != 1 {
		t.Fatalf("execution count = %d, want 1", len(executions))
	}
	execution := executions[0]
	if execution.Status != cron.ExecStatusSuccess {
		t.Fatalf("execution status = %s, want success (error: %s)", execution.Status, execution.Error)
	}
	const outputPrefix = "started run "
	if !strings.HasPrefix(execution.OutputSummary, outputPrefix) {
		t.Fatalf("execution output = %q, want %q prefix", execution.OutputSummary, outputPrefix)
	}
	runID := strings.TrimPrefix(execution.OutputSummary, outputPrefix)
	final := waitForTerminalStatus(t, runner, runID)
	if final.Status != harness.RunStatusCompleted {
		t.Fatalf("scheduled run status = %s, want completed (error: %s)", final.Status, final.Error)
	}
	if final.TenantID != "tenant-a" || final.ConversationID != "conversation-a" || final.AgentID != "agent-a" {
		t.Fatalf(
			"scheduled run scope = tenant:%q conversation:%q agent:%q",
			final.TenantID,
			final.ConversationID,
			final.AgentID,
		)
	}
	if final.Prompt != "updated scheduled follow-up" {
		t.Fatalf("scheduled run prompt = %q, want %q", final.Prompt, "updated scheduled follow-up")
	}
}

func TestEmbeddedCronAdapter_CreateJob(t *testing.T) {
	t.Parallel()
	adapter := newTestEmbeddedAdapter(t)
	ctx := context.Background()

	job, err := adapter.CreateJob(ctx, htools.CronCreateJobRequest{
		Name:           "test-job",
		Schedule:       "*/5 * * * *",
		ExecType:       "shell",
		ExecConfig:     `{"command":"echo hi"}`,
		TimeoutSec:     60,
		Tags:           "test",
		TenantID:       "tenant-a",
		ConversationID: "conversation-a",
		AgentID:        "agent-a",
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if job.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if job.Name != "test-job" {
		t.Fatalf("Name: got %q, want %q", job.Name, "test-job")
	}
	if job.Status != "active" {
		t.Fatalf("Status: got %q, want active", job.Status)
	}
	if job.TenantID != "tenant-a" || job.ConversationID != "conversation-a" || job.AgentID != "agent-a" {
		t.Fatalf("scope: got tenant=%q conversation=%q agent=%q", job.TenantID, job.ConversationID, job.AgentID)
	}
	if job.NextRunAt.IsZero() {
		t.Fatal("expected non-zero NextRunAt")
	}
	// Default timeout
	job2, err := adapter.CreateJob(ctx, htools.CronCreateJobRequest{
		Name:       "default-timeout",
		Schedule:   "0 * * * *",
		ExecType:   "shell",
		ExecConfig: `{"command":"echo hi"}`,
	})
	if err != nil {
		t.Fatalf("CreateJob default timeout: %v", err)
	}
	if job2.TimeoutSec != 30 {
		t.Fatalf("expected default timeout 30, got %d", job2.TimeoutSec)
	}
}

func TestEmbeddedCronAdapter_CreateJob_Validation(t *testing.T) {
	t.Parallel()
	adapter := newTestEmbeddedAdapter(t)
	ctx := context.Background()

	// Empty name
	if _, err := adapter.CreateJob(ctx, htools.CronCreateJobRequest{
		Schedule: "*/5 * * * *",
		ExecType: "shell",
	}); err == nil {
		t.Fatal("expected error for empty name")
	}

	// Empty schedule
	if _, err := adapter.CreateJob(ctx, htools.CronCreateJobRequest{
		Name:     "x",
		ExecType: "shell",
	}); err == nil {
		t.Fatal("expected error for empty schedule")
	}

	// Bad schedule
	if _, err := adapter.CreateJob(ctx, htools.CronCreateJobRequest{
		Name:     "x",
		Schedule: "bad-schedule",
		ExecType: "shell",
	}); err == nil {
		t.Fatal("expected error for bad schedule")
	}

	// Invalid exec type
	if _, err := adapter.CreateJob(ctx, htools.CronCreateJobRequest{
		Name:     "x",
		Schedule: "*/5 * * * *",
		ExecType: "invalid",
	}); err == nil {
		t.Fatal("expected error for invalid exec_type")
	}

	// Unsafe execution configurations and non-positive timeouts are rejected
	// before persistence, with actionable errors.
	for _, tc := range []struct {
		name string
		req  htools.CronCreateJobRequest
		want string
	}{
		{"empty shell command", htools.CronCreateJobRequest{Name: "x", Schedule: "*/5 * * * *", ExecType: "shell", ExecConfig: `{"command":""}`}, "non-empty command"},
		{"incomplete harness prompt", htools.CronCreateJobRequest{Name: "x", Schedule: "*/5 * * * *", ExecType: "harness", ExecConfig: `{"prompt":""}`}, "non-empty prompt"},
		{"negative timeout", htools.CronCreateJobRequest{Name: "x", Schedule: "*/5 * * * *", ExecType: "shell", ExecConfig: `{"command":"echo hi"}`, TimeoutSec: -1}, "timeout_seconds must be positive"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := adapter.CreateJob(ctx, tc.req)
			if tc.want != "" {
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("error = %v, want %q", err, tc.want)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestEmbeddedCronAdapter_GetJob(t *testing.T) {
	t.Parallel()
	adapter := newTestEmbeddedAdapter(t)
	ctx := context.Background()

	created, err := adapter.CreateJob(ctx, htools.CronCreateJobRequest{
		Name:       "get-test",
		Schedule:   "*/5 * * * *",
		ExecType:   "shell",
		ExecConfig: `{"command":"echo hi"}`,
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	// Get by ID
	got, err := adapter.GetJob(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetJob by ID: %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("ID mismatch: got %q, want %q", got.ID, created.ID)
	}

	// Explicit operator lookup by name.
	got2, err := adapter.GetJobByName(ctx, "get-test")
	if err != nil {
		t.Fatalf("GetJob by name: %v", err)
	}
	if got2.Name != "get-test" {
		t.Fatalf("Name mismatch: got %q", got2.Name)
	}

	// Not found
	if _, err := adapter.GetJob(ctx, "nonexistent"); err == nil {
		t.Fatal("expected error for nonexistent job")
	}
}

func TestEmbeddedCronAdapter_ListJobs(t *testing.T) {
	t.Parallel()
	adapter := newTestEmbeddedAdapter(t)
	ctx := context.Background()

	// Empty initially
	jobs, err := adapter.ListJobs(ctx)
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("expected 0 jobs, got %d", len(jobs))
	}

	// Create two
	adapter.CreateJob(ctx, htools.CronCreateJobRequest{Name: "j1", Schedule: "*/5 * * * *", ExecType: "shell", ExecConfig: `{"command":"echo hi"}`})
	adapter.CreateJob(ctx, htools.CronCreateJobRequest{Name: "j2", Schedule: "0 * * * *", ExecType: "shell", ExecConfig: `{"command":"echo hi"}`})

	jobs, err = adapter.ListJobs(ctx)
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
}

func TestEmbeddedCronAdapter_UpdateJob_Schedule(t *testing.T) {
	t.Parallel()
	adapter := newTestEmbeddedAdapter(t)
	ctx := context.Background()

	created, err := adapter.CreateJob(ctx, htools.CronCreateJobRequest{
		Name:       "update-sched",
		Schedule:   "*/5 * * * *",
		ExecType:   "shell",
		ExecConfig: `{"command":"echo hi"}`,
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	newSched := "0 * * * *"
	updated, err := adapter.UpdateJob(ctx, created.ID, htools.CronUpdateJobRequest{
		Schedule: &newSched,
	})
	if err != nil {
		t.Fatalf("UpdateJob: %v", err)
	}
	if updated.Schedule != "0 * * * *" {
		t.Fatalf("Schedule: got %q, want %q", updated.Schedule, "0 * * * *")
	}
	if updated.NextRunAt.IsZero() {
		t.Fatal("expected non-zero NextRunAt after schedule change")
	}
}

func TestEmbeddedCronAdapter_UpdateJob_PauseResume(t *testing.T) {
	t.Parallel()
	adapter := newTestEmbeddedAdapter(t)
	ctx := context.Background()

	created, err := adapter.CreateJob(ctx, htools.CronCreateJobRequest{
		Name:       "pause-resume",
		Schedule:   "*/5 * * * *",
		ExecType:   "shell",
		ExecConfig: `{"command":"echo hi"}`,
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	// Pause
	paused := "paused"
	got, err := adapter.UpdateJob(ctx, created.ID, htools.CronUpdateJobRequest{Status: &paused})
	if err != nil {
		t.Fatalf("Pause: %v", err)
	}
	if got.Status != "paused" {
		t.Fatalf("Status: got %q, want paused", got.Status)
	}

	// Resume
	active := "active"
	got, err = adapter.UpdateJob(ctx, created.ID, htools.CronUpdateJobRequest{Status: &active})
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if got.Status != "active" {
		t.Fatalf("Status: got %q, want active", got.Status)
	}
}

func TestEmbeddedCronAdapter_UpdateJob_Validation(t *testing.T) {
	t.Parallel()
	adapter := newTestEmbeddedAdapter(t)
	ctx := context.Background()

	// Not found
	if _, err := adapter.UpdateJob(ctx, "nonexistent", htools.CronUpdateJobRequest{}); err == nil {
		t.Fatal("expected error for nonexistent job")
	}

	created, _ := adapter.CreateJob(ctx, htools.CronCreateJobRequest{
		Name:       "val-test",
		Schedule:   "*/5 * * * *",
		ExecType:   "shell",
		ExecConfig: `{"command":"echo hi"}`,
	})

	// Empty schedule
	empty := ""
	if _, err := adapter.UpdateJob(ctx, created.ID, htools.CronUpdateJobRequest{Schedule: &empty}); err == nil {
		t.Fatal("expected error for empty schedule")
	}

	// Bad schedule
	bad := "bad-schedule"
	if _, err := adapter.UpdateJob(ctx, created.ID, htools.CronUpdateJobRequest{Schedule: &bad}); err == nil {
		t.Fatal("expected error for bad schedule")
	}

	// Invalid status
	invalid := "invalid"
	if _, err := adapter.UpdateJob(ctx, created.ID, htools.CronUpdateJobRequest{Status: &invalid}); err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestEmbeddedCronAdapter_DeleteJob(t *testing.T) {
	t.Parallel()
	adapter := newTestEmbeddedAdapter(t)
	ctx := context.Background()

	created, _ := adapter.CreateJob(ctx, htools.CronCreateJobRequest{
		Name:       "delete-me",
		Schedule:   "*/5 * * * *",
		ExecType:   "shell",
		ExecConfig: `{"command":"echo hi"}`,
	})

	if err := adapter.DeleteJob(ctx, created.ID); err != nil {
		t.Fatalf("DeleteJob: %v", err)
	}

	// Verify deleted (ListJobs should not include it — soft delete behavior depends on store)
	jobs, _ := adapter.ListJobs(ctx)
	for _, j := range jobs {
		if j.ID == created.ID {
			t.Fatal("expected job to be deleted")
		}
	}
}

func TestEmbeddedCronAdapter_ListExecutions(t *testing.T) {
	t.Parallel()
	adapter := newTestEmbeddedAdapter(t)
	ctx := context.Background()

	created, _ := adapter.CreateJob(ctx, htools.CronCreateJobRequest{
		Name:       "exec-test",
		Schedule:   "*/5 * * * *",
		ExecType:   "shell",
		ExecConfig: `{"command":"echo hi"}`,
	})

	execs, err := adapter.ListExecutions(ctx, created.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListExecutions: %v", err)
	}
	if len(execs) != 0 {
		t.Fatalf("expected 0 executions, got %d", len(execs))
	}
}

func TestEmbeddedCronAdapter_Health(t *testing.T) {
	t.Parallel()
	adapter := newTestEmbeddedAdapter(t)
	if err := adapter.Health(context.Background()); err != nil {
		t.Fatalf("Health: %v", err)
	}
}

func TestEmbeddedCronAdapter_Concurrent(t *testing.T) {
	t.Parallel()
	adapter := newTestEmbeddedAdapter(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	errs := make(chan error, 70)

	// Seed a job so concurrent reads/updates have something to hit.
	seed, _ := adapter.CreateJob(ctx, htools.CronCreateJobRequest{
		Name: "seed-job", Schedule: "*/5 * * * *", ExecType: "shell", ExecConfig: `{"command":"echo hi"}`,
	})

	for i := 0; i < 10; i++ {
		i := i
		wg.Add(5)
		go func() {
			defer wg.Done()
			if _, err := adapter.ListJobs(ctx); err != nil {
				errs <- fmt.Errorf("ListJobs: %w", err)
			}
		}()
		go func() {
			defer wg.Done()
			adapter.GetJob(ctx, seed.ID)
		}()
		go func() {
			defer wg.Done()
			adapter.Health(ctx)
		}()
		go func() {
			defer wg.Done()
			adapter.ListExecutions(ctx, seed.ID, 10, 0)
		}()
		go func() {
			defer wg.Done()
			// Writes may hit SQLITE_BUSY under extreme concurrency — acceptable.
			adapter.CreateJob(ctx, htools.CronCreateJobRequest{
				Name:       fmt.Sprintf("concurrent-%d", i),
				Schedule:   "*/5 * * * *",
				ExecType:   "shell",
				ExecConfig: `{"command":"echo hi"}`,
			})
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent error: %v", err)
	}
}

func TestCronSchedulerConfigFromResolvedConfig(t *testing.T) {
	resolved := config.CronConfig{
		JitterEnabled:    false,
		JitterMinSec:     7,
		JitterMaxSec:     19,
		AvoidMinuteMarks: []int{3, 17, 41},
		LogJitteredTimes: false,
	}

	got := cronSchedulerConfig(resolved)
	resolved.AvoidMinuteMarks[0] = 59
	if got.MaxConcurrent != 5 {
		t.Fatalf("MaxConcurrent = %d, want 5", got.MaxConcurrent)
	}
	if got.Jitter.Enabled {
		t.Fatal("Jitter.Enabled = true, want false")
	}
	if got.Jitter.MinSec != 7 || got.Jitter.MaxSec != 19 {
		t.Fatalf("jitter bounds = %d..%d, want 7..19", got.Jitter.MinSec, got.Jitter.MaxSec)
	}
	if !reflect.DeepEqual(got.Jitter.AvoidMarks, []int{3, 17, 41}) {
		t.Fatalf("avoid marks = %v, want [3 17 41]", got.Jitter.AvoidMarks)
	}
	if got.Jitter.LogJitteredTimes {
		t.Fatal("Jitter.LogJitteredTimes = true, want false")
	}
}
