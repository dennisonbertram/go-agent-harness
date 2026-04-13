package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	tools "go-agent-harness/internal/harness/tools"
	om "go-agent-harness/internal/observationalmemory"
	"go-agent-harness/internal/workingmemory"
)

type workingMemoryArgs struct {
	Action string `json:"action"`
	Key    string `json:"key,omitempty"`
	Value  any    `json:"value,omitempty"`
}

func WorkingMemoryTool(store workingmemory.Store) tools.Tool {
	def := tools.Definition{
		Name:         "working_memory",
		Description:  "Stores and retrieves explicit scoped working memory entries for the current run.",
		Action:       tools.ActionWrite,
		Mutating:     true,
		ParallelSafe: false,
		Tier:         tools.TierCore,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{"type": "string", "enum": []string{"set", "get", "delete", "list"}},
				"key":    map[string]any{"type": "string"},
				"value":  map[string]any{},
			},
			"required": []string{"action"},
		},
	}

	handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
		if store == nil {
			return "", fmt.Errorf("working memory store is not configured")
		}
		var args workingMemoryArgs
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse working_memory args: %w", err)
		}
		scope := workingMemoryScopeFromContext(ctx)
		switch strings.TrimSpace(strings.ToLower(args.Action)) {
		case "set":
			if err := store.Set(ctx, scope, args.Key, args.Value); err != nil {
				return "", err
			}
			return tools.MarshalToolResult(map[string]any{"ok": true, "key": strings.TrimSpace(args.Key)})
		case "get":
			value, ok, err := store.Get(ctx, scope, args.Key)
			if err != nil {
				return "", err
			}
			return tools.MarshalToolResult(map[string]any{"key": strings.TrimSpace(args.Key), "value": value, "found": ok})
		case "delete":
			if err := store.Delete(ctx, scope, args.Key); err != nil {
				return "", err
			}
			return tools.MarshalToolResult(map[string]any{"ok": true, "key": strings.TrimSpace(args.Key)})
		case "list":
			entries, err := store.List(ctx, scope)
			if err != nil {
				return "", err
			}
			return tools.MarshalToolResult(map[string]any{"entries": entries})
		default:
			return "", fmt.Errorf("unsupported action %q", args.Action)
		}
	}

	return tools.Tool{Definition: def, Handler: handler}
}

func workingMemoryScopeFromContext(ctx context.Context) om.ScopeKey {
	runID := tools.RunIDFromContext(ctx)
	meta, _ := tools.RunMetadataFromContext(ctx)
	tenantID := strings.TrimSpace(meta.TenantID)
	if tenantID == "" {
		tenantID = "default"
	}
	agentID := strings.TrimSpace(meta.AgentID)
	if agentID == "" {
		agentID = "default"
	}
	conversationID := strings.TrimSpace(meta.ConversationID)
	if conversationID == "" {
		conversationID = strings.TrimSpace(runID)
	}
	return om.ScopeKey{
		TenantID:       tenantID,
		ConversationID: conversationID,
		AgentID:        agentID,
	}
}
