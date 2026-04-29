package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-agent-harness/internal/harness"
)

// TestMapMessagesReasoningPassback verifies that mapMessages emits
// reasoning_content and reasoning_details on assistant turns when
// replayReasoning is true, and omits those fields when it is false.
//
// Scenario: two-turn tool-use conversation:
//  1. user question
//  2. assistant reply with reasoning + tool_call
//  3. tool result
//  4. final assistant answer (no reasoning)
func TestMapMessagesReasoningPassback(t *testing.T) {
	t.Parallel()

	const thinkingText = "I need to call the tool to get the answer."

	messages := []harness.Message{
		{Role: "user", Content: "What files are here?"},
		{
			Role:      "assistant",
			Reasoning: thinkingText,
			ToolCalls: []harness.ToolCall{
				{ID: "call-1", Name: "list_files", Arguments: `{"path":"."}`},
			},
		},
		{Role: "tool", Content: "file1.go\nfile2.go", ToolCallID: "call-1"},
		{Role: "assistant", Content: "There are two files: file1.go and file2.go."},
	}

	t.Run("quirk_active_emits_reasoning_fields", func(t *testing.T) {
		t.Parallel()

		mapped := mapMessages(messages, true /* replayReasoning */)
		if len(mapped) != 4 {
			t.Fatalf("expected 4 messages, got %d", len(mapped))
		}

		// Turn 2: assistant with tool_call — must carry reasoning.
		assistantWithTool := mapped[1]
		if assistantWithTool.ReasoningContent != thinkingText {
			t.Errorf("ReasoningContent = %q, want %q", assistantWithTool.ReasoningContent, thinkingText)
		}
		if len(assistantWithTool.ReasoningDetails) != 1 {
			t.Fatalf("len(ReasoningDetails) = %d, want 1", len(assistantWithTool.ReasoningDetails))
		}
		if assistantWithTool.ReasoningDetails[0].Type != "reasoning.text" {
			t.Errorf("ReasoningDetails[0].Type = %q, want %q", assistantWithTool.ReasoningDetails[0].Type, "reasoning.text")
		}
		if assistantWithTool.ReasoningDetails[0].Text != thinkingText {
			t.Errorf("ReasoningDetails[0].Text = %q, want %q", assistantWithTool.ReasoningDetails[0].Text, thinkingText)
		}

		// Turn 4: final assistant (no reasoning) — must NOT carry reasoning fields.
		finalAssistant := mapped[3]
		if finalAssistant.ReasoningContent != "" {
			t.Errorf("final assistant ReasoningContent should be empty, got %q", finalAssistant.ReasoningContent)
		}
		if len(finalAssistant.ReasoningDetails) != 0 {
			t.Errorf("final assistant ReasoningDetails should be empty, got %v", finalAssistant.ReasoningDetails)
		}

		// Verify JSON serialisation: reasoning_content and reasoning_details must
		// appear in the serialised assistant+tool_call message.
		raw, err := json.Marshal(assistantWithTool)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		serialised := string(raw)
		if !strings.Contains(serialised, `"reasoning_content"`) {
			t.Errorf("serialised JSON missing reasoning_content: %s", serialised)
		}
		if !strings.Contains(serialised, `"reasoning_details"`) {
			t.Errorf("serialised JSON missing reasoning_details: %s", serialised)
		}
		if !strings.Contains(serialised, `"reasoning.text"`) {
			t.Errorf("serialised JSON missing reasoning.text type value: %s", serialised)
		}

		// Tool result and user messages must never carry reasoning fields.
		toolMsg := mapped[2]
		rawTool, _ := json.Marshal(toolMsg)
		if strings.Contains(string(rawTool), "reasoning") {
			t.Errorf("tool message should not contain reasoning fields: %s", rawTool)
		}
	})

	t.Run("quirk_inactive_omits_reasoning_fields", func(t *testing.T) {
		t.Parallel()

		mapped := mapMessages(messages, false /* replayReasoning */)
		if len(mapped) != 4 {
			t.Fatalf("expected 4 messages, got %d", len(mapped))
		}

		for i, msg := range mapped {
			raw, err := json.Marshal(msg)
			if err != nil {
				t.Fatalf("marshal message %d: %v", i, err)
			}
			if strings.Contains(string(raw), "reasoning_content") {
				t.Errorf("message %d: reasoning_content should be absent when quirk is off: %s", i, raw)
			}
			if strings.Contains(string(raw), "reasoning_details") {
				t.Errorf("message %d: reasoning_details should be absent when quirk is off: %s", i, raw)
			}
		}
	})
}

