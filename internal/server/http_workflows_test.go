package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go-agent-harness/internal/checkpoints"
	"go-agent-harness/internal/workflows"
)

func TestWorkflowSSETerminalHistoryReturnsWithoutLiveChannelClosure(t *testing.T) {
	t.Parallel()

	for _, terminalType := range []string{"workflow.completed", "workflow.failed"} {
		terminalType := terminalType
		t.Run(terminalType, func(t *testing.T) {
			t.Parallel()
			live := make(chan workflows.Event)
			manager := &workflowSSEManager{history: []workflows.Event{{
				Seq:     7,
				Type:    terminalType,
				Payload: map[string]any{"result": terminalType},
			}}, live: live}
			handler := NewWithOptions(ServerOptions{AuthDisabled: true, Workflows: manager})
			recorder := httptest.NewRecorder()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			req := httptest.NewRequest(http.MethodGet, "/v1/workflow-runs/run-terminal/events", nil).WithContext(ctx)
			done := make(chan struct{})
			go func() {
				handler.ServeHTTP(recorder, req)
				close(done)
			}()

			select {
			case <-done:
			case <-time.After(250 * time.Millisecond):
				cancel() // Cleanup only: a terminal replay must not need cancellation to return.
				<-done
				t.Fatalf("terminal history %q did not return while live channel remained open", terminalType)
			}
			if got := recorder.Body.String(); strings.Count(got, "event: "+terminalType+"\n") != 1 {
				t.Fatalf("terminal SSE frames = %q, want exactly one %q frame", got, terminalType)
			}
			if manager.cancels != 1 {
				t.Fatalf("subscription cancellations = %d, want 1", manager.cancels)
			}
		})
	}
}

func TestWorkflowSSENonterminalHistoryContinuesToLiveTerminal(t *testing.T) {
	t.Parallel()

	live := make(chan workflows.Event, 2)
	manager := &workflowSSEManager{history: []workflows.Event{{
		Seq:     1,
		Type:    "workflow.started",
		Payload: map[string]any{"workflow": "deploy"},
	}}, live: live}
	handler := NewWithOptions(ServerOptions{AuthDisabled: true, Workflows: manager})
	recorder := httptest.NewRecorder()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/v1/workflow-runs/run-live/events", nil).WithContext(ctx)
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(recorder, req)
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("nonterminal history returned before a live event")
	case <-time.After(50 * time.Millisecond):
	}
	live <- workflows.Event{Seq: 2, Type: "workflow.step.completed", Payload: map[string]any{"step": "build"}}
	live <- workflows.Event{Seq: 3, Type: "workflow.completed", Payload: map[string]any{"workflow": "deploy"}}
	select {
	case <-done:
	case <-time.After(time.Second):
		cancel()
		<-done
		t.Fatal("live terminal event did not return")
	}

	got := recorder.Body.String()
	for _, eventType := range []string{"workflow.started", "workflow.step.completed", "workflow.completed"} {
		if strings.Count(got, "event: "+eventType+"\n") != 1 {
			t.Fatalf("SSE frames = %q, want exactly one %q frame", got, eventType)
		}
	}
	if manager.cancels != 1 {
		t.Fatalf("subscription cancellations = %d, want 1", manager.cancels)
	}
}

type workflowSSEManager struct {
	history []workflows.Event
	live    <-chan workflows.Event
	cancels int
}

func (m *workflowSSEManager) ListDefinitions() []workflows.Definition { return nil }

func (m *workflowSSEManager) GetDefinition(string) (workflows.Definition, bool) {
	return workflows.Definition{}, false
}

func (m *workflowSSEManager) Start(string, map[string]any) (workflows.Run, error) {
	return workflows.Run{}, nil
}

func (m *workflowSSEManager) GetRun(string) (workflows.Run, []workflows.StepState, error) {
	return workflows.Run{}, nil, nil
}

func (m *workflowSSEManager) Subscribe(string) ([]workflows.Event, <-chan workflows.Event, func(), error) {
	return m.history, m.live, func() { m.cancels++ }, nil
}

func (m *workflowSSEManager) ResumeRun(context.Context, string, map[string]any) error { return nil }

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
