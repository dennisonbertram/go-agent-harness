package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

func gitStatusTool(workspaceRoot string) Tool {
	def := Definition{
		Name:         "git_status",
		Description:  "Get git status for workspace repository",
		Action:       ActionRead,
		ParallelSafe: true,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"porcelain": map[string]any{"type": "boolean"},
			},
		},
	}

	handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
		args := struct {
			Porcelain bool `json:"porcelain"`
		}{Porcelain: true}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &args); err != nil {
				return "", fmt.Errorf("parse git_status args: %w", err)
			}
		}

		absRoot, err := filepath.Abs(workspaceRoot)
		if err != nil {
			return "", fmt.Errorf("resolve workspace root: %w", err)
		}

		cmdArgs := []string{"-C", absRoot, "status"}
		if args.Porcelain {
			cmdArgs = append(cmdArgs, "--porcelain=v1")
		}
		output, exitCode, timedOut, err := runCommand(ctx, 30*time.Second, "git", cmdArgs...)
		if err != nil {
			return "", fmt.Errorf("git status failed: %w", err)
		}

		trimmed := strings.TrimSpace(output)
		result := map[string]any{
			"clean":     trimmed == "",
			"output":    trimmed,
			"exit_code": exitCode,
			"timed_out": timedOut,
		}
		return marshalToolResult(result)
	}

	return Tool{Definition: def, Handler: handler}
}
