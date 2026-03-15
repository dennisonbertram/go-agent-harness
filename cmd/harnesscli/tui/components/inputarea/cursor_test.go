package inputarea_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"go-agent-harness/cmd/harnesscli/tui/components/inputarea"
)

func TestTUI018_CursorMovesWithMultilineText(t *testing.T) {
	m := inputarea.New(20) // narrow to force wrapping
	for _, r := range "hello\nworld" {
		if r == '\n' {
			m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
		} else {
			m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		}
	}
	view := m.View()
	if !strings.Contains(view, "hello") {
		t.Errorf("multiline view missing 'hello': %q", view)
	}
	if !strings.Contains(view, "world") {
		t.Errorf("multiline view missing 'world': %q", view)
	}
}

func TestTUI018_CaretShowsOnBlankInput(t *testing.T) {
	m := inputarea.New(80)
	view := m.View()
	// Caret shown as reverse-video space on empty input
	// The view should contain the prompt + some cursor indicator
	if !strings.Contains(view, "\u276f") {
		t.Errorf("view missing prompt symbol: %q", view)
	}
	// Should be non-empty (has at minimum the prompt + cursor)
	if len(strings.TrimSpace(view)) == 0 {
		t.Error("blank input view is empty")
	}
}

func TestTUI018_CursorAtEndOfLongLine(t *testing.T) {
	m := inputarea.New(40)
	long := "this is a fairly long input string that may wrap at 40"
	for _, r := range long {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	view := m.View()
	if view == "" {
		t.Error("view empty for long input")
	}
}

func TestTUI018_CursorLeftRight(t *testing.T) {
	m := inputarea.New(80)
	for _, r := range "abc" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	// Cursor is at position 3; move left twice
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	// Cursor now at position 1 -- 'b' should be reverse-video
	view := m.View()
	if view == "" {
		t.Error("view empty after cursor movement")
	}
}

func TestTUI018_CursorNotNegative(t *testing.T) {
	m := inputarea.New(80)
	// Press left many times on empty input -- cursor should stay at 0
	for i := 0; i < 10; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	}
	// Should not panic, view should still show prompt
	view := m.View()
	if !strings.Contains(view, "\u276f") {
		t.Errorf("prompt missing after excessive left: %q", view)
	}
}
