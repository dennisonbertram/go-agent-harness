package harness

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	runstore "go-agent-harness/internal/store"
)

func completeConversationReplayRun(t *testing.T, runner *Runner, req RunRequest) (Run, []Event) {
	t.Helper()
	run, err := runner.StartRun(req)
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	history, stream, cancel, err := runner.Subscribe(run.ID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer cancel()

	events := append([]Event(nil), history...)
	for _, ev := range history {
		if IsTerminalEvent(ev.Type) {
			return run, events
		}
	}

	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev, ok := <-stream:
			if !ok {
				t.Fatalf("run %q stream closed before a terminal event", run.ID)
			}
			events = append(events, ev)
			if IsTerminalEvent(ev.Type) {
				return run, events
			}
		case <-deadline:
			t.Fatalf("timed out waiting for run %q to complete", run.ID)
		}
	}
}

func assertConversationRunOrder(t *testing.T, history []Event, firstRunID, secondRunID string) {
	t.Helper()
	firstSeen := false
	secondSeen := false
	for _, ev := range history {
		switch ev.RunID {
		case firstRunID:
			if secondSeen {
				t.Fatalf("event from first run %q appeared after second run %q", firstRunID, secondRunID)
			}
			firstSeen = true
		case secondRunID:
			secondSeen = true
		}
	}
	if !firstSeen || !secondSeen {
		t.Fatalf(
			"conversation history run coverage = first:%t second:%t, want both; history=%v",
			firstSeen, secondSeen, history,
		)
	}
}

// Regression for #1008: once both callback/cron-style runs have completed,
// SubscribeConversation must still replay both. The pre-fix implementation
// deliberately replayed only a current non-terminal run, so this history was
// empty and the completed scheduled turn vanished after a GUI reconnect.
func TestSubscribeConversationReplaysAllCompletedRunsInOrder(t *testing.T) {
	t.Parallel()

	runner := NewRunner(
		&stubProvider{turns: []CompletionResult{{Content: "first"}, {Content: "scheduled"}}},
		NewRegistry(),
		RunnerConfig{DefaultModel: "test-model"},
	)
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })

	const convID = "conv-completed-replay"
	first, _ := completeConversationReplayRun(t, runner, RunRequest{
		Prompt: "first", ConversationID: convID,
	})
	second, _ := completeConversationReplayRun(t, runner, RunRequest{
		Prompt: "scheduled continuation", ConversationID: convID,
	})

	history, _, cancel, err := runner.SubscribeConversation(convID)
	if err != nil {
		t.Fatalf("SubscribeConversation: %v", err)
	}
	defer cancel()

	assertConversationRunOrder(t, history, first.ID, second.ID)
}

// Durable replay must not depend on the original Runner's in-memory run map.
// The native app configures both SQLite stores, so a daemon restart must still
// be able to rebuild the conversation event history from the run store.
func TestSubscribeConversationReplaysCompletedRunAfterRunnerRestart(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dir := t.TempDir()
	convStore, err := NewSQLiteConversationStore(filepath.Join(dir, "conversations.db"))
	if err != nil {
		t.Fatalf("NewSQLiteConversationStore: %v", err)
	}
	if err := convStore.Migrate(ctx); err != nil {
		t.Fatalf("conversation store Migrate: %v", err)
	}
	t.Cleanup(func() { _ = convStore.Close() })

	eventStore, err := runstore.NewSQLiteStore(filepath.Join(dir, "runs.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	if err := eventStore.Migrate(ctx); err != nil {
		t.Fatalf("run store Migrate: %v", err)
	}
	t.Cleanup(func() { _ = eventStore.Close() })
	runner1 := NewRunner(
		&stubProvider{turns: []CompletionResult{{Content: "persisted scheduled reply"}}},
		NewRegistry(),
		RunnerConfig{
			DefaultModel:      "test-model",
			ConversationStore: convStore,
			Store:             eventStore,
		},
	)
	const convID = "conv-restart-replay"
	run, _ := completeConversationReplayRun(t, runner1, RunRequest{
		Prompt: "scheduled continuation", ConversationID: convID,
	})
	if err := runner1.Shutdown(ctx); err != nil {
		t.Fatalf("runner1 Shutdown: %v", err)
	}

	runner2 := NewRunner(
		&stubProvider{},
		NewRegistry(),
		RunnerConfig{
			DefaultModel:      "test-model",
			ConversationStore: convStore,
			Store:             eventStore,
		},
	)
	t.Cleanup(func() { _ = runner2.Shutdown(ctx) })

	history, _, cancel, err := runner2.SubscribeConversation(convID)
	if err != nil {
		t.Fatalf("SubscribeConversation after restart: %v", err)
	}
	defer cancel()

	found := false
	for _, ev := range history {
		if ev.RunID == run.ID && IsTerminalEvent(ev.Type) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("restart replay did not include terminal event for run %q: %v", run.ID, history)
	}
}
