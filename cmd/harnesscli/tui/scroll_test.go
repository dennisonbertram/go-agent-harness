package tui_test

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	tui "go-agent-harness/cmd/harnesscli/tui"
)

// newScrollTestModel returns a Model that has received a WindowSizeMsg so all
// layout/viewport dimensions are initialized, sized to given dimensions.
func newScrollTestModel(width, height int) tui.Model {
	m := tui.New(tui.DefaultTUIConfig())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return m2.(tui.Model)
}

// appendNLines sends AssistantDeltaMsg events to add n distinct lines to the
// viewport, which forces the content to exceed the viewport height.
// Each delta ends with "\n" so that AppendChunk starts a new line per message.
func appendNLines(m tui.Model, n int) tui.Model {
	for i := 0; i < n; i++ {
		msg := tui.AssistantDeltaMsg{Delta: fmt.Sprintf("line %d\n", i)}
		m2, _ := m.Update(msg)
		m = m2.(tui.Model)
	}
	return m
}

// TestModel_ScrollUp_MovesViewport verifies that sending a PgUp key causes
// the viewport scroll offset to increase (i.e., it scrolled away from the
// bottom).
func TestModel_ScrollUp_MovesViewport(t *testing.T) {
	m := newScrollTestModel(80, 30)
	// Fill viewport with enough lines so scrolling is possible.
	m = appendNLines(m, 50)

	// Verify we start at the bottom.
	if !m.ViewportAtBottom() {
		t.Fatal("expected viewport to start at the bottom")
	}

	// Send PgUp (pgup key).
	pgUp := tea.KeyMsg{Type: tea.KeyPgUp}
	m2, _ := m.Update(pgUp)
	m = m2.(tui.Model)

	if m.ViewportAtBottom() {
		t.Error("expected viewport to no longer be at the bottom after PgUp")
	}
	if m.ViewportScrollOffset() <= 0 {
		t.Errorf("expected scroll offset > 0 after PgUp, got %d", m.ViewportScrollOffset())
	}
}

// TestModel_ScrollDown_FromScrolledUp verifies that scrolling up and then
// sending PgDown reduces the scroll offset.
func TestModel_ScrollDown_FromScrolledUp(t *testing.T) {
	m := newScrollTestModel(80, 30)
	m = appendNLines(m, 50)

	// Scroll up twice using PgUp.
	pgUp := tea.KeyMsg{Type: tea.KeyPgUp}
	m2, _ := m.Update(pgUp)
	m = m2.(tui.Model)
	m2, _ = m.Update(pgUp)
	m = m2.(tui.Model)

	offsetAfterUp := m.ViewportScrollOffset()
	if offsetAfterUp <= 0 {
		t.Fatalf("expected offset > 0 after two PgUp presses, got %d", offsetAfterUp)
	}

	// Send PgDown and verify offset decreased.
	pgDown := tea.KeyMsg{Type: tea.KeyPgDown}
	m2, _ = m.Update(pgDown)
	m = m2.(tui.Model)

	offsetAfterDown := m.ViewportScrollOffset()
	if offsetAfterDown >= offsetAfterUp {
		t.Errorf("expected offset to decrease after PgDown: before=%d after=%d",
			offsetAfterUp, offsetAfterDown)
	}
}

// TestModel_ScrollKeys_NotPassedToInput verifies that PgUp is consumed by the
// scroll handler and is NOT forwarded to the input area. The input area value
// should remain empty after a PgUp press.
func TestModel_ScrollKeys_NotPassedToInput(t *testing.T) {
	m := newScrollTestModel(80, 30)
	m = appendNLines(m, 50)

	// Ensure input is empty to start.
	if m.Input() != "" {
		t.Fatal("expected input to be empty before sending PgUp")
	}

	// Send PgUp.
	pgUp := tea.KeyMsg{Type: tea.KeyPgUp}
	m2, _ := m.Update(pgUp)
	m = m2.(tui.Model)

	if m.Input() != "" {
		t.Errorf("expected input to remain empty after PgUp; got %q", m.Input())
	}
}

// TestRegression_ViewportScrollWired verifies that after several content lines
// are appended, PgUp scrolls up and PgDown returns to the bottom.
func TestRegression_ViewportScrollWired(t *testing.T) {
	m := newScrollTestModel(80, 30)

	// Simulate three ping/pong turns by appending assistant deltas.
	// Each delta ends with "\n" so AppendChunk starts a new line per message.
	for turn := 0; turn < 3; turn++ {
		for line := 0; line < 10; line++ {
			msg := tui.AssistantDeltaMsg{Delta: fmt.Sprintf("turn %d line %d\n", turn, line)}
			m2, _ := m.Update(msg)
			m = m2.(tui.Model)
		}
	}

	if !m.ViewportAtBottom() {
		t.Fatal("expected viewport to be at the bottom after appending content")
	}

	// PgUp should scroll up.
	pgUp := tea.KeyMsg{Type: tea.KeyPgUp}
	m2, _ := m.Update(pgUp)
	m = m2.(tui.Model)

	if m.ViewportAtBottom() {
		t.Error("expected viewport NOT to be at the bottom after PgUp")
	}

	// PgDown should return to the bottom.
	pgDown := tea.KeyMsg{Type: tea.KeyPgDown}
	// May need multiple PgDown presses to fully return — call until AtBottom
	// or exhaust attempts.
	for i := 0; i < 20; i++ {
		if m.ViewportAtBottom() {
			break
		}
		m2, _ = m.Update(pgDown)
		m = m2.(tui.Model)
	}

	if !m.ViewportAtBottom() {
		t.Errorf("expected viewport to be at the bottom after PgDown; offset=%d",
			m.ViewportScrollOffset())
	}
}

// TestModel_ArrowKeys_NavigateHistory verifies that up/down arrow keys navigate
// command history (not viewport) when no overlay or dropdown is active.
// Viewport scrolling is handled by PgUp/PgDown — see TestModel_ScrollUp_MovesViewport.
func TestModel_ArrowKeys_NavigateHistory(t *testing.T) {
	m := newScrollTestModel(80, 30)
	m = appendNLines(m, 50)

	// Submit a command so history has an entry
	m = typeIntoModel(m, "scroll-test-cmd")
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m2.(tui.Model)

	// Verify viewport starts at bottom and input is empty
	if m.Input() != "" {
		t.Fatalf("pre-condition: input should be empty after submit, got %q", m.Input())
	}
	scrollBefore := m.ViewportScrollOffset()

	// Up arrow should navigate history (not scroll viewport)
	upKey := tea.KeyMsg{Type: tea.KeyUp}
	m2, _ = m.Update(upKey)
	m = m2.(tui.Model)

	if m.Input() != "scroll-test-cmd" {
		t.Errorf("expected Up to navigate history; want 'scroll-test-cmd', got %q", m.Input())
	}
	if m.ViewportScrollOffset() != scrollBefore {
		t.Errorf("expected viewport NOT to scroll when Up navigates history; offset changed from %d to %d",
			scrollBefore, m.ViewportScrollOffset())
	}

	// Down arrow should navigate back to empty draft
	downKey := tea.KeyMsg{Type: tea.KeyDown}
	m2, _ = m.Update(downKey)
	m = m2.(tui.Model)

	if m.Input() != "" {
		t.Errorf("expected Down to return to empty draft, got %q", m.Input())
	}
}
