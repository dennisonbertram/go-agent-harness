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
