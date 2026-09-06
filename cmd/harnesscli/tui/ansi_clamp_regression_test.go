package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TestTUI1409_ModelViewPreservesANSIStyledTranscriptLine is a regression
// guard for issue #1409 at the assembled tui.Model integration point, not
// just the standalone viewport package.
//
// Background: glamour (via messagebubble.RenderMarkdown) only emits real
// ANSI escape sequences when os.Stdout is a real terminal — messagebubble's
// stdoutIsTerminal probe falls back to the escape-free "notty" style
// otherwise, which is also why forcing lipgloss's global color profile
// (lipgloss.SetColorProfile(termenv.TrueColor)) does not make glamour emit
// escapes in this test process (verified experimentally: RenderMarkdown
// still produced zero ESC bytes for a code-span line with that profile set).
// That probe lives in the messagebubble package, which is out of scope for
// this fix, so this test injects a realistic ANSI-styled transcript line
// directly into the model's viewport (m.vp) — the same line shape
// messagebubble/glamour produce for a markdown list item with a styled code
// span under a real terminal — and exercises it through the full,
// assembled Model.View() (header/separators/viewport/input/status bar),
// not just viewport.Model.View() in isolation.
func TestTUI1409_ModelViewPreservesANSIStyledTranscriptLine(t *testing.T) {
	cfg := DefaultTUIConfig()
	m := New(cfg)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 24})
	model := m2.(Model)

	// Same shape as the proven issue #1409 example: a glamour list item with
	// a truecolor-background code span. Visible width 42, rune count 62 —
	// the naive rune-count clamp this issue fixed would slice mid-escape at
	// [:40] runes and drop "with an Add function" entirely.
	styledLine := "  • Created \x1b[48;2;40;40;40m calc.go \x1b[0m with an Add function"
	model.vp.AppendLine(styledLine)

	view := model.View()

	if !strings.Contains(view, "Add") {
		t.Errorf("assembled Model.View() dropped visible transcript content past an ANSI escape; view:\n%q", view)
	}

	for i, l := range strings.Split(view, "\n") {
		if w := lipgloss.Width(l); w > model.width {
			t.Errorf("line %d of assembled Model.View() exceeds width: got %d cells, want <= %d (%q)", i, w, model.width, l)
		}
	}
}
