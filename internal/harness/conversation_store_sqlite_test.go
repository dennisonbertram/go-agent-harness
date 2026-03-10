package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func newTestConversationStore(t *testing.T) *SQLiteConversationStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test-conv.db")
	store, err := NewSQLiteConversationStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteConversationStore: %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestConversationStoreSaveAndLoadMessages(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	msgs := []Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!", ToolCalls: []ToolCall{
			{ID: "call_1", Name: "read_file", Arguments: `{"path":"main.go"}`},
		}},
		{Role: "tool", ToolCallID: "call_1", Name: "read_file", Content: `{"content":"package main"}`},
		{Role: "assistant", Content: "I see the file."},
	}

	if err := store.SaveConversation(ctx, "conv-1", msgs); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}

	loaded, err := store.LoadMessages(ctx, "conv-1")
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}

	if len(loaded) != len(msgs) {
		t.Fatalf("expected %d messages, got %d", len(msgs), len(loaded))
	}

	for i, m := range loaded {
		if m.Role != msgs[i].Role {
			t.Errorf("msg[%d] role: got %q, want %q", i, m.Role, msgs[i].Role)
		}
		if m.Content != msgs[i].Content {
			t.Errorf("msg[%d] content: got %q, want %q", i, m.Content, msgs[i].Content)
		}
		if m.ToolCallID != msgs[i].ToolCallID {
			t.Errorf("msg[%d] tool_call_id: got %q, want %q", i, m.ToolCallID, msgs[i].ToolCallID)
		}
		if m.Name != msgs[i].Name {
			t.Errorf("msg[%d] name: got %q, want %q", i, m.Name, msgs[i].Name)
		}
	}
}

func TestConversationStoreSaveOverwrites(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	msgs1 := []Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi"},
	}
	if err := store.SaveConversation(ctx, "conv-overwrite", msgs1); err != nil {
		t.Fatalf("SaveConversation (1): %v", err)
	}

	msgs2 := []Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi"},
		{Role: "user", Content: "How are you?"},
		{Role: "assistant", Content: "Good, thanks!"},
	}
	if err := store.SaveConversation(ctx, "conv-overwrite", msgs2); err != nil {
		t.Fatalf("SaveConversation (2): %v", err)
	}

	loaded, err := store.LoadMessages(ctx, "conv-overwrite")
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}

	if len(loaded) != 4 {
		t.Fatalf("expected 4 messages after overwrite, got %d", len(loaded))
	}
	if loaded[3].Content != "Good, thanks!" {
		t.Errorf("expected last message content 'Good, thanks!', got %q", loaded[3].Content)
	}
}

