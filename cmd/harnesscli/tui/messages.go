package tui

import (
	"encoding/json"
	"time"
)

// ─── SSE Stream Messages ────────────────────────────────────────────────────

// SSEEventMsg carries a decoded harness event from the SSE stream.
type SSEEventMsg struct {
	EventType string
	Raw       json.RawMessage
}

// SSEErrorMsg signals a stream read/parse error.
type SSEErrorMsg struct{ Err error }

// SSEDoneMsg signals the stream ended (run.completed or run.failed).
type SSEDoneMsg struct{ EventType string }

// SSEDropMsg signals a message was dropped due to channel backpressure.
type SSEDropMsg struct{}

// ─── Assistant Messages ──────────────────────────────────────────────────────

// AssistantDeltaMsg carries a streaming text delta from the assistant.
type AssistantDeltaMsg struct{ Delta string }

// ThinkingDeltaMsg carries a streaming thinking/reasoning delta.
type ThinkingDeltaMsg struct{ Delta string }

// ─── Tool Call Messages ──────────────────────────────────────────────────────

// ToolStartMsg signals a tool call has begun.
type ToolStartMsg struct {
	CallID string
	Name   string
	Input  json.RawMessage
}

// ToolResultMsg signals a tool call completed with output.
type ToolResultMsg struct {
	CallID string
	Output string
}

// ToolErrorMsg signals a tool call failed.
type ToolErrorMsg struct {
	CallID string
	Err    error
}

// ToolCallChunkMsg is emitted when a streaming tool result chunk arrives.
type ToolCallChunkMsg struct {
	CallID string
	Chunk  string
	Done   bool // true when this is the final chunk
}

// ─── Run Lifecycle Messages ──────────────────────────────────────────────────

// RunStartedMsg signals a new run has been started.
type RunStartedMsg struct{ RunID string }

// RunCompletedMsg signals a run completed successfully.
type RunCompletedMsg struct{ RunID string }

// RunFailedMsg signals a run failed.
type RunFailedMsg struct {
	RunID string
	Error string
}

// ─── Usage / Cost Messages ───────────────────────────────────────────────────

// UsageDeltaMsg carries incremental token and cost usage.
type UsageDeltaMsg struct {
	InputTokens  int
	OutputTokens int
	CostUSD      float64
}

// ─── UI Action Messages ──────────────────────────────────────────────────────

// SpinnerTickMsg advances the thinking spinner animation.
type SpinnerTickMsg struct{}

// CommandMsg carries a parsed slash command from the input area.
type CommandMsg struct{ Input string }

// ClearMsg requests clearing the conversation view.
type ClearMsg struct{}

// OverlayOpenMsg requests opening a named overlay (help, context, stats, etc.).
type OverlayOpenMsg struct{ Kind string }

// OverlayCloseMsg requests closing the current overlay.
type OverlayCloseMsg struct{}

// WindowSizeMsg carries terminal dimension changes.
// Components that need size can also handle tea.WindowSizeMsg directly.
type WindowSizeMsg struct {
	Width  int
	Height int
}

// InterruptedMsg is emitted when the user cancels an active run.
type InterruptedMsg struct{ At time.Time }

// EscapeMsg is emitted when Escape closes an overlay.
type EscapeMsg struct{}

// ExportTranscriptMsg is emitted when a transcript export completes.
type ExportTranscriptMsg struct{ FilePath string }
