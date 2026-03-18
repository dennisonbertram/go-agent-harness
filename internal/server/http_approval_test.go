package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-agent-harness/internal/harness"
)

// TestHandleApprove_NotFound verifies that POST /v1/runs/{id}/approve on an
// unknown run ID returns 404.
func TestHandleApprove_NotFound(t *testing.T) {
	t.Parallel()

	approvalBroker := harness.NewInMemoryApprovalBroker()
	runner := harness.NewRunner(&staticProvider{}, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel:   "test-model",
		ApprovalBroker: approvalBroker,
	})
	handler := NewWithOptions(ServerOptions{Runner: runner, AuthDisabled: true})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	res, err := http.Post(ts.URL+"/v1/runs/no-such-run/approve", "application/json", nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 404, got %d: %s", res.StatusCode, string(body))
	}
}

// TestHandleDeny_NotFound verifies that POST /v1/runs/{id}/deny on an unknown
// run ID returns 404.
func TestHandleDeny_NotFound(t *testing.T) {
	t.Parallel()

	approvalBroker := harness.NewInMemoryApprovalBroker()
	runner := harness.NewRunner(&staticProvider{}, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel:   "test-model",
		ApprovalBroker: approvalBroker,
	})
	handler := NewWithOptions(ServerOptions{Runner: runner, AuthDisabled: true})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	res, err := http.Post(ts.URL+"/v1/runs/no-such-run/deny", "application/json", nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 404, got %d: %s", res.StatusCode, string(body))
	}
}

// TestHandleApprove_MethodNotAllowed verifies GET on /approve returns 405.
func TestHandleApprove_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	approvalBroker := harness.NewInMemoryApprovalBroker()
	runner := harness.NewRunner(&staticProvider{}, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel:   "test-model",
		ApprovalBroker: approvalBroker,
	})
	handler := NewWithOptions(ServerOptions{Runner: runner, AuthDisabled: true})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/runs/some-run/approve", nil)
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

// TestHandleDeny_MethodNotAllowed verifies GET on /deny returns 405.
func TestHandleDeny_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	approvalBroker := harness.NewInMemoryApprovalBroker()
	runner := harness.NewRunner(&staticProvider{}, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel:   "test-model",
		ApprovalBroker: approvalBroker,
	})
	handler := NewWithOptions(ServerOptions{Runner: runner, AuthDisabled: true})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/runs/some-run/deny", nil)
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

