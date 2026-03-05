package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

func globTool(workspaceRoot string) Tool {
	def := Definition{
		Name:         "glob",
		Description:  "Match files in workspace by glob pattern",
		Action:       ActionList,
		ParallelSafe: true,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern":     map[string]any{"type": "string", "description": "glob pattern relative to workspace"},
				"max_matches": map[string]any{"type": "integer", "minimum": 1, "maximum": 2000},
			},
			"required": []string{"pattern"},
		},
	}

	handler := func(_ context.Context, raw json.RawMessage) (string, error) {
		args := struct {
			Pattern    string `json:"pattern"`
			MaxMatches int    `json:"max_matches"`
		}{MaxMatches: 500}
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
		if err := validateWorkspaceRelativePattern(args.Pattern); err != nil {
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
		return marshalToolResult(result)
	}

	return Tool{Definition: def, Handler: handler}
}
