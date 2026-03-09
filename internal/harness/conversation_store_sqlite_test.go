package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
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
