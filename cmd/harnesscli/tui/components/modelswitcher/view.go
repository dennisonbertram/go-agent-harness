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

	reasoningBadgeStyle = lipgloss.NewStyle().
				Faint(true).
				Foreground(dimColor)
)

// View renders the model switcher dropdown.
// Returns "" when IsVisible() is false.
// width=0 defaults to 60.
func (m Model) View(width int) string {
	if !m.IsOpen {
		return ""
	}
	if m.reasoningMode {
		return m.viewReasoning(width)
	}
	return m.viewModelList(width)
}

// viewModelList renders the Level-0 model selection list.
func (m Model) viewModelList(width int) string {
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
		lastProvider := ""
		for i, entry := range m.Models {
			// Emit a provider group header whenever the provider changes.
			label := entry.ProviderLabel
			if label == "" {
				label = entry.Provider
			}
			if label != lastProvider {
				sb.WriteString(providerStyle.Render(label))
				sb.WriteByte('\n')
				lastProvider = label
			}

			isSelected := i == m.Selected

			var currentPart string
			if entry.IsCurrent {
				currentPart = "  " + currentStyle.Render("← current")
			}

			if isSelected {
				// Apply reverse-video highlight to the full row text (name + suffix).
				nameAndSuffix := entry.DisplayName
				if entry.ReasoningMode {
					nameAndSuffix += " [R]"
				}
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
				// Un-highlighted row: name normal, reasoning badge dim, current marker dim.
				sb.WriteString("  ")
				sb.WriteString(entry.DisplayName)
				if entry.ReasoningMode {
					sb.WriteString(" ")
					sb.WriteString(reasoningBadgeStyle.Render("[R]"))
				}
				if entry.IsCurrent {
					sb.WriteString(currentPart)
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

// viewReasoning renders the Level-1 reasoning effort selection list.
func (m Model) viewReasoning(width int) string {
	if width <= 0 {
		width = 60
	}

	const borderAndPad = 4
	innerWidth := width - borderAndPad
	if innerWidth < 20 {
		innerWidth = 20
	}

	// Look up the current model's display name.
	currentModelDisplayName := ""
	if m.Selected >= 0 && m.Selected < len(m.Models) {
		currentModelDisplayName = m.Models[m.Selected].DisplayName
	}

	var sb strings.Builder

	// Title shows context of which model we are configuring.
	title := "Reasoning Effort  [" + currentModelDisplayName + "]"
	sb.WriteString(titleStyle.Render(title))
	sb.WriteByte('\n')
	sb.WriteByte('\n')

	for i, entry := range ReasoningLevels {
		isSelected := i == m.reasoningSelected
		isCurrent := entry.ID == m.currentReasoning

		var prefix string
		if isSelected {
			prefix = "> "
		} else {
			prefix = "  "
		}

		var currentPart string
		if isCurrent {
			currentPart = "  " + currentStyle.Render("← current")
		}

		if isSelected {
			nameAndSuffix := entry.DisplayName
			if isCurrent {
				nameAndSuffix += "  ← current"
			}
			runes := []rune(prefix + nameAndSuffix)
			padNeeded := innerWidth - len(runes)
			if padNeeded < 0 {
				padNeeded = 0
			}
			highlighted := highlightStyle.Render(string(runes) + strings.Repeat(" ", padNeeded))
			sb.WriteString(highlighted)
		} else {
			sb.WriteString("  ")
			sb.WriteString(entry.DisplayName)
			if isCurrent {
				sb.WriteString(currentPart)
			}
		}
		sb.WriteByte('\n')
	}

	// Footer hint.
	sb.WriteByte('\n')
	sb.WriteString(dimStyle.Render("↑/↓ navigate  enter confirm  esc back"))

	box := boxStyle.
		Width(innerWidth).
		BorderForeground(lipgloss.Color("240")).
		Render(sb.String())

	return box
}
