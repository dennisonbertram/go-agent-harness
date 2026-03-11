package deferred

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	tools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/harness/tools/descriptions"
)

// TodoItem is the exported representation of a single todo entry.
// It is used both by the tool handler and by the HTTP API layer.
type TodoItem struct {
	ID     string `json:"id,omitempty"`
	Text   string `json:"text"`
	Status string `json:"status"`
}

// todoItem is the internal alias kept for backward-compat within this package.
type todoItem = TodoItem

// TodoManager provides HTTP-layer access to the per-run todo state.
type TodoManager interface {
	GetTodos(runID string) []TodoItem
	SetTodos(runID string, todos []TodoItem) error
}

type todoStore struct {
	mu    sync.Mutex
	items map[string][]TodoItem
}

func newTodoStore() *todoStore {
	return &todoStore{items: make(map[string][]TodoItem)}
}

// NewTodoStore creates a new todoStore and returns it as a TodoManager.
// Use this when the HTTP server needs a shared handle to the same store
// that is wired into the tool.
func NewTodoStore() (TodoManager, func() tools.Tool) {
	store := newTodoStore()
	return store, func() tools.Tool { return buildTodosTool(store) }
}

// GetTodos returns a snapshot of the todos for the given run ID.
// Returns an empty slice for unknown run IDs.
func (s *todoStore) GetTodos(runID string) []TodoItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.items[runID]
	if items == nil {
		return []TodoItem{}
	}
	out := make([]TodoItem, len(items))
	copy(out, items)
	return out
}

// SetTodos replaces the todo list for the given run ID.
// Returns an error if any todo has an invalid status value.
func (s *todoStore) SetTodos(runID string, todos []TodoItem) error {
	for _, td := range todos {
		st := td.Status
		if st == "" {
			st = "pending"
		}
		if st != "pending" && st != "in_progress" && st != "completed" {
			return fmt.Errorf("invalid todo status %q", td.Status)
		}
	}
	normalized := make([]TodoItem, len(todos))
	for i, td := range todos {
		if td.Status == "" {
			td.Status = "pending"
		}
		normalized[i] = td
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[runID] = normalized
	return nil
}

// TodosTool returns a deferred tool for managing run-scoped todo state.
// It creates its own internal todo store.
// To share the store with the HTTP layer, use NewTodoStore instead.
func TodosTool() tools.Tool {
	return buildTodosTool(newTodoStore())
}

// buildTodosTool constructs the todos tool using the provided store.
func buildTodosTool(store *todoStore) tools.Tool {
	def := tools.Definition{
		Name:         "todos",
		Description:  descriptions.Load("todos"),
		Action:       tools.ActionWrite,
		Mutating:     true,
		ParallelSafe: false,
		Tier:         tools.TierCore,
		Tags:         []string{"planning", "tasks"},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"todos": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"id":     map[string]any{"type": "string"},
							"text":   map[string]any{"type": "string"},
							"status": map[string]any{"type": "string"},
						},
					},
				},
			},
		},
	}

	handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
		args := struct {
			Todos []todoItem `json:"todos"`
		}{}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &args); err != nil {
				return "", fmt.Errorf("parse todos args: %w", err)
			}
		}

		runID := tools.RunIDFromContext(ctx)
		if runID == "" {
			runID = "global"
		}

		store.mu.Lock()
		defer store.mu.Unlock()

		if len(args.Todos) > 0 {
			for _, td := range args.Todos {
				if td.Status == "" {
					td.Status = "pending"
				}
				if td.Status != "pending" && td.Status != "in_progress" && td.Status != "completed" {
					return "", fmt.Errorf("invalid todo status %q", td.Status)
				}
			}
			store.items[runID] = append([]todoItem(nil), args.Todos...)
		}

		result := map[string]any{"run_id": runID, "todos": store.items[runID]}
		return tools.MarshalToolResult(result)
	}

	return tools.Tool{Definition: def, Handler: handler}
}
