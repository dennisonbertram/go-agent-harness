package messagebubble

import (
	"strings"
	"sync"

	"github.com/charmbracelet/glamour"
)

// MarkdownEnabled controls whether RenderMarkdown performs glamour rendering.
// When false, RenderMarkdown returns the raw input text unchanged.
// This can be set to false in tests or environments without ANSI support.
var MarkdownEnabled = true

// renderMu serialises glamour renderer creation when the per-call path is used.
// MarkdownRenderer instances are safe for concurrent calls to Render because
// glamour.TermRenderer.Render creates a fresh bytes.Buffer on each call.
var renderMu sync.Mutex

// RenderMarkdown renders markdown text using glamour for terminal display.
// Falls back to raw text if glamour returns an error or MarkdownEnabled is false.
// width controls the glamour word-wrap column; the effective wrap is width-2 to
// account for the leading indent applied by AssistantBubble.
func RenderMarkdown(text string, width int) string {
	if !MarkdownEnabled {
		return text
	}

	wrapWidth := width - 2
	if wrapWidth < 10 {
		wrapWidth = 10
	}

	r := NewMarkdownRenderer(width)
	return r.Render(text)
}

// MarkdownRenderer is a reusable renderer with a configured style and width.
// Each Render call creates a fresh glamour TermRenderer to avoid sharing
// internal buffer state across concurrent callers.
type MarkdownRenderer struct {
	width int
}

// NewMarkdownRenderer returns a MarkdownRenderer configured for the given
// terminal width. The glamour word-wrap is set to width-2.
func NewMarkdownRenderer(width int) *MarkdownRenderer {
	return &MarkdownRenderer{width: width}
}

// Render renders the given markdown text and returns the ANSI-styled result.
// Falls back to the raw text on any glamour error.
func (r *MarkdownRenderer) Render(text string) string {
	if !MarkdownEnabled {
		return text
	}

	wrapWidth := r.width - 2
	if wrapWidth < 10 {
		wrapWidth = 10
	}

	tr, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(wrapWidth),
	)
	if err != nil {
		return text
	}

	rendered, err := tr.Render(text)
	if err != nil {
		return text
	}

	// Trim trailing newlines so callers can control their own spacing.
	rendered = strings.TrimRight(rendered, "\n")
	return rendered
}