func TestConversationStoreListConversations(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	// Save 3 conversations
	for i := 0; i < 3; i++ {
		msgs := []Message{{Role: "user", Content: fmt.Sprintf("msg-%d", i)}}
		if err := store.SaveConversation(ctx, fmt.Sprintf("conv-%d", i), msgs); err != nil {
			t.Fatalf("SaveConversation conv-%d: %v", i, err)
		}
		// Small sleep to ensure distinct updated_at values
		time.Sleep(10 * time.Millisecond)
	}

	// List all
	convs, err := store.ListConversations(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if len(convs) != 3 {
		t.Fatalf("expected 3 conversations, got %d", len(convs))
	}

	// Should be ordered by updated_at DESC (most recent first)
	if convs[0].ID != "conv-2" {
		t.Errorf("expected first conversation 'conv-2', got %q", convs[0].ID)
	}

	// Test limit
	convs, err = store.ListConversations(ctx, 2, 0)
	if err != nil {
		t.Fatalf("ListConversations with limit: %v", err)
	}
	if len(convs) != 2 {
		t.Fatalf("expected 2 conversations with limit, got %d", len(convs))
	}

	// Test offset
	convs, err = store.ListConversations(ctx, 10, 2)
	if err != nil {
		t.Fatalf("ListConversations with offset: %v", err)
	}
	if len(convs) != 1 {
		t.Fatalf("expected 1 conversation with offset 2, got %d", len(convs))
	}
}

func TestConversationStoreDeleteConversation(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	msgs := []Message{{Role: "user", Content: "Hello"}}
	if err := store.SaveConversation(ctx, "conv-del", msgs); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}

	if err := store.DeleteConversation(ctx, "conv-del"); err != nil {
		t.Fatalf("DeleteConversation: %v", err)
	}

	// Messages should be gone (cascaded)
	loaded, err := store.LoadMessages(ctx, "conv-del")
	if err != nil {
		t.Fatalf("LoadMessages after delete: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("expected 0 messages after delete, got %d", len(loaded))
	}

	// Conversation should not appear in list
	convs, err := store.ListConversations(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListConversations after delete: %v", err)
	}
	if len(convs) != 0 {
		t.Fatalf("expected 0 conversations after delete, got %d", len(convs))
	}
}

func TestConversationStoreConcurrentAccess(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	errs := make(chan error, 20)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			convID := fmt.Sprintf("concurrent-conv-%d", idx)
			msgs := []Message{
				{Role: "user", Content: fmt.Sprintf("question-%d", idx)},
				{Role: "assistant", Content: fmt.Sprintf("answer-%d", idx)},
			}
			if err := store.SaveConversation(ctx, convID, msgs); err != nil {
				errs <- fmt.Errorf("save %d: %w", idx, err)
				return
			}
			loaded, err := store.LoadMessages(ctx, convID)
			if err != nil {
				errs <- fmt.Errorf("load %d: %w", idx, err)
				return
			}
			if len(loaded) != 2 {
				errs <- fmt.Errorf("expected 2 messages for conv %d, got %d", idx, len(loaded))
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}
}

func TestConversationStoreEmptyConversation(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	loaded, err := store.LoadMessages(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("LoadMessages for nonexistent: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("expected 0 messages for nonexistent conversation, got %d", len(loaded))
	}
}

func TestConversationStoreToolCallsSerialization(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	toolCalls := []ToolCall{
		{ID: "call_abc", Name: "bash", Arguments: `{"command":"ls -la"}`},
		{ID: "call_def", Name: "write_file", Arguments: `{"path":"test.go","content":"package test"}`},
	}

	msgs := []Message{
		{Role: "user", Content: "Do stuff"},
		{Role: "assistant", Content: "", ToolCalls: toolCalls},
		{Role: "tool", ToolCallID: "call_abc", Name: "bash", Content: "file.txt"},
		{Role: "tool", ToolCallID: "call_def", Name: "write_file", Content: "ok"},
		{Role: "assistant", Content: "Done"},
	}

	if err := store.SaveConversation(ctx, "conv-tools", msgs); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}

	loaded, err := store.LoadMessages(ctx, "conv-tools")
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}

	if len(loaded) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(loaded))
	}

	// Check tool calls round-trip
	if len(loaded[1].ToolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(loaded[1].ToolCalls))
	}

	for i, tc := range loaded[1].ToolCalls {
		if tc.ID != toolCalls[i].ID {
			t.Errorf("tool call %d ID: got %q, want %q", i, tc.ID, toolCalls[i].ID)
		}
		if tc.Name != toolCalls[i].Name {
			t.Errorf("tool call %d Name: got %q, want %q", i, tc.Name, toolCalls[i].Name)
		}

		// Parse arguments to compare as JSON (avoid whitespace issues)
		var gotArgs, wantArgs map[string]any
		if err := json.Unmarshal([]byte(tc.Arguments), &gotArgs); err != nil {
			t.Errorf("tool call %d: failed to parse got arguments: %v", i, err)
		}
		if err := json.Unmarshal([]byte(toolCalls[i].Arguments), &wantArgs); err != nil {
			t.Errorf("tool call %d: failed to parse want arguments: %v", i, err)
		}
		gotJSON, _ := json.Marshal(gotArgs)
		wantJSON, _ := json.Marshal(wantArgs)
		if string(gotJSON) != string(wantJSON) {
			t.Errorf("tool call %d Arguments: got %q, want %q", i, tc.Arguments, toolCalls[i].Arguments)
		}
	}

	// Messages without tool calls should have nil/empty ToolCalls
	if len(loaded[0].ToolCalls) != 0 {
		t.Errorf("expected no tool calls on user message, got %d", len(loaded[0].ToolCalls))
	}
}

func TestConversationStoreMsgCount(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	msgs := []Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi"},
		{Role: "user", Content: "Bye"},
	}
	if err := store.SaveConversation(ctx, "conv-count", msgs); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}

	convs, err := store.ListConversations(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if len(convs) != 1 {
		t.Fatalf("expected 1 conversation, got %d", len(convs))
	}
	if convs[0].MsgCount != 3 {
		t.Errorf("expected MsgCount 3, got %d", convs[0].MsgCount)
	}
}

func TestConversationStoreDeleteNonExistent(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	// Deleting a non-existent conversation should not error
	if err := store.DeleteConversation(ctx, "does-not-exist"); err != nil {
		t.Fatalf("DeleteConversation on non-existent: %v", err)
	}
}

