package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go-agent-harness/internal/harness/tools/descriptions"
)

func skillTool(lister SkillLister, runner AgentRunner) Tool {
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

		// Fork path: context == "fork"
		if info.Context == "fork" {
			return flatSkillFork(ctx, runner, info, content)
		}

		return MarshalToolResult(map[string]any{
			"skill":         info.Name,
			"instructions":  content,
			"allowed_tools": info.AllowedTools,
		})
	}
	return Tool{Definition: def, Handler: handler}
}

// flatSkillFork handles forked skill execution in the flat catalog.
func flatSkillFork(ctx context.Context, runner AgentRunner, info SkillInfo, content string) (string, error) {
	// Prevent nested forking
	if _, nested := ctx.Value(ContextKeyForkedSkill).(string); nested {
		return "", fmt.Errorf("nested skill forking is not supported")
	}

	if runner == nil {
		return "", fmt.Errorf("skill %q requires context: fork but no AgentRunner is configured", info.Name)
	}

	forkCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	forkCtx = context.WithValue(forkCtx, ContextKeyForkedSkill, info.Name)

	if forkedRunner, ok := runner.(ForkedAgentRunner); ok {
		config := ForkConfig{
			Prompt:       content,
			SkillName:    info.Name,
			Agent:        info.Agent,
			AllowedTools: info.AllowedTools,
		}
		result, err := forkedRunner.RunForkedSkill(forkCtx, config)
		if err != nil {
			return "", fmt.Errorf("forked skill %q failed: %w", info.Name, err)
		}
		output := result.Summary
		if output == "" {
			output = result.Output
		}
		return MarshalToolResult(map[string]any{
			"skill":   info.Name,
			"status":  "completed",
			"result":  output,
			"context": "fork",
		})
	}

	output, err := runner.RunPrompt(forkCtx, content)
	if err != nil {
		return "", fmt.Errorf("forked skill %q failed: %w", info.Name, err)
	}
	return MarshalToolResult(map[string]any{
		"skill":   info.Name,
		"status":  "completed",
		"result":  output,
		"context": "fork",
	})
}
