package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-agent-harness/internal/harness"
	htools "go-agent-harness/internal/harness/tools"
)

// mockSummarizerProvider is a provider whose Complete method returns a
// fixed summary string so the summarize endpoint works without a real LLM.
type mockSummarizerProvider struct {
	summary string
	err     error
}

func (p *mockSummarizerProvider) Complete(_ context.Context, _ harness.CompletionRequest) (harness.CompletionResult, error) {
	if p.err != nil {
		return harness.CompletionResult{}, p.err
	}
	return harness.CompletionResult{Content: p.summary}, nil
}

// nilProvider has no summarization capability (simulates missing provider).
// We achieve this by using a runner constructed without a provider via the
// testable summarizer injection approach: set runner's GetSummarizer to nil.
// Since we cannot nil out a real runner's provider easily, we use a separate
// mock that implements htools.MessageSummarizer directly for the nil case test.

// mockMessageSummarizer directly implements htools.MessageSummarizer for tests.
type mockMessageSummarizer struct {
	result string
	err    error
}

func (m *mockMessageSummarizer) SummarizeMessages(_ context.Context, _ []map[string]any) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.result, nil
}

// Compile-time check.
var _ htools.MessageSummarizer = (*mockMessageSummarizer)(nil)

func TestSummarizeEndpointSuccess(t *testing.T) {
	t.Parallel()

	// Use a provider that returns a fixed summary as completion content.
	provider := &mockSummarizerProvider{summary: "This is a test summary."}
	runner := harness.NewRunner(provider, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "gpt-4.1-mini",
		MaxSteps:     1,
	})

	ts := httptest.NewServer(New(runner))
	defer ts.Close()

	body := `{"messages":[{"role":"user","content":"Hello"},{"role":"assistant","content":"Hi there"}]}`
	res, err := http.Post(ts.URL+"/v1/summarize", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("POST /v1/summarize: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 200, got %d: %s", res.StatusCode, string(b))
	}

	var resp struct {
		Summary string `json:"summary"`
	}
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Summary == "" {
		t.Error("expected non-empty summary in response")
	}
}

func TestSummarizeEndpointNilSummarizer(t *testing.T) {
	t.Parallel()

	// NewRunner with a nil provider means GetSummarizer() returns nil.
	// runner.go's NewRunner accepts nil provider; StartRun will refuse it,
	// but the summarize endpoint checks it before calling provider methods.
	runner := harness.NewRunner(nil, harness.NewRegistry(), harness.RunnerConfig{DefaultModel: "gpt-4.1-mini"})

	ts := httptest.NewServer(New(runner))
	defer ts.Close()

	body := `{"messages":[{"role":"user","content":"Hello"}]}`
	res, err := http.Post(ts.URL+"/v1/summarize", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("POST /v1/summarize: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusServiceUnavailable {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 503, got %d: %s", res.StatusCode, string(b))
	}

	var resp map[string]map[string]string
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if resp["error"]["code"] != "summarizer_not_configured" {
		t.Errorf("expected error code summarizer_not_configured, got %q", resp["error"]["code"])
	}
}

func TestSummarizeEndpointMethodNotAllowed(t *testing.T) {
	t.Parallel()

	provider := &mockSummarizerProvider{summary: "ok"}
	runner := harness.NewRunner(provider, harness.NewRegistry(), harness.RunnerConfig{DefaultModel: "gpt-4.1-mini"})
	ts := httptest.NewServer(New(runner))
	defer ts.Close()

	res, err := http.Get(ts.URL + "/v1/summarize")
	if err != nil {
		t.Fatalf("GET /v1/summarize: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", res.StatusCode)
	}
	if got := res.Header.Get("Allow"); got != http.MethodPost {
		t.Errorf("expected Allow: POST, got %q", got)
	}
}

func TestSummarizeEndpointEmptyMessages(t *testing.T) {
	t.Parallel()

	provider := &mockSummarizerProvider{summary: "ok"}
	runner := harness.NewRunner(provider, harness.NewRegistry(), harness.RunnerConfig{DefaultModel: "gpt-4.1-mini"})
	ts := httptest.NewServer(New(runner))
	defer ts.Close()

	body := `{"messages":[]}`
	res, err := http.Post(ts.URL+"/v1/summarize", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("POST /v1/summarize: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 400, got %d: %s", res.StatusCode, string(b))
	}
}

func TestSummarizeEndpointInvalidJSON(t *testing.T) {
	t.Parallel()

	provider := &mockSummarizerProvider{summary: "ok"}
	runner := harness.NewRunner(provider, harness.NewRegistry(), harness.RunnerConfig{DefaultModel: "gpt-4.1-mini"})
	ts := httptest.NewServer(New(runner))
	defer ts.Close()

	res, err := http.Post(ts.URL+"/v1/summarize", "application/json", bytes.NewBufferString("{bad json"))
	if err != nil {
		t.Fatalf("POST /v1/summarize: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 400, got %d: %s", res.StatusCode, string(b))
	}
	var resp map[string]map[string]string
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if resp["error"]["code"] != "invalid_json" {
		t.Errorf("expected error code invalid_json, got %q", resp["error"]["code"])
	}
}
