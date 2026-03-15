package tooluse

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

const (
	treeSymbol      = "⎿"
	treePrefix      = "⎿  "
	treePrefixWidth = 3 // ⎿ (1 rune wide) + 2 spaces
	maxResultLines  = 20
)

// treeStyle renders the ⎿ tree connector in dim style.
var treeStyle = lipgloss.NewStyle().Faint(true)

// durationStyle renders the duration text in dim style.
var durationStyle = lipgloss.NewStyle().Faint(true)

// Param is a key-value pair from tool call arguments.
type Param struct {
	Key   string
	Value string
}

// ExpandedView renders a multi-line detailed view of a tool call.
//
// Format:
//
//	⏺ ToolName(arg1, arg2)
//	⎿  key: value
//	⎿  key: value
//	⎿  result line 1
//	⎿  result line 2
//	   Duration: 1.2s        12:34:56
type ExpandedView struct {
	// ToolName is the name of the tool being called.
	ToolName string
	// Args is the raw argument string for display (used in the header line).
	Args string
	// Params are the parsed key-value parameters (optional; rendered after header).
	Params []Param
	// Result is the result content (may be partial or empty).
	Result string
	// State is the current lifecycle state of the tool call.
	State State
	// Duration is a human-readable duration string (e.g. "1.2s"). Empty means omit.
	// When Duration is empty and Timer is set (started+stopped), Timer.FormatDuration()
	// is used as the effective duration.
	Duration string
	// Timestamp is a human-readable timestamp (e.g. "14:32:01"). Empty means omit.
	// When set, it is right-aligned on the same line as Duration.
	Timestamp string
	// Width is the available terminal width. Defaults to 80 if zero.
	Width int
	// Timer tracks the duration of the tool call. When Duration is empty and
	// Timer has been started+stopped, Timer.FormatDuration() is used as the duration.
	Timer Timer
}

// effectiveDuration returns the duration string to use for the footer line.
// It prefers the explicit Duration field; falls back to Timer.FormatDuration()
// when Duration is empty and the timer was started and stopped.
func (v ExpandedView) effectiveDuration() string {
	if v.Duration != "" {
		return v.Duration
	}
	if !v.Timer.startTime.IsZero() && !v.Timer.IsRunning() {
		return v.Timer.FormatDuration()
	}
	return ""
}

// View renders the expanded tool call as multiple lines.
func (v ExpandedView) View() string {
	width := v.Width
	if width <= 0 {
		width = defaultWidth
	}

	var sb strings.Builder

	// --- Header line (same as CollapsedView) ---
	header := CollapsedView{
		ToolName: v.ToolName,
		Args:     v.Args,
		State:    v.State,
		Width:    width,
	}.View()
	sb.WriteString(header)

	// --- Param lines ---
	for _, p := range v.Params {
		line := renderTreeLine(fmt.Sprintf("%s: %s", p.Key, p.Value), width)
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	// --- Result lines ---
	if v.Result != "" {
		resultLines := strings.Split(v.Result, "\n")
		totalLines := len(resultLines)
		truncated := false
		if totalLines > maxResultLines {
			resultLines = resultLines[:maxResultLines]
			truncated = true
		}
		for _, line := range resultLines {
			sb.WriteString(renderTreeLine(line, width))
			sb.WriteString("\n")
		}
		if truncated {
			remaining := totalLines - maxResultLines
			hint := fmt.Sprintf("+%d more lines", remaining)
			sb.WriteString(renderTreeLine(hint, width))
			sb.WriteString("\n")
		}
	}

	// --- Duration / Timestamp line ---
	dur := v.effectiveDuration()
	if dur != "" || v.Timestamp != "" {
		sb.WriteString(renderDurationLine(dur, v.Timestamp, width))
		sb.WriteString("\n")
	}

	return sb.String()
}

// renderTreeLine renders a single content line with the ⎿ prefix.
// Content is truncated if it exceeds width after the prefix.
func renderTreeLine(content string, width int) string {
	// Available width for content after "⎿  " (3 runes: ⎿ + 2 spaces)
	avail := width - treePrefixWidth
	if avail < 1 {
		avail = 1
	}

	// Truncate content if needed
	contentRunes := utf8.RuneCountInString(content)
	if contentRunes > avail {
		truncAt := avail - 1
		if truncAt < 0 {
			truncAt = 0
		}
		content = truncateRunes(content, truncAt) + ellipsis
	}

	return treeStyle.Render(treeSymbol) + "  " + content
}

// renderDurationLine renders the footer line with duration (left) and
// optional timestamp (right-aligned).
func renderDurationLine(duration, timestamp string, width int) string {
	// Build duration text
	var durText string
	if duration != "" {
		durText = durationStyle.Render(duration)
	}

	if timestamp == "" {
		// No timestamp — just render duration with a leading indent matching treePrefix
		return "   " + durText
	}

	// Both present: left = "   " + duration, right = timestamp right-aligned.
	leftPlain := "   " + duration
	rightPlain := timestamp

	leftRunes := utf8.RuneCountInString(leftPlain)
	rightRunes := utf8.RuneCountInString(rightPlain)

	padding := width - leftRunes - rightRunes
	if padding < 1 {
		padding = 1
	}

	return "   " + durText + strings.Repeat(" ", padding) + durationStyle.Render(timestamp)
}
