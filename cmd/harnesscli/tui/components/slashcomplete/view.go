package slashcomplete

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	// selectedPrefix is prepended to the currently highlighted row.
	selectedPrefix = "▶ "
	// normalPrefix is prepended to non-selected rows.
	normalPrefix = "  "
)

// View renders the dropdown overlay.
// Returns "" when the model is not active.
// width=0 defaults to 80.
func (m Model) View(width int) string {
	if !m.active {
		return ""
	}
	if width <= 0 {
		width = 80
	}

	maxVis := m.maxVisible
	if maxVis <= 0 {
		maxVis = 8
	}

	filtered := m.filtered
	total := len(filtered)
	if total == 0 {
		return ""
	}

	// Styles — built inline so view.go has no external theme dependency.
	selectedStyle := lipgloss.NewStyle().Reverse(true)
	dimStyle := lipgloss.NewStyle().Faint(true)

	// Determine the longest name for alignment.
	maxName := 0
	for _, s := range filtered {
		if len(s.Name) > maxName {
			maxName = len(s.Name)
		}
	}
	// Name column: "/" + name padded to maxName+1
	nameColWidth := maxName + 1 // +1 for leading "/"

	// Cap visible rows.
	visCount := total
	truncated := 0
	if total > maxVis {
		visCount = maxVis
		truncated = total - maxVis
	}

	var sb strings.Builder
	for i := 0; i < visCount; i++ {
		s := filtered[i]
		isSelected := i == m.selected

		// Build the name portion: "/name   " padded
		namePart := "/" + s.Name
		padding := strings.Repeat(" ", nameColWidth-len(namePart)+2)

		// Build the full row content (without prefix)
		rowContent := namePart + padding + s.Description

		// Trim to fit within width (prefix takes 2 chars)
		available := width - len(selectedPrefix)
		if available < 0 {
			available = 0
		}
		// Use rune-aware truncation
		runes := []rune(rowContent)
		if len(runes) > available {
			runes = runes[:available]
			rowContent = string(runes)
		}

		var line string
		if isSelected {
			line = selectedPrefix + selectedStyle.Render(rowContent)
		} else {
			line = normalPrefix + rowContent
		}
		sb.WriteString(line + "\n")
	}

	// Truncation indicator
	if truncated > 0 {
		indicator := fmt.Sprintf("  ... %d more", truncated)
		sb.WriteString(dimStyle.Render(indicator) + "\n")
	}

	return sb.String()
}
