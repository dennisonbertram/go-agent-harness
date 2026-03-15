package tooluse

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

// errorViewErrorStyle renders the error label and text in the error color.
var errorViewErrorStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
	Light: "#FF5F87",
	Dark:  "#FF5F87",
})

// errorViewHintStyle renders hint text in a dim/faint style.
var errorViewHintStyle = lipgloss.NewStyle().Faint(true)

// ErrorView renders a tool error with warning color and an actionable hint.
//
// Rendering format:
//
//	⏺ ToolName ✗
//	⎿  Error: permission denied
//	⎿  Hint: Check file permissions
type ErrorView struct {
	// ToolName is the name of the tool that failed.
	ToolName string
	// ErrorText is the error message to display.
	ErrorText string
	// Hint is an optional suggestion (e.g. "Try using a longer timeout").
	// When empty, no Hint line is rendered.
	Hint string
	// Width is the available terminal width. Defaults to 80 if zero.
	Width int
}

// View renders the error view as multiple lines.
func (e ErrorView) View() string {
	width := e.Width
	if width <= 0 {
		width = defaultWidth
	}

	var sb strings.Builder

	// --- Header line: ⏺ ToolName ✗ ---
	sb.WriteString(dotErrorStyle.Render(dotSymbol))
	sb.WriteString(" ")
	sb.WriteString(e.ToolName)
	sb.WriteString(" ")
	sb.WriteString(errorSuffixStyle.Render("✗"))
	sb.WriteString("\n")

	// --- Error line(s): ⎿  Error: {wrapped errorText} ---
	// Wrap at Width-12 to account for the "⎿  Error: " prefix (9 chars) plus margin.
	wrapWidth := width - 12
	if wrapWidth < 1 {
		wrapWidth = 1
	}

	if e.ErrorText != "" {
		errorPrefix := "Error: "
		wrapped := wrapText(e.ErrorText, wrapWidth)
		lines := strings.Split(wrapped, "\n")
		for i, line := range lines {
			if i == 0 {
				sb.WriteString(renderErrorTreeLine(errorPrefix+line, width))
			} else {
				// Continuation lines indented to align with text after "Error: "
				indent := strings.Repeat(" ", utf8.RuneCountInString(errorPrefix))
				sb.WriteString(renderErrorTreeLine(indent+line, width))
			}
			sb.WriteString("\n")
		}
	} else {
		// Empty error text — render an empty error line
		sb.WriteString(renderErrorTreeLine("Error: ", width))
		sb.WriteString("\n")
	}

	// --- Hint line: ⎿  Hint: {hint} ---
	if e.Hint != "" {
		hintLine := treeStyle.Render(treeSymbol) + "  " + errorViewHintStyle.Render("Hint: "+e.Hint)
		sb.WriteString(hintLine)
		sb.WriteString("\n")
	}

	return sb.String()
}

// renderErrorTreeLine renders a single error content line with the ⎿ prefix
// in error color. Content is truncated if it exceeds width.
func renderErrorTreeLine(content string, width int) string {
	avail := width - treePrefixWidth
	if avail < 1 {
		avail = 1
	}

	contentRunes := utf8.RuneCountInString(content)
	if contentRunes > avail {
		truncAt := avail - 1
		if truncAt < 0 {
			truncAt = 0
		}
		content = truncateRunes(content, truncAt) + ellipsis
	}

	return treeStyle.Render(treeSymbol) + "  " + errorViewErrorStyle.Render(content)
}

// wrapText wraps s to at most maxWidth runes per line, breaking on spaces.
// If a single word exceeds maxWidth, it is hard-wrapped.
func wrapText(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return s
	}

	words := strings.Fields(s)
	if len(words) == 0 {
		return s
	}

	var lines []string
	var currentLine strings.Builder
	currentWidth := 0

	for _, word := range words {
		wordRunes := utf8.RuneCountInString(word)

		if currentWidth == 0 {
			// Starting a new line
			if wordRunes > maxWidth {
				// Hard-wrap long word
				for _, chunk := range chunkString(word, maxWidth) {
					lines = append(lines, chunk)
				}
				currentLine.Reset()
				currentWidth = 0
			} else {
				currentLine.WriteString(word)
				currentWidth = wordRunes
			}
		} else {
			// Adding to existing line: need space + word
			needed := 1 + wordRunes
			if currentWidth+needed <= maxWidth {
				currentLine.WriteString(" ")
				currentLine.WriteString(word)
				currentWidth += needed
			} else {
				// Flush current line and start new one
				lines = append(lines, currentLine.String())
				currentLine.Reset()
				if wordRunes > maxWidth {
					for _, chunk := range chunkString(word, maxWidth) {
						lines = append(lines, chunk)
					}
					currentWidth = 0
				} else {
					currentLine.WriteString(word)
					currentWidth = wordRunes
				}
			}
		}
	}

	if currentWidth > 0 {
		lines = append(lines, currentLine.String())
	}

	return strings.Join(lines, "\n")
}

// chunkString splits s into chunks of at most maxWidth runes each.
func chunkString(s string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{s}
	}
	runes := []rune(s)
	var chunks []string
	for len(runes) > 0 {
		if len(runes) <= maxWidth {
			chunks = append(chunks, string(runes))
			break
		}
		chunks = append(chunks, string(runes[:maxWidth]))
		runes = runes[maxWidth:]
	}
	return chunks
}
