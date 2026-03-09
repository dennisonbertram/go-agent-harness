package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

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

func todosTool(store *todoStore) Tool {
	def := Definition{
		Name:         "todos",
		Description:  descriptions.Load("todos"),
		Action:       ActionWrite,
		Mutating:     true,
		ParallelSafe: false,
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

		runID := RunIDFromContext(ctx)
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
		return MarshalToolResult(result)
	}

	return Tool{Definition: def, Handler: handler}
}