// TestMapMessagesReasoningPassbackToolCallsPreserved verifies that enabling
// replayReasoning does not disturb the tool_calls field on the same message.
func TestMapMessagesReasoningPassbackToolCallsPreserved(t *testing.T) {
	t.Parallel()

	messages := []harness.Message{
		{
			Role:      "assistant",
			Reasoning: "thinking...",
			ToolCalls: []harness.ToolCall{
				{ID: "tc-1", Name: "run_bash", Arguments: `{"cmd":"ls"}`},
			},
		},
	}

	mapped := mapMessages(messages, true)
	if len(mapped) != 1 {
		t.Fatalf("expected 1 message, got %d", len(mapped))
	}
	msg := mapped[0]
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].ID != "tc-1" {
		t.Errorf("tool call ID = %q, want %q", msg.ToolCalls[0].ID, "tc-1")
	}
	if msg.ToolCalls[0].Function.Name != "run_bash" {
		t.Errorf("tool call Name = %q, want %q", msg.ToolCalls[0].Function.Name, "run_bash")
	}
	// Reasoning fields must also be set.
	if msg.ReasoningContent != "thinking..." {
		t.Errorf("ReasoningContent = %q, want %q", msg.ReasoningContent, "thinking...")
	}
}

// TestClientHonorsReasoningContentPassbackQuirk verifies end-to-end that when
// a Client is configured with the "reasoning_content_passback" quirk, the
// outgoing HTTP request body contains reasoning_content for a prior assistant
// turn that carries Reasoning text.
func TestClientHonorsReasoningContentPassbackQuirk(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"done","tool_calls":null}}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`))
	}))
	defer srv.Close()

	client, err := NewClient(Config{
		APIKey:       "test-key",
		BaseURL:      srv.URL,
		ProviderName: "deepseek",
		Quirks:       []string{"reasoning_content_passback"},
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = client.Complete(context.Background(), harness.CompletionRequest{
		Model: "deepseek-reasoner",
		Messages: []harness.Message{
			{Role: "user", Content: "call the tool"},
			{
				Role:      "assistant",
				Reasoning: "I will call list_files.",
				ToolCalls: []harness.ToolCall{
					{ID: "c1", Name: "list_files", Arguments: `{"path":"."}`},
				},
			},
			{Role: "tool", Content: "a.go\nb.go", ToolCallID: "c1"},
		},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	body := string(capturedBody)
	if !strings.Contains(body, `"reasoning_content"`) {
		t.Errorf("request body missing reasoning_content; body: %s", body)
	}
	if !strings.Contains(body, `"reasoning_details"`) {
		t.Errorf("request body missing reasoning_details; body: %s", body)
	}
	if !strings.Contains(body, "I will call list_files.") {
		t.Errorf("request body missing reasoning text; body: %s", body)
	}
}

// TestClientNoReasoningPassbackWithoutQuirk verifies that a client WITHOUT the
// quirk does NOT include reasoning fields in the request, even when messages
// carry Reasoning text.
func TestClientNoReasoningPassbackWithoutQuirk(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"done"}}],"usage":{"prompt_tokens":5,"completion_tokens":3,"total_tokens":8}}`))
	}))
	defer srv.Close()

	client, err := NewClient(Config{
		APIKey:       "test-key",
		BaseURL:      srv.URL,
		ProviderName: "openai",
		// No Quirks — reasoning passback must not fire.
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = client.Complete(context.Background(), harness.CompletionRequest{
		Model: "gpt-4.1",
		Messages: []harness.Message{
			{Role: "user", Content: "call the tool"},
			{
				Role:      "assistant",
				Reasoning: "some thinking",
				ToolCalls: []harness.ToolCall{
					{ID: "c2", Name: "run_bash", Arguments: `{}`},
				},
			},
			{Role: "tool", Content: "output", ToolCallID: "c2"},
		},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	body := string(capturedBody)
	if strings.Contains(body, `"reasoning_content"`) {
		t.Errorf("request body should NOT contain reasoning_content without quirk; body: %s", body)
	}
	if strings.Contains(body, `"reasoning_details"`) {
		t.Errorf("request body should NOT contain reasoning_details without quirk; body: %s", body)
	}
}
