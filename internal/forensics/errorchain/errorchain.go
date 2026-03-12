// Package errorchain provides error context snapshots and chain tracing for
// cascading failures. It captures the last N tool calls and messages at the
// time an error occurs, classifies the error by type, and supports chaining
// errors so cascading failures can be traced back to their root cause.
package errorchain

import (
	"fmt"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// ErrorClass taxonomy
// ---------------------------------------------------------------------------

// ErrorClass classifies an error by its origin.
type ErrorClass string

const (
	// ClassToolExecution is used when a tool handler returns an error.
	ClassToolExecution ErrorClass = "tool_execution"
	// ClassHallucination is used when the LLM produces output that cannot be
	// parsed or that violates protocol constraints.
	ClassHallucination ErrorClass = "hallucination"
	// ClassProvider is used when the LLM provider (API) returns an error.
	ClassProvider ErrorClass = "provider"
	// ClassResource is used when a system resource (disk, memory, network) is
	// exhausted or unavailable.
	ClassResource ErrorClass = "resource"
)

// ---------------------------------------------------------------------------
// ChainedError
// ---------------------------------------------------------------------------

// ChainedError wraps an error with an ErrorClass and an optional cause,
// supporting Go's standard errors.Is / errors.As chain traversal.
type ChainedError struct {
	// Class is the error taxonomy classification.
	Class ErrorClass
	// msg is the human-readable description of this error.
	msg string
	// Cause is the underlying error that triggered this one, or nil.
	Cause error
	// Context is an optional snapshot captured at the time of the error.
	Context *Snapshot
}

// NewChainedError creates a ChainedError with the given class, message, and
// optional cause. The cause may be nil.
func NewChainedError(class ErrorClass, msg string, cause error) *ChainedError {
	return &ChainedError{
		Class: class,
		msg:   msg,
		Cause: cause,
	}
}

// Error implements the error interface.
func (e *ChainedError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Class, e.msg, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Class, e.msg)
}

// Unwrap returns the cause so that errors.Is and errors.As can traverse the
// chain.
func (e *ChainedError) Unwrap() error {
	return e.Cause
}

// ---------------------------------------------------------------------------
// Snapshot
// ---------------------------------------------------------------------------

// DefaultSnapshotDepth is the default rolling window size used by
// NewSnapshotBuilder when depth <= 0.
const DefaultSnapshotDepth = 10

// ToolCallEntry records a single tool invocation in the snapshot.
type ToolCallEntry struct {
	// Name is the tool name (e.g. "bash", "read").
	Name string `json:"name"`
	// CallID is the LLM-assigned tool call identifier.
	CallID string `json:"call_id"`
	// Args is the raw JSON arguments string.
	Args string `json:"args"`
	// ErrorMsg is non-empty when the tool returned an error.
	ErrorMsg string `json:"error_msg,omitempty"`
}

// MessageEntry records a single conversation message in the snapshot.
type MessageEntry struct {
	// Role is the message author: "user", "assistant", "tool", "system".
	Role string `json:"role"`
	// Content is the message text.
	Content string `json:"content"`
}

// Snapshot is a point-in-time capture of the last N tool calls and messages
// at the moment an error occurred.
type Snapshot struct {
	// CapturedAt is the wall-clock time the snapshot was taken.
	CapturedAt time.Time `json:"captured_at"`
	// ToolCalls holds the last Depth tool invocations.
	ToolCalls []ToolCallEntry `json:"tool_calls"`
	// Messages holds the last Depth conversation messages.
	Messages []MessageEntry `json:"messages"`
	// Depth is the configured rolling window size.
	Depth int `json:"depth"`
}

// ---------------------------------------------------------------------------
// SnapshotBuilder
// ---------------------------------------------------------------------------

// SnapshotBuilder maintains a rolling window of recent tool calls and messages.
// It is safe for concurrent use.
type SnapshotBuilder struct {
	mu        sync.RWMutex
	depth     int
	toolCalls []ToolCallEntry
	messages  []MessageEntry
}

