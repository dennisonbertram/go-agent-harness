package harness

import (
	"testing"
	"time"
)

func TestRunnerTranscriptSnapshot(t *testing.T) {
	t.Parallel()

	runner := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{})
	now := time.Now().UTC()
	runner.mu.Lock()
	runner.runs["run_1"] = &runState{
		run: Run{
			ID:             "run_1",
			Status:         RunStatusRunning,
			TenantID:       "tenant",
			ConversationID: "conversation",
			AgentID:        "agent",
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		messages: []Message{
			{Role: "user", Content: "hello"},
			{Role: "tool", Content: "tool result", Name: "read"},
			{Role: "assistant", Content: "done"},
		},
		subscribers: make(map[chan Event]struct{}),
	}
	runner.mu.Unlock()

	snapNoTools := runner.transcriptSnapshot("run_1", 0, false)
	if len(snapNoTools.Messages) != 2 {
		t.Fatalf("expected tool messages filtered out, got %+v", snapNoTools.Messages)
	}
	if snapNoTools.Messages[0].Index != 0 || snapNoTools.Messages[1].Index != 2 {
		t.Fatalf("expected original indexes preserved, got %+v", snapNoTools.Messages)
	}

	snapLimited := runner.transcriptSnapshot("run_1", 1, true)
	if len(snapLimited.Messages) != 1 || snapLimited.Messages[0].Role != "assistant" {
		t.Fatalf("expected only final message, got %+v", snapLimited.Messages)
	}

	missing := runner.transcriptSnapshot("missing", 0, true)
	if missing.RunID != "missing" || missing.ConversationID != "missing" {
		t.Fatalf("unexpected default snapshot: %+v", missing)
	}
}

func TestRunTranscriptReaderSnapshot(t *testing.T) {
	t.Parallel()

	nilReader := runTranscriptReader{runID: "run_nil"}
	snap := nilReader.Snapshot(5, false)
	if snap.RunID != "run_nil" {
		t.Fatalf("unexpected run id for nil reader: %+v", snap)
	}
	if snap.GeneratedAt.IsZero() {
		t.Fatalf("expected generated_at for nil reader")
	}

	runner := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{})
	now := time.Now().UTC()
	runner.mu.Lock()
	runner.runs["run_real"] = &runState{
		run: Run{
			ID:             "run_real",
			TenantID:       "tenant",
			ConversationID: "conversation",
			AgentID:        "agent",
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		messages: []Message{{Role: "user", Content: "hello"}, {Role: "assistant", Content: "done"}},
	}
	runner.mu.Unlock()

	reader := runTranscriptReader{runner: runner, runID: "run_real"}
	snap = reader.Snapshot(1, true)
	if len(snap.Messages) != 1 || snap.Messages[0].Role != "assistant" {
		t.Fatalf("unexpected reader snapshot: %+v", snap.Messages)
	}
}
