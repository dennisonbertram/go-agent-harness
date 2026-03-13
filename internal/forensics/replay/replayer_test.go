package replay

import (
	"strings"
	"testing"

	"go-agent-harness/internal/forensics/rollout"
)

func TestReplay_BasicFlow(t *testing.T) {
	events := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0, Payload: map[string]any{"prompt": "hello"}},
		{Type: "llm.turn.completed", Step: 1, Payload: map[string]any{
			"content": "I'll run a command",
			"tool_calls": []any{
				map[string]any{"id": "call_1", "name": "bash", "arguments": `{"cmd":"ls"}`},
			},
		}},
		{Type: "tool.call.started", Step: 1, Payload: map[string]any{
			"call_id": "call_1", "tool": "bash", "arguments": `{"cmd":"ls"}`,
		}},
		{Type: "tool.call.completed", Step: 1, Payload: map[string]any{
			"call_id": "call_1", "tool": "bash", "result": "file1.go\nfile2.go",
		}},
		{Type: "llm.turn.completed", Step: 2, Payload: map[string]any{
			"content": "Here are the files",
		}},
		{Type: "run.completed", Step: 3, Payload: map[string]any{"output": "done"}},
	}

	result := Replay(events)

	if result.StepCount != 3 {
		t.Errorf("expected step count 3, got %d", result.StepCount)
	}
	if !result.Matched {
		t.Errorf("expected matched=true, got false; mismatches: %v", result.Mismatches)
	}
	if len(result.Events) != len(events) {
		t.Errorf("expected %d replay events, got %d", len(events), len(result.Events))
	}
}

func TestReplay_ToolCallWithRecordedResult(t *testing.T) {
	events := []rollout.RolloutEvent{
		{Type: "llm.turn.completed", Step: 1, Payload: map[string]any{
			"content": "reading file",
			"tool_calls": []any{
				map[string]any{"id": "call_1", "name": "read_file", "arguments": `{"path":"/tmp/x"}`},
			},
		}},
		{Type: "tool.call.started", Step: 1, Payload: map[string]any{
			"call_id": "call_1", "tool": "read_file", "arguments": `{"path":"/tmp/x"}`,
		}},
		{Type: "tool.call.completed", Step: 1, Payload: map[string]any{
			"call_id": "call_1", "tool": "read_file", "result": "file contents here",
		}},
	}

	result := Replay(events)

	// The tool.call.started event is at index 1 (after llm.turn.completed).
	startEvent := result.Events[1]
	if startEvent.Details["result"] != "file contents here" {
		t.Errorf("expected recorded result, got %v", startEvent.Details["result"])
	}
	if !startEvent.Matched {
		t.Errorf("expected matched=true for tool call with completion, mismatches: %v", result.Mismatches)
	}
}

func TestReplay_MissingCompletion(t *testing.T) {
	events := []rollout.RolloutEvent{
		{Type: "llm.turn.completed", Step: 1, Payload: map[string]any{
			"content": "running bash",
			"tool_calls": []any{
				map[string]any{"id": "call_orphan", "name": "bash"},
			},
		}},
		{Type: "tool.call.started", Step: 1, Payload: map[string]any{
			"call_id": "call_orphan", "tool": "bash",
		}},
		{Type: "run.failed", Step: 2, Payload: map[string]any{"error": "timeout"}},
	}

	result := Replay(events)

	if result.Matched {
		t.Error("expected matched=false for missing completion")
	}
	if len(result.Mismatches) == 0 {
		t.Error("expected at least one mismatch")
	}
	found := false
	for _, m := range result.Mismatches {
		if strings.Contains(m, "call_orphan") {
			found = true
		}
	}
	if !found {
		t.Error("expected mismatch to mention call_orphan")
	}
}

func TestReplay_EmptyEvents(t *testing.T) {
	result := Replay(nil)

	if result.StepCount != 0 {
		t.Errorf("expected step count 0, got %d", result.StepCount)
	}
	if !result.Matched {
		t.Error("expected matched=true for empty events")
	}
	if len(result.Events) != 0 {
		t.Errorf("expected 0 events, got %d", len(result.Events))
	}
}

