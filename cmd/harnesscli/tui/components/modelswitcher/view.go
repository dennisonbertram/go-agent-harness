package modelswitcher

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	dimColor  = lipgloss.AdaptiveColor{Light: "#9B9B9B", Dark: "#5C5C5C"}

	titleStyle = lipgloss.NewStyle().Bold(true)

	highlightStyle = lipgloss.NewStyle().
			Reverse(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"})

	dimStyle = lipgloss.NewStyle().
			Faint(true).
			Foreground(dimColor)

	providerStyle = lipgloss.NewStyle().
			Faint(true).
			Foreground(dimColor)

	currentStyle = lipgloss.NewStyle().
			Faint(true).
			Foreground(dimColor)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1)
)

// View renders the model switcher dropdown.
// Returns "" when IsVisible() is false.
// width=0 defaults to 60.
func (m Model) View(width int) string {
	if !m.IsOpen {
		return ""
	}
	if width <= 0 {
		width = 60
	}

	// Inner width accounting for border (2) and padding (2 each side = 4 total).
	const borderAndPad = 4
	innerWidth := width - borderAndPad
	if innerWidth < 20 {
		innerWidth = 20
	}

	var sb strings.Builder

	// Title.
	sb.WriteString(titleStyle.Render("Switch Model"))
	sb.WriteByte('\n')
	sb.WriteByte('\n')

	if len(m.Models) == 0 {
		sb.WriteString(dimStyle.Render("No models available"))
		sb.WriteByte('\n')
	} else {
		for i, entry := range m.Models {
			isSelected := i == m.Selected

			// Build the row content.
			var prefix string
			if isSelected {
				prefix = "> "
			} else {
				prefix = "  "
			}

			providerPart := providerStyle.Render(entry.Provider)

			var currentPart string
			if entry.IsCurrent {
				currentPart = "  " + currentStyle.Render("← current")
			}

			rowContent := prefix + entry.DisplayName + "  " + providerPart + currentPart

			if isSelected {
				// Apply reverse-video highlight to the full row text (name + suffix).
				nameAndSuffix := entry.DisplayName + "  " + entry.Provider
				if entry.IsCurrent {
					nameAndSuffix += "  ← current"
				}
				// Pad to innerWidth for consistent highlight width.
				runes := []rune("> " + nameAndSuffix)
				padNeeded := innerWidth - len(runes)
				if padNeeded < 0 {
					padNeeded = 0
				}
				highlighted := highlightStyle.Render(string(runes) + strings.Repeat(" ", padNeeded))
				sb.WriteString(highlighted)
			} else {
				_ = rowContent
				// Un-highlighted row: name normal, provider dim, current marker dim.
				sb.WriteString("  ")
				sb.WriteString(entry.DisplayName)
				sb.WriteString("  ")
				sb.WriteString(providerStyle.Render(entry.Provider))
				if entry.IsCurrent {
					sb.WriteString("  ")
					sb.WriteString(currentStyle.Render("← current"))
				}
			}
			sb.WriteByte('\n')
		}
	}

	// Footer hint.
	sb.WriteByte('\n')
	sb.WriteString(dimStyle.Render("↑/↓ navigate  enter select  esc cancel"))

	box := boxStyle.
		Width(innerWidth).
		BorderForeground(lipgloss.Color("240")).
		Render(sb.String())

	return box
}
