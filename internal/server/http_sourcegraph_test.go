package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-agent-harness/internal/harness"
)

func newSourcegraphTestServer(t *testing.T, cfg sourcegraphConfig, client *http.Client) *httptest.Server {
	t.Helper()
	registry := harness.NewRegistry()
	runner := harness.NewRunner(
		&staticProvider{result: harness.CompletionResult{Content: "done"}},
		registry,
		harness.RunnerConfig{
			DefaultModel:        "gpt-4.1-mini",
			DefaultSystemPrompt: "You are helpful.",
			MaxSteps:            1,
		},
	)
	handler := NewWithOptions(ServerOptions{
		Runner:      runner,
		Sourcegraph: cfg,
		HTTPClient:  client,
	})
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return ts
}

func TestSourcegraphNotConfigured(t *testing.T) {
	t.Parallel()
	ts := newSourcegraphTestServer(t, sourcegraphConfig{}, nil)

	payload := `{"query":"fmt.Println","limit":5}`
	resp, err := http.Post(ts.URL+"/v1/search/code", "application/json", bytes.NewBufferString(payload))
	if err != nil {
		t.Fatalf("POST /v1/search/code: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", resp.StatusCode)
	}
}

func TestSourcegraphMethodNotAllowed(t *testing.T) {
	t.Parallel()
	ts := newSourcegraphTestServer(t, sourcegraphConfig{}, nil)

	resp, err := http.Get(ts.URL + "/v1/search/code")
	if err != nil {
		t.Fatalf("GET /v1/search/code: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
}

func TestSourcegraphMissingQuery(t *testing.T) {
	t.Parallel()
	// Create a fake upstream to accept connections.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer upstream.Close()

	ts := newSourcegraphTestServer(t, sourcegraphConfig{Endpoint: upstream.URL}, upstream.Client())

	payload := `{"query":"","limit":5}`
	resp, err := http.Post(ts.URL+"/v1/search/code", "application/json", bytes.NewBufferString(payload))
	if err != nil {
		t.Fatalf("POST /v1/search/code: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestSourcegraphProxySuccess(t *testing.T) {
	t.Parallel()
	// Fake Sourcegraph upstream that returns structured results.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"results": []map[string]any{
				{"repository": "github.com/example/repo", "file": "main.go", "line": 42, "content": "fmt.Println"},
				{"repository": "github.com/other/repo", "file": "util.go", "line": 7, "content": "fmt.Println(\"hi\")"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer upstream.Close()

	ts := newSourcegraphTestServer(t, sourcegraphConfig{Endpoint: upstream.URL, Token: "test-token"}, upstream.Client())

	payload := `{"query":"fmt.Println","limit":10}`
	resp, err := http.Post(ts.URL+"/v1/search/code", "application/json", bytes.NewBufferString(payload))
	if err != nil {
		t.Fatalf("POST /v1/search/code: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Count   int `json:"count"`
		Results []struct {
			Repository string `json:"repository"`
			File       string `json:"file"`
			Line       int    `json:"line"`
			Content    string `json:"content"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Count != 2 {
		t.Errorf("count = %d, want 2", body.Count)
	}
	if len(body.Results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(body.Results))
	}
	if body.Results[0].Repository != "github.com/example/repo" {
		t.Errorf("results[0].repository = %q", body.Results[0].Repository)
	}
	if body.Results[0].Line != 42 {
		t.Errorf("results[0].line = %d, want 42", body.Results[0].Line)
	}
}

func TestSourcegraphProxyRawFallback(t *testing.T) {
	t.Parallel()
	// Fake upstream returns non-standard JSON (no "results" field).
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"matches":[{"file":"x.go"}]}`))
	}))
	defer upstream.Close()

	ts := newSourcegraphTestServer(t, sourcegraphConfig{Endpoint: upstream.URL}, upstream.Client())

	payload := `{"query":"something"}`
	resp, err := http.Post(ts.URL+"/v1/search/code", "application/json", bytes.NewBufferString(payload))
	if err != nil {
		t.Fatalf("POST /v1/search/code: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Count   int    `json:"count"`
		Raw     string `json:"raw"`
		Results []any  `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// raw fallback path: count=0, results=[], raw set
	if body.Count != 0 {
		t.Errorf("count = %d, want 0", body.Count)
	}
	if body.Raw == "" {
		t.Error("expected raw to be set in fallback mode")
	}
}

func TestSourcegraphInvalidJSON(t *testing.T) {
	t.Parallel()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	ts := newSourcegraphTestServer(t, sourcegraphConfig{Endpoint: upstream.URL}, upstream.Client())

	resp, err := http.Post(ts.URL+"/v1/search/code", "application/json", bytes.NewBufferString("not json"))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestSourcegraphDefaultLimit(t *testing.T) {
	t.Parallel()
	var capturedCount int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Count int `json:"count"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		capturedCount = body.Count
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer upstream.Close()

	ts := newSourcegraphTestServer(t, sourcegraphConfig{Endpoint: upstream.URL}, upstream.Client())

	// No limit specified — should default to 20.
	payload := `{"query":"test"}`
	resp, err := http.Post(ts.URL+"/v1/search/code", "application/json", bytes.NewBufferString(payload))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()

	if capturedCount != 20 {
		t.Errorf("default limit sent to upstream = %d, want 20", capturedCount)
	}
}
