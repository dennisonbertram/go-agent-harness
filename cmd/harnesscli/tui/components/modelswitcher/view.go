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

	starStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("220")) // gold/yellow
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

	// Search bar (when query is non-empty).
	if m.searchQuery != "" {
		sb.WriteString(dimStyle.Render("Filter: "))
		sb.WriteString(m.searchQuery)
		sb.WriteByte('\n')
		sb.WriteByte('\n')
	}

	// Error state (shown instead of list).
	if m.loadError != "" {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
		sb.WriteString(errStyle.Render(m.loadError))
		sb.WriteByte('\n')
	} else {
		// Loading indicator (shown above the list while fetching).
		if m.loading {
			sb.WriteString(dimStyle.Render("Loading models..."))
			sb.WriteByte('\n')
			sb.WriteByte('\n')
		}

		visible := m.visibleModels()
		if len(visible) == 0 && !m.loading {
			if m.searchQuery != "" {
				sb.WriteString(dimStyle.Render("No models match"))
			} else {
				sb.WriteString(dimStyle.Render("No models available"))
			}
			sb.WriteByte('\n')
		} else if len(visible) > 0 {
			// Decide whether to show provider headers.
			// Skip provider headers when searching or when starred models exist at top.
			showProviderHeaders := m.searchQuery == "" && len(m.starred) == 0

			lastProvider := ""
			for i, entry := range visible {
				if showProviderHeaders {
					label := entry.ProviderLabel
					if label == "" {
						label = entry.Provider
					}
					if label != lastProvider {
						sb.WriteString(providerStyle.Render(label))
						sb.WriteByte('\n')
						lastProvider = label
					}
				}

				isSelected := i == m.Selected
				isStarred := m.starred[entry.ID]

				// Build star prefix.
				var starPrefix string
				if isStarred {
					starPrefix = starStyle.Render("★") + " "
				} else {
					starPrefix = "  "
				}

				if isSelected {
					// Apply reverse-video highlight to the full row text.
					nameAndSuffix := entry.DisplayName
					if entry.ReasoningMode {
						nameAndSuffix += " [R]"
					}
					if entry.IsCurrent {
						nameAndSuffix += "  ← current"
					}
					// Star prefix for highlighted row — strip styling for reverse-video rendering.
					var starRaw string
					if isStarred {
						starRaw = "★ "
					} else {
						starRaw = "  "
					}
					// Pad to innerWidth for consistent highlight width.
					runes := []rune("> " + starRaw + nameAndSuffix)
					padNeeded := innerWidth - len(runes)
					if padNeeded < 0 {
						padNeeded = 0
					}
					highlighted := highlightStyle.Render(string(runes) + strings.Repeat(" ", padNeeded))
					sb.WriteString(highlighted)
				} else {
					// Un-highlighted row.
					sb.WriteString("  ")
					sb.WriteString(starPrefix)
					sb.WriteString(entry.DisplayName)
					if entry.ReasoningMode {
						sb.WriteString(" ")
						sb.WriteString(reasoningBadgeStyle.Render("[R]"))
					}
					if entry.IsCurrent {
						sb.WriteString("  " + currentStyle.Render("← current"))
					}
				}
				sb.WriteByte('\n')
			}
		}
	}

	// Footer hint.
	sb.WriteByte('\n')
	if m.loadError != "" {
		sb.WriteString(dimStyle.Render("esc cancel"))
	} else {
		sb.WriteString(dimStyle.Render("↑/↓ navigate  enter select  s star  esc cancel"))
	}

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

	// Look up the current model's display name from visible models.
	currentModelDisplayName := ""
	visible := m.visibleModels()
	if m.Selected >= 0 && m.Selected < len(visible) {
		currentModelDisplayName = visible[m.Selected].DisplayName
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
