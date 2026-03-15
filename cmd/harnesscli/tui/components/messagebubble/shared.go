package messagebubble

import "go-agent-harness/cmd/harnesscli/tui/components/streamrenderer"

// WrapUserMessage wraps a user message at the given width.
func WrapUserMessage(text string, width int) []string {
	return streamrenderer.WrapText(text, width)
}

// WrapToolResult wraps a tool result with the tree-connector prefix.
func WrapToolResult(text string, width int) []string {
	return streamrenderer.WrapWithPrefix(text, "\u23bf  ", width)
}

// WrapAssistantMessage wraps assistant text at the given width.
func WrapAssistantMessage(text string, width int) []string {
	return streamrenderer.WrapText(text, width)
}
