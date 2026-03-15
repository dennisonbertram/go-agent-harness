package permissionprompt

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View renders the permission prompt as a rounded-border modal box.
//
// width is the terminal column count used to constrain the modal. The modal
// adapts to very small widths rather than panicking.
func (m Model) View(width int) string {
	if width < 20 {
		width = 20
	}

	// Inner content width: box border uses 2 cols on each side (border + space).
	const padding = 4
	innerWidth := width - padding
	if innerWidth < 10 {
		innerWidth = 10
	}

	var sb strings.Builder

	// Header lines.
	sb.WriteString(truncate(fmt.Sprintf("Allow tool: %s", m.ToolName), innerWidth))
	sb.WriteByte('\n')

	// Resource line — show amended value when in amend mode.
	resource := m.Resource
	if m.amending && m.amended != "" {
		resource = m.amended
	} else if !m.amending && m.amended != "" {
		resource = m.amended
	}
	sb.WriteString(truncate(fmt.Sprintf("Resource:   %s", resource), innerWidth))
	sb.WriteByte('\n')

	// Empty separator line.
	sb.WriteByte('\n')

	// Option list, or fallback when no options provided.
	if len(m.Options) == 0 {
		sb.WriteString("(no options available — press Esc to dismiss)")
		sb.WriteByte('\n')
	} else {
		for i, opt := range m.Options {
			cursor := "  "
			if i == m.selected {
				cursor = "> "
			}
			label := optionLabel(opt)
			sb.WriteString(cursor + truncate(label, innerWidth-2))
			sb.WriteByte('\n')
		}
	}

	// Amend-mode hint or footer.
	sb.WriteByte('\n')
	if m.amending {
		sb.WriteString(truncate(fmt.Sprintf("Amend path: %s_", m.amended), innerWidth))
	} else {
		sb.WriteString("[Tab to amend path]")
	}

	// Render content inside a rounded lipgloss border.
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(innerWidth)

	return boxStyle.Render(sb.String())
}

// truncate clips s to at most maxLen runes, appending "…" if truncated.
func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return "…"
	}
	return string(runes[:maxLen-1]) + "…"
}
