package core

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"strings"
	"time"

	tools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/harness/tools/descriptions"
)

// defaultForkTimeout is the maximum duration for a forked skill execution.
const defaultForkTimeout = 120 * time.Second

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
		if s.Context == "fork" {
			b.WriteString(fmt.Sprintf(` context=%q`, "fork"))
		}
		b.WriteString(" />\n")
	}
	b.WriteString("</available_skills>")
	return b.String()
}

// SkillTool returns a core-tier tool for applying registered skills.
// The description is dynamically generated to include the list of available skills.
// The runner parameter can be nil; it is used only for context:fork skills.
func SkillTool(lister tools.SkillLister, runner tools.AgentRunner) tools.Tool {
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
		content, err := lister.ResolveSkill(ctx, name, skillArgs, workspace)
		if err != nil {
			return "", err
		}
		info, _ := lister.GetSkill(name)

		// Fork path: context == "fork"
		if info.Context == "fork" {
			return handleForkSkill(ctx, runner, info, content)
		}

		// Conversation path (default): inject as meta-message
		return handleConversationSkill(info, content)
	}

	return tools.Tool{Definition: def, Handler: handler}
}

// handleForkSkill dispatches a skill to a subagent for isolated execution.
func handleForkSkill(ctx context.Context, runner tools.AgentRunner, info tools.SkillInfo, content string) (string, error) {
	// Prevent nested forking
	if _, nested := ctx.Value(tools.ContextKeyForkedSkill).(string); nested {
		return "", fmt.Errorf("nested skill forking is not supported")
	}

	if runner == nil {
		return "", fmt.Errorf("skill %q requires context: fork but no AgentRunner is configured", info.Name)
	}

	// Apply timeout
	forkCtx, cancel := context.WithTimeout(ctx, defaultForkTimeout)
	defer cancel()

	// Mark context as forked to prevent recursion
	forkCtx = context.WithValue(forkCtx, tools.ContextKeyForkedSkill, info.Name)

	// Check if runner implements ForkedAgentRunner for richer invocation
	if forkedRunner, ok := runner.(tools.ForkedAgentRunner); ok {
		config := tools.ForkConfig{
			Prompt:       content,
			SkillName:    info.Name,
			Agent:        info.Agent,
			AllowedTools: info.AllowedTools,
		}
		result, err := forkedRunner.RunForkedSkill(forkCtx, config)
		if err != nil {
			return "", fmt.Errorf("forked skill %q failed: %w", info.Name, err)
		}

		// Prefer summary over full output
		output := result.Summary
		if output == "" {
			output = result.Output
		}

		return tools.MarshalToolResult(map[string]any{
			"skill":   info.Name,
			"status":  "completed",
			"result":  output,
			"context": "fork",
		})
	}

	// Fallback: basic AgentRunner.RunPrompt
	output, err := runner.RunPrompt(forkCtx, content)
	if err != nil {
		return "", fmt.Errorf("forked skill %q failed: %w", info.Name, err)
	}

	return tools.MarshalToolResult(map[string]any{
		"skill":   info.Name,
		"status":  "completed",
		"result":  output,
		"context": "fork",
	})
}

// handleConversationSkill injects the skill content into the current conversation
// as a meta-message (the default behavior).
func handleConversationSkill(info tools.SkillInfo, content string) (string, error) {
	ack, err := tools.MarshalToolResult(map[string]any{
		"skill":         info.Name,
		"status":        "activated",
		"allowed_tools": info.AllowedTools,
	})
	if err != nil {
		return "", err
	}

	metaMsg := fmt.Sprintf("<skill name=%q>\n%s\n</skill>", info.Name, content)
	return tools.WrapToolResult(tools.ToolResult{
		Output: ack,
		MetaMessages: []tools.MetaMessage{
			{Content: metaMsg},
		},
	})
}
