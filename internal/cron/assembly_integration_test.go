package cron_test

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"go-agent-harness/internal/cron"
	"go-agent-harness/internal/harness"
	harnessserver "go-agent-harness/internal/server"
	"go-agent-harness/internal/store"

	"golang.org/x/crypto/bcrypt"
)

type assembledCronProvider struct{}

func (assembledCronProvider) Complete(context.Context, harness.CompletionRequest) (harness.CompletionResult, error) {
	return harness.CompletionResult{Content: "assembled"}, nil
}

func TestSchedulerHarnessExecutorRemoteStarterAuthenticatedHarnessdAssembly(t *testing.T) {
	runStore := store.NewMemoryStore()
	token, key, err := store.GenerateAPIKey("tenant-assembly", "cron assembly", []string{store.ScopeRunsWrite, store.ScopeRunsRead})
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}
	// This assembly test exercises real bearer authentication inside a bounded
	// remote request. Keep the random token and real middleware, but remove the
	// production cost-12 CPU budget from the fixture: under aggregate -race
	// load it can consume the entire request deadline before the handler runs.
	keyHash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt.GenerateFromPassword: %v", err)
	}
	key.KeyHash = string(keyHash)
	if cost, costErr := bcrypt.Cost([]byte(key.KeyHash)); costErr != nil || cost != bcrypt.MinCost {
		t.Fatalf("assembly API key bcrypt cost = %d, err=%v, want test cost %d", cost, costErr, bcrypt.MinCost)
	}
	if err := runStore.CreateAPIKey(context.Background(), key); err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
	runner := harness.NewRunner(assembledCronProvider{}, harness.NewRegistry(), harness.RunnerConfig{DefaultModel: "test-model", MaxSteps: 1, Store: runStore})
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })
	harnessd := httptest.NewServer(harnessserver.NewWithOptions(harnessserver.ServerOptions{Runner: runner, Store: runStore}))
	t.Cleanup(harnessd.Close)

	cronStore, err := cron.NewSQLiteStore(filepath.Join(t.TempDir(), "cron.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = cronStore.Close() })
	if err := cronStore.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	// Bcrypt-backed harnessd authentication is deliberately expensive; allow
	// race-instrumented execution enough time without weakening other tests.
	starter := cron.NewRemoteRunStarter(cron.RemoteRunStarterConfig{BaseURL: harnessd.URL, APIKey: token, RequestTimeout: 5 * time.Second})
	scheduler := cron.NewScheduler(cronStore, &cron.HarnessExecutor{Starter: starter, Observer: starter}, cron.RealClock{}, cron.SchedulerConfig{MaxConcurrent: 1, Jitter: cron.JitterConfig{}})
	job, err := cronStore.CreateJob(context.Background(), cron.Job{ID: "job-assembly", TenantID: "tenant-assembly", ConversationID: "conversation-assembly", AgentID: "agent-assembly", Name: "assembled remote run", Schedule: "*/5 * * * *", ExecType: cron.ExecTypeHarness, ExecConfig: `{"prompt":"continue the watched conversation"}`, Status: cron.StatusActive, TimeoutSec: 10})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if err := scheduler.Start(context.Background()); err != nil {
		t.Fatalf("Scheduler.Start: %v", err)
	}
	t.Cleanup(func() { scheduler.Stop() })
	if err := scheduler.TriggerJob(context.Background(), job.ID); err != nil {
		t.Fatalf("TriggerJob: %v", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		executions, listErr := cronStore.ListExecutions(context.Background(), job.ID, 10, 0)
		if listErr != nil {
			t.Fatalf("ListExecutions: %v", listErr)
		}
		if len(executions) == 1 && executions[0].Status == cron.ExecStatusSuccess {
			if executions[0].RunID == "" {
				t.Fatalf("assembled execution did not durably link accepted run: %+v", executions[0])
			}
			if executions[0].OutputSummary != "assembled" {
				t.Fatalf("assembled remote terminal output = %q, want %q", executions[0].OutputSummary, "assembled")
			}
			run, found := runner.GetRun(executions[0].RunID)
			if !found || run.TenantID != job.TenantID || run.ConversationID != job.ConversationID || run.AgentID != job.AgentID {
				t.Fatalf("remote harness run scope = %+v found=%t", run, found)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("assembled execution did not succeed: %+v", executions)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
