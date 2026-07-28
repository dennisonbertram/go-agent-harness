package core

import (
	"context"
	"encoding/json"
	"testing"

	tools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/workingmemory"
)

func TestWorkingMemoryToolCRUD(t *testing.T) {
	t.Parallel()

	store := workingmemory.NewMemoryStore()
	tool := WorkingMemoryTool(store)
	ctx := context.WithValue(context.Background(), tools.ContextKeyRunMetadata, tools.RunMetadata{
		RunID:          "run-1",
		TenantID:       "tenant",
		ConversationID: "conv",
		AgentID:        "agent",
	})

	if _, err := tool.Handler(ctx, json.RawMessage(`{"action":"set","key":"plan","value":{"step":"collect"}}`)); err != nil {
		t.Fatalf("set: %v", err)
	}
	out, err := tool.Handler(ctx, json.RawMessage(`{"action":"get","key":"plan"}`))
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if out == "" {
		t.Fatal("expected get output")
	}
	if _, err := tool.Handler(ctx, json.RawMessage(`{"action":"delete","key":"plan"}`)); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

// TestWorkingMemoryTool_List verifies the "list" action returns every
// previously-set entry for the scope.
func TestWorkingMemoryTool_List(t *testing.T) {
	store := workingmemory.NewMemoryStore()
	tool := WorkingMemoryTool(store)
	ctx := context.WithValue(context.Background(), tools.ContextKeyRunMetadata, tools.RunMetadata{
		RunID: "run-1", TenantID: "tenant", ConversationID: "conv", AgentID: "agent",
	})

	if _, err := tool.Handler(ctx, json.RawMessage(`{"action":"set","key":"a","value":"1"}`)); err != nil {
		t.Fatalf("set a: %v", err)
	}
	if _, err := tool.Handler(ctx, json.RawMessage(`{"action":"set","key":"b","value":"2"}`)); err != nil {
		t.Fatalf("set b: %v", err)
	}
	out, err := tool.Handler(ctx, json.RawMessage(`{"action":"list"}`))
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var result struct {
		Entries map[string]string `json:"entries"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("parse list result: %v", err)
	}
	if len(result.Entries) != 2 {
		t.Errorf("expected 2 entries, got %d (%v)", len(result.Entries), result.Entries)
	}
}

// TestWorkingMemoryTool_UnsupportedAction verifies an unrecognized action
// string is rejected rather than silently ignored.
func TestWorkingMemoryTool_UnsupportedAction(t *testing.T) {
	store := workingmemory.NewMemoryStore()
	tool := WorkingMemoryTool(store)
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"action":"explode"}`))
	if err == nil {
		t.Fatal("expected error for unsupported action")
	}
}

// TestWorkingMemoryTool_NilStore verifies the tool refuses to operate when
// constructed without a store, rather than panicking on a nil dereference.
func TestWorkingMemoryTool_NilStore(t *testing.T) {
	tool := WorkingMemoryTool(nil)
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"action":"get","key":"x"}`))
	if err == nil {
		t.Fatal("expected error when store is not configured")
	}
}

// TestWorkingMemoryTool_BadJSON verifies malformed JSON args produce a parse error.
func TestWorkingMemoryTool_BadJSON(t *testing.T) {
	store := workingmemory.NewMemoryStore()
	tool := WorkingMemoryTool(store)
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"action": 5}`))
	if err == nil {
		t.Fatal("expected error for malformed JSON input")
	}
}

// TestWorkingMemoryScopeFromContext_Defaults verifies that when run metadata
// is absent from the context, the scope falls back to "default"
// tenant/agent and an empty conversation id, rather than panicking or using
// zero-value garbage.
func TestWorkingMemoryScopeFromContext_Defaults(t *testing.T) {
	scope := workingMemoryScopeFromContext(context.Background())
	if scope.TenantID != "default" {
		t.Errorf("expected default tenant, got %q", scope.TenantID)
	}
	if scope.AgentID != "default" {
		t.Errorf("expected default agent, got %q", scope.AgentID)
	}
}

// TestWorkingMemoryScopeFromContext_ConversationFallsBackToRunID verifies
// that when RunMetadata.ConversationID is empty, the scope's conversation id
// falls back to the run id carried separately in the context.
func TestWorkingMemoryScopeFromContext_ConversationFallsBackToRunID(t *testing.T) {
	ctx := context.WithValue(context.Background(), tools.ContextKeyRunMetadata, tools.RunMetadata{
		RunID: "run-xyz", TenantID: "tenant", AgentID: "agent",
	})
	scope := workingMemoryScopeFromContext(ctx)
	if scope.ConversationID != "run-xyz" {
		t.Errorf("expected conversation id to fall back to run id 'run-xyz', got %q", scope.ConversationID)
	}
}
