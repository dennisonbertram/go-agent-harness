package messagebubble

import (
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

// MarkdownEnabled controls whether RenderMarkdown performs glamour rendering.
// When false, RenderMarkdown returns the raw input text unchanged.
// This can be set to false in tests or environments without ANSI support.
var MarkdownEnabled = true

// renderMu serialises glamour renderer creation when the per-call path is used.
// MarkdownRenderer instances are safe for concurrent calls to Render because
// glamour.TermRenderer.Render creates a fresh bytes.Buffer on each call.
var renderMu sync.Mutex

// stdoutIsTerminal and backgroundIsDark are the two probes behind style
// resolution. They are variables so tests can drive both branches without a
// real terminal.
var (
	stdoutIsTerminal = func() bool { return term.IsTerminal(int(os.Stdout.Fd())) }
	backgroundIsDark = termenv.HasDarkBackground
)

var (
	styleOnce     sync.Once
	resolvedStyle string
)

// resolveGlamourStyle picks the glamour stylesheet for this terminal, probing
// the terminal at most once per process.
//
// It reproduces what glamour.WithAutoStyle would do internally (see
// glamour.getDefaultStyle), because the auto style re-runs the background probe
// on every render and each probe writes an OSC 11 and a cursor-position query to
// the TTY. termenv only caches that probe when its Output was built with a color
// cache, and the package-level default is not (termenv Output.BackgroundColor).
// Bubble Tea holds stdin in raw mode while a Program runs, so it reads those
// replies before termenv can and draws them as literal text in the input line.
//
// Call ResolveStyle before starting the Program so even the first probe happens
// while the terminal is still ours.
func resolveGlamourStyle() string {
	styleOnce.Do(func() {
		switch {
		case !stdoutIsTerminal():
			resolvedStyle = styles.NoTTYStyle
		case backgroundIsDark():
			resolvedStyle = styles.DarkStyle
		default:
			resolvedStyle = styles.LightStyle
		}
	})
	return resolvedStyle
}

// ResolveStyle probes the terminal background once and caches the result. Call
// it before Bubble Tea acquires the terminal; every later render reuses the
// cached answer instead of re-querying.
func ResolveStyle() string { return resolveGlamourStyle() }

// resetGlamourStyleForTest clears the cached resolution so tests can drive both
// terminal branches. Test-only.
func resetGlamourStyleForTest() {
	styleOnce = sync.Once{}
	resolvedStyle = ""
}

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
		glamour.WithStandardStyle(resolveGlamourStyle()),
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
