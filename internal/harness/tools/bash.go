package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"go-agent-harness/internal/harness/tools/descriptions"
)

var dangerousBashRegexps = compileDangerousPatterns()

func compileDangerousPatterns() []*regexp.Regexp {
	out := make([]*regexp.Regexp, 0, len(dangerousBashPatterns))
	for _, p := range dangerousBashPatterns {
		out = append(out, regexp.MustCompile(p))
	}
	return out
}

func isDangerousCommand(command string) bool {
	for _, pattern := range dangerousBashRegexps {
		if pattern.MatchString(command) {
			return true
		}
	}
	return false
}

func bashTool(manager *JobManager) Tool {
	def := Definition{
		Name:         "bash",
		Description:  descriptions.Load("bash"),
		Action:       ActionExecute,
		Mutating:     true,
		ParallelSafe: false,
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
		args := struct {
			Description     string `json:"description"`
			Command         string `json:"command"`
			TimeoutSeconds  int    `json:"timeout_seconds"`
			WorkingDir      string `json:"working_dir"`
			RunInBackground bool   `json:"run_in_background"`
		}{TimeoutSeconds: 30}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse bash args: %w", err)
		}
		if strings.TrimSpace(args.Command) == "" {
			return "", fmt.Errorf("command is required")
		}
		if isDangerousCommand(args.Command) {
			return "", fmt.Errorf("command rejected by safety policy")
		}

		if args.RunInBackground {
			result, err := manager.runBackground(args.Command, args.TimeoutSeconds, args.WorkingDir)
			if err != nil {
				return "", err
			}
			if args.Description != "" {
				result["description"] = args.Description
			}
			return MarshalToolResult(result)
		}

		result, err := manager.runForeground(ctx, args.Command, args.TimeoutSeconds, args.WorkingDir)
		if err != nil {
			return "", err
		}
		if args.Description != "" {
			result["description"] = args.Description
		}
		return MarshalToolResult(result)
	}

	return Tool{Definition: def, Handler: handler}
}

func jobOutputTool(manager *JobManager) Tool {
	def := Definition{
		Name:         "job_output",
		Description:  descriptions.Load("job_output"),
		Action:       ActionRead,
		ParallelSafe: true,
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
		args := struct {
			ShellID string `json:"shell_id"`
			Wait    bool   `json:"wait"`
		}{}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse job_output args: %w", err)
		}
		if args.ShellID == "" {
			return "", fmt.Errorf("shell_id is required")
		}
		result, err := manager.output(args.ShellID, args.Wait)
		if err != nil {
			return "", err
		}
		return MarshalToolResult(result)
	}

	return Tool{Definition: def, Handler: handler}
}

func jobKillTool(manager *JobManager) Tool {
	def := Definition{
		Name:         "job_kill",
		Description:  descriptions.Load("job_kill"),
		Action:       ActionExecute,
		Mutating:     true,
		ParallelSafe: false,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"shell_id": map[string]any{"type": "string"},
			},
			"required": []string{"shell_id"},
		},
	}

	handler := func(_ context.Context, raw json.RawMessage) (string, error) {
		args := struct {
			ShellID string `json:"shell_id"`
		}{}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse job_kill args: %w", err)
		}
		if args.ShellID == "" {
			return "", fmt.Errorf("shell_id is required")
		}
		result, err := manager.kill(args.ShellID)
		if err != nil {
			return "", err
		}
		return MarshalToolResult(result)
	}

	return Tool{Definition: def, Handler: handler}
}
