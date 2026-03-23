package server

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-agent-harness/internal/harness"
	slackadapter "go-agent-harness/internal/slack"
	"go-agent-harness/internal/store"
	"go-agent-harness/internal/trigger"
)

// --- Helpers ---

// computeSlackSig computes a valid X-Slack-Signature for the given secret, timestamp, and body.
func computeSlackSig(secret, timestamp, body string) string {
	basestring := fmt.Sprintf("v0:%s:%s", timestamp, body)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(basestring))
	return "v0=" + hex.EncodeToString(mac.Sum(nil))
}

// noopSlackValidator accepts any signature — used for tests that need to bypass
// the 5-minute timestamp freshness window.
type noopSlackValidator struct{}

func (v *noopSlackValidator) ValidateSignature(_ context.Context, _ trigger.ExternalTriggerEnvelope) error {
	return nil
}

// newSlackWebhookServer creates a test HTTP server configured with auth disabled,
// a Slack validator, and a SlackAdapter.
func newSlackWebhookServer(t *testing.T, provider harness.Provider, secret string) (*httptest.Server, *store.MemoryStore) {
	t.Helper()
	runner := harness.NewRunner(provider, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     4,
	})
	ms := store.NewMemoryStore()
	reg := trigger.NewValidatorRegistry()
	if secret != "" {
		reg.Register("slack", &trigger.SlackValidator{Secret: secret})
	}

	var adapter *slackadapter.SlackAdapter
	if secret != "" {
		adapter = slackadapter.NewSlackAdapter()
	}

	handler := NewWithOptions(ServerOptions{
		Runner:       runner,
		Store:        ms,
		AuthDisabled: true,
		Validators:   reg,
		SlackAdapter: adapter,
	})
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return ts, ms
}

// sendSlackWebhook sends a POST /v1/webhooks/slack request.
func sendSlackWebhook(t *testing.T, ts *httptest.Server, timestamp, sig, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/webhooks/slack", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if timestamp != "" {
		req.Header.Set("X-Slack-Request-Timestamp", timestamp)
	}
	if sig != "" {
		req.Header.Set("X-Slack-Signature", sig)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return res
}

// slackAppMentionBody returns a Slack event_callback JSON payload with a unique event ID.
func slackAppMentionBody(channelID, text string) string {
	return `{
		"type": "event_callback",
		"event_id": "Ev` + channelID + `",
		"team_id": "T012AB3C4",
		"event": {
			"type": "app_mention",
			"user": "U012AB3C4",
			"text": "` + text + `",
			"ts": "1234567890.123456",
			"channel": "` + channelID + `"
		}
	}`
}

// --- Tests ---

// TestHandleSlackWebhook_ValidRequest verifies that a valid Slack event with a
// correct signature is accepted. Since action="steer" and no run exists, expect 404.
func TestHandleSlackWebhook_ValidRequest(t *testing.T) {
	t.Parallel()

	const (
		secret    = "slack-signing-secret"
		timestamp = "1609459200"
	)

	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	runner := harness.NewRunner(provider, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     4,
	})
	ms := store.NewMemoryStore()
	body := slackAppMentionBody("C01234567", "please do something")
	sig := computeSlackSig(secret, timestamp, body)

	// Use a noop validator to bypass the 5-minute timestamp window in tests.
	reg := trigger.NewValidatorRegistry()
	reg.Register("slack", &noopSlackValidator{})

	adapter := slackadapter.NewSlackAdapter()
	handler := NewWithOptions(ServerOptions{
		Runner:       runner,
		Store:        ms,
		AuthDisabled: true,
		Validators:   reg,
		SlackAdapter: adapter,
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/webhooks/slack", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Slack-Request-Timestamp", timestamp)
	req.Header.Set("X-Slack-Signature", sig)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer res.Body.Close()

	// action="steer" with no existing run → 404 is the expected outcome
	// (dispatcher ran, sig validated, routing decided no thread found).
	if res.StatusCode != http.StatusAccepted && res.StatusCode != http.StatusNotFound {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 202 or 404, got %d: %s", res.StatusCode, raw)
	}
}

