package harness

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	htools "go-agent-harness/internal/harness/tools"
)

func TestConversationMessages_PersistedEmptyConversation(t *testing.T) {
	t.Parallel()

	store := newTestConversationStore(t)
	if err := store.SaveConversation(context.Background(), "conv-empty", nil); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}

	runner := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{
		DefaultModel:      "gpt-4.1-mini",
		MaxSteps:          1,
		ConversationStore: store,
	})

	msgs, ok := runner.ConversationMessages("conv-empty")
	if !ok {
		t.Fatal("expected persisted empty conversation to be found")
	}
	if msgs == nil {
		t.Fatal("expected empty slice, got nil")
	}
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages, got %d", len(msgs))
	}
}

func TestConversationMessages_PersistedMissingConversationStillReturnsFalse(t *testing.T) {
	t.Parallel()

	store := newTestConversationStore(t)
	runner := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{
		DefaultModel:      "gpt-4.1-mini",
		MaxSteps:          1,
		ConversationStore: store,
	})

	msgs, ok := runner.ConversationMessages("does-not-exist")
	if ok {
		t.Fatal("expected ok=false for missing persisted conversation")
	}
	if msgs != nil {
		t.Fatalf("expected nil messages for missing conversation, got %+v", msgs)
	}
}

func TestCompactRunWhileWaitingForUserPreservesCompactionAfterResume(t *testing.T) {
	t.Parallel()

	provider := &stubProvider{turns: []CompletionResult{
		{
			ToolCalls: []ToolCall{{
				ID:        "call_echo_1",
				Name:      "echo_json",
				Arguments: `{"message":"hello-1"}`,
			}},
		},
		{
			ToolCalls: []ToolCall{{
				ID:        "call_echo_2",
				Name:      "echo_json",
				Arguments: `{"message":"hello-2"}`,
			}},
		},
		{
			ToolCalls: []ToolCall{{
				ID:        "call_ask",
				Name:      htools.AskUserQuestionToolName,
				Arguments: `{"questions":[{"question":"Where next?","header":"Route","options":[{"label":"Docs","description":"Read docs"},{"label":"Code","description":"Read code"}],"multiSelect":false}]}`,
			}},
		},
		{Content: "All done"},
	}}

	broker := NewInMemoryAskUserQuestionBroker(time.Now)
	registry := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode:   ToolApprovalModeFullAuto,
		AskUserBroker:  broker,
		AskUserTimeout: 2 * time.Second,
	})
	if err := registry.Register(ToolDefinition{
		Name:        "echo_json",
		Description: "echoes payload",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{"type": "string"},
			},
			"required": []string{"message"},
		},
	}, func(_ context.Context, raw json.RawMessage) (string, error) {
		var payload struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(raw, &payload); err != nil {
			return "", err
		}
		return `{"echo":"` + payload.Message + `"}`, nil
	}); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	runner := NewRunner(provider, registry, RunnerConfig{
		DefaultModel:   "gpt-5-nano",
		MaxSteps:       8,
		AskUserBroker:  broker,
		AskUserTimeout: 2 * time.Second,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "Need clarification"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	deadline := time.Now().Add(1500 * time.Millisecond)
	for {
		pending, err := runner.PendingInput(run.ID)
		if err == nil {
			if pending.CallID != "call_ask" {
				t.Fatalf("PendingInput call id = %q, want %q", pending.CallID, "call_ask")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for pending input: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatal("expected run state")
	}
	if state.Status != RunStatusWaitingForUser {
		t.Fatalf("expected waiting_for_user status, got %q", state.Status)
	}

	msgsBefore := runner.GetRunMessages(run.ID)
	if len(msgsBefore) < 6 {
		t.Fatalf("expected at least 6 messages before compaction, got %d", len(msgsBefore))
	}

	result, err := runner.CompactRun(context.Background(), run.ID, CompactRunRequest{
		Mode:     "strip",
		KeepLast: 2,
	})
	if err != nil {
		t.Fatalf("CompactRun: %v", err)
	}
	if result.MessagesRemoved == 0 {
		t.Fatal("expected compaction to remove at least one message while waiting for user")
	}

	msgsAfterCompact := runner.GetRunMessages(run.ID)
	if len(msgsAfterCompact) >= len(msgsBefore) {
		t.Fatalf("expected compacted message count < pre-compact count, got %d >= %d", len(msgsAfterCompact), len(msgsBefore))
	}
	toolMessagesAfterCompact := 0
	for _, msg := range msgsAfterCompact {
		if msg.Role == "tool" {
			toolMessagesAfterCompact++
		}
	}

	if err := runner.SubmitInput(run.ID, map[string]string{"Where next?": "Docs"}); err != nil {
		t.Fatalf("SubmitInput: %v", err)
	}

	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect events: %v", err)
	}
	requireEventOrder(t, events, "run.waiting_for_user", "run.resumed", "run.completed")

	state, ok = runner.GetRun(run.ID)
	if !ok {
		t.Fatal("expected final run state")
	}
	if state.Status != RunStatusCompleted {
		t.Fatalf("expected completed status, got %q", state.Status)
	}
	if state.Output != "All done" {
		t.Fatalf("expected final output %q, got %q", "All done", state.Output)
	}

	msgsFinal := runner.GetRunMessages(run.ID)
	if got, want := len(msgsFinal), len(msgsAfterCompact)+2; got != want {
		t.Fatalf("compaction should survive resume: got %d messages, want %d (compacted=%d + tool result + assistant)",
			got, want, len(msgsAfterCompact))
	}

	toolMessages := 0
	for _, msg := range msgsFinal {
		if msg.Role == "tool" {
			toolMessages++
		}
	}
	if got, want := toolMessages, toolMessagesAfterCompact+1; got != want {
		t.Fatalf("expected resumed run to append exactly one tool result after compaction, got %d tool messages, want %d", got, want)
	}
}
