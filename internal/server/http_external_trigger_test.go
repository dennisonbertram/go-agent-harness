package server

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"go-agent-harness/internal/harness"
	"go-agent-harness/internal/store"
	"go-agent-harness/internal/trigger"
)

// --- test helpers ---

// githubWebhookSig computes the X-Hub-Signature-256 value for a body and secret.
func githubWebhookSig(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// buildTriggerRequest encodes an ExternalTriggerEnvelope as JSON (without the
// signature field) and computes an HMAC-SHA256 signature over that body.
// The signature is returned separately to be sent as X-Trigger-Signature header.
// This avoids the circular dependency where the HMAC would need to cover itself.
func buildTriggerRequest(t *testing.T, source, secret, action, message, threadID string, extras map[string]string) ([]byte, string) {
	t.Helper()
	env := map[string]string{
		"source":    source,
		"action":    action,
		"message":   message,
		"thread_id": threadID,
	}
	for k, v := range extras {
		env[k] = v
	}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	sig := githubWebhookSig(secret, string(raw))
	return raw, sig
}

// makeGitHubRegistry returns a ValidatorRegistry with a GitHub validator.
func makeGitHubRegistry(secret string) *trigger.ValidatorRegistry {
	reg := trigger.NewValidatorRegistry()
	reg.Register("github", &trigger.GitHubValidator{Secret: secret})
	return reg
}

// newTriggerServer creates a test HTTP server backed by a runner+MemoryStore with
// auth disabled and the provided ValidatorRegistry.
func newTriggerServer(t *testing.T, provider harness.Provider, reg *trigger.ValidatorRegistry) (*httptest.Server, *store.MemoryStore) {
	t.Helper()
	runner := harness.NewRunner(provider, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     4,
	})
	ms := store.NewMemoryStore()
	handler := NewWithOptions(ServerOptions{
		Runner:       runner,
		Store:        ms,
		AuthDisabled: true,
		Validators:   reg,
	})
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return ts, ms
}

// sendTrigger sends a POST /v1/external/trigger request with the given body and
// X-Trigger-Signature header.
func sendTrigger(t *testing.T, ts *httptest.Server, body []byte, sig string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/external/trigger", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Trigger-Signature", sig)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return res
}

// blockingExternalProvider is a scripted provider that calls beforeCall before
// returning so the test can gate on run execution (keeping the run active).
type blockingExternalProvider struct {
	mu         sync.Mutex
	results    []harness.CompletionResult
	calls      int
	beforeCall func(idx int)
}

func (p *blockingExternalProvider) Complete(_ context.Context, _ harness.CompletionRequest) (harness.CompletionResult, error) {
	p.mu.Lock()
	idx := p.calls
	p.calls++
	var result harness.CompletionResult
	if idx < len(p.results) {
		result = p.results[idx]
	}
	p.mu.Unlock()
	if p.beforeCall != nil {
		p.beforeCall(idx)
	}
	return result, nil
}

// --- Tests ---

// TestHandleExternalTrigger_StartNewRun verifies that action="start" creates a
// new run and returns 202 with a run_id.
func TestHandleExternalTrigger_StartNewRun(t *testing.T) {
	t.Parallel()

	const secret = "test-github-secret"
	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	reg := makeGitHubRegistry(secret)
	ts, _ := newTriggerServer(t, provider, reg)

	body, sig := buildTriggerRequest(t, "github", secret, "start", "build the feature", "PR#42", nil)
	res := sendTrigger(t, ts, body, sig)
	defer res.Body.Close()

	if res.StatusCode != http.StatusAccepted {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 202, got %d: %s", res.StatusCode, raw)
	}
	var resp struct {
		RunID  string `json:"run_id"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.RunID == "" {
		t.Error("expected non-empty run_id")
	}
}

func TestHandleExternalTrigger_StartPersistsExactlyOnce(t *testing.T) {
	t.Parallel()

	const secret = "test-github-secret"
	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	reg := makeGitHubRegistry(secret)
	countingStore := newCreateRunCountingStore()
	runner := harness.NewRunner(provider, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     4,
		Store:        countingStore,
	})
	handler := NewWithOptions(ServerOptions{
		Runner:       runner,
		Store:        countingStore,
		AuthDisabled: true,
		Validators:   reg,
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	body, sig := buildTriggerRequest(t, "github", secret, "start", "build the feature", "PR#43", nil)
	res := sendTrigger(t, ts, body, sig)
	defer res.Body.Close()

	if res.StatusCode != http.StatusAccepted {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 202, got %d: %s", res.StatusCode, raw)
	}
	var resp struct {
		RunID string `json:"run_id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.RunID == "" {
		t.Fatal("expected non-empty run_id")
	}
	if got := countingStore.createCount(resp.RunID); got != 1 {
		t.Fatalf("CreateRun calls for trigger-start run %s = %d, want 1", resp.RunID, got)
	}
}

