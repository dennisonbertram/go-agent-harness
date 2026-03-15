package costdisplay

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// dimColor is the adaptive color used for dimmed/secondary text.
var dimColor = lipgloss.AdaptiveColor{Light: "#9B9B9B", Dark: "#5C5C5C"}

// dimStyle renders secondary fields (token counts, arrows) in a faint color.
var dimStyle = lipgloss.NewStyle().Foreground(dimColor).Faint(true)

// faintStyle renders text with faint attribute only (model name).
var faintStyle = lipgloss.NewStyle().Faint(true)

// FormatTokens formats an integer with comma thousands separators.
// Examples: 1234 → "1,234", 1000000 → "1,000,000"
func FormatTokens(n int) string {
	s := strconv.Itoa(n)
	if len(s) <= 3 {
		return s
	}

	var b strings.Builder
	start := len(s) % 3
	if start == 0 {
		start = 3
	}
	b.WriteString(s[:start])
	for i := start; i < len(s); i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

// FormatCost formats a float64 USD amount as "$X.XXXX" (4 decimal places).
func FormatCost(usd float64) string {
	return fmt.Sprintf("$%.4f", usd)
}

// View renders a one-line cost summary bar.
// Returns "" when Visible is false.
//
// Format:  ↑ 1,234 in  ↓ 567 out  $0.0123  [gpt-4.1-mini]
func (m Model) View() string {
	if !m.Visible {
		return ""
	}

	snap := m.Snapshot

	inPart := dimStyle.Render("↑") + " " + dimStyle.Render(FormatTokens(snap.InputTokens)+" in")
	outPart := dimStyle.Render("↓") + " " + dimStyle.Render(FormatTokens(snap.OutputTokens)+" out")
	costPart := dimStyle.Render(FormatCost(snap.TotalCostUSD))

	parts := []string{inPart, outPart, costPart}

	if snap.Model != "" {
		modelPart := faintStyle.Render("[" + snap.Model + "]")
		parts = append(parts, modelPart)
	}

	line := "  " + strings.Join(parts, "  ")

	w := m.Width
	if w <= 0 {
		return line
	}

	// Right-align to Width using lipgloss.
	visLen := visibleLen(line)
	if visLen < w {
		padding := w - visLen
		line = strings.Repeat(" ", padding) + line
	}

	return line
}

// visibleLen returns the display width of s after stripping ANSI sequences.
func visibleLen(s string) int {
	return len([]rune(stripANSI(s)))
}

// stripANSI removes ANSI escape sequences from s.
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
