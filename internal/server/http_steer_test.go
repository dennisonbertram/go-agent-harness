package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"go-agent-harness/internal/harness"
)

// TestHandleSteer_Success verifies the steer endpoint accepts a message on an
// active run and returns 202 Accepted.
func TestHandleSteer_Success(t *testing.T) {
	t.Parallel()

	blockCh := make(chan struct{})
	releaseCh := make(chan struct{})

	// Provider that blocks on first call so the run stays active
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

	// Start a run
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

	// Wait for the run to be in-flight
	<-blockCh

	// Send a steer request
	steerBody := `{"prompt":"redirect to the right path"}`
	steerRes, err := http.Post(
		ts.URL+"/v1/runs/"+created.RunID+"/steer",
		"application/json",
		bytes.NewBufferString(steerBody),
	)
	if err != nil {
		t.Fatalf("steer request: %v", err)
	}
	defer steerRes.Body.Close()

	if steerRes.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(steerRes.Body)
		t.Fatalf("expected 202, got %d: %s", steerRes.StatusCode, string(body))
	}

	// Release the provider
	close(releaseCh)
}

// TestHandleSteer_RunNotFound verifies 404 for unknown run IDs.
func TestHandleSteer_RunNotFound(t *testing.T) {
	t.Parallel()

	runner := harness.NewRunner(&staticProvider{}, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     2,
	})
	handler := New(runner)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	res, err := http.Post(
		ts.URL+"/v1/runs/nonexistent-id/steer",
		"application/json",
		bytes.NewBufferString(`{"prompt":"hello"}`),
	)
	if err != nil {
		t.Fatalf("steer request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 404, got %d: %s", res.StatusCode, string(body))
	}
}

// TestHandleSteer_EmptyMessage verifies 400 for empty message.
func TestHandleSteer_EmptyMessage(t *testing.T) {
	t.Parallel()

	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
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

	// Wait a bit so run has started
	time.Sleep(30 * time.Millisecond)

	steerRes, err := http.Post(
		ts.URL+"/v1/runs/"+created.RunID+"/steer",
		"application/json",
		bytes.NewBufferString(`{"prompt":""}`),
	)
	if err != nil {
		t.Fatalf("steer request: %v", err)
	}
	defer steerRes.Body.Close()

	if steerRes.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(steerRes.Body)
		t.Fatalf("expected 400, got %d: %s", steerRes.StatusCode, string(body))
	}
}

// TestHandleSteer_MethodNotAllowed verifies only POST is accepted.
func TestHandleSteer_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	runner := harness.NewRunner(&staticProvider{}, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     2,
	})
	handler := New(runner)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/runs/some-id/steer", nil)
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

// TestHandleSteer_CompletedRun verifies 409 when steering a finished run.
func TestHandleSteer_CompletedRun(t *testing.T) {
	t.Parallel()

	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
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

	// Wait for completion
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

	steerRes, err := http.Post(
		ts.URL+"/v1/runs/"+created.RunID+"/steer",
		"application/json",
		bytes.NewBufferString(`{"prompt":"too late"}`),
	)
	if err != nil {
		t.Fatalf("steer request: %v", err)
	}
	defer steerRes.Body.Close()

	if steerRes.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(steerRes.Body)
		t.Fatalf("expected 409, got %d: %s", steerRes.StatusCode, string(body))
	}
}

// gatingServerProvider is a scripted provider with a beforeCall hook.
type gatingServerProvider struct {
	mu         sync.Mutex
	results    []harness.CompletionResult
	calls      int
	beforeCall func(idx int)
}

func (p *gatingServerProvider) Complete(ctx context.Context, req harness.CompletionRequest) (harness.CompletionResult, error) {
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
