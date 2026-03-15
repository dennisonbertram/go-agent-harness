package planoverlay

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Color definitions match the project theme (theme.go).
var (
	highlightColor = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	warningColor   = lipgloss.AdaptiveColor{Light: "#FFAF00", Dark: "#FFAF00"}
	specialColor   = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}
	errorColor     = lipgloss.AdaptiveColor{Light: "#FF5F87", Dark: "#FF5F87"}
	dimColor       = lipgloss.AdaptiveColor{Light: "#9B9B9B", Dark: "#5C5C5C"}
	subtleColor    = lipgloss.AdaptiveColor{Light: "#D9D9D9", Dark: "#383838"}

	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(highlightColor)

	stylePendingBadge = lipgloss.NewStyle().
				Foreground(warningColor)

	styleApprovedBadge = lipgloss.NewStyle().
				Foreground(specialColor)

	styleRejectedBadge = lipgloss.NewStyle().
				Foreground(errorColor)

	styleDim = lipgloss.NewStyle().
			Foreground(dimColor)

	styleSeparator = lipgloss.NewStyle().
			Foreground(subtleColor).
			Faint(true)

	styleFooter = lipgloss.NewStyle().
			Foreground(dimColor).
			Faint(true)

	styleBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(subtleColor).
			Padding(0, 1)
)

// View renders the plan overlay.
// Returns "" when IsVisible() is false.
func (m Model) View() string {
	if !m.IsVisible() {
		return ""
	}

	width := m.Width
	if width <= 0 {
		width = 80
	}
	height := m.Height
	if height <= 0 {
		height = 20
	}

	// Inner width: border (2) + padding (2 each side = 4) = 6 total overhead.
	const overhead = 6
	innerWidth := width - overhead
	if innerWidth < 10 {
		innerWidth = 10
	}

	// ── Header ────────────────────────────────────────────────────────────────
	titlePart := styleHeader.Render("📋 Plan Mode")
	badge := stateBadge(m.State)

	// Right-align the badge within the inner width.
	titleLen := lipgloss.Width(titlePart)
	badgeLen := lipgloss.Width(badge)
	gapLen := innerWidth - titleLen - badgeLen
	if gapLen < 1 {
		gapLen = 1
	}
	headerLine := titlePart + strings.Repeat(" ", gapLen) + badge

	// ── Separator ─────────────────────────────────────────────────────────────
	sep := styleSeparator.Render(strings.Repeat("─", innerWidth))

	// ── Scrollable content ────────────────────────────────────────────────────
	// Reserve rows: header (1) + sep (1) + footer hint (1 if pending) + "more" footer (1).
	footerRows := 1 // "more lines" footer slot
	if m.State == PlanStatePending {
		footerRows = 2 // hint + more-lines slot
	}
	contentHeight := height - 2 - footerRows // 2 for border top/bottom
	if contentHeight < 1 {
		contentHeight = 1
	}
	// Subtract the header and separator rows from available content space.
	visibleLines := contentHeight - 2 // header row + sep row
	if visibleLines < 1 {
		visibleLines = 1
	}

	allLines := splitLines(m.PlanText)
	totalLines := len(allLines)

	start := m.ScrollOffset
	if start < 0 {
		start = 0
	}
	if start >= totalLines && totalLines > 0 {
		start = totalLines - 1
	}

	end := start + visibleLines
	if end > totalLines {
		end = totalLines
	}

	var contentLines []string
	if totalLines == 0 {
		contentLines = []string{styleDim.Render("(no plan text)")}
	} else {
		visible := allLines[start:end]
		for _, l := range visible {
			contentLines = append(contentLines, l)
		}
	}

	// Pad content to visibleLines so the box height is stable.
	for len(contentLines) < visibleLines {
		contentLines = append(contentLines, "")
	}

	// ── "More lines" footer ───────────────────────────────────────────────────
	remaining := totalLines - end
	var moreFooter string
	if remaining > 0 {
		moreFooter = styleDim.Render(fmt.Sprintf("  ... %d more line(s)", remaining))
	}

	// ── Hint footer (pending only) ────────────────────────────────────────────
	var hintFooter string
	if m.State == PlanStatePending {
		hintFooter = styleFooter.Render("  y approve  n reject  ↑/↓ scroll")
	}

	// ── Assemble body ─────────────────────────────────────────────────────────
	var sb strings.Builder
	sb.WriteString(headerLine)
	sb.WriteByte('\n')
	sb.WriteString(sep)
	sb.WriteByte('\n')
	sb.WriteString(strings.Join(contentLines, "\n"))
	if moreFooter != "" {
		sb.WriteByte('\n')
		sb.WriteString(moreFooter)
	} else {
		sb.WriteByte('\n')
		// blank placeholder so height is consistent
	}
	if hintFooter != "" {
		sb.WriteByte('\n')
		sb.WriteString(hintFooter)
	}

	rendered := styleBox.Width(innerWidth).Render(sb.String())
	return rendered
}

// stateBadge returns the colored status badge string for a given PlanState.
func stateBadge(state PlanState) string {
	switch state {
	case PlanStatePending:
		return stylePendingBadge.Render("[Awaiting Approval]")
	case PlanStateApproved:
		return styleApprovedBadge.Render("[Approved ✓]")
	case PlanStateRejected:
		return styleRejectedBadge.Render("[Rejected ✗]")
	default:
		return ""
	}
}

// splitLines splits text into individual lines.
func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}
