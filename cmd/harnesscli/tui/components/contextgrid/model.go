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
	if barWidth < 10 {
		barWidth = 10
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
	sb.WriteString("Context Window Usage\n\n")
	sb.WriteString(fmt.Sprintf("  [%s]\n\n", bar))
	sb.WriteString(fmt.Sprintf("  Used:  %d tokens\n", used))
	sb.WriteString(fmt.Sprintf("  Total: %d tokens\n", total))
	sb.WriteString(fmt.Sprintf("  Usage: %.1f%%\n", pct))

	return sb.String()
}
