package harness

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// Issue #33: Context compaction tests
// ---------------------------------------------------------------------------

func TestCompactConversation_Basic(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	// Create a conversation with 6 messages (steps 0..5)
	msgs := []Message{
		{Role: "user", Content: "msg-0"},
		{Role: "assistant", Content: "msg-1"},
		{Role: "user", Content: "msg-2"},
		{Role: "assistant", Content: "msg-3"},
		{Role: "user", Content: "msg-4"},
		{Role: "assistant", Content: "msg-5"},
	}
	if err := store.SaveConversation(ctx, "conv-compact", msgs); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}

	// Compact: keep from step 4, insert summary at step 0
	summary := Message{
		Role:             "system",
		Content:          "Summary of first 4 messages",
		IsCompactSummary: true,
	}
	if err := store.CompactConversation(ctx, "conv-compact", 4, summary); err != nil {
		t.Fatalf("CompactConversation: %v", err)
	}

	loaded, err := store.LoadMessages(ctx, "conv-compact")
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}

	// Should have: 1 summary + 2 remaining messages (steps 4 and 5)
	if len(loaded) != 3 {
		t.Fatalf("expected 3 messages after compact, got %d: %+v", len(loaded), loaded)
	}

	// First message should be the summary
	if loaded[0].Role != "system" {
		t.Errorf("expected first message role=system, got %q", loaded[0].Role)
	}
	if loaded[0].Content != "Summary of first 4 messages" {
		t.Errorf("expected summary content, got %q", loaded[0].Content)
	}
	if !loaded[0].IsCompactSummary {
		t.Error("expected IsCompactSummary=true on summary message")
	}

	// Second message should be msg-4
	if loaded[1].Content != "msg-4" {
		t.Errorf("expected msg-4, got %q", loaded[1].Content)
	}

	// Third message should be msg-5
	if loaded[2].Content != "msg-5" {
		t.Errorf("expected msg-5, got %q", loaded[2].Content)
	}
}

func TestCompactConversation_SummaryFlagPersists(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	msgs := []Message{
		{Role: "user", Content: "q1"},
		{Role: "assistant", Content: "a1"},
		{Role: "user", Content: "q2"},
		{Role: "assistant", Content: "a2"},
	}
	if err := store.SaveConversation(ctx, "conv-flag", msgs); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}

	summary := Message{
		Role:             "system",
		Content:          "compact summary",
		IsCompactSummary: true,
	}
	if err := store.CompactConversation(ctx, "conv-flag", 2, summary); err != nil {
		t.Fatalf("CompactConversation: %v", err)
	}

	loaded, err := store.LoadMessages(ctx, "conv-flag")
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}

	// Verify IsCompactSummary is preserved after round-trip
	if !loaded[0].IsCompactSummary {
		t.Error("IsCompactSummary not persisted through round-trip")
	}
	// Non-summary messages should have IsCompactSummary=false
	for i := 1; i < len(loaded); i++ {
		if loaded[i].IsCompactSummary {
			t.Errorf("msg[%d] should not have IsCompactSummary=true", i)
		}
	}
}

func TestCompactConversation_KeepFromBeyondEnd(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	msgs := []Message{
		{Role: "user", Content: "old1"},
		{Role: "assistant", Content: "old2"},
		{Role: "user", Content: "old3"},
	}
	if err := store.SaveConversation(ctx, "conv-beyond", msgs); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}

	// keepFromStep > max step means no messages are kept (step indices 0,1,2 → keepFromStep=10 keeps nothing).
	// Only the summary remains.
	summary := Message{
		Role:             "system",
		Content:          "full summary",
		IsCompactSummary: true,
	}
	if err := store.CompactConversation(ctx, "conv-beyond", 10, summary); err != nil {
		t.Fatalf("CompactConversation: %v", err)
	}

	loaded, err := store.LoadMessages(ctx, "conv-beyond")
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}

	// Should have only the summary (no messages have step >= 10)
	if len(loaded) != 1 {
		t.Fatalf("expected 1 message (summary only), got %d", len(loaded))
	}
	if !loaded[0].IsCompactSummary {
		t.Error("expected summary flag set")
	}
}

