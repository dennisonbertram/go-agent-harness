package tui

import "encoding/json"

// SSEEventMsg carries a decoded harness event from the SSE stream.
type SSEEventMsg struct {
	EventType string
	Raw       json.RawMessage
}

// SSEErrorMsg signals an SSE connection error.
type SSEErrorMsg struct {
	Err error
}

// SSEDoneMsg signals the stream ended (run.completed or run.failed).
type SSEDoneMsg struct {
	EventType string
}

// SSEDropMsg signals a message was dropped due to channel backpressure.
type SSEDropMsg struct{}

// WindowSizeMsg carries terminal dimension changes.
type WindowSizeMsg struct {
	Width  int
	Height int
}

// RunStartedMsg signals that a new run has been created.
type RunStartedMsg struct {
	RunID string
}

// RunCompletedMsg signals that the current run has finished.
type RunCompletedMsg struct {
	RunID string
}
