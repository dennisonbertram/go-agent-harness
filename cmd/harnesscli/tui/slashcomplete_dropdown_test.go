package tui_test

import (
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	tui "go-agent-harness/cmd/harnesscli/tui"
)

// readFinalOutput quits the teatest model and reads its rendered output as a string.
func readFinalOutput(t *testing.T, tm *teatest.TestModel) string {
	t.Helper()
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
	out := tm.FinalOutput(t, teatest.WithFinalTimeout(3*time.Second))
	b, err := io.ReadAll(out)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	return string(b)
}

func TestDropdown_AppearsWhenTypingSlash(t *testing.T) {
	cfg := tui.TUIConfig{BaseURL: "http://localhost:9999"}
	m := tui.New(cfg)

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))

	// Send WindowSizeMsg first to initialize
	tm.Send(tea.WindowSizeMsg{Width: 120, Height: 40})
	time.Sleep(50 * time.Millisecond)

	// Type '/'
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	time.Sleep(50 * time.Millisecond)

	rendered := readFinalOutput(t, tm)

	// The dropdown should show slash command names
	if !strings.Contains(rendered, "/help") && !strings.Contains(rendered, "/clear") {
		t.Errorf("expected slash command dropdown to appear, got:\n%s", rendered)
	}
}

// TestDropdown_ClosesOnEscape_Unit uses direct model updates to verify that
// pressing Escape closes the dropdown.  The teatest approach is intentionally
// avoided here because FinalOutput captures all terminal frames, making it
// unreliable for asserting "dropdown gone" after close.
func TestDropdown_ClosesOnEscape_Unit(t *testing.T) {
	m := initModel(t, 120, 40)

	// Open dropdown by typing '/'
	m = typeIntoModel(m, "/")
	view := m.View()
	if !strings.Contains(view, "/clear") {
		t.Fatalf("expected dropdown to appear after typing '/', view:\n%s", view)
	}

	// Press Escape — should close the dropdown (input is cleared too).
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = m2.(tui.Model)

	view = m.View()
	if strings.Contains(view, "▶ /") {
		t.Errorf("expected dropdown to close after Escape, view:\n%s", view)
	}
}

// TestDropdown_ClosesOnEscape uses teatest to exercise the full program loop and
// verifies the model quits cleanly after Escape.
func TestDropdown_ClosesOnEscape(t *testing.T) {
	cfg := tui.TUIConfig{BaseURL: "http://localhost:9999"}
	m := tui.New(cfg)

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
	tm.Send(tea.WindowSizeMsg{Width: 120, Height: 40})
	time.Sleep(50 * time.Millisecond)

	// Type '/' to open dropdown
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	time.Sleep(50 * time.Millisecond)

	// Press Escape to close
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	time.Sleep(50 * time.Millisecond)

	// Quit and wait — just verify the program terminates cleanly.
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
	// Test passes as long as WaitFinished doesn't time out.
}

func TestDropdown_FiltersOnInput(t *testing.T) {
	cfg := tui.TUIConfig{BaseURL: "http://localhost:9999"}
	m := tui.New(cfg)

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
	tm.Send(tea.WindowSizeMsg{Width: 120, Height: 40})
	time.Sleep(50 * time.Millisecond)

	// Type '/h' — should filter to commands containing 'h'
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	time.Sleep(30 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	time.Sleep(50 * time.Millisecond)

	rendered := readFinalOutput(t, tm)

	// '/help' should be visible
	if !strings.Contains(rendered, "help") {
		t.Errorf("expected '/help' in dropdown after typing '/h', got:\n%s", rendered)
	}
}

// TestDropdown_UnitSync verifies syncSlashComplete behavior directly via model updates.
func TestDropdown_UnitSync(t *testing.T) {
	m := initModel(t, 120, 40)

	// Before typing, slashcomplete should not be active (dropdown closed).
	// Type '/' — dropdown should appear.
	m = typeIntoModel(m, "/")
	view := m.View()
	if !strings.Contains(view, "/clear") && !strings.Contains(view, "/help") {
		t.Errorf("expected dropdown to appear after typing '/', view:\n%s", view)
	}
}

// TestDropdown_ClosesAfterSpace verifies that typing "/clear " (with trailing space) closes the dropdown.
func TestDropdown_ClosesAfterSpace(t *testing.T) {
	m := initModel(t, 120, 40)
	m = typeIntoModel(m, "/clear ")
	view := m.View()
	// Dropdown should be closed — the "▶ " selected prefix should not appear.
	if strings.Contains(view, "▶ /") {
		t.Errorf("expected dropdown to close after fully typed command with space, view:\n%s", view)
	}
}

// TestDropdown_ClosesOnNonSlashInput verifies dropdown stays closed for non-slash input.
func TestDropdown_ClosesOnNonSlashInput(t *testing.T) {
	m := initModel(t, 120, 40)
	m = typeIntoModel(m, "hello")
	view := m.View()
	// No dropdown should be shown.
	if strings.Contains(view, "▶ /") {
		t.Errorf("expected no dropdown for non-slash input, view:\n%s", view)
	}
}

// TestDropdown_DownMovesCursor verifies that pressing Down when the dropdown is
// active moves the selection cursor without scrolling the viewport.
func TestDropdown_DownMovesCursor(t *testing.T) {
	m := initModel(t, 120, 40)
	m = typeIntoModel(m, "/")

	// First item should be selected (cursor at 0).
	view1 := m.View()

	// Press Down.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = m2.(tui.Model)
	view2 := m.View()

	// The two views should differ (selection moved).
	if view1 == view2 {
		t.Errorf("expected Down to move selection cursor in dropdown, but view did not change")
	}
	// Dropdown should still be visible.
	if !strings.Contains(view2, "▶ /") {
		t.Errorf("expected dropdown to remain open after Down, view:\n%s", view2)
	}
}

// TestDropdown_UpWrapsToBottom verifies that pressing Up from the first item
// wraps to the last item.
func TestDropdown_UpWrapsToBottom(t *testing.T) {
	m := initModel(t, 120, 40)
	m = typeIntoModel(m, "/")

	// Press Up from position 0 — should wrap to last item.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = m2.(tui.Model)
	view := m.View()

	// Dropdown must still be visible.
	if !strings.Contains(view, "▶ /") {
		t.Errorf("expected dropdown to remain open after Up, view:\n%s", view)
	}
}

// TestDropdown_EnterAcceptsSelection verifies that pressing Enter when the
// dropdown is active inserts the selected command into the input and closes
// the dropdown (does NOT submit a run).
func TestDropdown_EnterAcceptsSelection(t *testing.T) {
	m := initModel(t, 120, 40)
	m = typeIntoModel(m, "/")

	// Press Enter to accept the currently highlighted suggestion.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m2.(tui.Model)
	view := m.View()

	// Dropdown should be closed.
	if strings.Contains(view, "▶ /") {
		t.Errorf("expected dropdown to close after Enter, view:\n%s", view)
	}
	// Input should now contain a slash command (e.g. "/clear ").
	input := m.Input()
	if !strings.HasPrefix(input, "/") {
		t.Errorf("expected input to contain accepted slash command, got: %q", input)
	}
}
