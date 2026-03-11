package tools

import (
	"context"
	"encoding/json"
	"unicode/utf8"

	"go-agent-harness/internal/harness/tools/descriptions"
)

func contextStatusTool() Tool {
	return Tool{
		Definition: Definition{
			Name:         "context_status",
			Description:  descriptions.Load("context_status"),
			Parameters:   map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false},
			Action:       ActionRead,
			Mutating:     false,
			ParallelSafe: true,
			Tags:         []string{"context", "status", "tokens", "memory"},
			Tier:         TierCore,
		},
		Handler: handleContextStatus,
	}
}

func handleContextStatus(ctx context.Context, _ json.RawMessage) (string, error) {
	reader, ok := TranscriptReaderFromContext(ctx)
	if !ok {
		return MarshalToolResult(map[string]any{
			"error": "transcript reader not available",
		})
	}

	snap := reader.Snapshot(0, true) // 0 = no limit, include tools

	var (
		totalTokens       int
		toolResultCount   int
		userMsgCount      int
		assistantMsgCount int
		systemMsgCount    int
		hasCompactSummary bool
	)

	for _, msg := range snap.Messages {
		// Estimate tokens: (runes+3)/4 — matches RuneTokenEstimator
		runes := utf8.RuneCountInString(msg.Content)
		if runes > 0 {
			totalTokens += (runes + 3) / 4
		}

		switch msg.Role {
		case "user":
			userMsgCount++
		case "assistant":
			assistantMsgCount++
		case "tool":
			toolResultCount++
		case "system":
			systemMsgCount++
		}
	}

	// Each tool result corresponds to one tool call.
	toolCallCount := toolResultCount

	// Check for compact summary — look for system messages with the compact summary marker.
	for _, msg := range snap.Messages {
		if msg.Role == "system" && msg.Name == "compact_summary" {
			hasCompactSummary = true
			break
		}
	}

	msgCount := len(snap.Messages)
	recommendation := contextRecommendation(totalTokens, msgCount, toolResultCount)

	result := map[string]any{
		"estimated_context_tokens": totalTokens,
		"message_count":            msgCount,
		"tool_call_count":          toolCallCount,
		"tool_result_count":        toolResultCount,
		"user_message_count":       userMsgCount,
		"assistant_message_count":  assistantMsgCount,
		"system_message_count":     systemMsgCount,
		"has_compact_summary":      hasCompactSummary,
		"recommendation":           recommendation,
	}

	return MarshalToolResult(result)
}

// contextRecommendation returns a brief recommendation based on context pressure.
func contextRecommendation(estimatedTokens, msgCount, toolResultCount int) string {
	switch {
	case estimatedTokens > 100000:
		return "critical: context is very large, compact immediately with mode=strip or mode=hybrid"
	case estimatedTokens > 60000:
		return "warning: context is growing large, consider compacting with mode=hybrid"
	case estimatedTokens > 30000:
		return "elevated: context is moderate, consider compacting if more tool calls are expected"
	case toolResultCount > 20:
		return "elevated: many tool results accumulated, consider compacting tool outputs"
	default:
		return "healthy: context pressure is low"
	}
}
