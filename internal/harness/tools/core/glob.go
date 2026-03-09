package core

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	tools "go-agent-harness/internal/harness/tools"
)

// GlobTool returns a core tool that matches files in the workspace by glob pattern.
func GlobTool(opts tools.BuildOptions) tools.Tool {
	def := tools.Definition{
		Name:         "glob",
		Description:  "Match files in workspace by glob pattern",
		Action:       tools.ActionList,
		ParallelSafe: true,
		Tier:         tools.TierCore,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern":     map[string]any{"type": "string", "description": "glob pattern relative to workspace"},
				"max_matches": map[string]any{"type": "integer", "minimum": 1, "maximum": 2000},
			},
			"required": []string{"pattern"},
		},
	}

	workspaceRoot := opts.WorkspaceRoot

	handler := func(_ context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			Pattern    string `json:"pattern"`
			MaxMatches int    `json:"max_matches"`
		}
		args.MaxMatches = 500
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse glob args: %w", err)
		}
		if strings.TrimSpace(args.Pattern) == "" {
			return "", fmt.Errorf("pattern is required")
		}
		if args.MaxMatches <= 0 {
			args.MaxMatches = 500
		}
		if args.MaxMatches > 2000 {
			args.MaxMatches = 2000
		}
		if err := tools.ValidateWorkspaceRelativePattern(args.Pattern); err != nil {
			return "", err
		}

		absRoot, err := filepath.Abs(workspaceRoot)
		if err != nil {
			return "", fmt.Errorf("resolve workspace root: %w", err)
		}
		absPattern := filepath.Join(absRoot, filepath.FromSlash(args.Pattern))
		matches, err := filepath.Glob(absPattern)
		if err != nil {
			return "", fmt.Errorf("glob pattern: %w", err)
		}

		filtered := make([]string, 0, len(matches))
		for _, match := range matches {
			rel, err := filepath.Rel(absRoot, match)
			if err != nil {
				continue
			}
			if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				continue
			}
			filtered = append(filtered, filepath.ToSlash(rel))
			if len(filtered) >= args.MaxMatches {
				break
			}
		}
		sort.Strings(filtered)

		result := map[string]any{
			"pattern": args.Pattern,
			"matches": filtered,
		}
		return tools.MarshalToolResult(result)
	}

	return tools.Tool{Definition: def, Handler: handler}
}
