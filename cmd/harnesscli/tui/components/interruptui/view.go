package interruptui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Color definitions for the interrupt banner.
var (
	// warningColor is the amber/yellow used for the warning icon and confirm banner.
	warningColor = lipgloss.AdaptiveColor{Light: "#FFAF00", Dark: "#FFAF00"}
	// dimColor is the muted grey used for the waiting line.
	dimColor = lipgloss.AdaptiveColor{Light: "#9B9B9B", Dark: "#5C5C5C"}
)

// View renders the interrupt banner as a string.
// Returns "" when State is StateHidden or StateDone.
func (m Model) View() string {
	w := m.Width
	if w <= 0 {
		w = 80
	}

	switch m.State {
	case StateHidden, StateDone:
		return ""

	case StateConfirm:
		return m.renderConfirm(w)

	case StateWaiting:
		return m.renderWaiting(w)

	default:
		return ""
	}
}

// renderConfirm renders the yellow warning banner for the Confirm state.
func (m Model) renderConfirm(w int) string {
	warningStyle := lipgloss.NewStyle().
		Foreground(warningColor).
		Bold(true)

	icon := warningStyle.Render("⚠")
	text := "Press Ctrl+C again to stop, or Esc to continue"
	line := icon + "  " + text

	// Bordered box centered to width.
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(warningColor).
		Padding(0, 1).
		MaxWidth(w)

	// Center the box.
	centered := lipgloss.NewStyle().
		Width(w).
		Align(lipgloss.Center)

	box := boxStyle.Render(line)
	// Only center if width is wide enough.
	if w >= lipgloss.Width(box)+4 {
		return centered.Render(box)
	}
	return box
}

// renderWaiting renders the dim "Stopping…" line for the Waiting state.
func (m Model) renderWaiting(w int) string {
	text := "Stopping\u2026 (waiting for current tool to finish)"
	style := lipgloss.NewStyle().
		Foreground(dimColor).
		Faint(true).
		MaxWidth(w)

	rendered := style.Render(text)

	// Ensure it doesn't exceed width.
	if w > 0 && len([]rune(stripANSI(rendered))) > w {
		rendered = lipgloss.NewStyle().MaxWidth(w).Render(text)
	}

	return rendered
}

// stripANSI removes ANSI escape sequences from s for width calculation.
func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if inEsc {
			if r == 'm' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEsc = false
			}
			continue
		}
		if r == '\x1b' {
			inEsc = true
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
