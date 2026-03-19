package helpdialog

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Style constants for the dialog.
var (
	styleDialog = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240"))

	styleActiveTab = lipgloss.NewStyle().
			Bold(true).
			Underline(true).
			Foreground(lipgloss.Color("255"))

	styleDimTab = lipgloss.NewStyle().
			Faint(true).
			Foreground(lipgloss.Color("244"))

	styleSeparator = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	styleCommandName = lipgloss.NewStyle().
				Foreground(lipgloss.Color("33")).
				Bold(true)

	styleKeyName = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true)

	styleDescription = lipgloss.NewStyle().
				Foreground(lipgloss.Color("250"))

	styleAboutLine = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))
)

// tabNames holds the display names for each tab.
var tabNames = [tabCount]string{"Commands", "Keybindings", "About"}

// render produces the full dialog string at the given dimensions.
func render(m Model, width, height int) string {
	// Dialog inner width accounts for border (2 chars) and padding (2 chars each side).
	// We cap dialog width at min(width, 70) to avoid overly wide dialogs.
	dialogWidth := width - 4
	if dialogWidth > 68 {
		dialogWidth = 68
	}
	if dialogWidth < 20 {
		dialogWidth = 20
	}

	// Dialog inner height: border (2) + tab row (1) + separator (1) + content.
	// Available content lines = height - 4 (border top/bottom + tab + separator).
	contentLines := height - 6
	if contentLines < 3 {
		contentLines = 3
	}

	tabRow := renderTabs(m.activeTab, dialogWidth)
	sep := renderSeparator(dialogWidth)
	content := renderContent(m, dialogWidth, contentLines)

	body := tabRow + "\n" + sep + "\n" + content

	dialog := styleDialog.
		Width(dialogWidth).
		Render(body)

	// Center the dialog horizontally.
	return lipgloss.Place(width, height,
		lipgloss.Center, lipgloss.Center,
		dialog)
}

// renderTabs renders the tab header row with active tab highlighted.
func renderTabs(active Tab, width int) string {
	parts := make([]string, tabCount)
	for i, name := range tabNames {
		if Tab(i) == active {
			parts[i] = styleActiveTab.Render(name)
		} else {
			parts[i] = styleDimTab.Render(name)
		}
	}
	// Join with padding and center.
	row := strings.Join(parts, "    ")
	return lipgloss.NewStyle().
		Width(width).
		Align(lipgloss.Center).
		Render(row)
}

// renderSeparator renders a horizontal line the full inner width.
func renderSeparator(width int) string {
	line := strings.Repeat("─", width)
	return styleSeparator.Render(line)
}

// renderContent renders the scrollable body for the active tab.
func renderContent(m Model, width, maxLines int) string {
	var lines []string
	switch m.activeTab {
	case TabCommands:
		lines = renderCommandLines(m.commands, width)
	case TabKeybindings:
		lines = renderKeybindingLines(m.keybindings, width)
	case TabAbout:
		lines = renderAboutLines(m.aboutLines, width)
	default:
		// Clamp to Commands on unexpected tab value.
		lines = renderCommandLines(m.commands, width)
	}

	// Apply scroll offset.
	start := m.scrollOffset
	if start < 0 {
		start = 0
	}
	if start >= len(lines) && len(lines) > 0 {
		start = len(lines) - 1
	}
	if start > 0 {
		lines = lines[start:]
	}

	// Truncate to maxLines.
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}

	// Pad to maxLines with blank lines so the dialog height is stable.
	for len(lines) < maxLines {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

// truncateDesc truncates s to maxLen runes, appending "..." if truncation occurs.
func truncateDesc(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(r[:maxLen])
	}
	return string(r[:maxLen-3]) + "..."
}

// renderCommandLines builds one line per command entry.
func renderCommandLines(cmds []CommandEntry, width int) []string {
	if len(cmds) == 0 {
		return []string{styleDimTab.Render("  (no commands registered)")}
	}

	// Find the longest name for alignment.
	maxName := 0
	for _, c := range cmds {
		if len(c.Name) > maxName {
			maxName = len(c.Name)
		}
	}

	// Name column: "  /" prefix (3 chars) + name padded to maxName + 2 spaces gap.
	nameColWidth := 3 + maxName + 2
	// Description column gets what remains of inner width.
	descColWidth := width - nameColWidth
	if descColWidth < 10 {
		descColWidth = 10
	}

	lines := make([]string, len(cmds))
	for i, c := range cmds {
		name := fmt.Sprintf("  /%-*s", maxName, c.Name)
		// Include the 2-space gap as part of the description prefix, then truncate
		// so the total rendered description (gap + text) fits within descColWidth.
		descText := truncateDesc(c.Description, descColWidth-2)
		lines[i] = styleCommandName.Render(name) +
			styleDescription.Render("  "+descText)
	}
	return lines
}

// renderKeybindingLines builds one line per key entry.
func renderKeybindingLines(keys []KeyEntry, width int) []string {
	if len(keys) == 0 {
		return []string{styleDimTab.Render("  (no keybindings registered)")}
	}

	// Find the longest key string for alignment.
	maxKey := 0
	for _, k := range keys {
		if len(k.Keys) > maxKey {
			maxKey = len(k.Keys)
		}
	}

	lines := make([]string, len(keys))
	for i, k := range keys {
		keyStr := fmt.Sprintf("  %-*s", maxKey, k.Keys)
		desc := "  " + k.Description
		lines[i] = styleKeyName.Render(keyStr) +
			styleDescription.Render(desc)
	}
	return lines
}

// renderAboutLines builds one line per about string.
func renderAboutLines(about []string, width int) []string {
	if len(about) == 0 {
		return []string{styleDimTab.Render("  (no information available)")}
	}
	lines := make([]string, len(about))
	for i, line := range about {
		lines[i] = styleAboutLine.Render("  " + line)
	}
	return lines
}
