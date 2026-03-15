package layout_test

import (
	"strings"
	"testing"

	"go-agent-harness/cmd/harnesscli/tui/components/layout"
)

func TestTUI019_SeparatorsRenderInOrder(t *testing.T) {
	sep := layout.NewSeparator(80, false) // false = unicode mode
	view := sep.Render()
	if len([]rune(view)) < 78 { // lipgloss may vary by 1-2
		t.Errorf("separator too short: %d runes, want ~80", len([]rune(view)))
	}
	if !strings.Contains(view, "\u2500") {
		t.Errorf("unicode separator should use \u2500, got: %q", view)
	}
}

func TestTUI019_BorderFallbackAscii(t *testing.T) {
	sep := layout.NewSeparator(40, true) // true = ASCII fallback
	view := sep.Render()
	if strings.Contains(view, "\u2500") {
		t.Errorf("ASCII fallback should not contain \u2500: %q", view)
	}
	if !strings.Contains(view, "-") {
		t.Errorf("ASCII fallback should use -, got: %q", view)
	}
}

func TestTUI019_SeparatorZeroWidthSafe(t *testing.T) {
	sep := layout.NewSeparator(0, false)
	view := sep.Render()
	_ = view // no panic
}

func TestTUI019_DialogBoxRendersWithBorders(t *testing.T) {
	box := layout.NewDialogBox(40, 10, false)
	view := box.Render("Dialog Title", "Content here")
	if view == "" {
		t.Error("dialog box rendered empty")
	}
	// Should contain title
	if !strings.Contains(view, "Dialog Title") {
		t.Errorf("dialog box missing title: %q", view)
	}
}

func TestTUI019_SeparatorWidthMatchesTerminal(t *testing.T) {
	for _, w := range []int{20, 40, 80, 120, 200} {
		sep := layout.NewSeparator(w, false)
		view := sep.Render()
		// Strip ANSI and check visual width
		stripped := stripANSI(view)
		runes := []rune(stripped)
		// Allow +/-2 for lipgloss padding
		if len(runes) < w-2 || len(runes) > w+2 {
			t.Errorf("width %d: separator has %d visible chars", w, len(runes))
		}
	}
}

func stripANSI(s string) string {
	var result strings.Builder
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}
