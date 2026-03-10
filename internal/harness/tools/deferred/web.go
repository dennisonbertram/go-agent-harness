package deferred

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	tools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/harness/tools/descriptions"
)

// WebSearchTool returns a deferred tool for searching the web.
func WebSearchTool(fetcher tools.WebFetcher) tools.Tool {
	def := tools.Definition{
		Name:         "web_search",
		Description:  descriptions.Load("web_search"),
		Action:       tools.ActionFetch,
		ParallelSafe: true,
		Tier:         tools.TierDeferred,
		Tags:         []string{"search", "web", "internet", "query", "results"},
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
		return tools.MarshalToolResult(map[string]any{"query": args.Query, "results": items})
	}
	return tools.Tool{Definition: def, Handler: handler}
}

// WebFetchTool returns a deferred tool for fetching a webpage.
func WebFetchTool(fetcher tools.WebFetcher) tools.Tool {
	def := tools.Definition{
		Name:         "web_fetch",
		Description:  descriptions.Load("web_fetch"),
		Action:       tools.ActionFetch,
		ParallelSafe: true,
		Tier:         tools.TierDeferred,
		Tags:         []string{"web", "fetch", "page", "url", "browse"},
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
		return tools.MarshalToolResult(map[string]any{"url": args.URL, "content": content})
	}
	return tools.Tool{Definition: def, Handler: handler}
}
