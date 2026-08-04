package harness

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	runstore "go-agent-harness/internal/store"
)

type conversationSnapshotProvider struct {
	mu            sync.Mutex
	calls         int
	secondStarted chan struct{}
	releaseSecond chan struct{}
}

type invertedConversationSnapshotProvider struct {
	mu           sync.Mutex
	calls        int
	firstStarted chan struct{}
	releaseFirst chan struct{}
}

func (p *invertedConversationSnapshotProvider) Complete(ctx context.Context, _ CompletionRequest) (CompletionResult, error) {
	p.mu.Lock()
	p.calls++
	call := p.calls
	p.mu.Unlock()
	if call == 1 {
		close(p.firstStarted)
		select {
		case <-p.releaseFirst:
			return CompletionResult{Content: "first finishes last"}, nil
		case <-ctx.Done():
			return CompletionResult{}, ctx.Err()
		}
	}
	return CompletionResult{Content: "later run finishes first"}, nil
}

func (p *conversationSnapshotProvider) Complete(ctx context.Context, _ CompletionRequest) (CompletionResult, error) {
	p.mu.Lock()
	p.calls++
	call := p.calls
	p.mu.Unlock()
	if call == 1 {
		return CompletionResult{Content: "hello"}, nil
	}
	if call == 2 {
		close(p.secondStarted)
		select {
		case <-p.releaseSecond:
			return CompletionResult{Content: "hello"}, nil
		case <-ctx.Done():
			return CompletionResult{}, ctx.Err()
		}
	}
	return CompletionResult{Content: "unexpected"}, nil
}

// Regression for #1158: messages and their replay cursor must be one Runner
// snapshot. A later run may already have durable events, but its cursor cannot
// advance the response until that run's conversation messages are published.
func TestConversationMessagesSnapshotPairsMessagesWithDurableWatermark(t *testing.T) {
	t.Parallel()

	provider := &conversationSnapshotProvider{
		secondStarted: make(chan struct{}),
		releaseSecond: make(chan struct{}),
	}
	conversationStore := newTestConversationStore(t)
	eventStore := runstore.NewMemoryStore()
	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel:      "test-model",
		MaxSteps:          2,
		Store:             eventStore,
		ConversationStore: conversationStore,
	})
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })

	const conversationID = "conv-snapshot-watermark"
	completeConversationReplayRun(t, runner, RunRequest{
		Prompt: "first", ConversationID: conversationID,
	})

	first, ok := runner.ConversationMessagesSnapshot(conversationID, "")
	if !ok {
		t.Fatal("first snapshot not found")
	}
	if len(first.Messages) != 2 || first.Messages[1].Content != "hello" {
		t.Fatalf("first snapshot messages = %+v, want one completed turn", first.Messages)
	}
	if first.LastEventID == "" {
		t.Fatal("first snapshot last_event_id is empty; want durable event identity")
	}

	secondRun, err := runner.StartRun(RunRequest{
		Prompt: "scheduled continuation", ConversationID: conversationID,
	})
	if err != nil {
		t.Fatalf("StartRun second: %v", err)
	}
	select {
	case <-provider.secondStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("second provider call did not start")
	}

	during, ok := runner.ConversationMessagesSnapshot(conversationID, "")
	if !ok {
		t.Fatal("in-flight snapshot not found")
	}
	if len(during.Messages) != len(first.Messages) {
		t.Fatalf("in-flight message count = %d, want prior completed snapshot %d", len(during.Messages), len(first.Messages))
	}
	if during.LastEventID != first.LastEventID {
		t.Fatalf("in-flight last_event_id = %q, want prior snapshot cursor %q", during.LastEventID, first.LastEventID)
	}

	close(provider.releaseSecond)
	history, stream, cancel, err := runner.Subscribe(secondRun.ID)
	if err != nil {
		t.Fatalf("Subscribe second: %v", err)
	}
	defer cancel()
	terminalSeen := false
	for _, event := range history {
		if IsTerminalEvent(event.Type) {
			terminalSeen = true
			break
		}
	}
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for !terminalSeen {
		select {
		case event := <-stream:
			if IsTerminalEvent(event.Type) {
				terminalSeen = true
			}
		case <-deadline.C:
			t.Fatal("timed out waiting for second run terminal event")
		}
	}

	second, ok := runner.ConversationMessagesSnapshot(conversationID, "")
	if !ok {
		t.Fatal("second snapshot not found")
	}
	if len(second.Messages) != 4 || second.Messages[3].Content != "hello" {
		t.Fatalf("second snapshot messages = %+v, want two same-text completed turns", second.Messages)
	}
	if second.LastEventID == "" || second.LastEventID == first.LastEventID {
		t.Fatalf("second last_event_id = %q, want a new exact identity after %q", second.LastEventID, first.LastEventID)
	}

	second.Messages[0].Content = "mutated by caller"
	again, ok := runner.ConversationMessagesSnapshot(conversationID, "")
	if !ok || again.Messages[0].Content == "mutated by caller" {
		t.Fatal("snapshot leaked mutable message ownership to caller")
	}
}