func TestConversationStoreEmptyPath(t *testing.T) {
	t.Parallel()
	_, err := NewSQLiteConversationStore("")
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

// --- Regression tests for conversation persistence ---

func TestConversationStoreConcurrentSavesSameConversation(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	const goroutines = 10
	var wg sync.WaitGroup
	errs := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			msgs := []Message{
				{Role: "user", Content: fmt.Sprintf("question-%d", idx)},
				{Role: "assistant", Content: fmt.Sprintf("answer-%d", idx)},
			}
			if err := store.SaveConversation(ctx, "same-conv", msgs); err != nil {
				errs <- fmt.Errorf("goroutine %d save: %w", idx, err)
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}

	// After all concurrent saves, the conversation should exist with 2 messages
	loaded, err := store.LoadMessages(ctx, "same-conv")
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(loaded))
	}
}

func TestConversationStoreInvalidDBPath(t *testing.T) {
	t.Parallel()
	// Path to a file inside a non-writable location (root-level device path)
	_, err := NewSQLiteConversationStore("/dev/null/impossible/path.db")
	if err == nil {
		t.Fatal("expected error for invalid DB path")
	}
}

func TestConversationStoreClosedStoreOperations(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "closed.db")
	store, err := NewSQLiteConversationStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteConversationStore: %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// Close the store
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Operations on closed store should return errors
	ctx := context.Background()

	if err := store.SaveConversation(ctx, "conv-1", []Message{{Role: "user", Content: "hi"}}); err == nil {
		t.Error("expected error saving to closed store")
	}

	if _, err := store.LoadMessages(ctx, "conv-1"); err == nil {
		t.Error("expected error loading from closed store")
	}

	if _, err := store.ListConversations(ctx, 10, 0); err == nil {
		t.Error("expected error listing from closed store")
	}

	if err := store.DeleteConversation(ctx, "conv-1"); err == nil {
		t.Error("expected error deleting from closed store")
	}
}

func TestConversationStoreEmptyMessages(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	// Save with empty message slice
	if err := store.SaveConversation(ctx, "conv-empty", []Message{}); err != nil {
		t.Fatalf("SaveConversation with empty messages: %v", err)
	}

	loaded, err := store.LoadMessages(ctx, "conv-empty")
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("expected 0 messages, got %d", len(loaded))
	}

	// Conversation should still appear in list with msg_count=0
	convs, err := store.ListConversations(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if len(convs) != 1 {
		t.Fatalf("expected 1 conversation, got %d", len(convs))
	}
	if convs[0].MsgCount != 0 {
		t.Fatalf("expected MsgCount=0, got %d", convs[0].MsgCount)
	}
}

func TestConversationStoreNilMessages(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	// Save with nil message slice
	if err := store.SaveConversation(ctx, "conv-nil", nil); err != nil {
		t.Fatalf("SaveConversation with nil messages: %v", err)
	}

	loaded, err := store.LoadMessages(ctx, "conv-nil")
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("expected 0 messages, got %d", len(loaded))
	}
}

func TestConversationStoreLargeConversation(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	// Build a large conversation with 500 messages
	const msgCount = 500
	msgs := make([]Message, msgCount)
	for i := 0; i < msgCount; i++ {
		if i%2 == 0 {
			msgs[i] = Message{Role: "user", Content: fmt.Sprintf("question %d with some padding content to increase size", i)}
		} else {
			msgs[i] = Message{Role: "assistant", Content: fmt.Sprintf("answer %d with some padding content to increase size", i)}
		}
	}

	if err := store.SaveConversation(ctx, "conv-large", msgs); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}

	loaded, err := store.LoadMessages(ctx, "conv-large")
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}
	if len(loaded) != msgCount {
		t.Fatalf("expected %d messages, got %d", msgCount, len(loaded))
	}

	// Verify first and last
	if loaded[0].Content != msgs[0].Content {
		t.Errorf("first message content mismatch")
	}
	if loaded[msgCount-1].Content != msgs[msgCount-1].Content {
		t.Errorf("last message content mismatch")
	}

	// Verify msg_count in listing
	convs, err := store.ListConversations(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if convs[0].MsgCount != msgCount {
		t.Fatalf("expected MsgCount=%d, got %d", msgCount, convs[0].MsgCount)
	}
}