func TestReplay_MultipleToolCalls(t *testing.T) {
	events := []rollout.RolloutEvent{
		{Type: "llm.turn.completed", Step: 1, Payload: map[string]any{
			"content": "running c1",
			"tool_calls": []any{
				map[string]any{"id": "c1", "name": "bash"},
			},
		}},
		{Type: "tool.call.started", Step: 1, Payload: map[string]any{
			"call_id": "c1", "tool": "bash",
		}},
		{Type: "tool.call.completed", Step: 1, Payload: map[string]any{
			"call_id": "c1", "tool": "bash", "result": "result_1",
		}},
		{Type: "llm.turn.completed", Step: 2, Payload: map[string]any{
			"content": "running c2",
			"tool_calls": []any{
				map[string]any{"id": "c2", "name": "read_file"},
			},
		}},
		{Type: "tool.call.started", Step: 2, Payload: map[string]any{
			"call_id": "c2", "tool": "read_file",
		}},
		{Type: "tool.call.completed", Step: 2, Payload: map[string]any{
			"call_id": "c2", "tool": "read_file", "result": "result_2",
		}},
	}

	result := Replay(events)

	if !result.Matched {
		t.Errorf("expected matched, mismatches: %v", result.Mismatches)
	}

	// Each started event should have its recorded result.
	// Indices shift due to inserted llm.turn.completed events:
	// [0]=llm.turn(c1), [1]=started(c1), [2]=completed(c1),
	// [3]=llm.turn(c2), [4]=started(c2), [5]=completed(c2)
	if result.Events[1].Details["result"] != "result_1" {
		t.Errorf("expected result_1, got %v", result.Events[1].Details["result"])
	}
	if result.Events[4].Details["result"] != "result_2" {
		t.Errorf("expected result_2, got %v", result.Events[4].Details["result"])
	}
}

func TestReplay_NoCallID(t *testing.T) {
	// A tool.call.started event without a call_id is a schema violation and
	// must be flagged as a mismatch — silent omission would bypass integrity checks.
	events := []rollout.RolloutEvent{
		{Type: "tool.call.started", Step: 1, Payload: map[string]any{
			"tool": "bash",
		}},
	}

	result := Replay(events)

	if result.Matched {
		t.Error("expected mismatch for missing call_id in tool.call.started")
	}
	if len(result.Mismatches) == 0 {
		t.Error("expected at least one mismatch entry for missing call_id")
	}
}

func TestReconstructMessages_BasicFlow(t *testing.T) {
	events := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0, Payload: map[string]any{
			"prompt": "hello world", "system_prompt": "You are helpful",
		}},
		{Type: "llm.turn.completed", Step: 1, Payload: map[string]any{
			"content": "I'll help you",
			"tool_calls": []any{
				map[string]any{"id": "c1", "name": "bash", "arguments": `{"cmd":"echo hi"}`},
			},
		}},
		{Type: "tool.call.started", Step: 1, Payload: map[string]any{
			"call_id": "c1", "tool": "bash",
		}},
		{Type: "tool.call.completed", Step: 1, Payload: map[string]any{
			"call_id": "c1", "tool": "bash", "result": "hi",
		}},
		{Type: "llm.turn.completed", Step: 2, Payload: map[string]any{
			"content": "Done!",
		}},
		{Type: "run.completed", Step: 3},
	}

	msgs := ReconstructMessages(events, 3)

	if len(msgs) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(msgs))
	}

	// system prompt
	if msgs[0].Role != "system" || msgs[0].Content != "You are helpful" {
		t.Errorf("msg 0: expected system message, got %+v", msgs[0])
	}
	// user prompt
	if msgs[1].Role != "user" || msgs[1].Content != "hello world" {
		t.Errorf("msg 1: expected user message, got %+v", msgs[1])
	}
	// assistant with tool calls
	if msgs[2].Role != "assistant" || len(msgs[2].ToolCalls) != 1 {
		t.Errorf("msg 2: expected assistant with tool calls, got %+v", msgs[2])
	}
	if msgs[2].ToolCalls[0].ID != "c1" || msgs[2].ToolCalls[0].Name != "bash" {
		t.Errorf("msg 2 tool call: expected c1/bash, got %+v", msgs[2].ToolCalls[0])
	}
	// tool result
	if msgs[3].Role != "tool" || msgs[3].ToolCallID != "c1" || msgs[3].Content != "hi" {
		t.Errorf("msg 3: expected tool result, got %+v", msgs[3])
	}
	// final assistant message
	if msgs[4].Role != "assistant" || msgs[4].Content != "Done!" {
		t.Errorf("msg 4: expected assistant Done!, got %+v", msgs[4])
	}
}

func TestReconstructMessages_UpToStep(t *testing.T) {
	events := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0, Payload: map[string]any{"prompt": "hi"}},
		{Type: "llm.turn.completed", Step: 1, Payload: map[string]any{"content": "step 1"}},
		{Type: "llm.turn.completed", Step: 2, Payload: map[string]any{"content": "step 2"}},
		{Type: "llm.turn.completed", Step: 3, Payload: map[string]any{"content": "step 3"}},
	}

	msgs := ReconstructMessages(events, 1)

	// Should include: user prompt + step 1 assistant
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages up to step 1, got %d", len(msgs))
	}
	if msgs[1].Content != "step 1" {
		t.Errorf("expected step 1 content, got %s", msgs[1].Content)
	}
}

