package openai

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"

	"go-agent-harness/internal/harness"
)

// TestMapMessagesNonLeadingSystemSentAsUser is the issue #1395 regression:
// DeepSeek (via OpenRouter) returns an empty assistant response whenever the
// LAST message in the request has role "system". The harness places the
// per-turn <runtime_context> block last with role "system" (runner_step_engine.go
// buildTurnMessages) to keep the cacheable prefix unchanged, so the OpenAI-
// compatible mapper must rewrite any non-leading system message to role
// "user" on the wire while leaving the leading system message (and its
// content) untouched.
func TestMapMessagesNonLeadingSystemSentAsUser(t *testing.T) {
	t.Parallel()

	messages := []harness.Message{
		{Role: "system", Content: "You are a helpful agent."},
		{Role: "user", Content: "do the thing"},
		{Role: "assistant", Content: "", ToolCalls: []harness.ToolCall{{ID: "tc1", Name: "write", Arguments: "{}"}}},
		{Role: "tool", ToolCallID: "tc1", Content: "ok"},
		{Role: "system", Content: "<runtime_context>turn 2</runtime_context>"},
	}

	out := mapMessages(messages, false)
	if len(out) != 5 {
		t.Fatalf("len(out) = %d, want 5", len(out))
	}

	if out[0].Role != "system" {
		t.Errorf("leading message role = %q, want system (must stay system)", out[0].Role)
	}
	if s, ok := out[0].Content.(string); !ok || s != "You are a helpful agent." {
		t.Errorf("leading system content = %v, want unchanged", out[0].Content)
	}

	last := out[len(out)-1]
	if last.Role != "user" {
		t.Errorf("trailing runtime_context message role = %q, want user (this is the #1395 fix)", last.Role)
	}
	if s, ok := last.Content.(string); !ok || s != "<runtime_context>turn 2</runtime_context>" {
		t.Errorf("trailing message content = %v, want unchanged text", last.Content)
	}
}

// TestMapMessagesLeadingOnlySystemUnchanged guards the non-regression case:
// a request with only a single leading system message must not be altered.
func TestMapMessagesLeadingOnlySystemUnchanged(t *testing.T) {
	t.Parallel()

	messages := []harness.Message{
		{Role: "system", Content: "You are a helpful agent."},
		{Role: "user", Content: "hello"},
	}

	out := mapMessages(messages, false)
	if len(out) != 2 {
		t.Fatalf("len(out) = %d, want 2", len(out))
	}
	if out[0].Role != "system" {
		t.Errorf("out[0].Role = %q, want system", out[0].Role)
	}
	if out[1].Role != "user" {
		t.Errorf("out[1].Role = %q, want user", out[1].Role)
	}
}

// TestCompleteWireBodyTrailingSystemAsUser proves the fix end-to-end through
// Complete(): the JSON body sent to an OpenAI-compatible endpoint has the
// trailing runtime_context message on the wire as role "user", with the
// leading system message untouched.
func TestCompleteWireBodyTrailingSystemAsUser(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	var bodies atomic.Pointer[[]byte]
	srv := captureChatServer(t, &hits, &bodies)

	client, err := NewClient(Config{APIKey: "test-key", BaseURL: srv.URL, ProviderName: "openrouter"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = client.Complete(context.Background(), harness.CompletionRequest{
		Model: "deepseek/deepseek-v4-flash",
		Messages: []harness.Message{
			{Role: "system", Content: "You are a helpful agent."},
			{Role: "user", Content: "do the thing"},
			{Role: "system", Content: "<runtime_context>turn 1</runtime_context>"},
		},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if hits.Load() != 1 {
		t.Fatalf("server hit %d times, want 1", hits.Load())
	}

	var wire struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(*bodies.Load(), &wire); err != nil {
		t.Fatalf("unmarshal wire body: %v", err)
	}
	if len(wire.Messages) != 3 {
		t.Fatalf("wire messages len = %d, want 3", len(wire.Messages))
	}
	if wire.Messages[0].Role != "system" {
		t.Errorf("wire.Messages[0].Role = %q, want system", wire.Messages[0].Role)
	}
	last := wire.Messages[len(wire.Messages)-1]
	if last.Role != "user" {
		t.Errorf("wire.Messages[last].Role = %q, want user (trailing system must ride as user on the wire)", last.Role)
	}
	if last.Content != "<runtime_context>turn 1</runtime_context>" {
		t.Errorf("wire.Messages[last].Content = %q, want the runtime_context text unchanged", last.Content)
	}
}