// TestHandleExternalTrigger_SteerActiveRun verifies that action="steer" on an
// active run returns 202.
func TestHandleExternalTrigger_SteerActiveRun(t *testing.T) {
	t.Parallel()

	const secret = "test-github-secret"
	blockCh := make(chan struct{})
	releaseCh := make(chan struct{})
	provider := &blockingExternalProvider{
		results: []harness.CompletionResult{{Content: "done"}},
		beforeCall: func(idx int) {
			if idx == 0 {
				close(blockCh)
				<-releaseCh
			}
		},
	}
	reg := makeGitHubRegistry(secret)
	ts, ms := newTriggerServer(t, provider, reg)

	// Derive the thread ID so we can pre-populate the store.
	threadID := trigger.DeriveExternalThreadID("github", "org", "repo", "PR#42")

	// Start a run via the direct API so the runner knows about it.
	startBody, _ := json.Marshal(map[string]string{
		"prompt":          "hello",
		"conversation_id": threadID.String(),
	})
	startRes, err := http.Post(ts.URL+"/v1/runs", "application/json", bytes.NewReader(startBody))
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	var created struct {
		RunID string `json:"run_id"`
	}
	_ = json.NewDecoder(startRes.Body).Decode(&created)
	startRes.Body.Close()

	// Pre-populate the store so ListRuns finds the run.
	_ = ms.CreateRun(context.Background(), &store.Run{
		ID:             created.RunID,
		ConversationID: threadID.String(),
		TenantID:       "default",
		Status:         store.RunStatusRunning,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	})

	// Wait for run to block inside the provider.
	<-blockCh

	body, sig := buildTriggerRequest(t, "github", secret, "steer", "pivot the approach", "PR#42", map[string]string{
		"repo_owner": "org",
		"repo_name":  "repo",
	})
	res := sendTrigger(t, ts, body, sig)
	defer res.Body.Close()

	if res.StatusCode != http.StatusAccepted {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 202, got %d: %s", res.StatusCode, raw)
	}

	close(releaseCh)
}

