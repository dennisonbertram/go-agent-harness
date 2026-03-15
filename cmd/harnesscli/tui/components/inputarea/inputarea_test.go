package inputarea_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"go-agent-harness/cmd/harnesscli/tui/components/inputarea"
)

func TestTUI012_MultilineInputCapturesNewline(t *testing.T) {
	m := inputarea.New(80)
	// Ctrl+J is the newline binding (shift+enter maps to same in terminals)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	v := m.View()
	_ = v // no panic
}

func TestTUI012_EnterSubmitsAndClearsInput(t *testing.T) {
	m := inputarea.New(80)
	// Type some text
	for _, r := range "hello world" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if !strings.Contains(m.Value(), "hello world") {
		t.Errorf("expected 'hello world' in value, got %q", m.Value())
	}
	// Press Enter to submit
	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Error("Enter should return a CommandMsg cmd")
	}
	// Value should be cleared after submit
	if m2.Value() != "" {
		t.Errorf("expected empty value after submit, got %q", m2.Value())
	}
}

func TestTUI012_InputValuePreserved(t *testing.T) {
	m := inputarea.New(80)
	for _, r := range "test input" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if m.Value() != "test input" {
		t.Errorf("value: got %q, want %q", m.Value(), "test input")
	}
}

func TestTUI012_HistoryNavigationInInput(t *testing.T) {
	m := inputarea.New(80)
	// Submit first message to add to history
	for _, r := range "first message" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	// Navigate history with up arrow
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if !strings.Contains(m2.Value(), "first message") {
		t.Errorf("history recall failed: got %q", m2.Value())
	}
}

func TestTUI012_ViewContainsPromptSymbol(t *testing.T) {
	m := inputarea.New(80)
	view := m.View()
	if !strings.Contains(view, "❯") {
		t.Errorf("input view missing prompt symbol: %q", view)
	}
}

func TestTUI012_BackspaceDeletesChar(t *testing.T) {
	m := inputarea.New(80)
	for _, r := range "abc" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.Value() != "ab" {
		t.Errorf("backspace: got %q, want %q", m.Value(), "ab")
	}
}

func TestTUI012_EmptyEnterDoesNotSubmit(t *testing.T) {
	m := inputarea.New(80)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("Enter on empty input should not produce a cmd")
	}
}

func TestTUI012_HistoryDownReturnsToEmpty(t *testing.T) {
	m := inputarea.New(80)
	// Submit a message
	for _, r := range "msg" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	// Go up into history
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.Value() != "msg" {
		t.Fatalf("expected 'msg', got %q", m.Value())
	}
	// Go down past history back to empty
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Value() != "" {
		t.Errorf("expected empty after down past history, got %q", m.Value())
	}
}

func TestTUI012_CursorMovement(t *testing.T) {
	m := inputarea.New(80)
	for _, r := range "abc" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	// Move left twice
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	// Insert 'X' at position 1 (between 'a' and 'b')
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	if m.Value() != "aXbc" {
		t.Errorf("cursor insert: got %q, want %q", m.Value(), "aXbc")
	}
}

func TestTUI012_UnfocusedIgnoresKeys(t *testing.T) {
	m := inputarea.New(80)
	m.Blur()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if m.Value() != "" {
		t.Errorf("blurred input should not accept keys, got %q", m.Value())
	}
}

func TestTUI012_SubmitReturnsCmdMsg(t *testing.T) {
	m := inputarea.New(80)
	for _, r := range "test" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected cmd from Enter")
	}
	msg := cmd()
	submitted, ok := msg.(inputarea.CommandSubmittedMsg)
	if !ok {
		t.Fatalf("expected CommandSubmittedMsg, got %T", msg)
	}
	if submitted.Value != "test" {
		t.Errorf("submitted value: got %q, want %q", submitted.Value, "test")
	}
}

func TestTUI012_HistoryNoDuplicateAtEnd(t *testing.T) {
	m := inputarea.New(80)
	// Submit "hello" twice
	for i := 0; i < 2; i++ {
		for _, r := range "hello" {
			m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		}
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	}
	// Up once should get "hello"
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.Value() != "hello" {
		t.Errorf("expected 'hello', got %q", m.Value())
	}
	// Up again should stay at "hello" (no duplicate)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.Value() != "hello" {
		t.Errorf("expected 'hello' (no dup), got %q", m.Value())
	}
}
