package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-agent-harness/internal/harness"
)

// TestContextEndpointReturnsStatus verifies GET /v1/runs/{id}/context returns
// context status JSON for an existing run.
func TestContextEndpointReturnsStatus(t *testing.T) {
	t.Parallel()

	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	runner := harness.NewRunner(provider, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     2,
	})
	handler := New(runner)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Start a run.
	res, err := http.Post(ts.URL+"/v1/runs", "application/json",
		bytes.NewBufferString(`{"prompt":"hello"}`))
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	defer res.Body.Close()

	var created struct {
		RunID string `json:"run_id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.RunID == "" {
		t.Fatal("expected run_id in response")
	}

	// Poll until the run is registered (it may still be queued/running).
	deadline := time.Now().Add(5 * time.Second)
	var contextRes *http.Response
	for time.Now().Before(deadline) {
		contextRes, err = http.Get(ts.URL + "/v1/runs/" + created.RunID + "/context")
		if err != nil {
			t.Fatalf("context request: %v", err)
		}
		if contextRes.StatusCode == http.StatusOK {
			break
		}
		contextRes.Body.Close()
		time.Sleep(10 * time.Millisecond)
	}
	defer contextRes.Body.Close()

	if contextRes.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(contextRes.Body)
		t.Fatalf("expected 200, got %d: %s", contextRes.StatusCode, string(body))
	}

	var status struct {
		MessageCount    int    `json:"message_count"`
		EstimatedTokens int    `json:"estimated_tokens"`
		ContextPressure string `json:"context_pressure"`
	}
	if err := json.NewDecoder(contextRes.Body).Decode(&status); err != nil {
		t.Fatalf("decode context response: %v", err)
	}

	if status.ContextPressure == "" {
		t.Error("expected non-empty context_pressure")
	}
	validPressures := map[string]bool{"low": true, "medium": true, "high": true}
	if !validPressures[status.ContextPressure] {
		t.Errorf("unexpected context_pressure %q, want one of low/medium/high", status.ContextPressure)
	}
}

// TestContextEndpoint404ForUnknownRun verifies GET /v1/runs/{id}/context returns
// 404 for an unknown run ID.
func TestContextEndpoint404ForUnknownRun(t *testing.T) {
	t.Parallel()

	runner := harness.NewRunner(&staticProvider{}, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     2,
	})
	handler := New(runner)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/v1/runs/nonexistent-run-id/context")
	if err != nil {
		t.Fatalf("context request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 404, got %d: %s", res.StatusCode, string(body))
	}
}

// TestContextEndpointMethodNotAllowed verifies GET is the only allowed method.
func TestContextEndpointMethodNotAllowed(t *testing.T) {
	t.Parallel()

	runner := harness.NewRunner(&staticProvider{}, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     2,
	})
	handler := New(runner)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/runs/some-id/context", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusMethodNotAllowed {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 405, got %d: %s", res.StatusCode, string(body))
	}
}

// TestCompactEndpointTriggersCompaction verifies POST /v1/runs/{id}/compact
// triggers compaction on an active run and returns the expected JSON shape.
func TestCompactEndpointTriggersCompaction(t *testing.T) {
	t.Parallel()

	blockCh := make(chan struct{})
	releaseCh := make(chan struct{})

	provider := &gatingServerProvider{
		results: []harness.CompletionResult{
			{Content: "done"},
		},
		beforeCall: func(idx int) {
			if idx == 0 {
				close(blockCh)
				<-releaseCh
			}
		},
	}

	runner := harness.NewRunner(provider, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     2,
	})
	handler := New(runner)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Start a run.
	res, err := http.Post(ts.URL+"/v1/runs", "application/json",
		bytes.NewBufferString(`{"prompt":"hello"}`))
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	defer res.Body.Close()

	var created struct {
		RunID string `json:"run_id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Wait for the run to be in-flight.
	<-blockCh

	// POST compact while run is active.
	compactBody := `{"mode":"strip"}`
	compactRes, err := http.Post(
		ts.URL+"/v1/runs/"+created.RunID+"/compact",
		"application/json",
		bytes.NewBufferString(compactBody),
	)
	if err != nil {
		t.Fatalf("compact request: %v", err)
	}
	defer compactRes.Body.Close()

	if compactRes.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(compactRes.Body)
		t.Fatalf("expected 200, got %d: %s", compactRes.StatusCode, string(body))
	}

	var result struct {
		OK              bool `json:"ok"`
		MessagesRemoved int  `json:"messages_removed"`
	}
	if err := json.NewDecoder(compactRes.Body).Decode(&result); err != nil {
		t.Fatalf("decode compact response: %v", err)
	}
	if !result.OK {
		t.Error("expected ok=true in compact response")
	}

	// Release the provider so the run can finish.
	close(releaseCh)
}

