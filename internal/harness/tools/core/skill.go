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
		if meta, ok := tools.RunMetadataFromContext(ctx); ok {
			workspace = meta.RunID
		}
		content, err := lister.ResolveSkill(name, skillArgs, workspace)
		if err != nil {
			return "", err
		}
		info, _ := lister.GetSkill(name)

		// The normal tool output is a concise activation acknowledgment
		ack, err := tools.MarshalToolResult(map[string]any{
			"skill":         info.Name,
			"status":        "activated",
			"allowed_tools": info.AllowedTools,
		})
		if err != nil {
			return "", err
		}

		// The skill instructions are injected as a meta-message
		metaMsg := fmt.Sprintf("<skill name=%q>\n%s\n</skill>", info.Name, content)
		return tools.WrapToolResult(tools.ToolResult{
			Output: ack,
			MetaMessages: []tools.MetaMessage{
				{Content: metaMsg},
			},
		})
	}

	return tools.Tool{Definition: def, Handler: handler}
}
