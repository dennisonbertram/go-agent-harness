package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go-agent-harness/internal/harness/tools/descriptions"
)

func agentTool(runner AgentRunner) Tool {
	def := Definition{
		Name:         "agent",
		Description:  descriptions.Load("agent"),
		Action:       ActionExecute,
		Mutating:     true,
		ParallelSafe: false,
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
		return MarshalToolResult(map[string]any{"output": output})
	}
	return Tool{Definition: def, Handler: handler}
}

func agenticFetchTool(fetcher WebFetcher, runner AgentRunner) Tool {
	def := Definition{
		Name:         "agentic_fetch",
		Description:  descriptions.Load("agentic_fetch"),
		Action:       ActionFetch,
		Mutating:     false,
		ParallelSafe: true,
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
		return MarshalToolResult(result)
	}
	return Tool{Definition: def, Handler: handler}
}

func webSearchTool(fetcher WebFetcher) Tool {
	def := Definition{
		Name:         "web_search",
		Description:  descriptions.Load("web_search"),
		Action:       ActionFetch,
		ParallelSafe: true,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query":       map[string]any{"type": "string"},
				"max_results": map[string]any{"type": "integer", "minimum": 1, "maximum": 50},
			},
			"required": []string{"query"},
		},
	}
	handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
		args := struct {
			Query      string `json:"query"`
			MaxResults int    `json:"max_results"`
		}{MaxResults: 5}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse web_search args: %w", err)
		}
		if strings.TrimSpace(args.Query) == "" {
			return "", fmt.Errorf("query is required")
		}
		if args.MaxResults <= 0 {
			args.MaxResults = 5
		}
		if args.MaxResults > 50 {
			args.MaxResults = 50
		}
		items, err := fetcher.Search(ctx, args.Query, args.MaxResults)
		if err != nil {
			return "", err
		}
		return MarshalToolResult(map[string]any{"query": args.Query, "results": items})
	}
	return Tool{Definition: def, Handler: handler}
}

func webFetchTool(fetcher WebFetcher) Tool {
	def := Definition{
		Name:         "web_fetch",
		Description:  descriptions.Load("web_fetch"),
		Action:       ActionFetch,
		ParallelSafe: true,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"url": map[string]any{"type": "string"}},
			"required":   []string{"url"},
		},
	}
	handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
		args := struct {
			URL string `json:"url"`
		}{}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse web_fetch args: %w", err)
		}
		if strings.TrimSpace(args.URL) == "" {
			return "", fmt.Errorf("url is required")
		}
		content, err := fetcher.Fetch(ctx, args.URL)
		if err != nil {
			return "", err
		}
		return MarshalToolResult(map[string]any{"url": args.URL, "content": content})
	}
	return Tool{Definition: def, Handler: handler}
}
