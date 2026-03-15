package configpanel

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Style constants for the config panel.
var (
	styleDialog = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240"))

	styleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255"))

	styleSearch = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244"))

	styleSeparator = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	styleSelectedRow = lipgloss.NewStyle().
				Reverse(true)

	styleKey = lipgloss.NewStyle().
			Foreground(lipgloss.Color("33")).
			Bold(true)

	styleValue = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	styleDirtyMarker = lipgloss.NewStyle().
				Foreground(lipgloss.Color("214"))

	styleBadgeRW = lipgloss.NewStyle().
			Foreground(lipgloss.Color("70"))

	styleBadgeRO = lipgloss.NewStyle().
			Faint(true).
			Foreground(lipgloss.Color("244"))

	styleEditBuf = lipgloss.NewStyle().
			Foreground(lipgloss.Color("220")).
			Underline(true)

	styleFooter = lipgloss.NewStyle().
			Faint(true).
			Foreground(lipgloss.Color("244"))

	styleEmpty = lipgloss.NewStyle().
			Faint(true).
			Foreground(lipgloss.Color("244"))
)

// render produces the full config panel string at the given dimensions.
func render(m Model, width, height int) string {
	// Dialog inner width: cap at 80, minimum 30, accounting for border (2).
	dialogWidth := width - 4
	if dialogWidth > 78 {
		dialogWidth = 78
	}
	if dialogWidth < 28 {
		dialogWidth = 28
	}

	// Content lines: height minus border(2) + header(1) + separator(1) + footer(1) = 5 overhead.
	contentLines := height - 7
	if contentLines < 2 {
		contentLines = 2
	}

	header := renderHeader(m, dialogWidth)
	sep := renderSeparator(dialogWidth)
	content := renderContent(m, dialogWidth, contentLines)
	footer := renderFooter(m, dialogWidth)

	body := header + "\n" + sep + "\n" + content + "\n" + footer

	dialog := styleDialog.
		Width(dialogWidth).
		Render(body)

	return lipgloss.Place(width, height,
		lipgloss.Center, lipgloss.Center,
		dialog)
}

// renderHeader renders the title row with optional search query.
func renderHeader(m Model, width int) string {
	title := styleTitle.Render("Config")
	var searchPart string
	if m.query != "" {
		searchPart = "  " + styleSearch.Render("/ "+m.query)
	} else {
		searchPart = "  " + styleSearch.Render("/ search...")
	}
	row := title + searchPart
	return lipgloss.NewStyle().Width(width).Render(row)
}

// renderSeparator renders a horizontal line the full inner width.
func renderSeparator(width int) string {
	line := strings.Repeat("─", width)
	return styleSeparator.Render(line)
}

// renderContent renders the scrollable list of config entries.
func renderContent(m Model, width, maxLines int) string {
	if len(m.filtered) == 0 {
		empty := styleEmpty.Render("  (no entries)")
		lines := []string{empty}
		for len(lines) < maxLines {
			lines = append(lines, "")
		}
		return strings.Join(lines, "\n")
	}

	// Compute column widths.
	maxKeyLen := 0
	maxValLen := 0
	for _, e := range m.filtered {
		if len(e.Key) > maxKeyLen {
			maxKeyLen = len(e.Key)
		}
		valLen := len(e.Value)
		if valLen > maxValLen {
			maxValLen = valLen
		}
	}
	// Cap value column to avoid overflow.
	maxValDisplay := 20
	if maxValLen > maxValDisplay {
		maxValLen = maxValDisplay
	}

	// Build all row strings.
	allLines := make([]string, len(m.filtered))
	for i, e := range m.filtered {
		allLines[i] = renderRow(e, i == m.selected, m.editing && i == m.selected, m.editBuf,
			maxKeyLen, maxValLen, width)
	}

	// Apply scroll offset.
	start := m.scrollOffset
	if start < 0 {
		start = 0
	}
	if start >= len(allLines) && len(allLines) > 0 {
		start = len(allLines) - 1
	}
	visible := allLines[start:]
	if len(visible) > maxLines {
		visible = visible[:maxLines]
	}

	// Pad with blank lines to keep dialog height stable.
	for len(visible) < maxLines {
		visible = append(visible, "")
	}

	return strings.Join(visible, "\n")
}

// renderRow renders one config entry line.
func renderRow(e ConfigEntry, selected, editing bool, editBuf string, maxKeyLen, maxValLen, width int) string {
	// Prefix: "> " for selected, "  " otherwise.
	prefix := "  "
	if selected {
		prefix = "> "
	}

	// Key column.
	keyStr := fmt.Sprintf("%-*s", maxKeyLen, e.Key)

	// Value column: show edit buffer if in edit mode, else the current value.
	var valStr string
	if editing {
		valStr = fmt.Sprintf("%-*s", maxValLen, editBuf+"_")
	} else {
		v := e.Value
		if len(v) > maxValLen {
			v = v[:maxValLen]
		}
		valStr = fmt.Sprintf("%-*s", maxValLen, v)
	}

	// Dirty marker.
	dirtyStr := " "
	if e.Dirty {
		dirtyStr = "*"
	}

	// Badge.
	var badge string
	if e.ReadOnly {
		badge = styleBadgeRO.Render("[RO]")
	} else {
		badge = styleBadgeRW.Render("[RW]")
	}

	// Assemble the row.
	var row string
	if selected {
		keyRendered := keyStr
		var valRendered string
		if editing {
			valRendered = styleEditBuf.Render(valStr)
		} else {
			valRendered = valStr
		}
		inner := prefix + keyRendered + "  " + valRendered + dirtyStr + " " + badge
		row = styleSelectedRow.Render(inner)
	} else {
		keyRendered := styleKey.Render(keyStr)
		valRendered := styleValue.Render(valStr)
		dirtyRendered := dirtyStr
		if e.Dirty {
			dirtyRendered = styleDirtyMarker.Render("*")
		}
		row = prefix + keyRendered + "  " + valRendered + dirtyRendered + " " + badge
	}

	return row
}

// renderFooter renders the keybinding hint at the bottom.
func renderFooter(m Model, width int) string {
	var hint string
	if m.editing {
		hint = "[Enter] commit  [Esc] cancel"
	} else {
		hint = "[Enter] edit  [Esc] close"
	}
	return lipgloss.NewStyle().
		Width(width).
		Align(lipgloss.Center).
		Render(styleFooter.Render(hint))
}
