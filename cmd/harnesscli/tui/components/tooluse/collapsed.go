package tooluse

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

// State tracks the tool call lifecycle.
type State int

const (
	// StateRunning indicates the tool call is in progress.
	StateRunning State = iota
	// StateCompleted indicates the tool call finished successfully.
	StateCompleted
	// StateError indicates the tool call failed.
	StateError
)

// dotRunningStyle renders the ⏺ symbol in bright green for in-progress tool calls.
var dotRunningStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
	Light: "#43BF6D",
	Dark:  "#73F59F",
})

// dotCompletedStyle renders the ⏺ symbol in dim/faint for completed tool calls.
var dotCompletedStyle = lipgloss.NewStyle().Faint(true)

// dotErrorStyle renders the ⏺ symbol in dim/faint for errored tool calls.
var dotErrorStyle = lipgloss.NewStyle().Faint(true)

// errorSuffixStyle renders the ✗ indicator in red.
var errorSuffixStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
	Light: "#FF5F87",
	Dark:  "#FF5F87",
})

// dimStyle renders text in a faint/dim style for completed state.
var dimStyle = lipgloss.NewStyle().Faint(true)

const (
	dotSymbol     = "⏺"
	dotPrefix     = "⏺ "
	ellipsis      = "…"
	errorCross    = " ✗"
	runningSuffix = "…"

	// dotPrefixWidth is the visual width of "⏺ " (dot + space = 2 runes).
	dotPrefixWidth = 2
	// defaultWidth is the fallback width when Width is unset.
	defaultWidth = 80
)

// CollapsedView renders a single-line collapsed tool call display.
//
// Format:
//   - Running:   "⏺ ToolName(args)…"          — bright green dot, trailing ellipsis
//   - Completed: "⏺ ToolName(args) (N.Ns)"    — dim dot, optional timing suffix
//   - Error:     "⏺ ToolName(args) ✗"          — dim dot, red cross suffix
//
// Arguments are truncated with "…" if the total line would exceed Width.
// When State==StateError and Hint is non-empty, a hint line is rendered below.
// When State==StateCompleted and Timer is set (started+stopped), the duration
// is appended as " (N.Ns)" in dim style.
type CollapsedView struct {
	// ToolName is the name of the tool being called.
	ToolName string
	// Args is the pre-formatted argument string (e.g. "file.go, n=10").
	Args string
	// State is the current lifecycle state of the tool call.
	State State
	// Width is the available terminal width. Defaults to 80 if zero.
	Width int
	// Hint is an optional suggestion rendered below the collapsed line when
	// State==StateError and Hint is non-empty.
	Hint string
	// Timer tracks the duration of the tool call. When State==StateCompleted
	// and Timer has been started+stopped, the duration is appended to the line.
	Timer Timer
}

// View renders the collapsed tool call as a single line.
func (v CollapsedView) View() string {
	width := v.Width
	if width <= 0 {
		width = defaultWidth
	}

	// Build timing suffix for completed state when timer was used.
	var timingSuffix string
	if v.State == StateCompleted && !v.Timer.startTime.IsZero() && !v.Timer.IsRunning() {
		timingSuffix = " (" + v.Timer.FormatDuration() + ")"
	}

	// Determine the suffix string (plain, for width calculation).
	var plainSuffix string
	switch v.State {
	case StateRunning:
		plainSuffix = runningSuffix
	case StateError:
		plainSuffix = errorCross
	default:
		plainSuffix = timingSuffix
	}

	// Build the inner content: "ToolName(truncatedArgs)"
	// Available width for everything after "⏺ ":
	//   width - dotPrefixWidth
	// Then we need room for: toolName + "(" + args + ")" + suffix
	innerAvail := width - dotPrefixWidth
	if innerAvail < 1 {
		innerAvail = 1
	}

	// Calculate fixed costs: toolName + "()" + suffix
	toolNameRunes := utf8.RuneCountInString(v.ToolName)
	suffixRunes := utf8.RuneCountInString(plainSuffix)
	// Fixed: toolName + "(" + ")" + suffix = toolNameRunes + 2 + suffixRunes
	fixedCost := toolNameRunes + 2 + suffixRunes

	// Available space for args
	argsAvail := innerAvail - fixedCost

	// Truncate or use args as-is
	args := v.Args
	argsRunes := utf8.RuneCountInString(args)
	if argsAvail <= 0 {
		// No room for args at all
		args = ""
	} else if argsRunes > argsAvail {
		// Need to truncate — reserve 1 rune for the ellipsis
		truncAt := argsAvail - 1
		if truncAt < 0 {
			truncAt = 0
		}
		args = truncateRunes(args, truncAt) + ellipsis
	}

	// Build plain content string for the inner part
	var plain strings.Builder
	plain.WriteString(v.ToolName)
	plain.WriteString("(")
	plain.WriteString(args)
	plain.WriteString(")")

	// Apply styles and assemble the final line
	var line strings.Builder

	// Render the ⏺ dot with appropriate style
	switch v.State {
	case StateRunning:
		line.WriteString(dotRunningStyle.Render(dotSymbol))
	case StateCompleted:
		line.WriteString(dotCompletedStyle.Render(dotSymbol))
	case StateError:
		line.WriteString(dotErrorStyle.Render(dotSymbol))
	}
	line.WriteString(" ")

	// Render the tool name + args content
	switch v.State {
	case StateCompleted:
		line.WriteString(dimStyle.Render(plain.String()))
	default:
		line.WriteString(plain.String())
	}

	// Append state-specific suffix
	switch v.State {
	case StateRunning:
		line.WriteString(runningSuffix)
	case StateCompleted:
		if timingSuffix != "" {
			line.WriteString(dimStyle.Render(timingSuffix))
		}
	case StateError:
		line.WriteString(errorSuffixStyle.Render(errorCross))
	}

	line.WriteString("\n")

	// Render hint line for error state when Hint is set.
	if v.State == StateError && v.Hint != "" {
		hintLine := treeStyle.Render(treeSymbol) + "  " + dimStyle.Render(v.Hint)
		line.WriteString(hintLine)
		line.WriteString("\n")
	}

	return line.String()
}

// truncateRunes returns the first n runes of s.
func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
