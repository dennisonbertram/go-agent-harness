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
	"testing"
	"time"

	githubadapter "go-agent-harness/internal/github"
	"go-agent-harness/internal/harness"
	"go-agent-harness/internal/store"
	"go-agent-harness/internal/trigger"
)

// --- Helpers ---

// computeGitHubSig computes the X-Hub-Signature-256 header value for a body and secret.
func computeGitHubSig(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// newGitHubWebhookServer creates a test HTTP server configured with auth disabled,
// a GitHub validator, and a GitHubAdapter wired to the same secret.
func newGitHubWebhookServer(t *testing.T, provider harness.Provider, secret string) (*httptest.Server, *store.MemoryStore) {
	t.Helper()
	runner := harness.NewRunner(provider, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     4,
	})
	ms := store.NewMemoryStore()
	reg := trigger.NewValidatorRegistry()
	reg.Register("github", &trigger.GitHubValidator{Secret: secret})

	var adapter *githubadapter.GitHubAdapter
	if secret != "" {
		adapter = githubadapter.NewGitHubAdapter(secret)
	}

	handler := NewWithOptions(ServerOptions{
		Runner:        runner,
		Store:         ms,
		AuthDisabled:  true,
		Validators:    reg,
		GitHubAdapter: adapter,
	})
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return ts, ms
}

// sendGitHubWebhook sends a POST /v1/webhooks/github request.
func sendGitHubWebhook(t *testing.T, ts *httptest.Server, eventType, deliveryID, sig, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/webhooks/github", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", eventType)
	req.Header.Set("X-GitHub-Delivery", deliveryID)
	if sig != "" {
		req.Header.Set("X-Hub-Signature-256", sig)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return res
}

// issuesOpenedBody returns a realistic issues.opened JSON payload for the given issue number.
func issuesOpenedBody(number int) string {
	return `{
		"action": "opened",
		"issue": {
			"number": ` + itoa(number) + `,
			"title": "Bug: something is broken",
			"body": "Steps to reproduce the issue."
		},
		"repository": {
			"name": "go-agent-harness",
			"owner": {"login": "acme-corp"}
		}
	}`
}

// issueCommentCreatedBody returns a realistic issue_comment.created JSON payload.
func issueCommentCreatedBody(number int) string {
	return `{
		"action": "created",
		"issue": {
			"number": ` + itoa(number) + `,
			"title": "Bug: something is broken",
			"body": "Steps to reproduce the issue."
		},
		"comment": {"body": "I can also reproduce this."},
		"repository": {
			"name": "go-agent-harness",
			"owner": {"login": "acme-corp"}
		}
	}`
}

// itoa is a helper to avoid importing strconv in the test helper functions.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

// --- Tests ---

// TestHandleGitHubWebhook_StartNewRun verifies that an issues.opened event
// with a valid signature creates a new run and returns 202 with run_id.
func TestHandleGitHubWebhook_StartNewRun(t *testing.T) {
	t.Parallel()

	const secret = "test-webhook-secret"
	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	ts, _ := newGitHubWebhookServer(t, provider, secret)

	body := issuesOpenedBody(42)
	sig := computeGitHubSig(secret, body)
	res := sendGitHubWebhook(t, ts, "issues", "delivery-001", sig, body)
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
		t.Error("expected non-empty run_id in response")
	}
}

// TestHandleGitHubWebhook_SteerActiveRun verifies that an issue_comment.created
// event on an active thread steers the run and returns 202.
func TestHandleGitHubWebhook_SteerActiveRun(t *testing.T) {
	t.Parallel()

	const (
		secret    = "test-webhook-secret"
		issueNum  = 55
		repoOwner = "acme-corp"
		repoName  = "go-agent-harness"
	)

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

	ts, ms := newGitHubWebhookServer(t, provider, secret)

	// Derive the thread ID that the adapter will produce for this issue.
	threadID := trigger.DeriveExternalThreadID("github", repoOwner, repoName, itoa(issueNum))

	// Start a run via the direct API.
	startBody, _ := json.Marshal(map[string]string{
		"prompt":          "initial prompt",
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

	// Wait for the runner to enter the blocking call.
	<-blockCh

	// Pre-populate the store so ListRuns finds the active run.
	_ = ms.CreateRun(context.Background(), &store.Run{
		ID:             created.RunID,
		ConversationID: threadID.String(),
		TenantID:       "default",
		Status:         store.RunStatusRunning,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	})

	// Send the issue_comment event — should steer the active run.
	body := issueCommentCreatedBody(issueNum)
	sig := computeGitHubSig(secret, body)
	res := sendGitHubWebhook(t, ts, "issue_comment", "delivery-steer", sig, body)
	defer res.Body.Close()

	if res.StatusCode != http.StatusAccepted {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 202, got %d: %s", res.StatusCode, raw)
	}

	// Unblock the provider so the test server can shut down cleanly.
	close(releaseCh)
}

// TestHandleGitHubWebhook_InvalidSignature verifies that a bad X-Hub-Signature-256
// header returns 401.
func TestHandleGitHubWebhook_InvalidSignature(t *testing.T) {
	t.Parallel()

	const secret = "correct-secret"
	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	ts, _ := newGitHubWebhookServer(t, provider, secret)

	body := issuesOpenedBody(1)
	// Use the wrong secret to compute the signature.
	badSig := computeGitHubSig("wrong-secret", body)
	res := sendGitHubWebhook(t, ts, "issues", "delivery-bad-sig", badSig, body)
	defer res.Body.Close()

	if res.StatusCode != http.StatusUnauthorized {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 401, got %d: %s", res.StatusCode, raw)
	}
}

// TestHandleGitHubWebhook_MissingSig verifies that a missing X-Hub-Signature-256
// header results in a 401 (the GitHubValidator rejects an empty signature).
func TestHandleGitHubWebhook_MissingSig(t *testing.T) {
	t.Parallel()

	const secret = "test-webhook-secret"
	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	ts, _ := newGitHubWebhookServer(t, provider, secret)

	body := issuesOpenedBody(1)
	// No sig header — sig will be empty string.
	res := sendGitHubWebhook(t, ts, "issues", "delivery-no-sig", "", body)
	defer res.Body.Close()

	if res.StatusCode != http.StatusUnauthorized {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 401, got %d: %s", res.StatusCode, raw)
	}
}

// TestHandleGitHubWebhook_UnknownEventType verifies that an unsupported event
// type (e.g. "push") returns 400.
func TestHandleGitHubWebhook_UnknownEventType(t *testing.T) {
	t.Parallel()

	const secret = "test-webhook-secret"
	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	ts, _ := newGitHubWebhookServer(t, provider, secret)

	body := `{"action": "created", "ref": "refs/heads/main"}`
	sig := computeGitHubSig(secret, body)
	res := sendGitHubWebhook(t, ts, "push", "delivery-push", sig, body)
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 400, got %d: %s", res.StatusCode, raw)
	}
}

// TestHandleGitHubWebhook_MissingEventHeader verifies that a missing
// X-GitHub-Event header returns 400.
func TestHandleGitHubWebhook_MissingEventHeader(t *testing.T) {
	t.Parallel()

	const secret = "test-webhook-secret"
	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	ts, _ := newGitHubWebhookServer(t, provider, secret)

	body := issuesOpenedBody(10)
	sig := computeGitHubSig(secret, body)

	// Send without X-GitHub-Event header.
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/webhooks/github", bytes.NewReader([]byte(body)))
	req.Header.Set("X-GitHub-Delivery", "delivery-no-event")
	req.Header.Set("X-Hub-Signature-256", sig)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 400, got %d: %s", res.StatusCode, raw)
	}
}

