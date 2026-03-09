package deferred

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	tools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/harness/tools/descriptions"
)

// SkillTool returns a deferred tool for applying specialized skills.
func SkillTool(lister tools.SkillLister) tools.Tool {
	def := tools.Definition{
		Name:         "skill",
		Description:  descriptions.Load("skill"),
		Action:       tools.ActionRead,
		Mutating:     false,
		ParallelSafe: true,
		Tier:         tools.TierDeferred,
		Tags:         []string{"skills", "specialization"},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type":        "string",
					"enum":        []string{"list", "apply"},
					"description": "Action to perform: 'list' to see available skills, 'apply' to apply a skill",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "Name of the skill to apply (required for 'apply' action)",
				},
				"arguments": map[string]any{
					"type":        "string",
					"description": "Arguments to pass to the skill",
				},
			},
			"required": []string{"action"},
		},
	}
	handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
		args := struct {
			Action    string `json:"action"`
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse skill args: %w", err)
		}

		switch strings.TrimSpace(args.Action) {
		case "list":
			skills := lister.ListSkills()
			return tools.MarshalToolResult(map[string]any{
				"skills": skills,
				"count":  len(skills),
			})
		case "apply":
			name := strings.TrimSpace(args.Name)
			if name == "" {
				return "", fmt.Errorf("name is required for apply action")
			}
			workspace := ""
			if meta, ok := tools.RunMetadataFromContext(ctx); ok {
				workspace = meta.RunID
			}
			content, err := lister.ResolveSkill(name, args.Arguments, workspace)
			if err != nil {
				return "", err
			}
			info, _ := lister.GetSkill(name)
			return tools.MarshalToolResult(map[string]any{
				"skill":         info.Name,
				"instructions":  content,
				"allowed_tools": info.AllowedTools,
			})
		default:
			return "", fmt.Errorf("unknown action %q: must be 'list' or 'apply'", args.Action)
		}
	}
	return tools.Tool{Definition: def, Handler: handler}
}
