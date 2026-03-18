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

// validTodoStatus returns true if status is one of the accepted values.
func validTodoStatus(s string) bool {
	return s == "pending" || s == "in_progress" || s == "completed"
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
				"action": map[string]any{
					"type":        "string",
					"description": "Operation: 'set' (default, full replacement), 'update' (single item by id), 'delete' (remove item by id), 'get' (read list)",
					"enum":        []string{"set", "update", "delete", "get"},
				},
				"todos": map[string]any{
					"type":        "array",
					"description": "Full list of todo items (used with action=set or omitted)",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"id":     map[string]any{"type": "string"},
							"text":   map[string]any{"type": "string"},
							"status": map[string]any{"type": "string"},
						},
					},
				},
				"id": map[string]any{
					"type":        "string",
					"description": "Item ID to target (required for action=update and action=delete)",
				},
				"status": map[string]any{
					"type":        "string",
					"description": "New status for the item (used with action=update): 'pending', 'in_progress', or 'completed'",
				},
				"text": map[string]any{
					"type":        "string",
					"description": "New text/description for the item (used with action=update)",
				},
			},
		},
	}

	handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
		args := struct {
			Action string     `json:"action"`
			Todos  []todoItem `json:"todos"`
			ID     string     `json:"id"`
			Status string     `json:"status"`
			Text   string     `json:"text"`
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

		// Default action is "set" when todos array is provided, "get" otherwise.
		action := args.Action
		if action == "" {
			if len(args.Todos) > 0 {
				action = "set"
			} else {
				action = "get"
			}
		}

		switch action {
		case "set":
			for _, td := range args.Todos {
				st := td.Status
				if st == "" {
					st = "pending"
				}
				if !validTodoStatus(st) {
					return "", fmt.Errorf("invalid todo status %q", td.Status)
				}
			}
			normalized := make([]todoItem, len(args.Todos))
			for i, td := range args.Todos {
				if td.Status == "" {
					td.Status = "pending"
				}
				normalized[i] = td
			}
			store.items[runID] = normalized

		case "update":
			if args.ID == "" {
				return "", fmt.Errorf("todos update: 'id' is required")
			}
			if args.Status != "" && !validTodoStatus(args.Status) {
				return "", fmt.Errorf("todos update: invalid status %q", args.Status)
			}
			found := false
			for i := range store.items[runID] {
				if store.items[runID][i].ID == args.ID {
					if args.Status != "" {
						store.items[runID][i].Status = args.Status
					}
					if args.Text != "" {
						store.items[runID][i].Text = args.Text
					}
					found = true
					break
				}
			}
			if !found {
				return tools.MarshalToolResult(map[string]any{
					"error":  fmt.Sprintf("todo item with id %q not found", args.ID),
					"run_id": runID,
					"todos":  store.items[runID],
				})
			}

		case "delete":
			if args.ID == "" {
				return "", fmt.Errorf("todos delete: 'id' is required")
			}
			before := len(store.items[runID])
			filtered := store.items[runID][:0:0]
			for _, td := range store.items[runID] {
				if td.ID != args.ID {
					filtered = append(filtered, td)
				}
			}
			if len(filtered) == before {
				return tools.MarshalToolResult(map[string]any{
					"error":  fmt.Sprintf("todo item with id %q not found", args.ID),
					"run_id": runID,
					"todos":  store.items[runID],
				})
			}
			store.items[runID] = filtered

		case "get":
			// No mutation; just fall through to the result.

		default:
			return "", fmt.Errorf("todos: unknown action %q (must be 'set', 'update', 'delete', or 'get')", action)
		}

		result := map[string]any{"run_id": runID, "todos": store.items[runID]}
		return tools.MarshalToolResult(result)
	}

	return tools.Tool{Definition: def, Handler: handler}
}
