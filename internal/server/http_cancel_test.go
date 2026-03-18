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

// cancellingServerProvider blocks on first call, respects context cancellation.
type cancellingServerProvider struct {
	mu        sync.Mutex
	calls     int
	blockCh   chan struct{}
	releaseCh chan struct{}
}

func newCancellingServerProvider() *cancellingServerProvider {
	return &cancellingServerProvider{
		blockCh:   make(chan struct{}),
		releaseCh: make(chan struct{}),
	}
}

func (p *cancellingServerProvider) Complete(ctx context.Context, _ harness.CompletionRequest) (harness.CompletionResult, error) {
	p.mu.Lock()
	idx := p.calls
	p.calls++
	p.mu.Unlock()

	if idx == 0 {
		// Signal we are inside the first call
		select {
		case <-p.blockCh:
		default:
			close(p.blockCh)
		}
		select {
		case <-p.releaseCh:
			return harness.CompletionResult{Content: "done"}, nil
		case <-ctx.Done():
			return harness.CompletionResult{}, ctx.Err()
		}
	}
	return harness.CompletionResult{Content: "done"}, nil
}

// TestHandleCancel_Success verifies POST /v1/runs/{id}/cancel on an active run returns 200.
func TestHandleCancel_Success(t *testing.T) {
	t.Parallel()

	prov := newCancellingServerProvider()
	runner := harness.NewRunner(prov, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     5,
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

	// Wait for the provider to be blocking
	select {
	case <-prov.blockCh:
	case <-time.After(3 * time.Second):
		t.Fatal("provider never started blocking")
	}

	// POST /cancel
	cancelRes, err := http.Post(
		ts.URL+"/v1/runs/"+created.RunID+"/cancel",
		"application/json",
		nil,
	)
	if err != nil {
		t.Fatalf("cancel request: %v", err)
	}
	defer cancelRes.Body.Close()

	if cancelRes.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(cancelRes.Body)
		t.Fatalf("expected 200, got %d: %s", cancelRes.StatusCode, string(body))
	}

	// Verify the run eventually reaches cancelled status
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		checkRes, err := http.Get(ts.URL + "/v1/runs/" + created.RunID)
		if err != nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		var runState struct {
			Status string `json:"status"`
		}
		json.NewDecoder(checkRes.Body).Decode(&runState)
		checkRes.Body.Close()
		if runState.Status == "cancelled" {
			return // success
		}
		time.Sleep(10 * time.Millisecond)
	}
	// Last check
	checkRes, _ := http.Get(ts.URL + "/v1/runs/" + created.RunID)
	if checkRes != nil {
		var runState struct {
			Status string `json:"status"`
		}
		json.NewDecoder(checkRes.Body).Decode(&runState)
		checkRes.Body.Close()
		if runState.Status != "cancelled" {
			t.Errorf("expected status 'cancelled', got %q", runState.Status)
		}
	}
}

// TestHandleCancel_NotFound verifies POST /v1/runs/nonexistent/cancel returns 404.
func TestHandleCancel_NotFound(t *testing.T) {
	t.Parallel()

	runner := harness.NewRunner(&staticProvider{}, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     2,
	})
	handler := New(runner)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	res, err := http.Post(
		ts.URL+"/v1/runs/nonexistent-run-id/cancel",
		"application/json",
		nil,
	)
	if err != nil {
		t.Fatalf("cancel request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 404, got %d: %s", res.StatusCode, string(body))
	}
}

// TestHandleCancel_TerminalRun verifies POST /cancel on an already-completed run
// is idempotent (returns 200, not an error).
func TestHandleCancel_TerminalRun(t *testing.T) {
	t.Parallel()

	provider := &staticProvider{result: harness.CompletionResult{Content: "done"}}
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
	json.NewDecoder(res.Body).Decode(&created)

	// Wait for run to complete
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

	// Cancel a terminal run — should be idempotent (200)
	cancelRes, err := http.Post(
		ts.URL+"/v1/runs/"+created.RunID+"/cancel",
		"application/json",
		nil,
	)
	if err != nil {
		t.Fatalf("cancel request: %v", err)
	}
	defer cancelRes.Body.Close()

	if cancelRes.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(cancelRes.Body)
		t.Fatalf("expected 200 (idempotent), got %d: %s", cancelRes.StatusCode, string(body))
	}
}

// TestHandleCancel_MethodNotAllowed verifies only POST is accepted.
func TestHandleCancel_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	runner := harness.NewRunner(&staticProvider{}, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     2,
	})
	handler := New(runner)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/runs/some-id/cancel", nil)
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
