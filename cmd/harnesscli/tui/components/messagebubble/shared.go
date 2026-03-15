package messagebubble

import (
	"time"
	"unicode/utf8"

	"go-agent-harness/cmd/harnesscli/tui/components/streamrenderer"
)

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

// FormatTimestamp formats a time.Time as "3:04 PM" (12-hour, no date).
// Returns "" for zero time.
func FormatTimestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("3:04 PM")
}

// RightAlign returns a string padded so that label appears at the right edge
// of width columns. Returns label truncated if width too narrow.
func RightAlign(label string, width int) string {
	labelRunes := utf8.RuneCountInString(label)
	if width <= labelRunes {
		// Truncate to width runes
		runes := []rune(label)
		if width <= 0 {
			return ""
		}
		return string(runes[:width])
	}
	padding := width - labelRunes
	result := make([]byte, 0, padding+len(label))
	for i := 0; i < padding; i++ {
		result = append(result, ' ')
	}
	result = append(result, []byte(label)...)
	return string(result)
}
