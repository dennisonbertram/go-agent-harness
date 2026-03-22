package profilepicker_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"go-agent-harness/cmd/harnesscli/tui/components/profilepicker"
)

var testEntries = []profilepicker.ProfileEntry{
	{Name: "alpha", Description: "Alpha profile", Model: "gpt-4", ToolCount: 5, SourceTier: "project"},
	{Name: "beta", Description: "Beta profile", Model: "claude-opus-4-6", ToolCount: 10, SourceTier: "built-in"},
	{Name: "gamma", Description: "Gamma profile", Model: "gpt-4o", ToolCount: 3, SourceTier: "user"},
}

// TestNew verifies that New creates a model with the given entries and closed state.
func TestNew(t *testing.T) {
	m := profilepicker.New(testEntries)
	if m.IsOpen() {
		t.Error("New model should start closed")
	}
	_, ok := m.Selected()
	if !ok {
		t.Error("New model with entries should return Selected() ok=true")
	}
}

// TestNew_Empty verifies that New with empty entries works and Selected returns false.
func TestNew_Empty(t *testing.T) {
	m := profilepicker.New(nil)
	if m.IsOpen() {
		t.Error("New model should start closed")
	}
	_, ok := m.Selected()
	if ok {
		t.Error("New model with no entries should return Selected() ok=false")
	}
}

// TestOpen closes/opens state.
func TestOpen(t *testing.T) {
	m := profilepicker.New(testEntries)
	m2 := m.Open()
	if !m2.IsOpen() {
		t.Error("Open() should set IsOpen() to true")
	}
	// Original is not modified (value semantics).
	if m.IsOpen() {
		t.Error("Open() should not mutate original")
	}
}

// TestClose verifies Close sets IsOpen() to false.
func TestClose(t *testing.T) {
	m := profilepicker.New(testEntries).Open()
	m2 := m.Close()
	if m2.IsOpen() {
		t.Error("Close() should set IsOpen() to false")
	}
	// Value semantics: original remains open.
	if !m.IsOpen() {
		t.Error("Close() should not mutate original")
	}
}

// TestSetEntries replaces entries and resets selection.
func TestSetEntries(t *testing.T) {
	m := profilepicker.New(testEntries)
	// Move selection down.
	m = m.SelectDown().SelectDown()
	entry, _ := m.Selected()
	if entry.Name != "gamma" {
		t.Fatalf("expected gamma selected after 2 SelectDown, got %q", entry.Name)
	}

	newEntries := []profilepicker.ProfileEntry{
		{Name: "new1", Description: "New 1", Model: "gpt-4"},
		{Name: "new2", Description: "New 2", Model: "gpt-4o"},
	}
	m = m.SetEntries(newEntries)
	entry, ok := m.Selected()
	if !ok {
		t.Fatal("SetEntries should reset to valid selection")
	}
	if entry.Name != "new1" {
		t.Errorf("SetEntries should reset selection to 0; got %q", entry.Name)
	}
}

// TestSelectUp wraps around to the last entry.
func TestSelectUp(t *testing.T) {
	m := profilepicker.New(testEntries)
	// At index 0, SelectUp should wrap to last (index 2 = gamma).
	m = m.SelectUp()
	entry, _ := m.Selected()
	if entry.Name != "gamma" {
		t.Errorf("SelectUp() at index 0 should wrap to last; got %q", entry.Name)
	}
}

// TestSelectDown advances selection and wraps around.
func TestSelectDown(t *testing.T) {
	m := profilepicker.New(testEntries)
	m = m.SelectDown()
	entry, _ := m.Selected()
	if entry.Name != "beta" {
		t.Errorf("SelectDown() at 0 should yield beta; got %q", entry.Name)
	}

	// Advance to last and wrap.
	m = m.SelectDown().SelectDown()
	entry, _ = m.Selected()
	if entry.Name != "alpha" {
		t.Errorf("SelectDown() wrap should return alpha; got %q", entry.Name)
	}
}

// TestSelectUp_Empty does not panic on empty entries list.
func TestSelectUp_Empty(t *testing.T) {
	m := profilepicker.New(nil)
	m2 := m.SelectUp() // should be a no-op
	if m2.IsOpen() != m.IsOpen() {
		t.Error("SelectUp on empty should be no-op")
	}
}

// TestSelectDown_Empty does not panic on empty entries list.
func TestSelectDown_Empty(t *testing.T) {
	m := profilepicker.New(nil)
	m2 := m.SelectDown() // should be a no-op
	_ = m2
}

// TestUpdate_KeyUp routes Up key to SelectUp.
func TestUpdate_KeyUp(t *testing.T) {
	m := profilepicker.New(testEntries).Open()
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	entry, _ := m2.Selected()
	// was at alpha (0), up → gamma (2).
	if entry.Name != "gamma" {
		t.Errorf("Up key should move to gamma (wrap); got %q", entry.Name)
	}
}