// TestHandleExternalTrigger_ContinueCompletedRun verifies that action="continue"
// on a completed run returns 202 with a new run_id.
func TestHandleExternalTrigger_ContinueCompletedRun(t *testing.T) {
	t.Parallel()

	const secret = "test-github-secret"
	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	reg := makeGitHubRegistry(secret)
	ts, ms := newTriggerServer(t, provider, reg)

	threadID := trigger.DeriveExternalThreadID("github", "org", "repo", "PR#99")

	// Start a run via the API and wait for it to complete.
	startBody, _ := json.Marshal(map[string]string{
		"prompt":          "original prompt",
		"conversation_id": threadID.String(),
	})
	startRes, err := http.Post(ts.URL+"/v1/runs", "application/json", bytes.NewReader(startBody))
	if err != nil {
		t.Fatalf("start run via API: %v", err)
	}
	var created struct {
		RunID string `json:"run_id"`
	}
	_ = json.NewDecoder(startRes.Body).Decode(&created)
	startRes.Body.Close()

	// Wait for completion.
	waitForRunStatus(t, ts, created.RunID, "completed", "failed")

	// Update the store to reflect the completed run with the correct thread ID.
	_ = ms.UpdateRun(context.Background(), &store.Run{
		ID:             created.RunID,
		ConversationID: threadID.String(),
		TenantID:       "default",
		Status:         store.RunStatusCompleted,
		Prompt:         "original prompt",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	})

	body, sig := buildTriggerRequest(t, "github", secret, "continue", "follow up", "PR#99", map[string]string{
		"repo_owner": "org",
		"repo_name":  "repo",
	})
	res := sendTrigger(t, ts, body, sig)
	defer res.Body.Close()

	if res.StatusCode != http.StatusAccepted {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 202, got %d: %s", res.StatusCode, raw)
	}
	var resp struct {
		RunID string `json:"run_id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.RunID == "" {
		t.Error("expected non-empty run_id in continue response")
	}
}

func TestHandleExternalTrigger_ContinuePersistsExactlyOnce(t *testing.T) {
	t.Parallel()

	const secret = "test-github-secret"
	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	reg := makeGitHubRegistry(secret)
	countingStore := newCreateRunCountingStore()
	runner := harness.NewRunner(provider, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     4,
		Store:        countingStore,
	})
	handler := NewWithOptions(ServerOptions{
		Runner:       runner,
		Store:        countingStore,
		AuthDisabled: true,
		Validators:   reg,
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	threadID := trigger.DeriveExternalThreadID("github", "org", "repo", "PR#199")

	initialRun, err := runner.StartRun(harness.RunRequest{
		Prompt:         "original prompt",
		ConversationID: threadID.String(),
		TenantID:       "default",
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForRunStatus(t, ts, initialRun.ID, "completed", "failed")

	body, sig := buildTriggerRequest(t, "github", secret, "continue", "follow up", "PR#199", map[string]string{
		"repo_owner": "org",
		"repo_name":  "repo",
	})
	res := sendTrigger(t, ts, body, sig)
	defer res.Body.Close()

	if res.StatusCode != http.StatusAccepted {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 202, got %d: %s", res.StatusCode, raw)
	}
	var resp struct {
		RunID string `json:"run_id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.RunID == "" {
		t.Fatal("expected non-empty run_id in continue response")
	}
	if got := countingStore.createCount(resp.RunID); got != 1 {
		t.Fatalf("CreateRun calls for trigger-continue run %s = %d, want 1", resp.RunID, got)
	}
}

// TestHandleExternalTrigger_InvalidSignature verifies that a wrong HMAC returns 401.
func TestHandleExternalTrigger_InvalidSignature(t *testing.T) {
	t.Parallel()

	const secret = "correct-secret"
	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	reg := makeGitHubRegistry(secret)
	ts, _ := newTriggerServer(t, provider, reg)

	body, _ := buildTriggerRequest(t, "github", secret, "start", "build", "PR#1", nil)
	// Send a signature computed with the WRONG secret.
	wrongSig := githubWebhookSig("wrong-secret", string(body))

	res := sendTrigger(t, ts, body, wrongSig)
	defer res.Body.Close()

	if res.StatusCode != http.StatusUnauthorized {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 401, got %d: %s", res.StatusCode, raw)
	}
}

// TestHandleExternalTrigger_SteerCompletedRun_Conflict verifies that steering a
// completed run returns 409 Conflict.
func TestHandleExternalTrigger_SteerCompletedRun_Conflict(t *testing.T) {
	t.Parallel()

	const secret = "test-github-secret"
	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	reg := makeGitHubRegistry(secret)
	ts, ms := newTriggerServer(t, provider, reg)

	threadID := trigger.DeriveExternalThreadID("github", "org", "repo", "PR#77")

	// Pre-populate store with a completed run.
	_ = ms.CreateRun(context.Background(), &store.Run{
		ID:             "run-conflict-test",
		ConversationID: threadID.String(),
		TenantID:       "default",
		Status:         store.RunStatusCompleted,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	})

	body, sig := buildTriggerRequest(t, "github", secret, "steer", "too late", "PR#77", map[string]string{
		"repo_owner": "org",
		"repo_name":  "repo",
	})
	res := sendTrigger(t, ts, body, sig)
	defer res.Body.Close()

	if res.StatusCode != http.StatusConflict {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 409, got %d: %s", res.StatusCode, raw)
	}
}

