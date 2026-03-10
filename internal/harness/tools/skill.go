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
				"command": map[string]any{
					"type":        "string",
					"description": "Skill name followed by optional arguments. Example: 'deploy staging' or 'code-review'.",
				},
			},
			"required": []string{"command"},
		},
	}
	handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse skill args: %w", err)
		}

		command := strings.TrimSpace(args.Command)
		if command == "" {
			return "", fmt.Errorf("command is required: provide a skill name")
		}
		name, skillArgs, _ := strings.Cut(command, " ")
		skillArgs = strings.TrimSpace(skillArgs)

		workspace := ""
		if meta, ok := RunMetadataFromContext(ctx); ok {
			workspace = meta.RunID
		}
		content, err := lister.ResolveSkill(ctx, name, skillArgs, workspace)
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
