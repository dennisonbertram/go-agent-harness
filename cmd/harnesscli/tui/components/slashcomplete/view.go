package slashcomplete

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	// selectedPrefix marks the highlighted row; normalPrefix keeps the
	// other rows aligned with it. Both are two columns wide.
	selectedPrefix = "▶ "
	normalPrefix   = "  "
	// footerHint tells a first-time user how to drive the menu (#1401).
	footerHint = "↑↓ choose · Enter run · Tab complete · Esc close"
	// noMatchHint replaces the list when the query matches nothing, so the
	// menu never silently vanishes while the user is still typing (#1401).
	noMatchHint = "No matching commands"
	ellipsis    = "…"
)

// View renders the dropdown overlay as a block of rows without a trailing
// newline. Returns "" when the model is not active.
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

	// Styles — built inline so view.go has no external theme dependency.
	selectedStyle := lipgloss.NewStyle().Reverse(true)
	dimStyle := lipgloss.NewStyle().Faint(true)

	// Columns available to a row after the two-column prefix.
	available := width - lipgloss.Width(selectedPrefix)
	if available < 1 {
		available = 1
	}
	fit := func(s string) string { return truncateWithEllipsis(s, available) }

	filtered := m.filtered
	total := len(filtered)
	if total == 0 {
		if m.query == "" {
			return ""
		}
		return strings.Join([]string{
			normalPrefix + dimStyle.Render(fit(noMatchHint+" for \"/"+m.query+"\"")),
			normalPrefix + dimStyle.Render(fit("Enter shows the unknown-command hint · Esc close")),
		}, "\n")
	}

	// Name column width across the full filtered list for stable alignment.
	maxName := 0
	for _, s := range filtered {
		if w := lipgloss.Width(s.Name); w > maxName {
			maxName = w
		}
	}
	nameColWidth := maxName + 1 // leading "/"

	// Compute the scroll window: [windowStart, windowEnd), reserving rows
	// for the "more above/below" indicators while keeping m.selected visible.
	windowStart := m.scrollOffset
	if windowStart < 0 {
		windowStart = 0
	}
	rawEnd := windowStart + maxVis
	if rawEnd > total {
		rawEnd = total
	}
	showAbove := windowStart > 0
	showBelow := rawEnd < total
	contentCap := maxVis
	if showAbove {
		contentCap--
	}
	if showBelow {
		contentCap--
	}
	if contentCap < 1 {
		contentCap = 1
	}
	if m.selected >= windowStart+contentCap {
		windowStart = m.selected - contentCap + 1
	}
	if m.selected < windowStart {
		windowStart = m.selected
	}
	if windowStart < 0 {
		windowStart = 0
	}
	if windowStart >= total {
		windowStart = total - 1
	}
	windowEnd := windowStart + contentCap
	if windowEnd > total {
		windowEnd = total
	}
	showAbove = windowStart > 0
	showBelow = windowEnd < total

	lines := make([]string, 0, maxVis+3)
	if showAbove {
		lines = append(lines, normalPrefix+dimStyle.Render(fit(fmt.Sprintf("▲ %d more above", windowStart))))
	}
	for i := windowStart; i < windowEnd; i++ {
		s := filtered[i]
		namePart := "/" + s.Name
		padding := strings.Repeat(" ", nameColWidth-lipgloss.Width(namePart)+2)
		row := fit(namePart + padding + s.Description)
		if i == m.selected {
			// Pad so the highlight reads as a full-width bar, not a ragged
			// strip that ends where the description happens to end.
			row += strings.Repeat(" ", available-lipgloss.Width(row))
			lines = append(lines, selectedPrefix+selectedStyle.Render(row))
		} else {
			lines = append(lines, normalPrefix+row)
		}
	}
	if showBelow {
		lines = append(lines, normalPrefix+dimStyle.Render(fit(fmt.Sprintf("▼ %d more below", total-windowEnd))))
	}
	lines = append(lines, normalPrefix+dimStyle.Render(fit(footerHint)))
	return strings.Join(lines, "\n")
}

// truncateWithEllipsis shortens s to at most width terminal columns,
// replacing the cut with "…" so the reader can tell text was dropped.
func truncateWithEllipsis(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	if width <= 1 {
		return ellipsis
	}
	runes := []rune(s)
	// Trim runes until the text plus the ellipsis fits.
	for len(runes) > 0 && lipgloss.Width(string(runes))+1 > width {
		runes = runes[:len(runes)-1]
	}
	return strings.TrimRight(string(runes), " ") + ellipsis
}
