package tui_test

// Regression tests for overlay keyboard focus fixes:
//   BUG-3/4: Tab / Shift+Tab (and h/l) navigate help dialog tabs when help is open.
//   BUG-5:   'r' key toggles the stats panel period when stats is open.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	tui "go-agent-harness/cmd/harnesscli/tui"
)

// ─── BUG-3/4: help dialog keyboard focus ─────────────────────────────────────

// TestHelpDialog_TabNavigatesNextTab verifies that pressing Tab when the help
// overlay is open advances to the next tab (BUG-4/BUG-3 fix).
func TestHelpDialog_TabNavigatesNextTab(t *testing.T) {
	m := initModel(t, 80, 24)
	m2, _ := m.Update(tui.OverlayOpenMsg{Kind: "help"})
	m = m2.(tui.Model)

	if !m.OverlayActive() {
		t.Fatal("expected overlay to be active after OverlayOpenMsg{help}")
	}
	if m.ActiveOverlay() != "help" {
		t.Fatalf("expected activeOverlay to be 'help', got %q", m.ActiveOverlay())
	}

	initialTab := m.HelpDialogActiveTab()

	// Press Tab — should advance to next tab.
	m3, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = m3.(tui.Model)

	afterTab := m.HelpDialogActiveTab()
	if afterTab == initialTab {
		t.Errorf("Tab key should advance help dialog tab: before=%d after=%d", initialTab, afterTab)
	}
}

// TestHelpDialog_ShiftTabNavigatesPrevTab verifies that pressing Shift+Tab when
// the help overlay is open moves to the previous tab (BUG-4/BUG-3 fix).
func TestHelpDialog_ShiftTabNavigatesPrevTab(t *testing.T) {
	m := initModel(t, 80, 24)
	m2, _ := m.Update(tui.OverlayOpenMsg{Kind: "help"})
	m = m2.(tui.Model)

	// First advance one tab so we're not at tab 0 (to distinguish forward/back).
	m3, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = m3.(tui.Model)
	afterForward := m.HelpDialogActiveTab()

	// Now press Shift+Tab — should go back.
	m4, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = m4.(tui.Model)
	afterBack := m.HelpDialogActiveTab()

	if afterBack == afterForward {
		t.Errorf("Shift+Tab should move help dialog tab backward: forward=%d back=%d", afterForward, afterBack)
	}
}

// TestHelpDialog_LKeyNavigatesNextTab verifies that pressing 'l' when the help
// overlay is open advances to the next tab (vim-style navigation, BUG-3 fix).
func TestHelpDialog_LKeyNavigatesNextTab(t *testing.T) {
	m := initModel(t, 80, 24)
	m2, _ := m.Update(tui.OverlayOpenMsg{Kind: "help"})
	m = m2.(tui.Model)

	initialTab := m.HelpDialogActiveTab()

	m3, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	m = m3.(tui.Model)

	afterL := m.HelpDialogActiveTab()
	if afterL == initialTab {
		t.Errorf("'l' key should advance help dialog tab: before=%d after=%d", initialTab, afterL)
	}
}

// TestHelpDialog_HKeyNavigatesPrevTab verifies that pressing 'h' when the help
// overlay is open moves to the previous tab (vim-style navigation, BUG-3 fix).
func TestHelpDialog_HKeyNavigatesPrevTab(t *testing.T) {
	m := initModel(t, 80, 24)
	m2, _ := m.Update(tui.OverlayOpenMsg{Kind: "help"})
	m = m2.(tui.Model)

	// Advance first so we are not at tab 0.
	m3, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	m = m3.(tui.Model)
	afterL := m.HelpDialogActiveTab()

	m4, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	m = m4.(tui.Model)
	afterH := m.HelpDialogActiveTab()

	if afterH == afterL {
		t.Errorf("'h' key should move help dialog tab backward: after-l=%d after-h=%d", afterL, afterH)
	}
}

// TestHelpDialog_TabNavigationChangesView verifies that tab navigation changes
// the rendered view (different tab content is shown).
func TestHelpDialog_TabNavigationChangesView(t *testing.T) {
	m := initModel(t, 80, 24)
	m2, _ := m.Update(tui.OverlayOpenMsg{Kind: "help"})
	m = m2.(tui.Model)

	view1 := m.View()

	// Advance to next tab.
	m3, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = m3.(tui.Model)
	view2 := m.View()

	// The tab header line should differ when the active tab changes.
	if view1 == view2 {
		t.Error("View() should change when help dialog tab is advanced via Tab key")
	}

	// The help overlay should still be open.
	if !m.OverlayActive() {
		t.Error("overlay should remain active after tab navigation")
	}
	if !strings.Contains(view2, "Keybindings") || !strings.Contains(view2, "Commands") {
		t.Errorf("help dialog should show tab headers, got:\n%s", view2)
	}
}

// ─── BUG-5: stats panel 'r' toggle ───────────────────────────────────────────

// TestStatsPanel_RKeyTogglesPeriod verifies that pressing 'r' when the stats
// overlay is open cycles the period (BUG-5 fix).
func TestStatsPanel_RKeyTogglesPeriod(t *testing.T) {
	m := initModel(t, 80, 24)
	m2, _ := m.Update(tui.OverlayOpenMsg{Kind: "stats"})
	m = m2.(tui.Model)

	if !m.OverlayActive() {
		t.Fatal("expected overlay to be active after OverlayOpenMsg{stats}")
	}
	if m.ActiveOverlay() != "stats" {
		t.Fatalf("expected activeOverlay to be 'stats', got %q", m.ActiveOverlay())
	}

	initialPeriod := m.StatsPanelActivePeriod()

	// Press 'r' — should cycle to next period.
	m3, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m = m3.(tui.Model)

	afterR := m.StatsPanelActivePeriod()
	if afterR == initialPeriod {
		t.Errorf("'r' key should toggle stats panel period: before=%d after=%d", initialPeriod, afterR)
	}
}

// TestStatsPanel_RKeyCyclesAllPeriods verifies that pressing 'r' three times
// cycles through all periods and returns to the initial state (wrap-around).
func TestStatsPanel_RKeyCyclesAllPeriods(t *testing.T) {
	m := initModel(t, 80, 24)
	m2, _ := m.Update(tui.OverlayOpenMsg{Kind: "stats"})
	m = m2.(tui.Model)

	initialPeriod := m.StatsPanelActivePeriod()

	// Press 'r' three times — should cycle through Week→Month→Year→Week.
	for i := 0; i < 3; i++ {
		m3, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
		m = m3.(tui.Model)
	}

	finalPeriod := m.StatsPanelActivePeriod()
	if finalPeriod != initialPeriod {
		t.Errorf("after 3 'r' presses period should wrap back to %d, got %d", initialPeriod, finalPeriod)
	}
}

// TestStatsPanel_RKeyChangesView verifies that pressing 'r' updates the
// rendered stats view (the period label in the output changes).
func TestStatsPanel_RKeyChangesView(t *testing.T) {
	m := initModel(t, 80, 24)
	m2, _ := m.Update(tui.OverlayOpenMsg{Kind: "stats"})
	m = m2.(tui.Model)

	view1 := m.View()

	m3, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m = m3.(tui.Model)
	view2 := m.View()

	if view1 == view2 {
		t.Error("View() should change when stats panel period is toggled via 'r' key")
	}

	// Overlay should remain active.
	if !m.OverlayActive() {
		t.Error("overlay should remain active after 'r' key press in stats overlay")
	}
}
