package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-agent-harness/internal/checkpoints"
	"go-agent-harness/internal/workflows"
)

func TestHandleWorkflowRoutes(t *testing.T) {
	t.Parallel()

	engine := workflows.NewEngine(workflows.Options{
		Definitions: []workflows.Definition{{
			Name:        "tool-flow",
			Description: "tool flow",
			Steps: []workflows.StepDefinition{{
				ID:   "only",
				Type: workflows.StepTypeTool,
				Tool: "echo",
			}},
		}},
		Tools: workflowsToolExecutor(func(ctx context.Context, name string, args json.RawMessage) (string, error) {
			return `{"ok":true}`, nil
		}),
		Checkpoints: checkpoints.NewService(checkpoints.NewMemoryStore(), time.Now),
		Store:       workflows.NewMemoryStore(),
		Now:         time.Now,
	})

	handler := NewWithOptions(ServerOptions{
		AuthDisabled: true,
		Workflows:    engine,
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	listResp, err := http.Get(ts.URL + "/v1/workflows")
	if err != nil {
		t.Fatalf("GET /v1/workflows: %v", err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d, want %d", listResp.StatusCode, http.StatusOK)
	}

	startResp, err := http.Post(ts.URL+"/v1/workflows/tool-flow/runs", "application/json", bytes.NewBufferString(`{"input":{"ticket":"123"}}`))
	if err != nil {
		t.Fatalf("POST /v1/workflows/tool-flow/runs: %v", err)
	}
	defer startResp.Body.Close()
	if startResp.StatusCode != http.StatusAccepted {
		t.Fatalf("start status = %d, want %d", startResp.StatusCode, http.StatusAccepted)
	}

	var started struct {
		RunID string `json:"run_id"`
	}
	if err := json.NewDecoder(startResp.Body).Decode(&started); err != nil {
		t.Fatalf("decode started: %v", err)
	}
	if started.RunID == "" {
		t.Fatal("expected workflow run id")
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		runResp, err := http.Get(ts.URL + "/v1/workflow-runs/" + started.RunID)
		if err != nil {
			t.Fatalf("GET workflow run: %v", err)
		}
		var run struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(runResp.Body).Decode(&run); err != nil {
			runResp.Body.Close()
			t.Fatalf("decode workflow run: %v", err)
		}
		runResp.Body.Close()
		if run.Status == string(workflows.RunStatusCompleted) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for workflow run")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

type workflowsToolExecutor func(ctx context.Context, name string, args json.RawMessage) (string, error)

func (f workflowsToolExecutor) Execute(ctx context.Context, name string, args json.RawMessage) (string, error) {
	return f(ctx, name, args)
}
