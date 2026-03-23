package server

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-agent-harness/internal/harness"
	linearadapter "go-agent-harness/internal/linear"
	"go-agent-harness/internal/store"
	"go-agent-harness/internal/trigger"
)

// --- Helpers ---

// computeLinearSig computes the X-Linear-Signature HMAC for the given secret and body.
func computeLinearSig(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

// noopLinearValidator accepts any signature — used for tests that need to bypass
// the actual HMAC check (e.g. when body is constructed inline).
type noopLinearValidator struct{}

func (v *noopLinearValidator) ValidateSignature(_ context.Context, _ trigger.ExternalTriggerEnvelope) error {
	return nil
}

// newLinearWebhookServer creates a test HTTP server configured with auth disabled,
// a Linear validator, and a LinearAdapter.
func newLinearWebhookServer(t *testing.T, provider harness.Provider, secret string) (*httptest.Server, *store.MemoryStore) {
	t.Helper()
	runner := harness.NewRunner(provider, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     4,
	})
	ms := store.NewMemoryStore()
	reg := trigger.NewValidatorRegistry()
	if secret != "" {
		reg.Register("linear", &trigger.LinearValidator{Secret: secret})
	}

	var adapter *linearadapter.LinearAdapter
	if secret != "" {
		adapter = linearadapter.NewLinearAdapter()
	}

	handler := NewWithOptions(ServerOptions{
		Runner:        runner,
		Store:         ms,
		AuthDisabled:  true,
		Validators:    reg,
		LinearAdapter: adapter,
	})
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return ts, ms
}

// sendLinearWebhook sends a POST /v1/webhooks/linear request.
func sendLinearWebhook(t *testing.T, ts *httptest.Server, sig, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/webhooks/linear", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if sig != "" {
		req.Header.Set("X-Linear-Signature", sig)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return res
}

// linearIssueCreateBody returns a Linear Issue.create JSON payload.
func linearIssueCreateBody(identifier, title string) string {
	return `{
		"type": "Issue",
		"action": "create",
		"organizationId": "org-abc123",
		"data": {
			"id": "issue-uuid-test",
			"identifier": "` + identifier + `",
			"title": "` + title + `",
			"description": "Test description.",
			"teamId": "team-uuid-test"
		}
	}`
}

// linearCommentBody returns a Linear Comment.create JSON payload.
func linearCommentBody(issueIdentifier, commentText string) string {
	return `{
		"type": "Comment",
		"action": "create",
		"organizationId": "org-abc123",
		"data": {
			"id": "comment-uuid-test",
			"body": "` + commentText + `",
			"issueId": "issue-uuid-test",
			"issue": {
				"identifier": "` + issueIdentifier + `",
				"title": "Some issue title"
			}
		}
	}`
}

// --- Tests ---

