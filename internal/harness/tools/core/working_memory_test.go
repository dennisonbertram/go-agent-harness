package core

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	tools "go-agent-harness/internal/harness/tools"
	om "go-agent-harness/internal/observationalmemory"
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

func TestWorkingMemoryToolGetReturnsStoredJSONWithItsOriginalType(t *testing.T) {
	t.Parallel()

	store := workingmemory.NewMemoryStore()
	tool := WorkingMemoryTool(store)
	ctx := workingMemoryTestContext()

	tests := []struct {
		name  string
		key   string
		value any
		want  string
	}{
		{name: "string", key: "string", value: "api-memory-value", want: `"api-memory-value"`},
		{name: "object", key: "object", value: map[string]any{"step": "collect"}, want: `{"step":"collect"}`},
		{name: "array", key: "array", value: []any{"one", 2}, want: `["one",2]`},
		{name: "number", key: "number", value: 42, want: `42`},
		{name: "boolean", key: "boolean", value: true, want: `true`},
		{name: "null", key: "null", value: nil, want: `null`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, err := json.Marshal(map[string]any{"action": "set", "key": tt.key, "value": tt.value})
			if err != nil {
				t.Fatalf("marshal set args: %v", err)
			}
			if _, err := tool.Handler(ctx, args); err != nil {
				t.Fatalf("set: %v", err)
			}
			out, err := tool.Handler(ctx, json.RawMessage(`{"action":"get","key":"`+tt.key+`"}`))
			if err != nil {
				t.Fatalf("get: %v", err)
			}
			var result struct {
				Found bool            `json:"found"`
				Value json.RawMessage `json:"value"`
			}
			if err := json.Unmarshal([]byte(out), &result); err != nil {
				t.Fatalf("decode result: %v", err)
			}
			if !result.Found {
				t.Fatal("expected found result")
			}
			if got := string(result.Value); got != tt.want {
				t.Errorf("value = %s, want %s; output=%s", got, tt.want, out)
			}
		})
	}
}

func TestWorkingMemoryToolListReturnsStoredJSONWithItsOriginalType(t *testing.T) {
	t.Parallel()

	store := workingmemory.NewMemoryStore()
	tool := WorkingMemoryTool(store)
	ctx := workingMemoryTestContext()
	for key, value := range map[string]any{
		"string": "api-memory-value",
		"object": map[string]any{"step": "collect"},
		"array":  []any{"one", 2},
	} {
		args, err := json.Marshal(map[string]any{"action": "set", "key": key, "value": value})
		if err != nil {
			t.Fatalf("marshal set args: %v", err)
		}
		if _, err := tool.Handler(ctx, args); err != nil {
			t.Fatalf("set %q: %v", key, err)
		}
	}

	out, err := tool.Handler(ctx, json.RawMessage(`{"action":"list"}`))
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var result struct {
		Entries map[string]json.RawMessage `json:"entries"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	want := map[string]string{
		"string": `"api-memory-value"`,
		"object": `{"step":"collect"}`,
		"array":  `["one",2]`,
	}
	for key, expected := range want {
		if got := string(result.Entries[key]); got != expected {
			t.Errorf("entries[%q] = %s, want %s; output=%s", key, got, expected, out)
		}
	}
}

func TestWorkingMemoryToolFallsBackToStringForMalformedLegacyStorage(t *testing.T) {
	t.Parallel()

	store := malformedWorkingMemoryStore{entries: map[string]string{"legacy": "not valid json"}}
	tool := WorkingMemoryTool(store)
	ctx := workingMemoryTestContext()

	for _, raw := range []json.RawMessage{
		json.RawMessage(`{"action":"get","key":"legacy"}`),
		json.RawMessage(`{"action":"list"}`),
	} {
		out, err := tool.Handler(ctx, raw)
		if err != nil {
			t.Fatalf("handler %s: %v", raw, err)
		}
		var result any
		if err := json.Unmarshal([]byte(out), &result); err != nil {
			t.Fatalf("result must remain valid JSON: %v; output=%s", err, out)
		}
		if string(raw) == `{"action":"get","key":"legacy"}` && !strings.Contains(out, `"value":"not valid json"`) {
			t.Errorf("get did not preserve malformed legacy entry as a string: %s", out)
		}
		if string(raw) == `{"action":"list"}` && !strings.Contains(out, `"legacy":"not valid json"`) {
			t.Errorf("list did not preserve malformed legacy entry as a string: %s", out)
		}
	}
}

func TestWorkingMemoryToolGetNotFoundShapeIsUnchanged(t *testing.T) {
	t.Parallel()

	out, err := WorkingMemoryTool(workingmemory.NewMemoryStore()).Handler(workingMemoryTestContext(), json.RawMessage(`{"action":"get","key":"missing"}`))
	if err != nil {
		t.Fatalf("get missing: %v", err)
	}
	if out != `{"found":false,"key":"missing","value":""}` {
		t.Fatalf("not-found result changed: %s", out)
	}
}

func workingMemoryTestContext() context.Context {
	return context.WithValue(context.Background(), tools.ContextKeyRunMetadata, tools.RunMetadata{
		RunID: "run-1", TenantID: "tenant", ConversationID: "conv", AgentID: "agent",
	})
}

type malformedWorkingMemoryStore struct {
	entries map[string]string
}

func (s malformedWorkingMemoryStore) Set(context.Context, om.ScopeKey, string, any) error { return nil }
func (s malformedWorkingMemoryStore) Get(_ context.Context, _ om.ScopeKey, key string) (string, bool, error) {
	value, ok := s.entries[key]
	return value, ok, nil
}
func (s malformedWorkingMemoryStore) Delete(context.Context, om.ScopeKey, string) error { return nil }
func (s malformedWorkingMemoryStore) List(context.Context, om.ScopeKey) (map[string]string, error) {
	return s.entries, nil
}
func (s malformedWorkingMemoryStore) Snippet(context.Context, om.ScopeKey) (string, error) {
	return "", nil
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
