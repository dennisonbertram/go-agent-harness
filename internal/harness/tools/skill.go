package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go-agent-harness/internal/harness/tools/descriptions"
)

func skillTool(lister SkillLister) Tool {
	def := Definition{
		Name:         "skill",
		Description:  descriptions.Load("skill"),
		Action:       ActionRead,
		Mutating:     false,
		ParallelSafe: true,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Name of the skill to apply",
				},
				"arguments": map[string]any{
					"type":        "string",
					"description": "Optional arguments to pass to the skill",
				},
			},
			"required": []string{"name"},
		},
	}
	handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse skill args: %w", err)
		}

		name := strings.TrimSpace(args.Name)
		if name == "" {
			return "", fmt.Errorf("name is required")
		}

		workspace := ""
		if meta, ok := RunMetadataFromContext(ctx); ok {
			workspace = meta.RunID
		}
		content, err := lister.ResolveSkill(name, args.Arguments, workspace)
		if err != nil {
			return "", err
		}
		info, _ := lister.GetSkill(name)
		return MarshalToolResult(map[string]any{
			"skill":         info.Name,
			"instructions":  content,
			"allowed_tools": info.AllowedTools,
		})
	}
	return Tool{Definition: def, Handler: handler}
}
