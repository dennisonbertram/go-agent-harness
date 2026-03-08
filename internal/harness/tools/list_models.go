package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"go-agent-harness/internal/provider/catalog"
)

func listModelsTool(cat *catalog.Catalog) Tool {
	def := Definition{
		Name:         "list_models",
		Description:  "List, filter, and inspect available LLM models across providers",
		Action:       ActionRead,
		ParallelSafe: true,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action":       map[string]any{"type": "string", "enum": []string{"list", "info", "providers"}, "description": "Action to perform (default: list)"},
				"provider":     map[string]any{"type": "string", "description": "Filter by provider key"},
				"model_id":     map[string]any{"type": "string", "description": "Model ID for info action"},
				"tool_calling": map[string]any{"type": "boolean", "description": "Filter by tool_calling support"},
				"streaming":    map[string]any{"type": "boolean", "description": "Filter by streaming support"},
				"speed_tier":   map[string]any{"type": "string", "description": "Filter by speed tier"},
				"cost_tier":    map[string]any{"type": "string", "description": "Filter by cost tier"},
				"modality":     map[string]any{"type": "string", "description": "Filter by modality (e.g. text, vision)"},
				"best_for":     map[string]any{"type": "string", "description": "Filter by best_for tag"},
				"strength":     map[string]any{"type": "string", "description": "Filter by strength tag"},
				"min_context":  map[string]any{"type": "integer", "minimum": 1, "description": "Minimum context window size"},
				"reasoning":    map[string]any{"type": "boolean", "description": "Filter by reasoning mode"},
			},
		},
	}

	handler := func(_ context.Context, raw json.RawMessage) (string, error) {
		args := struct {
			Action      string `json:"action"`
			Provider    string `json:"provider"`
			ModelID     string `json:"model_id"`
			ToolCalling *bool  `json:"tool_calling"`
			Streaming   *bool  `json:"streaming"`
			SpeedTier   string `json:"speed_tier"`
			CostTier    string `json:"cost_tier"`
			Modality    string `json:"modality"`
			BestFor     string `json:"best_for"`
			Strength    string `json:"strength"`
			MinContext  int    `json:"min_context"`
			Reasoning   *bool  `json:"reasoning"`
		}{}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse list_models args: %w", err)
		}
		if args.Action == "" {
			args.Action = "list"
		}

		switch args.Action {
		case "providers":
			providers := cat.ListProviders()
			return marshalToolResult(map[string]any{
				"action":    "providers",
				"providers": providers,
			})

		case "info":
			if args.Provider == "" || args.ModelID == "" {
				return "", fmt.Errorf("provider and model_id are required for info action")
			}
			result, ok := cat.ModelInfo(args.Provider, args.ModelID)
			if !ok {
				return marshalToolResult(map[string]any{
					"action": "info",
					"error":  fmt.Sprintf("model %s/%s not found", args.Provider, args.ModelID),
				})
			}
			return marshalToolResult(map[string]any{
				"action": "info",
				"model":  result,
			})

		case "list":
			opts := catalog.FilterOptions{
				Provider:    args.Provider,
				ToolCalling: args.ToolCalling,
				Streaming:   args.Streaming,
				SpeedTier:   args.SpeedTier,
				CostTier:    args.CostTier,
				Modality:    args.Modality,
				BestFor:     args.BestFor,
				Strength:    args.Strength,
				MinContext:  args.MinContext,
				Reasoning:   args.Reasoning,
			}
			models := cat.FilterModels(opts)
			return marshalToolResult(map[string]any{
				"action": "list",
				"count":  len(models),
				"models": models,
			})

		default:
			return "", fmt.Errorf("unknown action %q, must be list, info, or providers", args.Action)
		}
	}

	return Tool{Definition: def, Handler: handler}
}
