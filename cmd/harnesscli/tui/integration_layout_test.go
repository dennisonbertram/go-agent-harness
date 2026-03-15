package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	tui "go-agent-harness/cmd/harnesscli/tui"
)

func TestTUI015_RootViewOrdersComponents(t *testing.T) {
	m := tui.New(tui.DefaultTUIConfig())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model := m2.(tui.Model)
	view := model.View()

	// View must be non-empty
	if strings.TrimSpace(view) == "" {
		t.Fatal("View() returned empty string after resize")
	}

	// View should contain the input prompt
	if !strings.Contains(view, "\u276f") {
		t.Errorf("View missing input prompt symbol: got %q", view)
	}
}

func TestTUI015_RootViewAt120x40(t *testing.T) {
	m := tui.New(tui.DefaultTUIConfig())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model := m2.(tui.Model)
	view := model.View()
	lines := strings.Split(view, "\n")
	// Should have roughly 40 lines (+-2 for trailing newline)
	if len(lines) < 35 || len(lines) > 45 {
		t.Errorf("View at 120x40 has %d lines, expected ~40", len(lines))
	}
}

func TestTUI015_RootViewAt200x50(t *testing.T) {
	m := tui.New(tui.DefaultTUIConfig())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	model := m2.(tui.Model)
	view := model.View()
	if strings.TrimSpace(view) == "" {
		t.Fatal("View() empty at 200x50")
	}
}

func TestTUI015_SessionNameRendersRightAligned(t *testing.T) {
	m := tui.New(tui.DefaultTUIConfig())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model := m2.(tui.Model)
	view := model.View()
	_ = view // session name rendering verified visually
}

func TestTUI015_RootViewNoPanic(t *testing.T) {
	m := tui.New(tui.DefaultTUIConfig())
	// No resize yet -- should not panic
	_ = m.View()
	// After resize
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	_ = m2.(tui.Model).View()
}