func TestConversationMessagesSnapshotWithoutDurableEventReaderUsesEmptyCursor(t *testing.T) {
	t.Parallel()

	conversationStore := newTestConversationStore(t)
	if err := conversationStore.SaveConversation(context.Background(), "conv-old-store", []Message{
		{Role: "assistant", Content: "history"},
	}); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}
	runner := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{
		ConversationStore: conversationStore,
	})

	snapshot, ok := runner.ConversationMessagesSnapshot("conv-old-store", "")
	if !ok {
		t.Fatal("snapshot not found")
	}
	if snapshot.LastEventID != "" {
		t.Fatalf("last_event_id = %q, want safe empty fallback without event reader", snapshot.LastEventID)
	}
}

// Concurrent runs can interleave events that are absent from either run's
// message slice. No single Last-Event-ID can skip that interleaving safely, so
// the completing snapshot must fall back to an empty/full-replay cursor.
func TestConversationMessagesSnapshotOverlapNeverAdvancesPastUnpublishedRun(t *testing.T) {
	t.Parallel()

	provider := &invertedConversationSnapshotProvider{
		firstStarted: make(chan struct{}),
		releaseFirst: make(chan struct{}),
	}
	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel: "test-model", MaxSteps: 2, Store: runstore.NewMemoryStore(),
	})
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })

	const conversationID = "conv-overlap-watermark"
	first, err := runner.StartRun(RunRequest{Prompt: "first", ConversationID: conversationID})
	if err != nil {
		t.Fatalf("StartRun first: %v", err)
	}
	select {
	case <-provider.firstStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("first provider call did not start")
	}
	completeConversationReplayRun(t, runner, RunRequest{
		Prompt: "later overlapping run", ConversationID: conversationID,
	})
	close(provider.releaseFirst)
	history, stream, cancel, err := runner.Subscribe(first.ID)
	if err != nil {
		t.Fatalf("Subscribe first: %v", err)
	}
	defer cancel()
	terminal := false
	for _, event := range history {
		terminal = terminal || IsTerminalEvent(event.Type)
	}
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for !terminal {
		select {
		case event := <-stream:
			terminal = IsTerminalEvent(event.Type)
		case <-deadline.C:
			t.Fatal("timed out waiting for first overlapping run")
		}
	}

	snapshot, ok := runner.ConversationMessagesSnapshot(conversationID, "")
	if !ok {
		t.Fatal("overlap snapshot not found")
	}
	if snapshot.LastEventID != "" {
		t.Fatalf("overlap last_event_id = %q, want empty rather than skipping interleaved events", snapshot.LastEventID)
	}
}

func TestConversationMessagesSnapshotAfterRestartUsesEmptyCursorAfterHistoryMutation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dir := t.TempDir()
	conversationStore, err := NewSQLiteConversationStore(filepath.Join(dir, "conversations.db"))
	if err != nil {
		t.Fatalf("NewSQLiteConversationStore: %v", err)
	}
	if err := conversationStore.Migrate(ctx); err != nil {
		t.Fatalf("conversation store Migrate: %v", err)
	}
	t.Cleanup(func() { _ = conversationStore.Close() })
	eventStore, err := runstore.NewSQLiteStore(filepath.Join(dir, "runs.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	if err := eventStore.Migrate(ctx); err != nil {
		t.Fatalf("event store Migrate: %v", err)
	}
	t.Cleanup(func() { _ = eventStore.Close() })

	const conversationID = "conv-recovered-watermark"
	runner1 := NewRunner(
		&stubProvider{turns: []CompletionResult{{Content: "persisted hello"}}},
		NewRegistry(),
		RunnerConfig{DefaultModel: "test-model", Store: eventStore, ConversationStore: conversationStore},
	)
	completeConversationReplayRun(t, runner1, RunRequest{
		Prompt: "persist this", ConversationID: conversationID,
	})
	if err := runner1.Shutdown(ctx); err != nil {
		t.Fatalf("runner1 Shutdown: %v", err)
	}
	if _, err := conversationStore.UndoPrompts(ctx, conversationID, 1); err != nil {
		t.Fatalf("UndoPrompts: %v", err)
	}

	runner2 := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{
		DefaultModel: "test-model", Store: eventStore, ConversationStore: conversationStore,
	})
	t.Cleanup(func() { _ = runner2.Shutdown(ctx) })
	snapshot, ok := runner2.ConversationMessagesSnapshot(conversationID, "")
	if !ok {
		t.Fatal("recovered snapshot not found")
	}
	if snapshot.LastEventID != "" {
		t.Fatalf("restart last_event_id = %q, want empty without durable snapshot equivalence", snapshot.LastEventID)
	}
	for _, message := range snapshot.Messages {
		if message.Content == "persisted hello" {
			t.Fatalf("undo mutation did not change recovered messages: %+v", snapshot.Messages)
		}
	}
}
