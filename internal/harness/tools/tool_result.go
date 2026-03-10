package tools

import (
	"encoding/json"
	"fmt"
)

// MetaMessage is a hidden instruction message injected into the conversation.
// Meta-messages are sent to the LLM API but not shown to the user.
type MetaMessage struct {
	Content string `json:"content"`
}

// ToolResult extends a plain string tool result with optional side-effects.
// Tools that need to inject meta-messages return a ToolResult serialized as JSON
// with a sentinel wrapper, instead of a plain string.
type ToolResult struct {
	// Output is the normal tool output string (what the model sees as the tool result).
	Output string `json:"output"`

	// MetaMessages are additional messages to inject into the conversation.
	// They are inserted as system-role messages with IsMeta=true.
	MetaMessages []MetaMessage `json:"meta_messages,omitempty"`
}

// toolResultEnvelope wraps a ToolResult with the sentinel key for detection.
type toolResultEnvelope struct {
	Result ToolResult `json:"__tool_result__"`
}

// WrapToolResult wraps a ToolResult with the sentinel marker so the runner
// can detect enriched results without changing the Handler func signature.
func WrapToolResult(tr ToolResult) (string, error) {
	data, err := json.Marshal(toolResultEnvelope{Result: tr})
	if err != nil {
		return "", fmt.Errorf("marshal tool result envelope: %w", err)
	}
	return string(data), nil
}

// UnwrapToolResult checks if a tool output string is a wrapped ToolResult.
// Returns the ToolResult and true if it is, or zero value and false if not.
func UnwrapToolResult(output string) (ToolResult, bool) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		return ToolResult{}, false
	}
	raw, ok := envelope["__tool_result__"]
	if !ok {
		return ToolResult{}, false
	}
	var tr ToolResult
	if err := json.Unmarshal(raw, &tr); err != nil {
		return ToolResult{}, false
	}
	return tr, true
}