// TestUpdate_KeyDown routes Down key to SelectDown.
func TestUpdate_KeyDown(t *testing.T) {
	m := profilepicker.New(testEntries).Open()
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	entry, _ := m2.Selected()
	if entry.Name != "beta" {
		t.Errorf("Down key should move to beta; got %q", entry.Name)
	}
}

// TestUpdate_KeyVimJ routes 'j' to SelectDown.
func TestUpdate_KeyVimJ(t *testing.T) {
	m := profilepicker.New(testEntries).Open()
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	entry, _ := m2.Selected()
	if entry.Name != "beta" {
		t.Errorf("'j' key should move to beta; got %q", entry.Name)
	}
}

// TestUpdate_KeyVimK routes 'k' to SelectUp.
func TestUpdate_KeyVimK(t *testing.T) {
	m := profilepicker.New(testEntries).Open()
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	entry, _ := m2.Selected()
	if entry.Name != "gamma" {
		t.Errorf("'k' key should wrap to gamma; got %q", entry.Name)
	}
}

// TestUpdate_KeyEnter emits ProfileSelectedMsg with the current selection.
func TestUpdate_KeyEnter(t *testing.T) {
	m := profilepicker.New(testEntries).Open()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter key should return a non-nil tea.Cmd")
	}
	msg := cmd()
	sel, ok := msg.(profilepicker.ProfileSelectedMsg)
	if !ok {
		t.Fatalf("Enter key cmd should emit ProfileSelectedMsg; got %T", msg)
	}
	if sel.Entry.Name != "alpha" {
		t.Errorf("ProfileSelectedMsg.Entry.Name: want %q, got %q", "alpha", sel.Entry.Name)
	}
}

// TestUpdate_KeyEnter_Empty does not emit when list is empty.
func TestUpdate_KeyEnter_Empty(t *testing.T) {
	m := profilepicker.New(nil).Open()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("Enter with empty list should return nil cmd")
	}
}

// TestUpdate_KeyEsc closes the picker.
func TestUpdate_KeyEsc(t *testing.T) {
	m := profilepicker.New(testEntries).Open()
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m2.IsOpen() {
		t.Error("Escape key should close the picker")
	}
}

// TestUpdate_WhenClosed ignores all keys.
func TestUpdate_WhenClosed(t *testing.T) {
	m := profilepicker.New(testEntries) // closed
	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m2.IsOpen() {
		t.Error("Closed model should ignore Enter")
	}
	if cmd != nil {
		t.Error("Closed model should not emit commands")
	}
}

// TestScrollLogic verifies that scroll adjusts when going past the visible window.
func TestScrollLogic(t *testing.T) {
	// Create 15 entries to exceed maxVisibleRows (10).
	entries := make([]profilepicker.ProfileEntry, 15)
	for i := range entries {
		entries[i] = profilepicker.ProfileEntry{
			Name:        string(rune('a' + i)),
			Description: "desc",
			Model:       "gpt-4",
		}
	}
	m := profilepicker.New(entries).Open()
	// Navigate down past maxVisibleRows.
	for i := 0; i < 11; i++ {
		m = m.SelectDown()
	}
	entry, _ := m.Selected()
	// After 11 SelectDown from 0, we should be at index 11 (letter 'l').
	if entry.Name != "l" {
		t.Errorf("After 11 SelectDown, expected 'l'; got %q", entry.Name)
	}
	// The view should not panic.
	m.Width = 80
	_ = m.View()
}

// TestView_Closed returns empty string when not open.
func TestView_Closed(t *testing.T) {
	m := profilepicker.New(testEntries)
	m.Width = 80
	v := m.View()
	if v != "" {
		t.Errorf("View() when closed should return empty string; got %q", v)
	}
}

// TestView_Open renders box with entries.
func TestView_Open(t *testing.T) {
	m := profilepicker.New(testEntries).Open()
	m.Width = 80
	v := m.View()
	if v == "" {
		t.Error("View() when open should return non-empty string")
	}
	// Should contain profile names.
	for _, e := range testEntries {
		if !containsStr(v, e.Name) {
			t.Errorf("View() should contain profile name %q", e.Name)
		}
	}
}

// TestView_Empty shows "no profiles" message.
func TestView_Empty(t *testing.T) {
	m := profilepicker.New(nil).Open()
	m.Width = 80
	v := m.View()
	if v == "" {
		t.Error("View() with no profiles should return non-empty box")
	}
	if !containsStr(v, "No profiles") {
		t.Errorf("View() with no entries should contain 'No profiles'; got:\n%s", v)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || findSubstr(s, sub))
}

func findSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
