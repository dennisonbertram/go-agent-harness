package inputarea_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"go-agent-harness/cmd/harnesscli/tui/components/inputarea"
)

func TestInputAreaCoverageHelpers(t *testing.T) {
	t.Parallel()

	m := inputarea.New(40)
	if m.HistoryState().Len() != 0 {
		t.Fatalf("expected empty history, got %d entries", m.HistoryState().Len())
	}

	m = m.SetValue("first line\nsecond line")
	m.SetWidth(12)
	m.Blur()
	m.Focus()

	view := m.MultilineView(1)
	if view == "" {
		t.Fatal("expected multiline view to render content")
	}
	if strings.Contains(view, "\n") {
		t.Fatalf("expected single visible line, got %q", view)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'!'}})
	if !strings.HasSuffix(m.Value(), "!") {
		t.Fatalf("expected focused input to accept keys, got %q", m.Value())
	}

	if cmd := m.Init(); cmd != nil {
		t.Fatalf("expected nil init cmd, got %v", cmd)
	}
}