// NewSnapshotBuilder creates a SnapshotBuilder with the given rolling window
// depth. If depth <= 0, DefaultSnapshotDepth is used.
func NewSnapshotBuilder(depth int) *SnapshotBuilder {
	if depth <= 0 {
		depth = DefaultSnapshotDepth
	}
	return &SnapshotBuilder{
		depth:     depth,
		toolCalls: make([]ToolCallEntry, 0, depth),
		messages:  make([]MessageEntry, 0, depth),
	}
}

// RecordToolCall appends a tool invocation to the rolling window. If the
// window is full the oldest entry is evicted.
func (sb *SnapshotBuilder) RecordToolCall(name, callID, args, errMsg string) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	entry := ToolCallEntry{
		Name:     name,
		CallID:   callID,
		Args:     args,
		ErrorMsg: errMsg,
	}
	sb.toolCalls = appendRolling(sb.toolCalls, entry, sb.depth)
}

// RecordMessage appends a conversation message to the rolling window. If the
// window is full the oldest entry is evicted.
func (sb *SnapshotBuilder) RecordMessage(role, content string) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	entry := MessageEntry{Role: role, Content: content}
	sb.messages = appendRolling(sb.messages, entry, sb.depth)
}

// Build returns a point-in-time Snapshot of the current rolling window.
// The returned Snapshot is a deep copy; further calls to RecordToolCall or
// RecordMessage do not affect it.
func (sb *SnapshotBuilder) Build() Snapshot {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	toolCalls := make([]ToolCallEntry, len(sb.toolCalls))
	copy(toolCalls, sb.toolCalls)

	messages := make([]MessageEntry, len(sb.messages))
	copy(messages, sb.messages)

	return Snapshot{
		CapturedAt: time.Now().UTC(),
		ToolCalls:  toolCalls,
		Messages:   messages,
		Depth:      sb.depth,
	}
}

// appendRolling appends entry to s and evicts the oldest element if
// len(s) would exceed maxLen. It returns the resulting slice.
func appendRolling[T any](s []T, entry T, maxLen int) []T {
	if len(s) >= maxLen {
		// Evict the oldest (index 0) by shifting left.
		copy(s, s[1:])
		s[len(s)-1] = entry
		return s
	}
	return append(s, entry)
}

// ---------------------------------------------------------------------------
// DiagnosticContext interface
// ---------------------------------------------------------------------------

// DiagnosticContext is an optional interface that tools may implement to
// provide richer error diagnostics. When a tool that implements this interface
// fails, its DiagnosticInfo is included in the error context payload.
type DiagnosticContext interface {
	// DiagnosticInfo returns a map of key/value pairs describing the tool's
	// internal state at the time of the error. Values must be JSON-serialisable.
	DiagnosticInfo() map[string]any
}

// ---------------------------------------------------------------------------
// BuildErrorContextPayload
// ---------------------------------------------------------------------------

// BuildErrorContextPayload constructs the map[string]any payload that is
// emitted as the error.context SSE event. It includes the error class,
// message, optional cause, and a snapshot from sb.
func BuildErrorContextPayload(ce *ChainedError, sb *SnapshotBuilder) map[string]any {
	snap := sb.Build()

	// Serialise snapshot into a nested map for JSON-friendliness.
	toolCallMaps := make([]map[string]any, len(snap.ToolCalls))
	for i, tc := range snap.ToolCalls {
		m := map[string]any{
			"name":    tc.Name,
			"call_id": tc.CallID,
			"args":    tc.Args,
		}
		if tc.ErrorMsg != "" {
			m["error_msg"] = tc.ErrorMsg
		}
		toolCallMaps[i] = m
	}

	messageMaps := make([]map[string]any, len(snap.Messages))
	for i, msg := range snap.Messages {
		messageMaps[i] = map[string]any{
			"role":    msg.Role,
			"content": msg.Content,
		}
	}

	snapMap := map[string]any{
		"captured_at": snap.CapturedAt,
		"tool_calls":  toolCallMaps,
		"messages":    messageMaps,
		"depth":       snap.Depth,
	}

	payload := map[string]any{
		"class":    string(ce.Class),
		"error":    ce.Error(),
		"snapshot": snapMap,
	}

	if ce.Cause != nil {
		payload["cause"] = ce.Cause.Error()
	}

	return payload
}
