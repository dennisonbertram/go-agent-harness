package messagebubble_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"go-agent-harness/cmd/harnesscli/tui/components/messagebubble"
)

// Issue #1407: a rendered assistant bubble must never be wider than the
// terminal and must not contain tab characters. Glamour pads lines to its
// wrap width, and the bubble then adds a 4-column indent; the overflowing
// rows wrapped in the terminal and the renderer's bookkeeping cut and merged
// lines ("• Created  calc.go  w", "ok  PASS: calcAdd/0.123s").
func TestAssistantBubble_FitsWidthAndHasNoTabs(t *testing.T) {
	md := "Done. Summary:\n\n- Created `calc.go` with an `Add` function that returns the sum of two integers.\n- Created `calc_test.go` with table-driven tests covering positive, negative, mixed signs, and zeros.\n\n```\nPASS\nok  \tcalc\t0.123s\n```\n"
	for _, width := range []int{120, 80, 60} {
		out := messagebubble.AssistantBubble{Content: md, Width: width}.View()
		for i, line := range strings.Split(out, "\n") {
			if strings.Contains(line, "\t") {
				t.Errorf("width %d line %d contains a tab: %q", width, i, line)
			}
			if w := lipgloss.Width(line); w > width {
				t.Errorf("width %d line %d is %d columns wide: %q", width, i, w, line)
			}
		}
		// Narrow widths wrap mid-phrase; compare with whitespace collapsed.
		flat := strings.Join(strings.Fields(out), " ")
		if !strings.Contains(flat, "sum of two integers") || !strings.Contains(flat, "mixed signs, and zeros") {
			t.Errorf("width %d: content lost:\n%s", width, out)
		}
	}
}