// TestHandleLinearWebhook_IssueCreate verifies that a valid Linear Issue.create
// event with correct signature returns 202 (action="start").
func TestHandleLinearWebhook_IssueCreate(t *testing.T) {
	t.Parallel()

	const secret = "linear-webhook-secret"

	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	runner := harness.NewRunner(provider, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     4,
	})
	ms := store.NewMemoryStore()

	body := linearIssueCreateBody("ENG-999", "Test issue")
	sig := computeLinearSig(secret, body)

	reg := trigger.NewValidatorRegistry()
	reg.Register("linear", &trigger.LinearValidator{Secret: secret})

	adapter := linearadapter.NewLinearAdapter()
	handler := NewWithOptions(ServerOptions{
		Runner:        runner,
		Store:         ms,
		AuthDisabled:  true,
		Validators:    reg,
		LinearAdapter: adapter,
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	res := sendLinearWebhook(t, ts, sig, body)
	defer res.Body.Close()

	if res.StatusCode != http.StatusAccepted {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 202, got %d: %s", res.StatusCode, raw)
	}
}

// TestHandleLinearWebhook_InvalidSignature verifies that a bad X-Linear-Signature
// returns 401.
func TestHandleLinearWebhook_InvalidSignature(t *testing.T) {
	t.Parallel()

	const secret = "correct-secret"
	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	ts, _ := newLinearWebhookServer(t, provider, secret)

	body := linearIssueCreateBody("ENG-1", "Bug fix")
	badSig := computeLinearSig("wrong-secret", body)
	res := sendLinearWebhook(t, ts, badSig, body)
	defer res.Body.Close()

	if res.StatusCode != http.StatusUnauthorized {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 401, got %d: %s", res.StatusCode, raw)
	}
}

// TestHandleLinearWebhook_UnsupportedEventType verifies that an unsupported
// Linear event type returns 400.
func TestHandleLinearWebhook_UnsupportedEventType(t *testing.T) {
	t.Parallel()

	const secret = "test-secret"
	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	// Use noop validator since we want to test the 400 from the adapter.
	runner := harness.NewRunner(provider, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     4,
	})
	ms := store.NewMemoryStore()
	reg := trigger.NewValidatorRegistry()
	reg.Register("linear", &noopLinearValidator{})
	adapter := linearadapter.NewLinearAdapter()
	handler := NewWithOptions(ServerOptions{
		Runner:        runner,
		Store:         ms,
		AuthDisabled:  true,
		Validators:    reg,
		LinearAdapter: adapter,
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	body := `{"type":"Project","action":"create","data":{"id":"proj-001"}}`
	sig := computeLinearSig(secret, body)
	res := sendLinearWebhook(t, ts, sig, body)
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 400, got %d: %s", res.StatusCode, raw)
	}
}

// TestHandleLinearWebhook_UnrecognisedAction verifies that a supported event type
// with an unrecognised action (e.g. Issue.delete) returns 400.
func TestHandleLinearWebhook_UnrecognisedAction(t *testing.T) {
	t.Parallel()

	const secret = "test-secret"
	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	runner := harness.NewRunner(provider, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     4,
	})
	ms := store.NewMemoryStore()
	reg := trigger.NewValidatorRegistry()
	reg.Register("linear", &noopLinearValidator{})
	adapter := linearadapter.NewLinearAdapter()
	handler := NewWithOptions(ServerOptions{
		Runner:        runner,
		Store:         ms,
		AuthDisabled:  true,
		Validators:    reg,
		LinearAdapter: adapter,
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	body := `{
		"type": "Issue",
		"action": "delete",
		"data": {"id": "issue-uuid", "identifier": "ENG-1", "title": "T", "teamId": "t"}
	}`
	sig := computeLinearSig(secret, body)
	res := sendLinearWebhook(t, ts, sig, body)
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 400, got %d: %s", res.StatusCode, raw)
	}
}

// TestHandleLinearWebhook_AdapterNotConfigured verifies that when no Linear
// adapter is registered the endpoint returns 401.
func TestHandleLinearWebhook_AdapterNotConfigured(t *testing.T) {
	t.Parallel()

	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	runner := harness.NewRunner(provider, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "test-model",
	})
	// Build a server without a LinearAdapter.
	handler := NewWithOptions(ServerOptions{
		Runner:       runner,
		AuthDisabled: true,
	})
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	body := linearIssueCreateBody("ENG-1", "Test")
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/webhooks/linear", bytes.NewReader([]byte(body)))
	req.Header.Set("X-Linear-Signature", "abc123")
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

// TestHandleLinearWebhook_MethodNotAllowed verifies that GET returns 405.
func TestHandleLinearWebhook_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	const secret = "test-secret"
	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	ts, _ := newLinearWebhookServer(t, provider, secret)

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/webhooks/linear", nil)
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

// TestHandleLinearWebhook_SlackEndpointUnaffected verifies that the Slack
// webhook endpoint is not affected by Linear adapter configuration.
func TestHandleLinearWebhook_SlackEndpointUnaffected(t *testing.T) {
	t.Parallel()

	const secret = "test-secret"
	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	ts, _ := newLinearWebhookServer(t, provider, secret)

	// POST to /v1/webhooks/slack should return 401 (no slack adapter on this server).
	body := `{"type":"event_callback","event_id":"Ev1","team_id":"T1","event":{"type":"app_mention","user":"U1","text":"hi","ts":"1234.5","channel":"C1"}}`
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/webhooks/slack", bytes.NewReader([]byte(body)))
	req.Header.Set("X-Slack-Request-Timestamp", "1609459200")
	req.Header.Set("X-Slack-Signature", "v0=abc123")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer res.Body.Close()

	// Should be 401 (no slack adapter) — the Linear route doesn't interfere.
	if res.StatusCode != http.StatusUnauthorized {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 401, got %d: %s", res.StatusCode, raw)
	}
}
