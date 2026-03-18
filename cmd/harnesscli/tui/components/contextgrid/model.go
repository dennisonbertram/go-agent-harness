package contextgrid

import (
	"fmt"
	"strings"
)

// defaultContextWindow is the default model context window size in tokens.
// Used when TotalTokens is not set.
const defaultContextWindow = 200000

// Model renders a grid showing context window usage and token counts.
type Model struct {
	// TotalTokens is the total context window size.
	TotalTokens int
	// UsedTokens is the number of tokens consumed.
	UsedTokens int
	// Width is the available rendering width.
	Width int
}

// New creates a new context grid model.
func New() Model {
	return Model{}
}

// View renders the context grid.
func (m Model) View() string {
	total := m.TotalTokens
	if total <= 0 {
		total = defaultContextWindow
	}
	used := m.UsedTokens
	if used < 0 {
		used = 0
	}
	if used > total {
		used = total
	}

	w := m.Width
	if w <= 0 {
		w = 80
	}

	pct := float64(used) / float64(total) * 100.0

	// Build the progress bar. Reserve space for label prefix and suffix.
	// "Context: [####...] 12.3% (12345 / 200000 tokens)"
	barWidth := w - 4
	if barWidth < 1 {
		barWidth = 1
	}
	if barWidth > 60 {
		barWidth = 60
	}

	filled := int(float64(barWidth) * float64(used) / float64(total))
	if filled > barWidth {
		filled = barWidth
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	var sb strings.Builder
	sb.WriteString(clampLine("Context Window Usage", w))
	sb.WriteString("\n\n")
	sb.WriteString(clampLine(fmt.Sprintf("  [%s]", bar), w))
	sb.WriteString("\n\n")
	sb.WriteString(clampLine(fmt.Sprintf("  Used:  %d tokens", used), w))
	sb.WriteString("\n")
	sb.WriteString(clampLine(fmt.Sprintf("  Total: %d tokens", total), w))
	sb.WriteString("\n")
	sb.WriteString(clampLine(fmt.Sprintf("  Usage: %.1f%%", pct), w))
	sb.WriteString("\n")

	return sb.String()
}

func clampLine(line string, width int) string {
	runes := []rune(line)
	if width <= 0 || len(runes) <= width {
		return line
	}
	return string(runes[:width])
}
