package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"go-agent-harness/internal/harness"
	"go-agent-harness/internal/store"
	"go-agent-harness/internal/trigger"
)

type createRunCountingStore struct {
	*store.MemoryStore

	mu             sync.Mutex
	createRunCalls map[string]int
}

func newCreateRunCountingStore() *createRunCountingStore {
	return &createRunCountingStore{
		MemoryStore:    store.NewMemoryStore(),
		createRunCalls: make(map[string]int),
	}
}

func (s *createRunCountingStore) CreateRun(ctx context.Context, run *store.Run) error {
	s.mu.Lock()
	s.createRunCalls[run.ID]++
	s.mu.Unlock()
	return s.MemoryStore.CreateRun(ctx, run)
}

func (s *createRunCountingStore) createRunCount(runID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.createRunCalls[runID]
}

func TestPostRunPersistsExactlyOnce(t *testing.T) {
	t.Parallel()

	runStore := newCreateRunCountingStore()
	runner := harness.NewRunner(
		&staticProvider{result: harness.CompletionResult{Content: "ok"}},
		harness.NewRegistry(),
		harness.RunnerConfig{
			DefaultModel:        "gpt-4.1-mini",
			DefaultSystemPrompt: "You are helpful.",
			MaxSteps:            1,
			Store:               runStore,
		},
	)
	ts := httptest.NewServer(NewWithOptions(ServerOptions{Runner: runner, Store: runStore, AuthDisabled: true}))
	defer ts.Close()

	res, err := http.Post(ts.URL+"/v1/runs", "application/json", bytes.NewBufferString(`{"prompt":"Hello"}`))
	if err != nil {
		t.Fatalf("POST /v1/runs: %v", err)
	}
	defer res.Body.Close()

	var created struct {
		RunID string `json:"run_id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.RunID == "" {
		t.Fatal("expected non-empty run_id")
	}

	if got := runStore.createRunCount(created.RunID); got != 1 {
		t.Fatalf("expected exactly 1 CreateRun call for %q, got %d", created.RunID, got)
	}
}

func TestExternalTriggerStartPersistsExactlyOnce(t *testing.T) {
	t.Parallel()

	const secret = "test-github-secret"
	runStore := newCreateRunCountingStore()
	runner := harness.NewRunner(
		&staticProvider{result: harness.CompletionResult{Content: "done"}},
		harness.NewRegistry(),
		harness.RunnerConfig{DefaultModel: "test-model", MaxSteps: 4, Store: runStore},
	)
	ts := httptest.NewServer(NewWithOptions(ServerOptions{
		Runner:       runner,
		Store:        runStore,
		AuthDisabled: true,
		Validators:   makeGitHubRegistry(secret),
	}))
	defer ts.Close()

	body, sig := buildTriggerRequest(t, "github", secret, "start", "build the feature", "PR#422", nil)
	res := sendTrigger(t, ts, body, sig)
	defer res.Body.Close()

	var created struct {
		RunID string `json:"run_id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.RunID == "" {
		t.Fatal("expected non-empty run_id")
	}

	if got := runStore.createRunCount(created.RunID); got != 1 {
		t.Fatalf("expected exactly 1 CreateRun call for %q, got %d", created.RunID, got)
	}
}

func TestExternalTriggerContinuePersistsExactlyOnce(t *testing.T) {
	t.Parallel()

	const secret = "test-github-secret"
	runStore := newCreateRunCountingStore()
	runner := harness.NewRunner(
		&staticProvider{result: harness.CompletionResult{Content: "done"}},
		harness.NewRegistry(),
		harness.RunnerConfig{DefaultModel: "test-model", MaxSteps: 4, Store: runStore},
	)
	ts := httptest.NewServer(NewWithOptions(ServerOptions{
		Runner:       runner,
		Store:        runStore,
		AuthDisabled: true,
		Validators:   makeGitHubRegistry(secret),
	}))
	defer ts.Close()

	threadID := trigger.DeriveExternalThreadID("github", "org", "repo", "PR#422").String()
	run, err := runner.StartRun(harness.RunRequest{
		Prompt:         "original prompt",
		ConversationID: threadID,
		TenantID:       "default",
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForRunStatus(t, ts, run.ID, "completed", "failed")

	body, sig := buildTriggerRequest(t, "github", secret, "continue", "follow up", "PR#422", map[string]string{
		"repo_owner": "org",
		"repo_name":  "repo",
	})
	res := sendTrigger(t, ts, body, sig)
	defer res.Body.Close()

	var created struct {
		RunID string `json:"run_id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.RunID == "" {
		t.Fatal("expected non-empty run_id")
	}

	if got := runStore.createRunCount(created.RunID); got != 1 {
		t.Fatalf("expected exactly 1 CreateRun call for %q, got %d", created.RunID, got)
	}
}
