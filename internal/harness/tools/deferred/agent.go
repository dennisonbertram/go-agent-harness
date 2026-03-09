package deferred

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	tools "go-agent-harness/internal/harness/tools"
)

// AgentTool returns a deferred tool for running a delegated sub-agent prompt.
func AgentTool(runner tools.AgentRunner) tools.Tool {
	def := tools.Definition{
		Name:         "agent",
		Description:  "Run a delegated sub-agent prompt",
		Action:       tools.ActionExecute,
		Mutating:     true,
		ParallelSafe: false,
		Tier:         tools.TierDeferred,
		Tags:         []string{"agent", "sub-agent", "delegation"},
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"prompt": map[string]any{"type": "string"}},
			"required":   []string{"prompt"},
		},
	}
	handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
		args := struct {
			Prompt string `json:"prompt"`
		}{}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse agent args: %w", err)
		}
		if strings.TrimSpace(args.Prompt) == "" {
			return "", fmt.Errorf("prompt is required")
		}
		output, err := runner.RunPrompt(ctx, args.Prompt)
		if err != nil {
			return "", err
		}
		return tools.MarshalToolResult(map[string]any{"output": output})
	}
	return tools.Tool{Definition: def, Handler: handler}
}

// AgenticFetchTool returns a deferred tool that fetches/analyzes web content with optional delegated reasoning.
func AgenticFetchTool(fetcher tools.WebFetcher, runner tools.AgentRunner) tools.Tool {
	def := tools.Definition{
		Name:         "agentic_fetch",
		Description:  "Fetch/analyze web content with optional delegated reasoning",
		Action:       tools.ActionFetch,
		Mutating:     false,
		ParallelSafe: true,
		Tier:         tools.TierDeferred,
		Tags:         []string{"agent", "sub-agent", "delegation"},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"prompt": map[string]any{"type": "string"},
				"url":    map[string]any{"type": "string"},
			},
			"required": []string{"prompt"},
		},
	}
	handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
		args := struct {
			Prompt string `json:"prompt"`
			URL    string `json:"url"`
		}{}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse agentic_fetch args: %w", err)
		}
		if strings.TrimSpace(args.Prompt) == "" {
			return "", fmt.Errorf("prompt is required")
		}
		result := map[string]any{"prompt": args.Prompt}
		if strings.TrimSpace(args.URL) != "" {
			content, err := fetcher.Fetch(ctx, args.URL)
			if err != nil {
				return "", err
			}
			result["url"] = args.URL
			result["content"] = content
		}
		analysis, err := runner.RunPrompt(ctx, args.Prompt)
		if err != nil {
			return "", err
		}
		result["analysis"] = analysis
		return tools.MarshalToolResult(result)
	}
	return tools.Tool{Definition: def, Handler: handler}
}