// TestHandleExternalTrigger_SteerNoThread_NotFound verifies that steering a
// thread with no existing run returns 404.
func TestHandleExternalTrigger_SteerNoThread_NotFound(t *testing.T) {
	t.Parallel()

	const secret = "test-github-secret"
	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	reg := makeGitHubRegistry(secret)
	ts, _ := newTriggerServer(t, provider, reg)

	body, sig := buildTriggerRequest(t, "github", secret, "steer", "where am I", "PR#9999", nil)
	res := sendTrigger(t, ts, body, sig)
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 404, got %d: %s", res.StatusCode, raw)
	}
}

// TestHandleExternalTrigger_NoValidatorForSource verifies that an unknown source
// returns 401 when no validator is registered for it.
func TestHandleExternalTrigger_NoValidatorForSource(t *testing.T) {
	t.Parallel()

	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	// Registry has only GitHub, not Slack.
	reg := makeGitHubRegistry("secret")
	ts, _ := newTriggerServer(t, provider, reg)

	body, sig := buildTriggerRequest(t, "slack", "slack-secret", "start", "hello", "C012/123", nil)
	res := sendTrigger(t, ts, body, sig)
	defer res.Body.Close()

	if res.StatusCode != http.StatusUnauthorized {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 401, got %d: %s", res.StatusCode, raw)
	}
}

// TestHandleExternalTrigger_MethodNotAllowed verifies that non-POST methods
// return 405.
func TestHandleExternalTrigger_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	reg := makeGitHubRegistry("secret")
	ts, _ := newTriggerServer(t, provider, reg)

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/external/trigger", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusMethodNotAllowed {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 405, got %d: %s", res.StatusCode, raw)
	}
}

// TestHandleExternalTrigger_DirectAPIUnaffected verifies that the existing
// POST /v1/runs endpoint still works normally after adding the new route.
func TestHandleExternalTrigger_DirectAPIUnaffected(t *testing.T) {
	t.Parallel()

	const secret = "test-github-secret"
	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	reg := makeGitHubRegistry(secret)
	ts, _ := newTriggerServer(t, provider, reg)

	res, err := http.Post(ts.URL+"/v1/runs", "application/json",
		bytes.NewBufferString(`{"prompt":"test direct api"}`))
	if err != nil {
		t.Fatalf("POST /v1/runs: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusAccepted {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 202 from direct /v1/runs, got %d: %s", res.StatusCode, raw)
	}
	var created struct {
		RunID string `json:"run_id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.RunID == "" {
		t.Error("expected non-empty run_id from direct API")
	}
}

// TestHandleExternalTrigger_MissingRequiredFields verifies 400 for missing
// required envelope fields (source, action, message, thread_id).
func TestHandleExternalTrigger_MissingRequiredFields(t *testing.T) {
	t.Parallel()

	const secret = "test-github-secret"
	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	reg := makeGitHubRegistry(secret)
	ts, _ := newTriggerServer(t, provider, reg)

	cases := []struct {
		name string
		body string
	}{
		{"missing_source", `{"action":"start","message":"hi","thread_id":"t1"}`},
		{"missing_action", `{"source":"github","message":"hi","thread_id":"t1"}`},
		{"missing_message", `{"source":"github","action":"start","thread_id":"t1"}`},
		{"missing_thread_id", `{"source":"github","action":"start","message":"hi"}`},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sig := githubWebhookSig(secret, tc.body)
			res := sendTrigger(t, ts, []byte(tc.body), sig)
			defer res.Body.Close()
			if res.StatusCode != http.StatusBadRequest {
				raw, _ := io.ReadAll(res.Body)
				t.Fatalf("%s: expected 400, got %d: %s", tc.name, res.StatusCode, raw)
			}
		})
	}
}
