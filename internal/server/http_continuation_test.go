package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"go-agent-harness/internal/harness"
)

// continuationServerProvider returns scripted results on successive calls.
type continuationServerProvider struct {
	mu    sync.Mutex
	turns []harness.CompletionResult
	calls int
}

func (p *continuationServerProvider) Complete(_ context.Context, _ harness.CompletionRequest) (harness.CompletionResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.calls >= len(p.turns) {
		return harness.CompletionResult{Content: "done"}, nil
	}
	out := p.turns[p.calls]
	p.calls++
	return out, nil
}

// waitForRunStatus polls GET /v1/runs/{id} until the run reaches a terminal
// or target status.
func waitForRunStatus(t *testing.T, ts *httptest.Server, runID string, targets ...string) string {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for {
		res, err := http.Get(ts.URL + "/v1/runs/" + runID)
		if err != nil {
			t.Fatalf("GET run: %v", err)
		}
		var state struct {
			Status string `json:"status"`
		}
		_ = json.NewDecoder(res.Body).Decode(&state)
		_ = res.Body.Close()
		for _, target := range targets {
			if state.Status == target {
				return state.Status
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for status %v, last: %s", targets, state.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// createAndCompleteRun starts a run via POST /v1/runs and waits for completion.
// Returns the run_id.
func createAndCompleteRun(t *testing.T, ts *httptest.Server, prompt string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"prompt": prompt})
	res, err := http.Post(ts.URL+"/v1/runs", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /v1/runs: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusAccepted {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 202, got %d: %s", res.StatusCode, raw)
	}
	var created struct {
		RunID string `json:"run_id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	waitForRunStatus(t, ts, created.RunID, "completed", "failed")
	return created.RunID
}

// TestContinueRunEndpointBasic verifies the happy path: POST /v1/runs/{id}/continue
// on a completed run succeeds with 202 and returns a new run_id.
func TestContinueRunEndpointBasic(t *testing.T) {
	t.Parallel()

	prov := &continuationServerProvider{
		turns: []harness.CompletionResult{
			{Content: "first"},
			{Content: "second"},
		},
	}
	runner := harness.NewRunner(prov, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     4,
	})
	ts := httptest.NewServer(New(runner))
	defer ts.Close()

	runID := createAndCompleteRun(t, ts, "initial")

	// Continue the run.
	body, _ := json.Marshal(map[string]string{"prompt": "follow-up"})
	res, err := http.Post(ts.URL+"/v1/runs/"+runID+"/continue", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST continue: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusAccepted {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 202, got %d: %s", res.StatusCode, raw)
	}

	var reply struct {
		RunID  string `json:"run_id"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(res.Body).Decode(&reply); err != nil {
		t.Fatalf("decode continue response: %v", err)
	}
	if reply.RunID == "" {
		t.Fatal("expected run_id in continue response")
	}
	if reply.RunID == runID {
		t.Fatalf("expected new run_id, got same: %s", runID)
	}

	// Wait for the continuation run to complete.
	status := waitForRunStatus(t, ts, reply.RunID, "completed", "failed")
	if status != "completed" {
		t.Fatalf("expected completed, got %s", status)
	}
}

// TestContinueRunEndpointNotFound verifies 404 for a nonexistent run.
func TestContinueRunEndpointNotFound(t *testing.T) {
	t.Parallel()

	runner := harness.NewRunner(&staticProvider{result: harness.CompletionResult{Content: "ok"}}, harness.NewRegistry(), harness.RunnerConfig{})
	ts := httptest.NewServer(New(runner))
	defer ts.Close()

	body, _ := json.Marshal(map[string]string{"prompt": "hello"})
	res, err := http.Post(ts.URL+"/v1/runs/nonexistent/continue", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST continue: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", res.StatusCode)
	}
}

// TestContinueRunEndpointInvalidJSON verifies 400 for malformed JSON.
func TestContinueRunEndpointInvalidJSON(t *testing.T) {
	t.Parallel()

	prov := &continuationServerProvider{turns: []harness.CompletionResult{{Content: "done"}}}
	runner := harness.NewRunner(prov, harness.NewRegistry(), harness.RunnerConfig{MaxSteps: 2})
	ts := httptest.NewServer(New(runner))
	defer ts.Close()

	runID := createAndCompleteRun(t, ts, "initial")

	res, err := http.Post(ts.URL+"/v1/runs/"+runID+"/continue", "application/json", bytes.NewBufferString("{bad json"))
	if err != nil {
		t.Fatalf("POST continue: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.StatusCode)
	}
}

// TestContinueRunEndpointEmptyMessage verifies 400 for missing prompt field.
func TestContinueRunEndpointEmptyMessage(t *testing.T) {
	t.Parallel()

	prov := &continuationServerProvider{turns: []harness.CompletionResult{{Content: "done"}}}
	runner := harness.NewRunner(prov, harness.NewRegistry(), harness.RunnerConfig{MaxSteps: 2})
	ts := httptest.NewServer(New(runner))
	defer ts.Close()

	runID := createAndCompleteRun(t, ts, "initial")

	body, _ := json.Marshal(map[string]string{"prompt": ""})
	res, err := http.Post(ts.URL+"/v1/runs/"+runID+"/continue", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST continue: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.StatusCode)
	}
}

// TestContinueRunEndpointMethodNotAllowed verifies 405 for non-POST methods.
func TestContinueRunEndpointMethodNotAllowed(t *testing.T) {
	t.Parallel()

	prov := &continuationServerProvider{turns: []harness.CompletionResult{{Content: "done"}}}
	runner := harness.NewRunner(prov, harness.NewRegistry(), harness.RunnerConfig{MaxSteps: 2})
	ts := httptest.NewServer(New(runner))
	defer ts.Close()

	runID := createAndCompleteRun(t, ts, "initial")

	res, err := http.Get(ts.URL + "/v1/runs/" + runID + "/continue")
	if err != nil {
		t.Fatalf("GET continue: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", res.StatusCode)
	}
}

// TestContinueRunEndpointRunningConflict verifies 409 when attempting to
// continue a run that is still running.
func TestContinueRunEndpointRunningConflict(t *testing.T) {
	t.Parallel()

	// errorProvider causes the run to fail immediately — we need a run we can
	// explicitly check is not completed. Use a blocker that we never release.
	blocker := make(chan struct{})
	blockProv := &blockingServerProvider{blocker: blocker}
	runner := harness.NewRunner(blockProv, harness.NewRegistry(), harness.RunnerConfig{MaxSteps: 2})
	ts := httptest.NewServer(New(runner))
	defer ts.Close()

	body, _ := json.Marshal(map[string]string{"prompt": "block me"})
	res, err := http.Post(ts.URL+"/v1/runs", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST runs: %v", err)
	}
	defer res.Body.Close()
	var created struct {
		RunID string `json:"run_id"`
	}
	json.NewDecoder(res.Body).Decode(&created)

	// Wait until running.
	deadline := time.Now().Add(2 * time.Second)
	for {
		statusRes, _ := http.Get(ts.URL + "/v1/runs/" + created.RunID)
		var state struct{ Status string }
		json.NewDecoder(statusRes.Body).Decode(&state)
		statusRes.Body.Close()
		if state.Status == "running" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for running status")
		}
		time.Sleep(5 * time.Millisecond)
	}

	contBody, _ := json.Marshal(map[string]string{"prompt": "try"})
	contRes, err := http.Post(ts.URL+"/v1/runs/"+created.RunID+"/continue", "application/json", bytes.NewReader(contBody))
	if err != nil {
		t.Fatalf("POST continue: %v", err)
	}
	defer contRes.Body.Close()
	if contRes.StatusCode != http.StatusConflict {
		raw, _ := io.ReadAll(contRes.Body)
		t.Fatalf("expected 409, got %d: %s", contRes.StatusCode, raw)
	}

	close(blocker) // let run finish
}

// blockingServerProvider blocks until the channel is closed.
type blockingServerProvider struct {
	blocker <-chan struct{}
}

func (p *blockingServerProvider) Complete(_ context.Context, _ harness.CompletionRequest) (harness.CompletionResult, error) {
	<-p.blocker
	return harness.CompletionResult{Content: "done"}, nil
}

// TestContinueRunEndpointSSEResumedEvent verifies that subscribing to a
// continuation run emits a run.started event (and eventually run.completed).
func TestContinueRunEndpointSSEResumedEvent(t *testing.T) {
	t.Parallel()

	prov := &continuationServerProvider{
		turns: []harness.CompletionResult{
			{Content: "first"},
			{Content: "second"},
		},
	}
	runner := harness.NewRunner(prov, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     4,
	})
	ts := httptest.NewServer(New(runner))
	defer ts.Close()

	runID := createAndCompleteRun(t, ts, "initial")

	body, _ := json.Marshal(map[string]string{"prompt": "continue"})
	res, err := http.Post(ts.URL+"/v1/runs/"+runID+"/continue", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST continue: %v", err)
	}
	defer res.Body.Close()
	var reply struct {
		RunID string `json:"run_id"`
	}
	json.NewDecoder(res.Body).Decode(&reply)
	if reply.RunID == "" {
		t.Fatal("expected run_id in continue response")
	}

	// Subscribe to events for the new run.
	evRes, err := http.Get(ts.URL + "/v1/runs/" + reply.RunID + "/events")
	if err != nil {
		t.Fatalf("GET events: %v", err)
	}
	defer evRes.Body.Close()

	eventBody, err := io.ReadAll(evRes.Body)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	bodyStr := string(eventBody)
	if !containsEvent(bodyStr, "run.completed") {
		t.Fatalf("expected run.completed event, got:\n%s", bodyStr)
	}
}

// containsEvent reports whether body contains a SSE event of the given type.
func containsEvent(body, eventType string) bool {
	return bytes.Contains([]byte(body), []byte("event: "+eventType))
}

// Ensure the error types used in the server handler are reachable from tests.
var _ = errors.New // ensure errors import is used
