package replay

import (
	"testing"

	"go-agent-harness/internal/forensics/rollout"
)

func TestFork_BasicFlow(t *testing.T) {
	events := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0, Payload: map[string]any{
			"prompt": "hello", "system_prompt": "You are helpful",
		}},
		{Type: "llm.turn.completed", Step: 1, Payload: map[string]any{
			"content": "I'll run bash",
			"tool_calls": []any{
				map[string]any{"id": "c1", "name": "bash", "arguments": `{"cmd":"ls"}`},
			},
		}},
		{Type: "tool.call.completed", Step: 1, Payload: map[string]any{
			"call_id": "c1", "tool": "bash", "result": "file1.go",
		}},
		{Type: "llm.turn.completed", Step: 2, Payload: map[string]any{
			"content": "Here's the listing",
		}},
		{Type: "run.completed", Step: 3, Payload: map[string]any{"output": "done"}},
	}

	// Fork from step 1 — should include system, user, assistant+tool_call, tool result.
	result, err := Fork(events, 1, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.FromStep != 1 {
		t.Errorf("expected FromStep=1, got %d", result.FromStep)
	}
	if result.OriginalStepCount != 3 {
		t.Errorf("expected OriginalStepCount=3, got %d", result.OriginalStepCount)
	}
	if result.OriginalOutcome != "completed" {
		t.Errorf("expected outcome=completed, got %s", result.OriginalOutcome)
	}

	// Messages should be: user, assistant, tool (system prompt stripped by default).
	if len(result.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result.Messages))
	}
	if result.Messages[0].Role != "user" {
		t.Errorf("expected user role, got %s", result.Messages[0].Role)
	}
	if result.Messages[1].Role != "assistant" {
		t.Errorf("expected assistant role, got %s", result.Messages[1].Role)
	}
	if result.Messages[2].Role != "tool" {
		t.Errorf("expected tool role, got %s", result.Messages[2].Role)
	}
	if !result.SystemPromptStripped {
		t.Error("expected SystemPromptStripped=true")
	}
}

func TestFork_FromStepZero(t *testing.T) {
	events := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0, Payload: map[string]any{"prompt": "hello"}},
		{Type: "llm.turn.completed", Step: 1, Payload: map[string]any{"content": "hi"}},
		{Type: "run.completed", Step: 2},
	}

	result, err := Fork(events, 0, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only have the user message from run.started.
	if len(result.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result.Messages))
	}
	if result.Messages[0].Role != "user" || result.Messages[0].Content != "hello" {
		t.Errorf("expected user prompt, got %+v", result.Messages[0])
	}
}

func TestFork_NegativeStep(t *testing.T) {
	events := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0},
	}

	_, err := Fork(events, -1, nil)
	if err == nil {
		t.Fatal("expected error for negative fromStep")
	}
}

func TestFork_StepExceedsMax(t *testing.T) {
	events := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0},
		{Type: "run.completed", Step: 2},
	}

	_, err := Fork(events, 5, nil)
	if err == nil {
		t.Fatal("expected error for fromStep exceeding max")
	}
}

func TestFork_EmptyRollout(t *testing.T) {
	_, err := Fork(nil, 0, nil)
	if err == nil {
		t.Fatal("expected error for empty rollout")
	}
}

func TestFork_FailedRun(t *testing.T) {
	events := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0, Payload: map[string]any{"prompt": "hello"}},
		{Type: "llm.turn.completed", Step: 1, Payload: map[string]any{"content": "working..."}},
		{Type: "run.failed", Step: 2, Payload: map[string]any{"error": "timeout"}},
	}

	result, err := Fork(events, 1, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.OriginalOutcome != "failed" {
		t.Errorf("expected outcome=failed, got %s", result.OriginalOutcome)
	}

	// Should include messages up to step 1.
	if len(result.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result.Messages))
	}
}

func TestFork_UnknownOutcome(t *testing.T) {
	events := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0, Payload: map[string]any{"prompt": "hello"}},
		{Type: "llm.turn.completed", Step: 1, Payload: map[string]any{"content": "thinking..."}},
	}

	result, err := Fork(events, 1, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.OriginalOutcome != "unknown" {
		t.Errorf("expected outcome=unknown, got %s", result.OriginalOutcome)
	}
}

func TestFork_FullRun(t *testing.T) {
	events := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0, Payload: map[string]any{
			"prompt": "fix the bug", "system_prompt": "expert dev",
		}},
		{Type: "llm.turn.completed", Step: 1, Payload: map[string]any{
			"content": "Let me look",
			"tool_calls": []any{
				map[string]any{"id": "c1", "name": "read_file", "arguments": `{"path":"main.go"}`},
			},
		}},
		{Type: "tool.call.completed", Step: 1, Payload: map[string]any{
			"call_id": "c1", "tool": "read_file", "result": "package main\nfunc main() {}",
		}},
		{Type: "llm.turn.completed", Step: 2, Payload: map[string]any{
			"content": "I see the issue",
			"tool_calls": []any{
				map[string]any{"id": "c2", "name": "write_file", "arguments": `{"path":"main.go","content":"fixed"}`},
			},
		}},
		{Type: "tool.call.completed", Step: 2, Payload: map[string]any{
			"call_id": "c2", "tool": "write_file", "result": "ok",
		}},
		{Type: "llm.turn.completed", Step: 3, Payload: map[string]any{
			"content": "Fixed!",
		}},
		{Type: "run.completed", Step: 4},
	}

	// Fork at step 2 — should include everything up to and including step 2.
	result, err := Fork(events, 2, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected: user, assistant+tc, tool, assistant+tc, tool = 5 messages
	// (system prompt stripped by default for untrusted rollouts)
	if len(result.Messages) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(result.Messages))
	}

	// Verify message roles.
	expectedRoles := []string{"user", "assistant", "tool", "assistant", "tool"}
	for i, msg := range result.Messages {
		if msg.Role != expectedRoles[i] {
			t.Errorf("msg %d: expected role %s, got %s", i, expectedRoles[i], msg.Role)
		}
	}

	if result.OriginalStepCount != 4 {
		t.Errorf("expected OriginalStepCount=4, got %d", result.OriginalStepCount)
	}
}

func TestFork_AtMaxStep(t *testing.T) {
	events := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0, Payload: map[string]any{"prompt": "hi"}},
		{Type: "run.completed", Step: 2},
	}

	result, err := Fork(events, 2, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FromStep != 2 {
		t.Errorf("expected FromStep=2, got %d", result.FromStep)
	}
}