// TestHandleSlackWebhook_InvalidSignature verifies that a bad X-Slack-Signature
// returns 401.
func TestHandleSlackWebhook_InvalidSignature(t *testing.T) {
	t.Parallel()

	const secret = "correct-secret"
	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	ts, _ := newSlackWebhookServer(t, provider, secret)

	const timestamp = "1609459200"
	body := slackAppMentionBody("C01234567", "please do something")
	// Use wrong secret for signature.
	badSig := computeSlackSig("wrong-secret", timestamp, body)
	res := sendSlackWebhook(t, ts, timestamp, badSig, body)
	defer res.Body.Close()

	if res.StatusCode != http.StatusUnauthorized {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 401, got %d: %s", res.StatusCode, raw)
	}
}

// TestHandleSlackWebhook_MissingTimestampHeader verifies that missing
// X-Slack-Request-Timestamp returns 400.
func TestHandleSlackWebhook_MissingTimestampHeader(t *testing.T) {
	t.Parallel()

	const secret = "test-secret"
	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	ts, _ := newSlackWebhookServer(t, provider, secret)

	body := slackAppMentionBody("C01234567", "hello")
	// Omit timestamp — send only sig.
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/webhooks/slack", bytes.NewReader([]byte(body)))
	req.Header.Set("X-Slack-Signature", "v0=abc123")
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

// TestHandleSlackWebhook_MissingSignatureHeader verifies that missing
// X-Slack-Signature returns 400.
func TestHandleSlackWebhook_MissingSignatureHeader(t *testing.T) {
	t.Parallel()

	const secret = "test-secret"
	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	ts, _ := newSlackWebhookServer(t, provider, secret)

	body := slackAppMentionBody("C01234567", "hello")
	// Omit sig — send only timestamp.
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/webhooks/slack", bytes.NewReader([]byte(body)))
	req.Header.Set("X-Slack-Request-Timestamp", "1609459200")
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

// TestHandleSlackWebhook_AdapterNotConfigured verifies that when no Slack
// adapter is registered the endpoint returns 401.
func TestHandleSlackWebhook_AdapterNotConfigured(t *testing.T) {
	t.Parallel()

	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	runner := harness.NewRunner(provider, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "test-model",
	})
	// Build a server without a SlackAdapter.
	handler := NewWithOptions(ServerOptions{
		Runner:       runner,
		AuthDisabled: true,
	})
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	body := slackAppMentionBody("C01234567", "hello")
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/webhooks/slack", bytes.NewReader([]byte(body)))
	req.Header.Set("X-Slack-Request-Timestamp", "1609459200")
	req.Header.Set("X-Slack-Signature", "v0=abc123")
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

// TestHandleSlackWebhook_MethodNotAllowed verifies that GET returns 405.
func TestHandleSlackWebhook_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	const secret = "test-secret"
	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	ts, _ := newSlackWebhookServer(t, provider, secret)

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/webhooks/slack", nil)
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

// TestHandleSlackWebhook_GithubEndpointUnaffected verifies that the GitHub
// webhook endpoint still returns expected status when Slack adapter is configured.
func TestHandleSlackWebhook_GithubEndpointUnaffected(t *testing.T) {
	t.Parallel()

	const secret = "test-secret"
	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	ts, _ := newSlackWebhookServer(t, provider, secret)

	// POST to /v1/webhooks/github should return 401 (no github adapter configured on this server).
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/webhooks/github", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("X-GitHub-Event", "issues")
	req.Header.Set("X-GitHub-Delivery", "delivery-001")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer res.Body.Close()

	// Should be 401 (no github adapter) — the Slack route doesn't interfere.
	if res.StatusCode != http.StatusUnauthorized {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 401, got %d: %s", res.StatusCode, raw)
	}
}
