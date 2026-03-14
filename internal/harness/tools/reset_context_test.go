package tools

import (
	"context"
	"encoding/json"
	"testing"
)

func TestIsResetContextResult_WrongToolName(t *testing.T) {
	t.Parallel()

	persist, ok := IsResetContextResult("some_other_tool", `{"__reset_context__":true,"persist":{"key":"val"}}`)
	if ok {
		t.Fatal("expected false for wrong tool name")
	}
	if persist != nil {
		t.Fatalf("expected nil persist, got %s", persist)
	}
}

func TestIsResetContextResult_InvalidJSON(t *testing.T) {
	t.Parallel()

	persist, ok := IsResetContextResult(ResetContextToolName, `not-json`)
	if ok {
		t.Fatal("expected false for invalid JSON")
	}
	if persist != nil {
		t.Fatal("expected nil persist")
	}
}

func TestIsResetContextResult_SentinelFalse(t *testing.T) {
	t.Parallel()

	persist, ok := IsResetContextResult(ResetContextToolName, `{"__reset_context__":false}`)
	if ok {
		t.Fatal("expected false when sentinel is false")
	}
	if persist != nil {
		t.Fatal("expected nil persist")
	}
}

func TestIsResetContextResult_ValidSentinel(t *testing.T) {
	t.Parallel()

	payload := `{"__reset_context__":true,"persist":{"task":"write code","step":3}}`
	persist, ok := IsResetContextResult(ResetContextToolName, payload)
	if !ok {
		t.Fatal("expected true for valid sentinel")
	}
	if persist == nil {
		t.Fatal("expected non-nil persist")
	}

	var m map[string]any
	if err := json.Unmarshal(persist, &m); err != nil {
		t.Fatalf("persist is not valid JSON: %v", err)
	}
	if m["task"] != "write code" {
		t.Errorf("expected task=write code, got %v", m["task"])
	}
}

func TestIsResetContextResult_NoPersistField(t *testing.T) {
	t.Parallel()

	// Sentinel true but no persist field — persist should be nil/empty but ok=true.
	payload := `{"__reset_context__":true}`
	persist, ok := IsResetContextResult(ResetContextToolName, payload)
	if !ok {
		t.Fatal("expected true even without persist field")
	}
	// persist may be nil or empty JSON
	_ = persist
}

func TestResetContextTool_Definition(t *testing.T) {
	t.Parallel()

	tl := ResetContextTool()

	if tl.Definition.Name != ResetContextToolName {
		t.Errorf("expected name %q, got %q", ResetContextToolName, tl.Definition.Name)
	}
	if !tl.Definition.Mutating {
		t.Error("reset_context should be mutating")
	}
	if tl.Definition.ParallelSafe {
		t.Error("reset_context should not be parallel safe")
	}
	if tl.Definition.Tier != TierCore {
		t.Errorf("expected TierCore, got %q", tl.Definition.Tier)
	}
	if tl.Definition.Description == "" {
		t.Error("expected non-empty description")
	}
	if tl.Handler == nil {
		t.Error("expected non-nil handler")
	}
}

func TestResetContextTool_Handler_InvalidJSON(t *testing.T) {
	t.Parallel()

	tl := ResetContextTool()
	out, err := tl.Handler(context.Background(), json.RawMessage(`not-json`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("handler output is not valid JSON: %v", err)
	}
	if _, ok := result["error"]; !ok {
		t.Error("expected error key in result for invalid JSON args")
	}
}

func TestResetContextTool_Handler_MissingPersist(t *testing.T) {
	t.Parallel()

	tl := ResetContextTool()
	out, err := tl.Handler(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("handler output is not valid JSON: %v", err)
	}
	if _, ok := result["error"]; !ok {
		t.Error("expected error key when persist is missing")
	}
}

func TestResetContextTool_Handler_NullPersist(t *testing.T) {
	t.Parallel()

	tl := ResetContextTool()
	out, err := tl.Handler(context.Background(), json.RawMessage(`{"persist":null}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("handler output is not valid JSON: %v", err)
	}
	if _, ok := result["error"]; !ok {
		t.Error("expected error key when persist is null")
	}
}

func TestResetContextTool_Handler_ValidPersist(t *testing.T) {
	t.Parallel()

	tl := ResetContextTool()
	args := json.RawMessage(`{"persist":{"current_task":"write tests","files_changed":["foo.go"]}}`)
	out, err := tl.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Output should be the sentinel JSON.
	persist, ok := IsResetContextResult(ResetContextToolName, out)
	if !ok {
		t.Fatalf("handler output is not a valid reset sentinel; got: %s", out)
	}
	if persist == nil {
		t.Fatal("expected non-nil persist in sentinel output")
	}

	var m map[string]any
	if err := json.Unmarshal(persist, &m); err != nil {
		t.Fatalf("persist is not valid JSON: %v", err)
	}
	if m["current_task"] != "write tests" {
		t.Errorf("expected current_task=write tests, got %v", m["current_task"])
	}
}

func TestResetContextTool_Tags(t *testing.T) {
	t.Parallel()

	tl := ResetContextTool()
	tagSet := make(map[string]bool)
	for _, tag := range tl.Definition.Tags {
		tagSet[tag] = true
	}
	for _, want := range []string{"context", "reset", "memory", "transcript"} {
		if !tagSet[want] {
			t.Errorf("expected tag %q in Definition.Tags", want)
		}
	}
}
