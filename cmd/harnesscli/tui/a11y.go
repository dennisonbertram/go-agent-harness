package tui

// A11yHints contains text hints for screen reader / accessibility tooling.
// These are rendered only in no-color / non-TTY mode as plain-text labels.
var A11yHints = struct {
	UserMessage      string
	AssistantMessage string
	ToolCall         string
	ToolResult       string
	ThinkingSpinner  string
	ErrorMessage     string
}{
	UserMessage:      "[You]",
	AssistantMessage: "[Assistant]",
	ToolCall:         "[Tool call]",
	ToolResult:       "[Tool result]",
	ThinkingSpinner:  "[Thinking...]",
	ErrorMessage:     "[Error]",
}