// TestHandleApproveAndDeny_IntegrationFlow starts a run with ApprovalPolicyAll,
// waits for it to pause, calls /approve, and verifies the run completes.
func TestHandleApproveAndDeny_IntegrationFlow(t *testing.T) {
	t.Parallel()

	approvalBroker := harness.NewInMemoryApprovalBroker()

	provider := &scriptedProvider{
		turns: []harness.CompletionResult{
			{
				ToolCalls: []harness.ToolCall{{
					ID:        "call_http_approve",
					Name:      "echo_json",
					Arguments: `{"value":"test"}`,
				}},
			},
			{Content: "done after approval"},
		},
	}

	registry := harness.NewRegistry()
	_ = registry.Register(harness.ToolDefinition{
		Name:        "echo_json",
		Description: "echoes",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"value": map[string]any{"type": "string"}},
		},
		ParallelSafe: true,
	}, func(_ context.Context, args json.RawMessage) (string, error) {
		return string(args), nil
	})

	runner := harness.NewRunner(provider, registry, harness.RunnerConfig{
		DefaultModel:   "test-model",
		ApprovalBroker: approvalBroker,
	})

	handler := NewWithOptions(ServerOptions{Runner: runner, AuthDisabled: true})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Start the run.
	runResp, err := http.Post(ts.URL+"/v1/runs", "application/json", bytes.NewBufferString(`{
		"prompt": "run with approval",
		"permissions": {"sandbox": "unrestricted", "approval": "all"}
	}`))
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	defer runResp.Body.Close()
	if runResp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(runResp.Body)
		t.Fatalf("expected 202, got %d: %s", runResp.StatusCode, body)
	}

	var created struct {
		RunID string `json:"run_id"`
	}
	if err := json.NewDecoder(runResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Wait for the run to pause for approval.
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, ok := approvalBroker.Pending(created.RunID); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for pending approval")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// POST /approve.
	approveResp, err := http.Post(ts.URL+"/v1/runs/"+created.RunID+"/approve", "application/json", nil)
	if err != nil {
		t.Fatalf("approve request: %v", err)
	}
	defer approveResp.Body.Close()
	if approveResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(approveResp.Body)
		t.Fatalf("expected 200, got %d: %s", approveResp.StatusCode, body)
	}

	// Verify approved response body.
	var approveBody map[string]any
	json.NewDecoder(approveResp.Body).Decode(&approveBody)
	if approveBody["status"] != "approved" {
		t.Errorf("approve response status = %v, want approved", approveBody["status"])
	}

	// Wait for run to complete.
	deadline = time.Now().Add(5 * time.Second)
	for {
		checkResp, err := http.Get(ts.URL + "/v1/runs/" + created.RunID)
		if err != nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		var runState struct {
			Status string `json:"status"`
		}
		json.NewDecoder(checkResp.Body).Decode(&runState)
		checkResp.Body.Close()
		if runState.Status == "completed" || runState.Status == "failed" {
			if runState.Status != "completed" {
				t.Errorf("run status = %q, want completed", runState.Status)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for run to complete")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestHandleDeny_IntegrationFlow verifies that POST /deny causes the tool call
// to return an error to the LLM and the run continues to completion.
func TestHandleDeny_IntegrationFlow(t *testing.T) {
	t.Parallel()

	approvalBroker := harness.NewInMemoryApprovalBroker()

	provider := &scriptedProvider{
		turns: []harness.CompletionResult{
			{
				ToolCalls: []harness.ToolCall{{
					ID:        "call_http_deny",
					Name:      "echo_json",
					Arguments: `{"value":"test"}`,
				}},
			},
			{Content: "understood, tool was denied"},
		},
	}

	registry := harness.NewRegistry()
	_ = registry.Register(harness.ToolDefinition{
		Name:        "echo_json",
		Description: "echoes",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"value": map[string]any{"type": "string"}},
		},
		ParallelSafe: true,
	}, func(_ context.Context, args json.RawMessage) (string, error) {
		return string(args), nil
	})

	runner := harness.NewRunner(provider, registry, harness.RunnerConfig{
		DefaultModel:   "test-model",
		ApprovalBroker: approvalBroker,
	})

	handler := NewWithOptions(ServerOptions{Runner: runner, AuthDisabled: true})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	runResp, err := http.Post(ts.URL+"/v1/runs", "application/json", bytes.NewBufferString(`{
		"prompt": "run with denial",
		"permissions": {"sandbox": "unrestricted", "approval": "all"}
	}`))
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	defer runResp.Body.Close()
	if runResp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(runResp.Body)
		t.Fatalf("expected 202, got %d: %s", runResp.StatusCode, body)
	}

	var created struct {
		RunID string `json:"run_id"`
	}
	json.NewDecoder(runResp.Body).Decode(&created)

	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, ok := approvalBroker.Pending(created.RunID); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for pending approval")
		}
		time.Sleep(10 * time.Millisecond)
	}

	denyResp, err := http.Post(ts.URL+"/v1/runs/"+created.RunID+"/deny", "application/json", nil)
	if err != nil {
		t.Fatalf("deny request: %v", err)
	}
	defer denyResp.Body.Close()
	if denyResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(denyResp.Body)
		t.Fatalf("expected 200, got %d: %s", denyResp.StatusCode, body)
	}

	var denyBody map[string]any
	json.NewDecoder(denyResp.Body).Decode(&denyBody)
	if denyBody["status"] != "denied" {
		t.Errorf("deny response status = %v, want denied", denyBody["status"])
	}

	deadline = time.Now().Add(5 * time.Second)
	for {
		checkResp, err := http.Get(ts.URL + "/v1/runs/" + created.RunID)
		if err != nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		var runState struct {
			Status string `json:"status"`
		}
		json.NewDecoder(checkResp.Body).Decode(&runState)
		checkResp.Body.Close()
		if runState.Status == "completed" || runState.Status == "failed" {
			if runState.Status != "completed" {
				t.Errorf("run status = %q after denial, want completed", runState.Status)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for run to complete after denial")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
