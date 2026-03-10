package tools

import (
	"strings"
	"testing"
)

func TestWrapUnwrapToolResult_RoundTrip(t *testing.T) {
	t.Parallel()

	tr := ToolResult{
		Output: `{"skill":"deploy","status":"activated"}`,
		MetaMessages: []MetaMessage{
			{Content: "You are now in deploy mode."},
		},
	}

	wrapped, err := WrapToolResult(tr)
	if err != nil {
		t.Fatalf("WrapToolResult: %v", err)
	}

	unwrapped, ok := UnwrapToolResult(wrapped)
	if !ok {
		t.Fatal("UnwrapToolResult returned false for wrapped result")
	}

	if unwrapped.Output != tr.Output {
		t.Errorf("Output = %q, want %q", unwrapped.Output, tr.Output)
	}
	if len(unwrapped.MetaMessages) != 1 {
		t.Fatalf("expected 1 meta-message, got %d", len(unwrapped.MetaMessages))
	}
	if unwrapped.MetaMessages[0].Content != "You are now in deploy mode." {
		t.Errorf("MetaMessage content = %q, want %q", unwrapped.MetaMessages[0].Content, "You are now in deploy mode.")
	}
}

func TestUnwrapToolResult_PlainString(t *testing.T) {
	t.Parallel()

	// A plain string (not JSON) should not be treated as enriched
	_, ok := UnwrapToolResult("just a plain string output")
	if ok {
		t.Fatal("UnwrapToolResult should return false for a plain string")
	}
}

func TestUnwrapToolResult_InvalidJSON(t *testing.T) {
	t.Parallel()

	_, ok := UnwrapToolResult("{invalid json")
	if ok {
		t.Fatal("UnwrapToolResult should return false for invalid JSON")
	}
}

func TestUnwrapToolResult_MissingSentinel(t *testing.T) {
	t.Parallel()

	// Valid JSON but without the __tool_result__ key
	_, ok := UnwrapToolResult(`{"skill":"deploy","status":"activated"}`)
	if ok {
		t.Fatal("UnwrapToolResult should return false for JSON without __tool_result__ key")
	}
}

func TestUnwrapToolResult_EmptyMetaMessages(t *testing.T) {
	t.Parallel()

	tr := ToolResult{
		Output:       `{"status":"ok"}`,
		MetaMessages: nil,
	}

	wrapped, err := WrapToolResult(tr)
	if err != nil {
		t.Fatalf("WrapToolResult: %v", err)
	}

	unwrapped, ok := UnwrapToolResult(wrapped)
	if !ok {
		t.Fatal("UnwrapToolResult returned false")
	}

	if unwrapped.Output != tr.Output {
		t.Errorf("Output = %q, want %q", unwrapped.Output, tr.Output)
	}
	if len(unwrapped.MetaMessages) != 0 {
		t.Errorf("expected 0 meta-messages, got %d", len(unwrapped.MetaMessages))
	}
}

func TestWrapToolResult_MultipleMessages(t *testing.T) {
	t.Parallel()

	tr := ToolResult{
		Output: `{"status":"ok"}`,
		MetaMessages: []MetaMessage{
			{Content: "First meta-message"},
			{Content: "Second meta-message"},
			{Content: "Third meta-message"},
		},
	}

	wrapped, err := WrapToolResult(tr)
	if err != nil {
		t.Fatalf("WrapToolResult: %v", err)
	}

	unwrapped, ok := UnwrapToolResult(wrapped)
	if !ok {
		t.Fatal("UnwrapToolResult returned false")
	}

	if len(unwrapped.MetaMessages) != 3 {
		t.Fatalf("expected 3 meta-messages, got %d", len(unwrapped.MetaMessages))
	}
	for i, expected := range []string{"First meta-message", "Second meta-message", "Third meta-message"} {
		if unwrapped.MetaMessages[i].Content != expected {
			t.Errorf("MetaMessage[%d] = %q, want %q", i, unwrapped.MetaMessages[i].Content, expected)
		}
	}
}

func TestUnwrapToolResult_NestedJSONWithDifferentKeys(t *testing.T) {
	t.Parallel()

	// JSON with other keys but no __tool_result__
	_, ok := UnwrapToolResult(`{"key":"value","nested":{"data":123}}`)
	if ok {
		t.Fatal("should not detect enriched result in normal JSON")
	}
}

func TestUnwrapToolResult_MalformedSentinelValue(t *testing.T) {
	t.Parallel()

	// Has the sentinel key but with invalid value
	_, ok := UnwrapToolResult(`{"__tool_result__":"not an object"}`)
	if ok {
		t.Fatal("should not detect enriched result when sentinel value is not a ToolResult object")
	}
}

func TestWrapToolResult_LargeContent(t *testing.T) {
	t.Parallel()

	// Test with large skill content (> 100KB)
	largeContent := strings.Repeat("x", 100*1024)
	tr := ToolResult{
		Output: `{"status":"ok"}`,
		MetaMessages: []MetaMessage{
			{Content: largeContent},
		},
	}

	wrapped, err := WrapToolResult(tr)
	if err != nil {
		t.Fatalf("WrapToolResult with large content: %v", err)
	}

	unwrapped, ok := UnwrapToolResult(wrapped)
	if !ok {
		t.Fatal("UnwrapToolResult returned false for large content")
	}

	if len(unwrapped.MetaMessages) != 1 {
		t.Fatalf("expected 1 meta-message, got %d", len(unwrapped.MetaMessages))
	}
	if len(unwrapped.MetaMessages[0].Content) != 100*1024 {
		t.Errorf("expected content length %d, got %d", 100*1024, len(unwrapped.MetaMessages[0].Content))
	}
}

func TestWrapToolResult_EmptyOutput(t *testing.T) {
	t.Parallel()

	tr := ToolResult{
		Output: "",
		MetaMessages: []MetaMessage{
			{Content: "meta"},
		},
	}

	wrapped, err := WrapToolResult(tr)
	if err != nil {
		t.Fatalf("WrapToolResult: %v", err)
	}

	unwrapped, ok := UnwrapToolResult(wrapped)
	if !ok {
		t.Fatal("UnwrapToolResult returned false")
	}

	if unwrapped.Output != "" {
		t.Errorf("expected empty output, got %q", unwrapped.Output)
	}
}

func TestUnwrapToolResult_EmptyString(t *testing.T) {
	t.Parallel()

	_, ok := UnwrapToolResult("")
	if ok {
		t.Fatal("UnwrapToolResult should return false for empty string")
	}
}