func TestConversationStoreLargeContent(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	// Single message with very large content (100KB)
	largeContent := string(make([]byte, 100*1024))
	msgs := []Message{{Role: "user", Content: largeContent}}

	if err := store.SaveConversation(ctx, "conv-large-content", msgs); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}

	loaded, err := store.LoadMessages(ctx, "conv-large-content")
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 message, got %d", len(loaded))
	}
	if len(loaded[0].Content) != 100*1024 {
		t.Fatalf("expected content length %d, got %d", 100*1024, len(loaded[0].Content))
	}
}

func TestConversationStoreListConversationsPagination(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	// Save 5 conversations
	for i := 0; i < 5; i++ {
		msgs := []Message{{Role: "user", Content: fmt.Sprintf("msg-%d", i)}}
		if err := store.SaveConversation(ctx, fmt.Sprintf("pag-%d", i), msgs); err != nil {
			t.Fatalf("SaveConversation: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Offset beyond total should return empty
	convs, err := store.ListConversations(ctx, 10, 100)
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if len(convs) != 0 {
		t.Fatalf("expected 0 conversations for large offset, got %d", len(convs))
	}

	// Zero limit should default to 50 (per implementation)
	convs, err = store.ListConversations(ctx, 0, 0)
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if len(convs) != 5 {
		t.Fatalf("expected 5 conversations with limit=0 (defaults to 50), got %d", len(convs))
	}
}

func TestConversationStoreSpecialCharactersInContent(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	msgs := []Message{
		{Role: "user", Content: "Hello 'world' \"quotes\" & <html> \x00\n\ttabs"},
		{Role: "assistant", Content: "Response with unicode: \u2603 \U0001F600"},
	}

	if err := store.SaveConversation(ctx, "conv-special", msgs); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}

	loaded, err := store.LoadMessages(ctx, "conv-special")
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(loaded))
	}
	if loaded[0].Content != msgs[0].Content {
		t.Errorf("content mismatch: got %q, want %q", loaded[0].Content, msgs[0].Content)
	}
	if loaded[1].Content != msgs[1].Content {
		t.Errorf("content mismatch: got %q, want %q", loaded[1].Content, msgs[1].Content)
	}
}

func TestConversationStoreDoubleClose(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "double-close.db")
	store, err := NewSQLiteConversationStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteConversationStore: %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// First close should succeed
	if err := store.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}

	// Second close should not panic (may or may not error)
	_ = store.Close()
}

func TestConversationStoreSaveAndDeleteConcurrent(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	errs := make(chan error, 20)

	// Concurrently save and delete different conversations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			convID := fmt.Sprintf("sd-conv-%d", idx)
			msgs := []Message{{Role: "user", Content: fmt.Sprintf("msg-%d", idx)}}
			if err := store.SaveConversation(ctx, convID, msgs); err != nil {
				errs <- fmt.Errorf("save %d: %w", idx, err)
				return
			}
			if err := store.DeleteConversation(ctx, convID); err != nil {
				errs <- fmt.Errorf("delete %d: %w", idx, err)
			}
		}(i)
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

// --- Tests for per-message unique IDs (issue #40) ---

func TestConversationStoreMessageIDAssignment(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	// Save messages WITHOUT MessageID set (zero-value empty string)
	msgs := []Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
		{Role: "user", Content: "How are you?"},
	}

	if err := store.SaveConversation(ctx, "conv-id-assign", msgs); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}

	loaded, err := store.LoadMessages(ctx, "conv-id-assign")
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}

	if len(loaded) != len(msgs) {
		t.Fatalf("expected %d messages, got %d", len(msgs), len(loaded))
	}

	for i, msg := range loaded {
		if msg.MessageID == "" {
			t.Errorf("message[%d] has empty MessageID, expected a UUID", i)
			continue
		}
		if _, err := uuid.Parse(msg.MessageID); err != nil {
			t.Errorf("message[%d] MessageID %q is not a valid UUID: %v", i, msg.MessageID, err)
		}
	}
}

func TestConversationStoreMessageIDUniqueness(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	// Save a conversation with 6 messages (5+ as required)
	msgs := []Message{
		{Role: "user", Content: "msg-0"},
		{Role: "assistant", Content: "msg-1"},
		{Role: "user", Content: "msg-2"},
		{Role: "assistant", Content: "msg-3"},
		{Role: "user", Content: "msg-4"},
		{Role: "assistant", Content: "msg-5"},
	}

	if err := store.SaveConversation(ctx, "conv-id-unique", msgs); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}

	loaded, err := store.LoadMessages(ctx, "conv-id-unique")
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}

	if len(loaded) != len(msgs) {
		t.Fatalf("expected %d messages, got %d", len(msgs), len(loaded))
	}

	seen := make(map[string]bool)
	for i, msg := range loaded {
		if msg.MessageID == "" {
			t.Errorf("message[%d] has empty MessageID", i)
			continue
		}
		if seen[msg.MessageID] {
			t.Errorf("message[%d] has duplicate MessageID %q", i, msg.MessageID)
		}
		seen[msg.MessageID] = true
	}
}