func TestReconstructMessages_SteeringMessage(t *testing.T) {
	events := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0, Payload: map[string]any{"prompt": "start"}},
		{Type: "llm.turn.completed", Step: 1, Payload: map[string]any{"content": "working"}},
		{Type: "steering.received", Step: 2, Payload: map[string]any{"content": "change direction"}},
		{Type: "llm.turn.completed", Step: 3, Payload: map[string]any{"content": "changed"}},
	}

	msgs := ReconstructMessages(events, 3)

	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(msgs))
	}
	if msgs[2].Role != "user" || msgs[2].Content != "change direction" {
		t.Errorf("msg 2: expected steering as user message, got %+v", msgs[2])
	}
}

func TestReconstructMessages_ContinuedConversation(t *testing.T) {
	events := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0, Payload: map[string]any{"prompt": "start"}},
		{Type: "llm.turn.completed", Step: 1, Payload: map[string]any{"content": "done"}},
		{Type: "conversation.continued", Step: 2, Payload: map[string]any{"message": "follow up"}},
		{Type: "llm.turn.completed", Step: 3, Payload: map[string]any{"content": "more done"}},
	}

	msgs := ReconstructMessages(events, 3)

	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(msgs))
	}
	if msgs[2].Role != "user" || msgs[2].Content != "follow up" {
		t.Errorf("msg 2: expected conversation.continued as user message, got %+v", msgs[2])
	}
}

func TestReconstructMessages_NoSystemPrompt(t *testing.T) {
	events := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0, Payload: map[string]any{"prompt": "just user"}},
	}

	msgs := ReconstructMessages(events, 0)

	if len(msgs) != 1 {
		t.Fatalf("expected 1 message (user only), got %d", len(msgs))
	}
	if msgs[0].Role != "user" {
		t.Errorf("expected user role, got %s", msgs[0].Role)
	}
}

func TestReconstructMessages_EmptyEvents(t *testing.T) {
	msgs := ReconstructMessages(nil, 0)
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}
}

func TestIndexToolCompletions(t *testing.T) {
	events := []rollout.RolloutEvent{
		{Type: "tool.call.completed", Payload: map[string]any{"call_id": "c1", "tool": "bash", "result": "r1"}},
		{Type: "tool.call.completed", Payload: map[string]any{"call_id": "c2", "tool": "read_file", "result": "r2"}},
		{Type: "run.completed"},
	}

	idx := indexToolCompletions(events)
	if len(idx.entries) != 2 {
		t.Fatalf("expected 2 completions, got %d", len(idx.entries))
	}
	if idx.entries["c1"].result != "r1" {
		t.Errorf("expected r1 for c1, got %s", idx.entries["c1"].result)
	}
	if idx.entries["c2"].result != "r2" {
		t.Errorf("expected r2 for c2, got %s", idx.entries["c2"].result)
	}
	// Verify file-order indices.
	if idx.entries["c1"].fileIndex != 0 {
		t.Errorf("expected fileIndex=0 for c1, got %d", idx.entries["c1"].fileIndex)
	}
	if idx.entries["c2"].fileIndex != 1 {
		t.Errorf("expected fileIndex=1 for c2, got %d", idx.entries["c2"].fileIndex)
	}
	// Verify tool names are stored.
	if idx.entries["c1"].toolName != "bash" {
		t.Errorf("expected toolName=bash for c1, got %s", idx.entries["c1"].toolName)
	}
	if idx.entries["c2"].toolName != "read_file" {
		t.Errorf("expected toolName=read_file for c2, got %s", idx.entries["c2"].toolName)
	}
}

func TestReplay_UnannouncedToolCallRejected(t *testing.T) {
	// A rollout with tool.call.started + tool.call.completed but no announcing
	// llm.turn.completed is an integrity violation — the call was fabricated.
	// Replay must flag this even though lifecycle ordering is correct.
	events := []rollout.RolloutEvent{
		{Type: "tool.call.started", Step: 1, Payload: map[string]any{
			"call_id": "c1", "tool": "bash",
		}},
		{Type: "tool.call.completed", Step: 1, Payload: map[string]any{
			"call_id": "c1", "tool": "bash", "result": "injected",
		}},
	}

	result := Replay(events)

	if result.Matched {
		t.Error("expected mismatch for unannounced tool call")
	}
	found := false
	for _, m := range result.Mismatches {
		if strings.Contains(m, "announced") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'announced' in mismatch, got: %v", result.Mismatches)
	}
}

