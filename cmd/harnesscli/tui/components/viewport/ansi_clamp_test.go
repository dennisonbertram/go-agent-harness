package viewport_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"go-agent-harness/cmd/harnesscli/tui/components/viewport"
)

// TestTUI1409_ANSIStyledLineNotTruncatedByRuneCount reproduces issue #1409:
// a line containing a real ANSI escape sequence (as glamour emits under a
// real color profile) has more runes than visible cells. The viewport's
// horizontal clamp must cut by visible width, not rune count, or trailing
// visible content is lost even though the escape bytes push it past the
// rune-count budget.
func TestTUI1409_ANSIStyledLineNotTruncatedByRuneCount(t *testing.T) {
	// Styled segment "\x1b[48;2;40;40;40m calc.go \x1b[0m" renders as
	// " calc.go " (9 visible cells) but contributes many more runes than
	// that via the escape codes. The full line has 42 visible cells and
	// 62 runes, matching the diagnosis in issue #1409.
	line := "  • Created \x1b[48;2;40;40;40m calc.go \x1b[0m with an Add function"

	if got := lipgloss.Width(line); got != 42 {
		t.Fatalf("test fixture invariant broken: want visible width 42, got %d", got)
	}
	if got := len([]rune(line)); got != 62 {
		t.Fatalf("test fixture invariant broken: want rune count 62, got %d", got)
	}

	vp := viewport.New(40, 5)
	vp.AppendLine(line)
	view := vp.View()

	if !strings.Contains(view, "Add") {
		t.Errorf("visible content within the width-40 budget was dropped by the clamp; view:\n%q", view)
	}

	for i, l := range strings.Split(view, "\n") {
		if w := lipgloss.Width(l); w > 40 {
			t.Errorf("line %d exceeds viewport width: got %d cells, want <= 40 (%q)", i, w, l)
		}
	}
}
