package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// contextStatusTestReader implements TranscriptReader for context_status tests.
type contextStatusTestReader struct {
	messages []TranscriptMessage
	runID    string
}

func (r contextStatusTestReader) Snapshot(limit int, includeTools bool) TranscriptSnapshot {
	msgs := r.messages
	if !includeTools {
		var filtered []TranscriptMessage
		for _, m := range msgs {
			if m.Role != "tool" {
				filtered = append(filtered, m)
			}
		}
		msgs = filtered
	}
	if limit > 0 && len(msgs) > limit {
		msgs = msgs[len(msgs)-limit:]
	}
	return TranscriptSnapshot{
		RunID:       r.runID,
		Messages:    msgs,
		GeneratedAt: time.Now(),
	}
}

func TestContextStatusTool_Definition(t *testing.T) {
	t.Parallel()
	tool := contextStatusTool()
	if tool.Definition.Name != "context_status" {
		t.Errorf("expected name context_status, got %s", tool.Definition.Name)
	}
	if tool.Definition.Mutating {
		t.Error("context_status should not be mutating")
	}
	if !tool.Definition.ParallelSafe {
		t.Error("context_status should be parallel safe")
	}
	if tool.Definition.Tier != TierCore {
		t.Errorf("expected TierCore, got %s", tool.Definition.Tier)
	}
	if tool.Definition.Action != ActionRead {
		t.Errorf("expected ActionRead, got %s", tool.Definition.Action)
	}
}

func TestContextStatusTool_NoTranscriptReader(t *testing.T) {
	t.Parallel()
	tool := contextStatusTool()
	out, err := tool.Handler(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	if _, ok := result["error"]; !ok {
		t.Error("expected error field when no transcript reader")
	}
}

func TestContextStatusTool_EmptyConversation(t *testing.T) {
	t.Parallel()
	tool := contextStatusTool()
	reader := contextStatusTestReader{runID: "test-run"}
	ctx := context.WithValue(context.Background(), ContextKeyTranscriptReader, TranscriptReader(reader))

	out, err := tool.Handler(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	if result["message_count"].(float64) != 0 {
		t.Errorf("expected 0 messages, got %v", result["message_count"])
	}
	if result["recommendation"].(string) != "healthy: context pressure is low" {
		t.Errorf("unexpected recommendation: %s", result["recommendation"])
	}
}

func TestContextStatusTool_MixedMessages(t *testing.T) {
	t.Parallel()
	tool := contextStatusTool()
	reader := contextStatusTestReader{
		runID: "test-run",
		messages: []TranscriptMessage{
			{Index: 0, Role: "system", Content: "You are a helpful assistant."},
			{Index: 1, Role: "user", Content: "Hello"},
			{Index: 2, Role: "assistant", Content: "Hi there!"},
			{Index: 3, Role: "tool", ToolCallID: "call_1", Content: "file contents here"},
			{Index: 4, Role: "assistant", Content: "I found the file."},
			{Index: 5, Role: "user", Content: "Thanks"},
		},
	}
	ctx := context.WithValue(context.Background(), ContextKeyTranscriptReader, TranscriptReader(reader))

	out, err := tool.Handler(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	assertEqual := func(key string, expected float64) {
		t.Helper()
		got, ok := result[key].(float64)
		if !ok {
			t.Errorf("key %s not a number: %v", key, result[key])
			return
		}
		if got != expected {
			t.Errorf("%s: expected %v, got %v", key, expected, got)
		}
	}

	assertEqual("message_count", 6)
	assertEqual("user_message_count", 2)
	assertEqual("assistant_message_count", 2)
	assertEqual("tool_result_count", 1)
	assertEqual("tool_call_count", 1)
	assertEqual("system_message_count", 1)

	if result["has_compact_summary"].(bool) != false {
		t.Error("expected has_compact_summary to be false")
	}
}

func TestContextStatusTool_CompactSummaryDetected(t *testing.T) {
	t.Parallel()
	tool := contextStatusTool()
	reader := contextStatusTestReader{
		runID: "test-run",
		messages: []TranscriptMessage{
			{Index: 0, Role: "system", Name: "compact_summary", Content: "Summary of prior conversation."},
			{Index: 1, Role: "user", Content: "Continue please"},
		},
	}
	ctx := context.WithValue(context.Background(), ContextKeyTranscriptReader, TranscriptReader(reader))

	out, err := tool.Handler(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if result["has_compact_summary"].(bool) != true {
		t.Error("expected has_compact_summary to be true")
	}
}

func TestContextStatusTool_LargeContext(t *testing.T) {
	t.Parallel()
	tool := contextStatusTool()

	// Create messages with enough content to trigger "critical" recommendation
	// Need > 100k estimated tokens. Each rune ~ 0.25 tokens, so ~400k+ runes
	bigContent := make([]byte, 200000) // 200k ASCII chars ~ 50k tokens each
	for i := range bigContent {
		bigContent[i] = 'a'
	}

	reader := contextStatusTestReader{
		runID: "test-run",
		messages: []TranscriptMessage{
			{Index: 0, Role: "user", Content: "Hello"},
			{Index: 1, Role: "tool", ToolCallID: "call_1", Content: string(bigContent)},
			{Index: 2, Role: "tool", ToolCallID: "call_2", Content: string(bigContent)},
			{Index: 3, Role: "tool", ToolCallID: "call_3", Content: string(bigContent)},
		},
	}
	ctx := context.WithValue(context.Background(), ContextKeyTranscriptReader, TranscriptReader(reader))

	out, err := tool.Handler(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	rec := result["recommendation"].(string)
	if rec != "critical: context is very large, compact immediately with mode=strip or mode=hybrid" {
		t.Errorf("expected critical recommendation, got: %s", rec)
	}
}

func TestContextRecommendation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		tokens      int
		msgCount    int
		toolResults int
		wantPrefix  string
	}{
		{"healthy", 1000, 5, 2, "healthy"},
		{"elevated_tokens", 35000, 10, 5, "elevated"},
		{"elevated_tools", 5000, 25, 25, "elevated"},
		{"warning", 65000, 20, 10, "warning"},
		{"critical", 110000, 30, 15, "critical"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := contextRecommendation(tt.tokens, tt.msgCount, tt.toolResults)
			if len(got) < len(tt.wantPrefix) || got[:len(tt.wantPrefix)] != tt.wantPrefix {
				t.Errorf("expected prefix %q, got %q", tt.wantPrefix, got)
			}
		})
	}
}