func TestConversationStoreMessageIDStability(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	// Save a conversation and load to get assigned IDs
	msgs := []Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi"},
		{Role: "user", Content: "Bye"},
	}

	if err := store.SaveConversation(ctx, "conv-id-stable", msgs); err != nil {
		t.Fatalf("SaveConversation (1): %v", err)
	}

	loaded1, err := store.LoadMessages(ctx, "conv-id-stable")
	if err != nil {
		t.Fatalf("LoadMessages (1): %v", err)
	}

	// Record the IDs from first load
	firstIDs := make([]string, len(loaded1))
	for i, msg := range loaded1 {
		if msg.MessageID == "" {
			t.Fatalf("message[%d] has empty MessageID on first load", i)
		}
		firstIDs[i] = msg.MessageID
	}

	// Save the SAME messages again (with their IDs populated from load)
	if err := store.SaveConversation(ctx, "conv-id-stable", loaded1); err != nil {
		t.Fatalf("SaveConversation (2): %v", err)
	}

	loaded2, err := store.LoadMessages(ctx, "conv-id-stable")
	if err != nil {
		t.Fatalf("LoadMessages (2): %v", err)
	}

	if len(loaded2) != len(firstIDs) {
		t.Fatalf("expected %d messages on second load, got %d", len(firstIDs), len(loaded2))
	}

	// Assert the IDs are unchanged
	for i, msg := range loaded2 {
		if msg.MessageID != firstIDs[i] {
			t.Errorf("message[%d] MessageID changed: was %q, now %q", i, firstIDs[i], msg.MessageID)
		}
	}
}

func TestConversationStoreMessageIDPreserved(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	// Create messages with pre-set MessageID values
	msgs := []Message{
		{Role: "user", Content: "Hello", MessageID: "custom-id-1"},
		{Role: "assistant", Content: "Hi", MessageID: "custom-id-2"},
		{Role: "user", Content: "Bye", MessageID: "custom-id-3"},
	}

	if err := store.SaveConversation(ctx, "conv-id-preserved", msgs); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}

	loaded, err := store.LoadMessages(ctx, "conv-id-preserved")
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}

	if len(loaded) != len(msgs) {
		t.Fatalf("expected %d messages, got %d", len(msgs), len(loaded))
	}

	// Assert the custom IDs are preserved as-is (not overwritten with new UUIDs)
	for i, msg := range loaded {
		if msg.MessageID != msgs[i].MessageID {
			t.Errorf("message[%d] MessageID: got %q, want %q", i, msg.MessageID, msgs[i].MessageID)
		}
	}
}

func TestConversationStoreMessageIDMigration(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "migration-msgid.db")
	store, err := NewSQLiteConversationStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteConversationStore: %v", err)
	}

	// First migration: creates table with message_id column
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate (1): %v", err)
	}

	// Second migration: idempotent check -- should not error
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate (2): %v", err)
	}

	ctx := context.Background()

	// Save messages and verify IDs are assigned on load
	msgs := []Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi"},
	}

	if err := store.SaveConversation(ctx, "conv-migration-test", msgs); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}

	loaded, err := store.LoadMessages(ctx, "conv-migration-test")
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(loaded))
	}

	for i, msg := range loaded {
		if msg.MessageID == "" {
			t.Errorf("message[%d] has empty MessageID after migration", i)
		}
	}

	store.Close()
}

