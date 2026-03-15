package tui

// SSEEventMsg wraps an incoming SSE event for the BubbleTea update loop.
type SSEEventMsg struct {
	EventType string
	Data      string
}

// SSEErrorMsg signals an SSE connection error.
type SSEErrorMsg struct {
	Err error
}

// SSEDoneMsg signals the SSE stream has completed.
type SSEDoneMsg struct {
	RunID string
}

// SSEDropMsg signals that events were dropped due to back-pressure.
type SSEDropMsg struct {
	Count int
}

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