func TestReplay_CompletionWithoutStarted(t *testing.T) {
	// A crafted rollout with llm.turn.completed + tool.call.completed but
	// no tool.call.started must be flagged as a mismatch. Without this check,
	// Replay() would produce Matched=true while ReconstructMessages() would
	// inject the fabricated tool result (since the call was announced).
	events := []rollout.RolloutEvent{
		{Type: "llm.turn.completed", Step: 1, Payload: map[string]any{
			"content": "running bash",
			"tool_calls": []any{
				map[string]any{"id": "c1", "name": "bash"},
			},
		}},
		// No tool.call.started — only a direct completion.
		{Type: "tool.call.completed", Step: 1, Payload: map[string]any{
			"call_id": "c1", "tool": "bash", "result": "injected",
		}},
	}

	result := Replay(events)

	if result.Matched {
		t.Error("expected mismatch when tool.call.completed has no corresponding tool.call.started")
	}
	found := false
	for _, m := range result.Mismatches {
		if strings.Contains(m, "no corresponding tool.call.started") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'no corresponding tool.call.started' in mismatches, got: %v", result.Mismatches)
	}
}

func TestReconstructMessages_CompletionWithoutStarted(t *testing.T) {
	// A tool.call.completed without a prior tool.call.started must NOT be
	// included in the reconstructed messages — even if the call_id was announced.
	events := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0, Payload: map[string]any{"prompt": "hi"}},
		{Type: "llm.turn.completed", Step: 1, Payload: map[string]any{
			"content": "calling bash",
			"tool_calls": []any{
				map[string]any{"id": "c1", "name": "bash"},
			},
		}},
		// No tool.call.started — only a direct completion.
		{Type: "tool.call.completed", Step: 1, Payload: map[string]any{
			"call_id": "c1", "tool": "bash", "result": "injected result",
		}},
	}

	msgs := ReconstructMessages(events, 2)
	// Expected: user + assistant. The tool message must NOT be included.
	for _, m := range msgs {
		if m.Role == "tool" {
			t.Errorf("expected no tool message, but got one: %+v", m)
		}
	}
}

func TestReplay_ToolNameAbsentInCompletion(t *testing.T) {
	// An attacker can strip the "tool" field from tool.call.completed to bypass
	// the name-consistency check. If the started event declares a tool name,
	// a completion without a tool name must still be flagged as a mismatch.
	events := []rollout.RolloutEvent{
		{Type: "llm.turn.completed", Step: 1, Payload: map[string]any{
			"content": "running bash",
			"tool_calls": []any{
				map[string]any{"id": "c1", "name": "bash"},
			},
		}},
		{Type: "tool.call.started", Step: 1, Payload: map[string]any{
			"call_id": "c1", "tool": "bash",
		}},
		{Type: "tool.call.completed", Step: 1, Payload: map[string]any{
			"call_id": "c1", "result": "ok", // no "tool" field — bypass attempt
		}},
	}

	result := Replay(events)

	if result.Matched {
		t.Error("expected mismatch when completion omits tool name but started declares one")
	}
	found := false
	for _, m := range result.Mismatches {
		if strings.Contains(m, "mismatch") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'mismatch' in messages, got: %v", result.Mismatches)
	}
}

func TestReplay_ToolNameMismatch(t *testing.T) {
	// An attacker can craft a rollout where tool.call.completed has a different
	// tool name than tool.call.started (same call_id). This splices a result
	// from one tool into a different tool's replay record.
	events := []rollout.RolloutEvent{
		{Type: "llm.turn.completed", Step: 1, Payload: map[string]any{
			"content": "reading file",
			"tool_calls": []any{
				map[string]any{"id": "c1", "name": "read_file"},
			},
		}},
		{Type: "tool.call.started", Step: 1, Payload: map[string]any{
			"call_id": "c1", "tool": "read_file",
		}},
		{Type: "tool.call.completed", Step: 1, Payload: map[string]any{
			"call_id": "c1", "tool": "bash", "result": "spliced result",
		}},
	}

	result := Replay(events)

	if result.Matched {
		t.Error("expected mismatch for tool name mismatch between start and completion")
	}
	found := false
	for _, m := range result.Mismatches {
		if strings.Contains(m, "mismatch") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'mismatch' in mismatch messages, got: %v", result.Mismatches)
	}
}

