package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go-agent-harness/internal/harness/tools/descriptions"
)

func sourcegraphTool(client *http.Client, cfg SourcegraphConfig) Tool {
	def := Definition{
		Name:         "sourcegraph",
		Description:  descriptions.Load("sourcegraph"),
		Action:       ActionRead,
		ParallelSafe: true,
		Tags:         []string{"search", "code", "repositories", "cross-repo", "sourcegraph"},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query":           map[string]any{"type": "string"},
				"count":           map[string]any{"type": "integer", "minimum": 1, "maximum": 200},
				"context_window":  map[string]any{"type": "integer", "minimum": 0, "maximum": 2000},
				"timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "maximum": 60},
			},
			"required": []string{"query"},
		},
	}

	handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
		if strings.TrimSpace(cfg.Endpoint) == "" {
			return "", fmt.Errorf("sourcegraph endpoint is not configured")
		}
		args := struct {
			Query          string `json:"query"`
			Count          int    `json:"count"`
			ContextWindow  int    `json:"context_window"`
			TimeoutSeconds int    `json:"timeout_seconds"`
		}{Count: 20, TimeoutSeconds: 15}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse sourcegraph args: %w", err)
		}
		if strings.TrimSpace(args.Query) == "" {
			return "", fmt.Errorf("query is required")
		}
		if args.Count <= 0 {
			args.Count = 20
		}
		if args.Count > 200 {
			args.Count = 200
		}
		if args.TimeoutSeconds <= 0 {
			args.TimeoutSeconds = 15
		}
		if args.TimeoutSeconds > 60 {
			args.TimeoutSeconds = 60
		}

		tctx, cancel := context.WithTimeout(ctx, time.Duration(args.TimeoutSeconds)*time.Second)
		defer cancel()
		payload := map[string]any{"query": args.Query, "count": args.Count, "context_window": args.ContextWindow}
		body, _ := json.Marshal(payload)
		req, err := http.NewRequestWithContext(tctx, http.MethodPost, cfg.Endpoint, bytes.NewReader(body))
		if err != nil {
			return "", fmt.Errorf("build sourcegraph request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		if strings.TrimSpace(cfg.Token) != "" {
			req.Header.Set("Authorization", "token "+cfg.Token)
		}

		res, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("sourcegraph request failed: %w", err)
		}
		defer res.Body.Close()
		resBody, err := io.ReadAll(io.LimitReader(res.Body, 1024*1024))
		if err != nil {
			return "", fmt.Errorf("read sourcegraph response: %w", err)
		}

		result := map[string]any{
			"status_code": res.StatusCode,
			"response":    string(resBody),
		}
		return MarshalToolResult(result)
	}

	return Tool{Definition: def, Handler: handler}
}
