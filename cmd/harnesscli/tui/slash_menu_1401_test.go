package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// Issue #1401: slash-command menu behaviour a first-time user expects.

// Tab completes the item the user highlighted with the arrow keys, not the
// input box's own prefix guess.
func TestSlashMenu_TabCompletesHighlighted(t *testing.T) {
	m := initModel(t, 120, 40)
	m = typeIntoModel(m, "/co")
	m = sendKey(m, tea.KeyDown) // move off the top match
	m = typeTab(m)
	got := m.Input()
	if got == "/co" || !strings.HasPrefix(got, "/co") || !strings.HasSuffix(got, " ") {
		t.Fatalf("Tab must complete the highlighted command into the input, got %q", got)
	}
	// Tab must fill the input, not execute the command.
	if m.OverlayActive() {
		t.Fatalf("Tab must not run the command (an overlay opened)")
	}
}

// Enter on a bare "/" with nothing typed and no navigation must not run the
// first command in the list.
func TestSlashMenu_BareSlashEnterDoesNotRun(t *testing.T) {
	m := initModel(t, 120, 40)
	m = typeIntoModel(m, "/")
	m = sendKey(m, tea.KeyEnter)
	if m.OverlayActive() {
		t.Fatalf("Enter on bare '/' must not open the first command's overlay")
	}
	if m.Input() != "/" {
		t.Fatalf("input must be preserved, got %q", m.Input())
	}
	if !strings.Contains(m.StatusMsg(), "↑↓") {
		t.Fatalf("status must explain how to choose a command, got %q", m.StatusMsg())
	}
}

// Enter after choosing with the arrow keys still runs the chosen command.
func TestSlashMenu_EnterAfterDownRuns(t *testing.T) {
	m := initModel(t, 120, 40)
	m = typeIntoModel(m, "/")
	for i := 0; i < 3; i++ { // add-dir, attach, cancel, clear
		m = sendKey(m, tea.KeyDown)
	}
	m = sendKey(m, tea.KeyEnter)
	if m.Input() != "" {
		t.Fatalf("Enter on a navigated item must run it and clear the input, got %q", m.Input())
	}
}

// Opening the menu must not change the total screen height: the transcript
// viewport gives up the rows the menu needs.
func TestSlashMenu_ScreenHeightStableWhenOpen(t *testing.T) {
	m := initModel(t, 120, 40)
	before := strings.Count(m.View(), "\n")
	m = typeIntoModel(m, "/")
	after := strings.Count(m.View(), "\n")
	if after != before {
		t.Fatalf("screen height changed when the menu opened: %d -> %d rows", before+1, after+1)
	}
	if !strings.Contains(m.View(), "/add-dir") {
		t.Fatalf("menu must be visible")
	}
}
