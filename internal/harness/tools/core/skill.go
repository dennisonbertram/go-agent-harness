package core

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"strings"

	tools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/harness/tools/descriptions"
)

// buildSkillDescription loads the base description from embed and appends
// an <available_skills> XML block listing all registered skills.
func buildSkillDescription(lister tools.SkillLister) string {
	base := descriptions.Load("skill")
	skills := lister.ListSkills()
	if len(skills) == 0 {
		return base
	}

	var b strings.Builder
	b.WriteString(base)
	b.WriteString("\n\n<available_skills>\n")
	for _, s := range skills {
		b.WriteString(fmt.Sprintf(`<skill name=%q description=%q`,
			html.EscapeString(s.Name),
			html.EscapeString(s.Description)))
		if s.ArgumentHint != "" {
			b.WriteString(fmt.Sprintf(` argument_hint=%q`, html.EscapeString(s.ArgumentHint)))
		}
		b.WriteString(" />\n")
	}
	b.WriteString("</available_skills>")
	return b.String()
}

// SkillTool returns a core-tier tool for applying registered skills.
// The description is dynamically generated to include the list of available skills.
func SkillTool(lister tools.SkillLister) tools.Tool {
	def := tools.Definition{
		Name:         "skill",
		Description:  buildSkillDescription(lister),
		Action:       tools.ActionRead,
		Mutating:     false,
		ParallelSafe: true,
		Tier:         tools.TierCore,
		Tags:         []string{"skills", "specialization"},
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
	}

	return tools.Tool{Definition: def, Handler: handler}
}
