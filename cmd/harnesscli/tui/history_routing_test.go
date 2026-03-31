package tui_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	tui "go-agent-harness/cmd/harnesscli/tui"
)

// sendKey sends a single key message to the model.
func sendKey(m tui.Model, keyType tea.KeyType) tui.Model {
	m2, _ := m.Update(tea.KeyMsg{Type: keyType})
	return m2.(tui.Model)
}

// submitCommand types a command and presses Enter.
func submitCommand(m tui.Model, cmd string) tui.Model {
	m = typeIntoModel(m, cmd)
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return m2.(tui.Model)
}

// BT-008: When no overlay or slash-complete dropdown is active,
// Up/Down arrows navigate history (not viewport).
func TestBT008_UpDownRoutesToHistoryWhenNoOverlayActive(t *testing.T) {
	m := initModel(t, 80, 24)

	// Pre-condition: no overlay, no slash complete
	if m.OverlayActive() {
		t.Fatal("BT-008 pre-condition: overlay must not be active")
	}

	// Submit a command so history has an entry
	m = submitCommand(m, "history-entry-one")

	// Input should be empty after submit
	if m.Input() != "" {
		t.Fatalf("BT-008 pre-condition: input should be empty after submit, got %q", m.Input())
	}

	// Record viewport scroll offset before pressing Up
	scrollBefore := m.ViewportScrollOffset()

	// Press Up — should navigate history, NOT scroll viewport
	m = sendKey(m, tea.KeyUp)

	// Input should now show the last submitted command
	if m.Input() != "history-entry-one" {
		t.Errorf("BT-008: Up when no overlay active should navigate history; want 'history-entry-one', got %q", m.Input())
	}

	// Viewport scroll offset should NOT have changed (history navigation, not viewport scroll)
	scrollAfter := m.ViewportScrollOffset()
	if scrollAfter != scrollBefore {
		t.Errorf("BT-008: Up should not scroll viewport when navigating history; scroll changed from %d to %d", scrollBefore, scrollAfter)
	}

	// Press Down — should return to empty draft
	m = sendKey(m, tea.KeyDown)
	if m.Input() != "" {
		t.Errorf("BT-008: Down after Up should return to empty draft, got %q", m.Input())
	}
}

// TestBT008_UpScrollsViewportWhenOverlayActive verifies that when an overlay IS active,
// Up still does overlay/viewport navigation (not history).
func TestBT008_UpDoesNotRouteHistoryWhenOverlayActive(t *testing.T) {
	m := initModel(t, 80, 24)
	m = submitCommand(m, "history-check")

	// Open an overlay
	m2, _ := m.Update(tui.OverlayOpenMsg{Kind: "help"})
	m = m2.(tui.Model)
	if !m.OverlayActive() {
		t.Fatal("BT-008 overlay pre-condition failed")
	}

	// Press Up — should NOT change the input (overlay is active, key goes elsewhere)
	m = sendKey(m, tea.KeyUp)
	// Input should still be empty (we didn't route Up to history)
	if m.Input() == "history-check" {
		t.Errorf("BT-008: Up when overlay is active should NOT navigate input history, but it did")
	}
}

// Regression: After navigating history and pressing Down back to draft,
// submitting a new command should add it to history.
func TestRegression_HistoryNavigateAndSubmitNewCommand(t *testing.T) {
	m := initModel(t, 80, 24)

	// Submit two commands
	m = submitCommand(m, "cmd-first")
	m = submitCommand(m, "cmd-second")

	// Navigate up to cmd-second, then down to draft
	m = sendKey(m, tea.KeyUp) // shows "cmd-second"
	if m.Input() != "cmd-second" {
		t.Fatalf("regression setup: expected 'cmd-second', got %q", m.Input())
	}
	m = sendKey(m, tea.KeyDown) // back to draft

	// Type and submit a new command
	m = submitCommand(m, "cmd-third")

	// Navigate up — should show "cmd-third" as most recent
	m = sendKey(m, tea.KeyUp)
	if m.Input() != "cmd-third" {
		t.Errorf("regression: after navigate+submit, Up should show 'cmd-third', got %q", m.Input())
	}
}

// Regression: Window resize must not clear history that was built up during a session.
func TestRegression_WindowResizePreservesHistory(t *testing.T) {
	m := initModel(t, 80, 24)

	// Submit a command to populate history
	m = submitCommand(m, "before-resize")

	// Simulate window resize
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = m2.(tui.Model)

	// History should still be navigable after resize
	m = sendKey(m, tea.KeyUp)
	if m.Input() != "before-resize" {
		t.Errorf("regression: window resize must not clear history; want 'before-resize', got %q", m.Input())
	}
}

// Regression: ScrollUp/ScrollDown key bindings (ctrl+p / ctrl+n) must also navigate history.
func TestRegression_CtrlPNavigatesHistory(t *testing.T) {
	m := initModel(t, 80, 24)
	m = submitCommand(m, "ctrl-p-test")

	// ctrl+p is also bound to ScrollUp; it should navigate history when no overlay active
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}, Alt: false})
	// Actually ctrl+p is a special key type. Use the proper key binding:
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m = m2.(tui.Model)
	if m.Input() != "ctrl-p-test" {
		t.Errorf("regression: ctrl+p (ScrollUp alt binding) should navigate history; want 'ctrl-p-test', got %q", m.Input())
	}
}
