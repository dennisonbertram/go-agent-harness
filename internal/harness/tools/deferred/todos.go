package deferred

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	tools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/harness/tools/descriptions"
)

type todoItem struct {
	ID     string `json:"id,omitempty"`
	Text   string `json:"text"`
	Status string `json:"status"`
}

type todoStore struct {
	mu    sync.Mutex
	items map[string][]todoItem
}

func newTodoStore() *todoStore {
	return &todoStore{items: make(map[string][]todoItem)}
}

// TodosTool returns a deferred tool for managing run-scoped todo state.
// It creates its own internal todo store.
func TodosTool() tools.Tool {
	store := newTodoStore()

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