func TestCompactConversation_KeepAllMessages(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	msgs := []Message{
		{Role: "user", Content: "q1"},
		{Role: "assistant", Content: "a1"},
		{Role: "user", Content: "q2"},
	}
	if err := store.SaveConversation(ctx, "conv-keepall", msgs); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}

	// keepFromStep=0 means keep from the very first message (keep all), summary is prepended.
	summary := Message{
		Role:             "system",
		Content:          "context from before",
		IsCompactSummary: true,
	}
	if err := store.CompactConversation(ctx, "conv-keepall", 0, summary); err != nil {
		t.Fatalf("CompactConversation: %v", err)
	}

	loaded, err := store.LoadMessages(ctx, "conv-keepall")
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}

	// summary + all 3 original messages = 4 total
	if len(loaded) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(loaded))
	}
	if !loaded[0].IsCompactSummary {
		t.Error("first message should be the compact summary")
	}
}

func TestCompactConversation_NonExistentConversation(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	summary := Message{
		Role:             "system",
		Content:          "summary",
		IsCompactSummary: true,
	}
	// Compacting a non-existent conversation should return an error
	err := store.CompactConversation(ctx, "does-not-exist", 0, summary)
	if err == nil {
		t.Error("expected error for non-existent conversation, got nil")
	}
}

func TestCompactConversation_NegativeKeepFrom(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	msgs := []Message{
		{Role: "user", Content: "hello"},
	}
	if err := store.SaveConversation(ctx, "conv-neg", msgs); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}

	summary := Message{Role: "system", Content: "s", IsCompactSummary: true}
	// Negative keepFromStep should error
	err := store.CompactConversation(ctx, "conv-neg", -1, summary)
	if err == nil {
		t.Error("expected error for negative keepFromStep, got nil")
	}
}

func TestCompactConversation_UpdatesMsgCount(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	msgs := []Message{
		{Role: "user", Content: "q1"},
		{Role: "assistant", Content: "a1"},
		{Role: "user", Content: "q2"},
		{Role: "assistant", Content: "a2"},
		{Role: "user", Content: "q3"},
	}
	if err := store.SaveConversation(ctx, "conv-count", msgs); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}

	// Compact keeping last 2 messages
	summary := Message{Role: "system", Content: "summary", IsCompactSummary: true}
	if err := store.CompactConversation(ctx, "conv-count", 3, summary); err != nil {
		t.Fatalf("CompactConversation: %v", err)
	}

	// msg_count should reflect new count: 1 summary + 2 kept = 3
	convs, err := store.ListConversations(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if len(convs) != 1 {
		t.Fatalf("expected 1 conversation, got %d", len(convs))
	}
	if convs[0].MsgCount != 3 {
		t.Errorf("expected MsgCount=3 after compact, got %d", convs[0].MsgCount)
	}
}

func TestCompactConversation_ConcurrentSafety(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	// Create 5 separate conversations and compact them concurrently
	const n = 5
	for i := 0; i < n; i++ {
		msgs := []Message{
			{Role: "user", Content: fmt.Sprintf("q%d-1", i)},
			{Role: "assistant", Content: fmt.Sprintf("a%d-1", i)},
			{Role: "user", Content: fmt.Sprintf("q%d-2", i)},
			{Role: "assistant", Content: fmt.Sprintf("a%d-2", i)},
		}
		if err := store.SaveConversation(ctx, fmt.Sprintf("cc-conv-%d", i), msgs); err != nil {
			t.Fatalf("SaveConversation %d: %v", i, err)
		}
	}

	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			summary := Message{
				Role:             "system",
				Content:          fmt.Sprintf("summary-%d", idx),
				IsCompactSummary: true,
			}
			convID := fmt.Sprintf("cc-conv-%d", idx)
			if err := store.CompactConversation(ctx, convID, 2, summary); err != nil {
				errs <- fmt.Errorf("compact %d: %w", idx, err)
			}
		}(i)
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}

	// Verify each conversation has the right structure: 1 summary + 2 kept messages
	for i := 0; i < n; i++ {
		loaded, err := store.LoadMessages(ctx, fmt.Sprintf("cc-conv-%d", i))
		if err != nil {
			t.Fatalf("LoadMessages cc-conv-%d: %v", i, err)
		}
		if len(loaded) != 3 {
			t.Errorf("cc-conv-%d: expected 3 messages, got %d", i, len(loaded))
		}
		if !loaded[0].IsCompactSummary {
			t.Errorf("cc-conv-%d: first message should be compact summary", i)
		}
	}
}

func TestCompactConversation_MigrationIdempotent(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	// Running Migrate twice should still allow CompactConversation to work.
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}

	msgs := []Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "world"},
	}
	if err := store.SaveConversation(ctx, "idem-conv", msgs); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}

	summary := Message{Role: "system", Content: "ctx", IsCompactSummary: true}
	if err := store.CompactConversation(ctx, "idem-conv", 1, summary); err != nil {
		t.Fatalf("CompactConversation after double migrate: %v", err)
	}
}
