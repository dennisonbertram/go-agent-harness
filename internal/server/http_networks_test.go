package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-agent-harness/internal/checkpoints"
	"go-agent-harness/internal/harness"
	"go-agent-harness/internal/networks"
	"go-agent-harness/internal/workflows"
)

func TestHandleNetworkRoutes(t *testing.T) {
	t.Parallel()

	workflowEngine := workflows.NewEngine(workflows.Options{
		Runner:      &networkRouteRunner{},
		Checkpoints: checkpoints.NewService(checkpoints.NewMemoryStore(), time.Now),
		Store:       workflows.NewMemoryStore(),
		Now:         time.Now,
	})
	engine := networks.NewEngine(networks.Options{
		Definitions: []networks.Definition{{
			Name:        "planner-reviewer",
			Description: "planner then reviewer",
			Roles: []networks.RoleDefinition{
				{ID: "planner", Prompt: "Plan", Model: "gpt-test"},
				{ID: "reviewer", Prompt: "Review", Model: "gpt-test"},
			},
		}},
		Workflows: workflowEngine,
	})

	handler := NewWithOptions(ServerOptions{
		AuthDisabled: true,
		Networks:     engine,
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	listResp, err := http.Get(ts.URL + "/v1/networks")
	if err != nil {
		t.Fatalf("GET /v1/networks: %v", err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d, want %d", listResp.StatusCode, http.StatusOK)
	}

	startResp, err := http.Post(ts.URL+"/v1/networks/planner-reviewer/runs", "application/json", bytes.NewBufferString(`{"input":{"ticket":"123"}}`))
	if err != nil {
		t.Fatalf("POST /v1/networks/planner-reviewer/runs: %v", err)
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
		t.Fatal("expected network run id")
	}
}

type networkRouteRunner struct{}

func (r *networkRouteRunner) StartRun(req harness.RunRequest) (harness.Run, error) {
	return harness.Run{ID: "route-" + req.Prompt, Status: harness.RunStatusCompleted, Output: `{"summary":"ok","status":"completed"}`}, nil
}

func (r *networkRouteRunner) GetRun(runID string) (harness.Run, bool) {
	return harness.Run{ID: runID, Status: harness.RunStatusCompleted, Output: `{"summary":"ok","status":"completed"}`}, true
}

func (r *networkRouteRunner) Subscribe(runID string) ([]harness.Event, <-chan harness.Event, func(), error) {
	ch := make(chan harness.Event, 1)
	ch <- harness.Event{RunID: runID, Type: harness.EventRunCompleted, Timestamp: time.Now().UTC()}
	close(ch)
	return nil, ch, func() {}, nil
}