// TestHandleGitHubWebhook_MissingDeliveryHeader verifies that a missing
// X-GitHub-Delivery header returns 400.
func TestHandleGitHubWebhook_MissingDeliveryHeader(t *testing.T) {
	t.Parallel()

	const secret = "test-webhook-secret"
	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	ts, _ := newGitHubWebhookServer(t, provider, secret)

	body := issuesOpenedBody(10)
	sig := computeGitHubSig(secret, body)

	// Send without X-GitHub-Delivery header.
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/webhooks/github", bytes.NewReader([]byte(body)))
	req.Header.Set("X-GitHub-Event", "issues")
	req.Header.Set("X-Hub-Signature-256", sig)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 400, got %d: %s", res.StatusCode, raw)
	}
}

// TestHandleGitHubWebhook_AdapterNotConfigured verifies that when no GitHub
// adapter is registered the endpoint returns 401.
func TestHandleGitHubWebhook_AdapterNotConfigured(t *testing.T) {
	t.Parallel()

	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	runner := harness.NewRunner(provider, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "test-model",
	})
	// Build a server without a GitHubAdapter.
	handler := NewWithOptions(ServerOptions{
		Runner:       runner,
		AuthDisabled: true,
	})
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	body := issuesOpenedBody(5)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/webhooks/github", bytes.NewReader([]byte(body)))
	req.Header.Set("X-GitHub-Event", "issues")
	req.Header.Set("X-GitHub-Delivery", "delivery-no-adapter")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusUnauthorized {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 401, got %d: %s", res.StatusCode, raw)
	}
}

// TestHandleGitHubWebhook_MethodNotAllowed verifies that GET returns 405.
func TestHandleGitHubWebhook_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	const secret = "test-webhook-secret"
	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	ts, _ := newGitHubWebhookServer(t, provider, secret)

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/webhooks/github", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusMethodNotAllowed {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 405, got %d: %s", res.StatusCode, raw)
	}
}

// TestHandleGitHubWebhook_UnrecognisedAction verifies that a GitHub event with
// a known type but unrecognised action (e.g. issues.deleted) returns 400
// because no trigger action can be derived.
func TestHandleGitHubWebhook_UnrecognisedAction(t *testing.T) {
	t.Parallel()

	const secret = "test-webhook-secret"
	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	ts, _ := newGitHubWebhookServer(t, provider, secret)

	body := `{
		"action": "deleted",
		"issue": {"number": 1, "title": "T", "body": "B"},
		"repository": {"name": "repo", "owner": {"login": "owner"}}
	}`
	sig := computeGitHubSig(secret, body)
	res := sendGitHubWebhook(t, ts, "issues", "delivery-deleted", sig, body)
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 400, got %d: %s", res.StatusCode, raw)
	}
}

// TestHandleGitHubWebhook_ExistingTriggerEndpointUnaffected verifies that the
// existing /v1/external/trigger endpoint still works correctly after the refactor.
func TestHandleGitHubWebhook_ExistingTriggerEndpointUnaffected(t *testing.T) {
	t.Parallel()

	const secret = "test-webhook-secret"
	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	ts, _ := newGitHubWebhookServer(t, provider, secret)

	// Use the existing trigger helper to send a start request.
	body, sig := buildTriggerRequest(t, "github", secret, "start", "test message", "thread-123", nil)
	res := sendTrigger(t, ts, body, sig)
	defer res.Body.Close()

	if res.StatusCode != http.StatusAccepted {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 202, got %d: %s", res.StatusCode, raw)
	}
}
