package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	tools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/harness/tools/descriptions"
)

// BashTool returns a core tool that runs bash commands in the workspace.
func BashTool(manager *tools.JobManager) tools.Tool {
	def := tools.Definition{
		Name:         "bash",
		Description:  descriptions.Load("bash"),
		Action:       tools.ActionExecute,
		Mutating:     true,
		ParallelSafe: false,
		Tier:         tools.TierCore,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"description":       map[string]any{"type": "string"},
				"command":           map[string]any{"type": "string"},
				"timeout_seconds":   map[string]any{"type": "integer", "minimum": 1, "maximum": 3600},
				"working_dir":       map[string]any{"type": "string"},
				"run_in_background": map[string]any{"type": "boolean"},
			},
			"required": []string{"command"},
		},
	}

	handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			Description     string `json:"description"`
			Command         string `json:"command"`
			TimeoutSeconds  int    `json:"timeout_seconds"`
			WorkingDir      string `json:"working_dir"`
			RunInBackground bool   `json:"run_in_background"`
		}
		args.TimeoutSeconds = 30
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse bash args: %w", err)
		}
		if strings.TrimSpace(args.Command) == "" {
			return "", fmt.Errorf("command is required")
		}
		if tools.IsDangerousCommand(args.Command) {
			return "", fmt.Errorf("command rejected by safety policy")
		}

		if args.RunInBackground {
			result, err := manager.RunBackground(args.Command, args.TimeoutSeconds, args.WorkingDir)
			if err != nil {
				return "", err
			}
			if args.Description != "" {
				result["description"] = args.Description
			}
			return tools.MarshalToolResult(result)
		}

		result, err := manager.RunForeground(ctx, args.Command, args.TimeoutSeconds, args.WorkingDir)
		if err != nil {
			return "", err
		}
		if args.Description != "" {
			result["description"] = args.Description
		}
		return tools.MarshalToolResult(result)
	}

	return tools.Tool{Definition: def, Handler: handler}
}