func TestConversationStoreMessageIDCrossConversation(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	// Save two different conversations with messages
	msgs1 := []Message{
		{Role: "user", Content: "Hello from conv1"},
		{Role: "assistant", Content: "Hi from conv1"},
		{Role: "user", Content: "More from conv1"},
	}
	msgs2 := []Message{
		{Role: "user", Content: "Hello from conv2"},
		{Role: "assistant", Content: "Hi from conv2"},
		{Role: "user", Content: "More from conv2"},
	}

	if err := store.SaveConversation(ctx, "conv-cross-1", msgs1); err != nil {
		t.Fatalf("SaveConversation conv-cross-1: %v", err)
	}
	if err := store.SaveConversation(ctx, "conv-cross-2", msgs2); err != nil {
		t.Fatalf("SaveConversation conv-cross-2: %v", err)
	}

	loaded1, err := store.LoadMessages(ctx, "conv-cross-1")
	if err != nil {
		t.Fatalf("LoadMessages conv-cross-1: %v", err)
	}
	loaded2, err := store.LoadMessages(ctx, "conv-cross-2")
	if err != nil {
		t.Fatalf("LoadMessages conv-cross-2: %v", err)
	}

	// Collect all message IDs and assert no duplicates across conversations
	allIDs := make(map[string]string) // messageID -> conversationID for error reporting
	for i, msg := range loaded1 {
		if msg.MessageID == "" {
			t.Errorf("conv-cross-1 message[%d] has empty MessageID", i)
			continue
		}
		if prevConv, exists := allIDs[msg.MessageID]; exists {
			t.Errorf("duplicate MessageID %q: found in conv-cross-1[%d] and %s", msg.MessageID, i, prevConv)
		}
		allIDs[msg.MessageID] = fmt.Sprintf("conv-cross-1[%d]", i)
	}
	for i, msg := range loaded2 {
		if msg.MessageID == "" {
			t.Errorf("conv-cross-2 message[%d] has empty MessageID", i)
			continue
		}
		if prevConv, exists := allIDs[msg.MessageID]; exists {
			t.Errorf("duplicate MessageID %q: found in conv-cross-2[%d] and %s", msg.MessageID, i, prevConv)
		}
		allIDs[msg.MessageID] = fmt.Sprintf("conv-cross-2[%d]", i)
	}
}

func TestConversationStoreMessageIDConcurrency(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	const numConversations = 10
	var wg sync.WaitGroup
	errs := make(chan error, numConversations*2)

	// Concurrently save 10 conversations
	for i := 0; i < numConversations; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			convID := fmt.Sprintf("concurrent-msgid-%d", idx)
			msgs := []Message{
				{Role: "user", Content: fmt.Sprintf("question-%d", idx)},
				{Role: "assistant", Content: fmt.Sprintf("answer-%d", idx)},
				{Role: "user", Content: fmt.Sprintf("followup-%d", idx)},
			}
			if err := store.SaveConversation(ctx, convID, msgs); err != nil {
				errs <- fmt.Errorf("save conv %d: %w", idx, err)
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatal(err)
	}

	// Load all messages from all conversations and check global uniqueness
	allIDs := make(map[string]string) // messageID -> source for error reporting
	for i := 0; i < numConversations; i++ {
		convID := fmt.Sprintf("concurrent-msgid-%d", i)
		loaded, err := store.LoadMessages(ctx, convID)
		if err != nil {
			t.Fatalf("LoadMessages %s: %v", convID, err)
		}
		if len(loaded) != 3 {
			t.Fatalf("expected 3 messages for %s, got %d", convID, len(loaded))
		}
		for j, msg := range loaded {
			if msg.MessageID == "" {
				t.Errorf("%s message[%d] has empty MessageID", convID, j)
				continue
			}
			source := fmt.Sprintf("%s[%d]", convID, j)
			if prev, exists := allIDs[msg.MessageID]; exists {
				t.Errorf("duplicate MessageID %q: found in %s and %s", msg.MessageID, source, prev)
			}
			allIDs[msg.MessageID] = source
		}
	}

	// Verify we collected the expected total number of unique IDs
	expectedTotal := numConversations * 3
	if len(allIDs) != expectedTotal {
		t.Errorf("expected %d unique message IDs, got %d", expectedTotal, len(allIDs))
	}
}

func TestConversationStoreMessageIDEmptyOnOldRows(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	// Save messages (they should get IDs assigned by SaveConversation)
	msgs := []Message{
		{Role: "user", Content: "First message"},
		{Role: "assistant", Content: "Second message"},
		{Role: "user", Content: "Third message"},
	}

	if err := store.SaveConversation(ctx, "conv-old-rows", msgs); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}

	// Load messages and verify the IDs are populated (non-empty)
	loaded, err := store.LoadMessages(ctx, "conv-old-rows")
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}

	if len(loaded) != len(msgs) {
		t.Fatalf("expected %d messages, got %d", len(msgs), len(loaded))
	}

	for i, msg := range loaded {
		if msg.MessageID == "" {
			t.Errorf("message[%d] has empty MessageID; expected a non-empty value", i)
		}
	}
}
