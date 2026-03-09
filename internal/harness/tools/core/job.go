package core

import (
	"context"
	"encoding/json"
	"fmt"

	tools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/harness/tools/descriptions"
)

// JobOutputTool returns a core tool that reads output of a background bash job.
func JobOutputTool(manager *tools.JobManager) tools.Tool {
	def := tools.Definition{
		Name:         "job_output",
		Description:  descriptions.Load("job_output"),
		Action:       tools.ActionRead,
		ParallelSafe: true,
		Tier:         tools.TierCore,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"shell_id": map[string]any{"type": "string"},
				"wait":     map[string]any{"type": "boolean"},
			},
			"required": []string{"shell_id"},
		},
	}

	handler := func(_ context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			ShellID string `json:"shell_id"`
			Wait    bool   `json:"wait"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse job_output args: %w", err)
		}
		if args.ShellID == "" {
			return "", fmt.Errorf("shell_id is required")
		}
		result, err := manager.Output(args.ShellID, args.Wait)
		if err != nil {
			return "", err
		}
		return tools.MarshalToolResult(result)
	}

	return tools.Tool{Definition: def, Handler: handler}
}

// JobKillTool returns a core tool that terminates a background bash job.
func JobKillTool(manager *tools.JobManager) tools.Tool {
	def := tools.Definition{
		Name:         "job_kill",
		Description:  descriptions.Load("job_kill"),
		Action:       tools.ActionExecute,
		Mutating:     true,
		ParallelSafe: false,
		Tier:         tools.TierCore,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"shell_id": map[string]any{"type": "string"},
			},
			"required": []string{"shell_id"},
		},
	}

	handler := func(_ context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			ShellID string `json:"shell_id"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse job_kill args: %w", err)
		}
		if args.ShellID == "" {
			return "", fmt.Errorf("shell_id is required")
		}
		result, err := manager.Kill(args.ShellID)
		if err != nil {
			return "", err
		}
		return tools.MarshalToolResult(result)
	}

	return tools.Tool{Definition: def, Handler: handler}
}
