package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-agent-harness/internal/harness"
	"go-agent-harness/internal/subagents"
)

type mockSubagentManager struct {
	createFn func(context.Context, subagents.Request) (subagents.Subagent, error)
	getFn    func(context.Context, string) (subagents.Subagent, error)
	listFn   func(context.Context) ([]subagents.Subagent, error)
	deleteFn func(context.Context, string) error
}

func (m *mockSubagentManager) Create(ctx context.Context, req subagents.Request) (subagents.Subagent, error) {
	return m.createFn(ctx, req)
}

func (m *mockSubagentManager) Get(ctx context.Context, id string) (subagents.Subagent, error) {
	return m.getFn(ctx, id)
}

func (m *mockSubagentManager) List(ctx context.Context) ([]subagents.Subagent, error) {
	return m.listFn(ctx)
}

func (m *mockSubagentManager) Delete(ctx context.Context, id string) error {
	return m.deleteFn(ctx, id)
}

func TestSubagentsEndpoint_Create(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	mgr := &mockSubagentManager{
		createFn: func(_ context.Context, req subagents.Request) (subagents.Subagent, error) {
			if req.Isolation != subagents.IsolationWorktree {
				t.Fatalf("Isolation = %q, want %q", req.Isolation, subagents.IsolationWorktree)
			}
			return subagents.Subagent{
				ID:            "subagent-1",
				RunID:         "run-1",
				Status:        harness.RunStatusRunning,
				Isolation:     subagents.IsolationWorktree,
				CleanupPolicy: subagents.CleanupPreserve,
				WorkspacePath: "/tmp/worktree",
				BranchName:    "workspace-subagent-1",
				BaseRef:       "HEAD",
				CreatedAt:     now,
				UpdatedAt:     now,
			}, nil
		},
		getFn: func(context.Context, string) (subagents.Subagent, error) {
			return subagents.Subagent{}, subagents.ErrNotFound
		},
		listFn:   func(context.Context) ([]subagents.Subagent, error) { return nil, nil },
		deleteFn: func(context.Context, string) error { return nil },
	}

	handler := NewWithOptions(ServerOptions{Runner: testRunnerForAgents(t), SubagentManager: mgr})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	body := map[string]any{
		"prompt":         "fix this",
		"isolation":      "worktree",
		"cleanup_policy": "preserve",
	}
	raw, _ := json.Marshal(body)
	res, err := http.Post(ts.URL+"/v1/subagents", "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("POST /v1/subagents: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 201, got %d: %s", res.StatusCode, string(b))
	}
	var item subagents.Subagent
	if err := json.NewDecoder(res.Body).Decode(&item); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if item.ID != "subagent-1" {
		t.Fatalf("ID = %q, want subagent-1", item.ID)
	}
}

func TestSubagentsEndpoint_ListAndGet(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	item := subagents.Subagent{
		ID:            "subagent-1",
		RunID:         "run-1",
		Status:        harness.RunStatusCompleted,
		Isolation:     subagents.IsolationInline,
		CleanupPolicy: subagents.CleanupPreserve,
		Output:        "done",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	mgr := &mockSubagentManager{
		createFn: func(context.Context, subagents.Request) (subagents.Subagent, error) { return item, nil },
		getFn: func(_ context.Context, id string) (subagents.Subagent, error) {
			if id != item.ID {
				return subagents.Subagent{}, subagents.ErrNotFound
			}
			return item, nil
		},
		listFn:   func(context.Context) ([]subagents.Subagent, error) { return []subagents.Subagent{item}, nil },
		deleteFn: func(context.Context, string) error { return nil },
	}

	handler := NewWithOptions(ServerOptions{Runner: testRunnerForAgents(t), SubagentManager: mgr})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	listRes, err := http.Get(ts.URL + "/v1/subagents")
	if err != nil {
		t.Fatalf("GET /v1/subagents: %v", err)
	}
	defer listRes.Body.Close()
	if listRes.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", listRes.StatusCode)
	}

	getRes, err := http.Get(ts.URL + "/v1/subagents/subagent-1")
	if err != nil {
		t.Fatalf("GET /v1/subagents/subagent-1: %v", err)
	}
	defer getRes.Body.Close()
	if getRes.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", getRes.StatusCode)
	}
}

func TestSubagentsEndpoint_DeleteActiveReturns409(t *testing.T) {
	t.Parallel()

	mgr := &mockSubagentManager{
		createFn: func(context.Context, subagents.Request) (subagents.Subagent, error) { return subagents.Subagent{}, nil },
		getFn:    func(context.Context, string) (subagents.Subagent, error) { return subagents.Subagent{}, nil },
		listFn:   func(context.Context) ([]subagents.Subagent, error) { return nil, nil },
		deleteFn: func(context.Context, string) error { return subagents.ErrActive },
	}

	handler := NewWithOptions(ServerOptions{Runner: testRunnerForAgents(t), SubagentManager: mgr})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/v1/subagents/subagent-1", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /v1/subagents/subagent-1: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", res.StatusCode)
	}
}

func TestSubagentsEndpoint_NotConfigured(t *testing.T) {
	t.Parallel()

	handler := NewWithOptions(ServerOptions{Runner: testRunnerForAgents(t)})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/v1/subagents")
	if err != nil {
		t.Fatalf("GET /v1/subagents: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", res.StatusCode)
	}
}

func TestSubagentsEndpoint_GetNotFound(t *testing.T) {
	t.Parallel()

	mgr := &mockSubagentManager{
		createFn: func(context.Context, subagents.Request) (subagents.Subagent, error) { return subagents.Subagent{}, nil },
		getFn: func(context.Context, string) (subagents.Subagent, error) {
			return subagents.Subagent{}, subagents.ErrNotFound
		},
		listFn:   func(context.Context) ([]subagents.Subagent, error) { return nil, nil },
		deleteFn: func(context.Context, string) error { return errors.New("unexpected") },
	}

	handler := NewWithOptions(ServerOptions{Runner: testRunnerForAgents(t), SubagentManager: mgr})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/v1/subagents/missing")
	if err != nil {
		t.Fatalf("GET /v1/subagents/missing: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", res.StatusCode)
	}
}
