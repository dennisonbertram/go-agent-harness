package harness

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"go-agent-harness/internal/forensics/redaction"
)

// payloadInt extracts an integer value from a map[string]any payload. JSON-decoded
// numeric values arrive as float64; this helper handles both float64 and int.
func payloadInt(payload map[string]any, key string) int {
	switch v := payload[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}

// TestThinkingDeltaNotEmittedWhenCaptureDisabled verifies that when
// CaptureReasoning is false, streaming reasoning deltas are NOT emitted as
// assistant.thinking.delta events, preventing chain-of-thought leakage.
func TestThinkingDeltaNotEmittedWhenCaptureDisabled(t *testing.T) {
	t.Parallel()

	prov := &stubProvider{turns: []CompletionResult{
		{
			Content: "done",
			Deltas: []CompletionDelta{
				{Reasoning: "secret thought 1"},
				{Reasoning: "secret thought 2"},
				{Content: "done"},
			},
		},
	}}
	// CaptureReasoning not set — defaults to false.
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     2,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

	events := collectEvents(t, runner, run.ID)
	for _, ev := range events {
		if ev.Type == EventAssistantThinkingDelta {
			t.Errorf("unexpected %s event when CaptureReasoning=false: %v", EventAssistantThinkingDelta, ev.Payload)
		}
	}
}

// TestThinkingDeltaEmittedWhenCaptureEnabled verifies that when CaptureReasoning
// is true, streaming reasoning deltas are emitted as assistant.thinking.delta.
func TestThinkingDeltaEmittedWhenCaptureEnabled(t *testing.T) {
	t.Parallel()

	prov := &stubProvider{turns: []CompletionResult{
		{
			Content: "done",
			Deltas: []CompletionDelta{
				{Reasoning: "thought chunk"},
				{Content: "done"},
			},
		},
	}}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:     "test-model",
		MaxSteps:         2,
		CaptureReasoning: true,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

	events := collectEvents(t, runner, run.ID)
	var found bool
	for _, ev := range events {
		if ev.Type == EventAssistantThinkingDelta {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected assistant.thinking.delta event when CaptureReasoning=true with reasoning deltas")
	}
}

// TestReasoningCaptureDisabledByDefault verifies that when CaptureReasoning is
// false (default), no reasoning.complete event is emitted even if the provider
// returns ReasoningText.
func TestReasoningCaptureDisabledByDefault(t *testing.T) {
	t.Parallel()

	prov := &stubProvider{turns: []CompletionResult{
		{
			Content:         "done",
			ReasoningText:   "I thought about it carefully",
			ReasoningTokens: 10,
		},
	}}
	// CaptureReasoning not set — defaults to false
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     2,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

	events := collectEvents(t, runner, run.ID)
	for _, ev := range events {
		if ev.Type == EventReasoningComplete {
			t.Errorf("unexpected reasoning.complete event when CaptureReasoning=false: %v", ev)
		}
	}
}

// TestReasoningCaptureEnabled verifies that when CaptureReasoning is true and
// the provider returns ReasoningText, a reasoning.complete event is emitted with
// the correct text and token count.
func TestReasoningCaptureEnabled(t *testing.T) {
	t.Parallel()

	reasoningText := "I need to think carefully about this problem"
	reasoningTokens := 42

	prov := &stubProvider{turns: []CompletionResult{
		{
			Content:         "done",
			ReasoningText:   reasoningText,
			ReasoningTokens: reasoningTokens,
		},
	}}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:     "test-model",
		MaxSteps:         2,
		CaptureReasoning: true,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

	events := collectEvents(t, runner, run.ID)
	var found bool
	for _, ev := range events {
		if ev.Type != EventReasoningComplete {
			continue
		}
		found = true
		text, _ := ev.Payload["text"].(string)
		if text != reasoningText {
			t.Errorf("reasoning text mismatch: got %q, want %q", text, reasoningText)
		}
		gotTokens := payloadInt(ev.Payload, "tokens")
		if gotTokens != reasoningTokens {
			t.Errorf("reasoning tokens mismatch: got %v, want %d", gotTokens, reasoningTokens)
		}
		if _, ok := ev.Payload["step"]; !ok {
			t.Error("reasoning.complete event missing 'step' field")
		}
	}
	if !found {
		t.Error("no reasoning.complete event found when CaptureReasoning=true")
	}
}

// TestReasoningNotEmittedWhenEmpty verifies that no reasoning.complete event is
// emitted when CaptureReasoning=true but the provider returns empty ReasoningText.
func TestReasoningNotEmittedWhenEmpty(t *testing.T) {
	t.Parallel()

	prov := &stubProvider{turns: []CompletionResult{
		{
			Content:         "done",
			ReasoningText:   "", // empty reasoning
			ReasoningTokens: 0,
		},
	}}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:     "test-model",
		MaxSteps:         2,
		CaptureReasoning: true,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

	events := collectEvents(t, runner, run.ID)
	for _, ev := range events {
		if ev.Type == EventReasoningComplete {
			t.Errorf("unexpected reasoning.complete event when reasoning text is empty")
		}
	}
}

// TestReasoningStoredInMessageReasoning verifies that when CaptureReasoning is
// true, the assistant Message is extended with the Reasoning field populated.
func TestReasoningStoredInMessageReasoning(t *testing.T) {
	t.Parallel()

	reasoningText := "Let me think step by step"

	prov := &stubProvider{turns: []CompletionResult{
		{
			Content:         "final answer",
			ReasoningText:   reasoningText,
			ReasoningTokens: 20,
		},
	}}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:     "test-model",
		MaxSteps:         2,
		CaptureReasoning: true,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

	msgs := runner.GetRunMessages(run.ID)
	var foundAssistant bool
	for _, msg := range msgs {
		if msg.Role == "assistant" {
			foundAssistant = true
			if msg.Reasoning != reasoningText {
				t.Errorf("Message.Reasoning mismatch: got %q, want %q", msg.Reasoning, reasoningText)
			}
		}
	}
	if !foundAssistant {
		t.Error("no assistant message found in conversation")
	}
}

// TestReasoningMessageJSONRoundTrip verifies that Message.Reasoning is included
// in JSON serialization with the correct tag (reasoning,omitempty).
func TestReasoningMessageJSONRoundTrip(t *testing.T) {
	t.Parallel()

	msg := Message{
		Role:      "assistant",
		Content:   "hello",
		Reasoning: "I was thinking about this",
	}
	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var out Message
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Reasoning != msg.Reasoning {
		t.Errorf("Reasoning mismatch after round-trip: got %q, want %q", out.Reasoning, msg.Reasoning)
	}

	// Verify omitempty: a message without reasoning should not have the field
	emptyMsg := Message{Role: "user", Content: "hi"}
	bEmpty, _ := json.Marshal(emptyMsg)
	if strings.Contains(string(bEmpty), `"reasoning"`) {
		t.Errorf("expected reasoning field to be omitted when empty, got: %s", string(bEmpty))
	}
}

// TestReasoningRedactionApplied verifies that when a RedactionPipeline is set,
// it is applied to the reasoning text before emission and storage.
func TestReasoningRedactionApplied(t *testing.T) {
	t.Parallel()

	// The reasoning text contains a secret that should be redacted.
	reasoningText := "I checked the DB at postgres://user:secret@localhost:5432/prod and it worked"

	pipeline := redaction.NewPipeline(redaction.NewRedactor(nil), redaction.EventClassConfig{})

	prov := &stubProvider{turns: []CompletionResult{
		{
			Content:         "done",
			ReasoningText:   reasoningText,
			ReasoningTokens: 15,
		},
	}}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:      "test-model",
		MaxSteps:          2,
		CaptureReasoning:  true,
		RedactionPipeline: pipeline,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

	events := collectEvents(t, runner, run.ID)
	var found bool
	for _, ev := range events {
		if ev.Type != EventReasoningComplete {
			continue
		}
		found = true
		text, _ := ev.Payload["text"].(string)
		if strings.Contains(text, "postgres://") {
			t.Errorf("reasoning event text contains unredacted secret: %q", text)
		}
		if !strings.Contains(text, "[REDACTED:connection_string]") {
			t.Errorf("reasoning event text missing redaction marker: %q", text)
		}
	}
	if !found {
		t.Error("no reasoning.complete event found")
	}

	// Also verify the stored message has redacted reasoning.
	msgs := runner.GetRunMessages(run.ID)
	for _, msg := range msgs {
		if msg.Role == "assistant" && msg.Reasoning != "" {
			if strings.Contains(msg.Reasoning, "postgres://") {
				t.Errorf("Message.Reasoning contains unredacted secret: %q", msg.Reasoning)
			}
		}
	}
}

// TestReasoningMultipleSteps verifies that reasoning.complete events are emitted
// per-step when CaptureReasoning=true and multiple turns have reasoning.
func TestReasoningMultipleSteps(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	_ = registry.Register(ToolDefinition{
		Name:        "noop_reasoning",
		Description: "does nothing",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		return "ok", nil
	})

	// Two turns: first has a tool call + reasoning, second has final answer + reasoning.
	prov := &stubProvider{turns: []CompletionResult{
		{
			Content: "",
			ToolCalls: []ToolCall{
				{ID: "call_1", Name: "noop_reasoning", Arguments: `{}`},
			},
			ReasoningText:   "thinking for step 1",
			ReasoningTokens: 10,
		},
		{
			Content:         "all done",
			ReasoningText:   "thinking for step 2",
			ReasoningTokens: 8,
		},
	}}

	runner := NewRunner(prov, registry, RunnerConfig{
		DefaultModel:     "test-model",
		MaxSteps:         5,
		CaptureReasoning: true,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

	events := collectEvents(t, runner, run.ID)
	var reasoningEvents []Event
	for _, ev := range events {
		if ev.Type == EventReasoningComplete {
			reasoningEvents = append(reasoningEvents, ev)
		}
	}
	if len(reasoningEvents) != 2 {
		t.Errorf("expected 2 reasoning.complete events, got %d", len(reasoningEvents))
	}

	// Check they have different step numbers.
	if len(reasoningEvents) == 2 {
		step1 := payloadInt(reasoningEvents[0].Payload, "step")
		step2 := payloadInt(reasoningEvents[1].Payload, "step")
		if step1 == step2 {
			t.Errorf("both reasoning events have same step %v, expected different steps", step1)
		}
	}
}

// TestCompletionResultReasoningFields verifies that CompletionResult has the
// new ReasoningText and ReasoningTokens fields.
func TestCompletionResultReasoningFields(t *testing.T) {
	t.Parallel()

	result := CompletionResult{
		Content:         "hello",
		ReasoningText:   "I thought about it",
		ReasoningTokens: 25,
	}
	if result.ReasoningText != "I thought about it" {
		t.Errorf("unexpected ReasoningText: %q", result.ReasoningText)
	}
	if result.ReasoningTokens != 25 {
		t.Errorf("unexpected ReasoningTokens: %d", result.ReasoningTokens)
	}
}

// TestRunnerConfigCaptureReasoningDefault verifies that CaptureReasoning
// defaults to false (zero value).
func TestRunnerConfigCaptureReasoningDefault(t *testing.T) {
	t.Parallel()

	cfg := RunnerConfig{}
	if cfg.CaptureReasoning {
		t.Error("expected CaptureReasoning to default to false")
	}
}

// TestReasoningCaptureEnabledWithToolCalls verifies that reasoning is captured
// even when the LLM response includes tool calls (not just final answers).
func TestReasoningCaptureEnabledWithToolCalls(t *testing.T) {
	t.Parallel()

	reasoningText := "I will call a tool to help me"

	registry := NewRegistry()
	_ = registry.Register(ToolDefinition{
		Name:        "tool_with_reasoning",
		Description: "a test tool",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		return "result", nil
	})

	prov := &stubProvider{turns: []CompletionResult{
		{
			Content: "",
			ToolCalls: []ToolCall{
				{ID: "call_x", Name: "tool_with_reasoning", Arguments: `{}`},
			},
			ReasoningText:   reasoningText,
			ReasoningTokens: 5,
		},
		{Content: "done"},
	}}
	runner := NewRunner(prov, registry, RunnerConfig{
		DefaultModel:     "test-model",
		MaxSteps:         5,
		CaptureReasoning: true,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

	events := collectEvents(t, runner, run.ID)
	var found bool
	for _, ev := range events {
		if ev.Type == EventReasoningComplete {
			found = true
			text, _ := ev.Payload["text"].(string)
			if text != reasoningText {
				t.Errorf("reasoning text mismatch: got %q, want %q", text, reasoningText)
			}
			break
		}
	}
	if !found {
		t.Error("no reasoning.complete event found when CaptureReasoning=true with tool calls")
	}
}