func TestReplay_CompletionBeforeStartedRejected(t *testing.T) {
	// An attacker can place tool.call.completed before tool.call.started
	// in file order at the same step — the monotonic step check in the loader
	// is satisfied because steps are equal. Replay must detect this lifecycle
	// inversion and flag it as a mismatch, preventing fabricated tool results
	// from being injected via out-of-order completion events.
	// The llm.turn.completed is included so the announcement check passes
	// and only the lifecycle ordering violation is flagged.
	events := []rollout.RolloutEvent{
		{Type: "llm.turn.completed", Step: 1, Payload: map[string]any{
			"content": "running c1",
			"tool_calls": []any{
				map[string]any{"id": "c1", "name": "bash"},
			},
		}},
		{Type: "tool.call.completed", Step: 1, Payload: map[string]any{
			"call_id": "c1", "tool": "bash", "result": "injected",
		}},
		{Type: "tool.call.started", Step: 1, Payload: map[string]any{
			"call_id": "c1", "tool": "bash",
		}},
	}

	result := Replay(events)

	if result.Matched {
		t.Error("expected mismatch for completion-before-started in file order")
	}
	found := false
	for _, m := range result.Mismatches {
		if strings.Contains(m, "before") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'before' in mismatch message, got: %v", result.Mismatches)
	}
}

func TestPayloadString(t *testing.T) {
	tests := []struct {
		name     string
		payload  map[string]any
		key      string
		expected string
		ok       bool
	}{
		{"found", map[string]any{"key": "value"}, "key", "value", true},
		{"not_found", map[string]any{"key": "value"}, "other", "", false},
		{"nil_payload", nil, "key", "", false},
		{"non_string", map[string]any{"key": 42}, "key", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := payloadString(tt.payload, tt.key)
			if got != tt.expected || ok != tt.ok {
				t.Errorf("expected (%q, %v), got (%q, %v)", tt.expected, tt.ok, got, ok)
			}
		})
	}
}

func TestCopyPayload(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if copyPayload(nil) != nil {
			t.Error("expected nil for nil input")
		}
	})
	t.Run("copy", func(t *testing.T) {
		original := map[string]any{"a": 1, "b": "two"}
		cp := copyPayload(original)
		if len(cp) != 2 {
			t.Errorf("expected 2 keys, got %d", len(cp))
		}
		// Mutating copy should not affect original.
		cp["c"] = 3
		if _, ok := original["c"]; ok {
			t.Error("copy mutated original")
		}
	})
}

func TestExtractToolCalls(t *testing.T) {
	t.Run("no_tool_calls", func(t *testing.T) {
		tcs := extractToolCalls(map[string]any{"content": "hello"})
		if len(tcs) != 0 {
			t.Errorf("expected 0 tool calls, got %d", len(tcs))
		}
	})

	t.Run("with_tool_calls", func(t *testing.T) {
		payload := map[string]any{
			"tool_calls": []any{
				map[string]any{"id": "c1", "name": "bash", "arguments": `{"cmd":"ls"}`},
				map[string]any{"id": "c2", "name": "read_file", "arguments": `{"path":"/tmp"}`},
			},
		}
		tcs := extractToolCalls(payload)
		if len(tcs) != 2 {
			t.Fatalf("expected 2 tool calls, got %d", len(tcs))
		}
		if tcs[0].ID != "c1" || tcs[0].Name != "bash" {
			t.Errorf("unexpected first tool call: %+v", tcs[0])
		}
		if tcs[1].ID != "c2" || tcs[1].Name != "read_file" {
			t.Errorf("unexpected second tool call: %+v", tcs[1])
		}
	})

	t.Run("wrong_type", func(t *testing.T) {
		tcs := extractToolCalls(map[string]any{"tool_calls": "not_array"})
		if len(tcs) != 0 {
			t.Errorf("expected 0 for wrong type, got %d", len(tcs))
		}
	})

	t.Run("nil_payload", func(t *testing.T) {
		tcs := extractToolCalls(nil)
		if len(tcs) != 0 {
			t.Errorf("expected 0 for nil, got %d", len(tcs))
		}
	})

	t.Run("map_arguments", func(t *testing.T) {
		payload := map[string]any{
			"tool_calls": []any{
				map[string]any{
					"id":        "c1",
					"name":      "bash",
					"arguments": map[string]any{"cmd": "ls"},
				},
			},
		}
		tcs := extractToolCalls(payload)
		if len(tcs) != 1 {
			t.Fatalf("expected 1 tool call, got %d", len(tcs))
		}
		if tcs[0].Arguments != `{"cmd":"ls"}` {
			t.Errorf("expected marshalled args, got %s", tcs[0].Arguments)
		}
	})
}