// TestCompactEndpoint404ForUnknownRun verifies POST /v1/runs/{id}/compact
// returns 404 for an unknown run ID.
func TestCompactEndpoint404ForUnknownRun(t *testing.T) {
	t.Parallel()

	runner := harness.NewRunner(&staticProvider{}, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     2,
	})
	handler := New(runner)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	res, err := http.Post(
		ts.URL+"/v1/runs/nonexistent-run-id/compact",
		"application/json",
		bytes.NewBufferString(`{"mode":"strip"}`),
	)
	if err != nil {
		t.Fatalf("compact request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 404, got %d: %s", res.StatusCode, string(body))
	}
}

// TestCompactEndpoint409ForInactiveRun verifies POST /v1/runs/{id}/compact
// returns 409 when the run has already completed.
func TestCompactEndpoint409ForInactiveRun(t *testing.T) {
	t.Parallel()

	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
	runner := harness.NewRunner(provider, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     2,
	})
	handler := New(runner)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Start and wait for completion.
	res, err := http.Post(ts.URL+"/v1/runs", "application/json",
		bytes.NewBufferString(`{"prompt":"hello"}`))
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	defer res.Body.Close()

	var created struct {
		RunID string `json:"run_id"`
	}
	json.NewDecoder(res.Body).Decode(&created)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		checkRes, _ := http.Get(ts.URL + "/v1/runs/" + created.RunID)
		if checkRes != nil {
			var runState struct {
				Status string `json:"status"`
			}
			json.NewDecoder(checkRes.Body).Decode(&runState)
			checkRes.Body.Close()
			if runState.Status == "completed" || runState.Status == "failed" {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Now try to compact the finished run.
	compactRes, err := http.Post(
		ts.URL+"/v1/runs/"+created.RunID+"/compact",
		"application/json",
		bytes.NewBufferString(`{"mode":"strip"}`),
	)
	if err != nil {
		t.Fatalf("compact request: %v", err)
	}
	defer compactRes.Body.Close()

	if compactRes.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(compactRes.Body)
		t.Fatalf("expected 409, got %d: %s", compactRes.StatusCode, string(body))
	}
}

// TestCompactEndpointMethodNotAllowed verifies only POST is accepted.
func TestCompactEndpointMethodNotAllowed(t *testing.T) {
	t.Parallel()

	runner := harness.NewRunner(&staticProvider{}, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     2,
	})
	handler := New(runner)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/runs/some-id/compact", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusMethodNotAllowed {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 405, got %d: %s", res.StatusCode, string(body))
	}
}

// TestCompactEndpointInvalidMode verifies a bad mode value returns 400.
func TestCompactEndpointInvalidMode(t *testing.T) {
	t.Parallel()

	blockCh := make(chan struct{})
	releaseCh := make(chan struct{})

	provider := &gatingServerProvider{
		results: []harness.CompletionResult{{Content: "done"}},
		beforeCall: func(idx int) {
			if idx == 0 {
				close(blockCh)
				<-releaseCh
			}
		},
	}

	runner := harness.NewRunner(provider, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     2,
	})
	handler := New(runner)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	res, err := http.Post(ts.URL+"/v1/runs", "application/json",
		bytes.NewBufferString(`{"prompt":"hello"}`))
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	defer res.Body.Close()

	var created struct {
		RunID string `json:"run_id"`
	}
	json.NewDecoder(res.Body).Decode(&created)

	// Wait for run to be active.
	<-blockCh

	compactRes, err := http.Post(
		ts.URL+"/v1/runs/"+created.RunID+"/compact",
		"application/json",
		bytes.NewBufferString(`{"mode":"badmode"}`),
	)
	if err != nil {
		t.Fatalf("compact request: %v", err)
	}
	defer compactRes.Body.Close()

	if compactRes.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(compactRes.Body)
		t.Fatalf("expected 400, got %d: %s", compactRes.StatusCode, string(body))
	}

	close(releaseCh)
}
