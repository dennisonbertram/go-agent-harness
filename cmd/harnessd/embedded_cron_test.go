package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	createdA := decodeCronToolJob(t, mustToolCall(t, create, ctxA, `{"name":"scope-a","schedule":"0 0 * * *","command":"echo initial","timeout_seconds":30}`))
	createdB := decodeCronToolJob(t, mustToolCall(t, create, ctxB, `{"name":"scope-b","schedule":"0 0 * * *","command":"echo other"}`))
	if createdA.ID == createdB.ID {
		t.Fatal("scoped jobs must have distinct stable identities")
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
	if _, err := pause.Handler(ctxB, json.RawMessage(fmt.Sprintf(`{"id":%q}`, createdA.ID))); err == nil {
		t.Fatal("scope B must not pause scope A's job")
	}
	if _, err := delete.Handler(ctxB, json.RawMessage(fmt.Sprintf(`{"id":%q}`, createdA.ID))); err == nil {
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

	paused := decodeCronToolJob(t, mustToolCall(t, pause, ctxA, fmt.Sprintf(`{"id":%q}`, createdA.ID)))
	if paused.Status != cron.StatusPaused || adapter.scheduler.HasEntry(createdA.ID) {
		t.Fatalf("pause state = %q, scheduler entry = %v", paused.Status, adapter.scheduler.HasEntry(createdA.ID))
	}
	resumed := decodeCronToolJob(t, mustToolCall(t, resume, ctxA, fmt.Sprintf(`{"id":%q}`, createdA.ID)))
	if resumed.Status != cron.StatusActive || !adapter.scheduler.HasEntry(createdA.ID) {
		t.Fatalf("resume state = %q, scheduler entry = %v", resumed.Status, adapter.scheduler.HasEntry(createdA.ID))
	}

	if _, err := get.Handler(ctxB, json.RawMessage(fmt.Sprintf(`{"id":%q}`, createdA.ID))); err == nil {
		t.Fatal("scope B must not mutate or inspect scope A's job")
	}
	if _, err := delete.Handler(ctxA, json.RawMessage(fmt.Sprintf(`{"id":%q}`, createdA.ID))); err != nil {
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

func mustToolCall(t *testing.T, tool htools.Tool, ctx context.Context, args string) string {
	t.Helper()
	result, err := tool.Handler(ctx, json.RawMessage(args))
	if err != nil {
		t.Fatalf("%s: %v", tool.Definition.Name, err)
	}
	return result
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

	// Get by name (fallback)
	got2, err := adapter.GetJob(ctx, "get-test")
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
