package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	tui "go-agent-harness/cmd/harnesscli/tui"
	"go-agent-harness/cmd/harnesscli/tui/components/inputarea"
)

// sendSlashCommand types a slash command into the model and submits it.
func sendSlashCommand(m tui.Model, cmd string) tui.Model {
	m2, _ := m.Update(inputarea.CommandSubmittedMsg{Value: cmd})
	return m2.(tui.Model)
}

// TestHelp_OverlayRendersInView verifies that /help changes View() to show
// help dialog content rather than the empty viewport.
func TestHelp_OverlayRendersInView(t *testing.T) {
	m := initModel(t, 80, 24)
	viewBefore := m.View()

	m = sendSlashCommand(m, "/help")

	if !m.OverlayActive() {
		t.Fatal("overlayActive must be true after /help")
	}
	if m.ActiveOverlay() != "help" {
		t.Errorf("activeOverlay: want %q, got %q", "help", m.ActiveOverlay())
	}

	viewAfter := m.View()

	// The view must differ from the baseline viewport view.
	if viewAfter == viewBefore {
		t.Error("View() must change when overlay is active; got same output as before")
	}

	// The help dialog renders tab headers — "Commands" is always the first tab.
	if !strings.Contains(viewAfter, "Commands") {
		t.Errorf("View() with help overlay must contain 'Commands' tab header; got:\n%s", viewAfter)
	}
}

// TestStats_OverlayRendersInView verifies that /stats changes View() to show
// stats panel content rather than the empty viewport.
func TestStats_OverlayRendersInView(t *testing.T) {
	m := initModel(t, 80, 24)
	viewBefore := m.View()

	m = sendSlashCommand(m, "/stats")

	if !m.OverlayActive() {
		t.Fatal("overlayActive must be true after /stats")
	}
	if m.ActiveOverlay() != "stats" {
		t.Errorf("activeOverlay: want %q, got %q", "stats", m.ActiveOverlay())
	}

	viewAfter := m.View()

	// The view must differ from the baseline viewport view.
	if viewAfter == viewBefore {
		t.Error("View() must change when stats overlay is active; got same output as before")
	}

	// The stats panel always renders an "Activity" header.
	if !strings.Contains(viewAfter, "Activity") {
		t.Errorf("View() with stats overlay must contain 'Activity'; got:\n%s", viewAfter)
	}
}

// TestContext_OverlayRendersInView verifies that /context changes View() to show
// context grid content (or fallback text) rather than the empty viewport.
func TestContext_OverlayRendersInView(t *testing.T) {
	m := initModel(t, 80, 24)

	m = sendSlashCommand(m, "/context")

	if !m.OverlayActive() {
		t.Fatal("overlayActive must be true after /context")
	}
	if m.ActiveOverlay() != "context" {
		t.Errorf("activeOverlay: want %q, got %q", "context", m.ActiveOverlay())
	}

	viewAfter := m.View()
	// Context grid View() is a stub that returns ""; the model falls back to
	// "Context grid not available". Either way the overlay must be rendered,
	// not silently ignored.
	if viewAfter == "" {
		t.Error("View() must not be empty when context overlay is active")
	}
	// The fallback message must appear (stub returns "").
	if !strings.Contains(viewAfter, "Context grid not available") {
		t.Errorf("View() with context overlay must contain fallback message; got:\n%s", viewAfter)
	}
}

// TestOverlay_EscapeClosesAndRestoresViewport verifies that pressing Escape
// when an overlay is open closes it and restores normal viewport rendering.
func TestOverlay_EscapeClosesAndRestoresViewport(t *testing.T) {
	m := initModel(t, 80, 24)

	// Open help overlay.
	m = sendSlashCommand(m, "/help")
	if !m.OverlayActive() {
		t.Fatal("overlayActive must be true after /help")
	}

	viewWithOverlay := m.View()

	// Press Escape.
	m, _ = sendEscape(m)

	if m.OverlayActive() {
		t.Error("overlayActive must be false after Escape")
	}
	if m.ActiveOverlay() != "" {
		t.Errorf("activeOverlay must be empty after Escape, got %q", m.ActiveOverlay())
	}

	viewAfterClose := m.View()

	// After closing, the view must differ from the overlay view.
	if viewAfterClose == viewWithOverlay {
		t.Error("View() after closing overlay must differ from the overlay view")
	}

	// The help dialog content should no longer be present.
	if strings.Contains(viewAfterClose, "Commands") && strings.Contains(viewAfterClose, "Keybindings") {
		t.Errorf("View() after closing help overlay must not contain help tab headers; got:\n%s", viewAfterClose)
	}
}

// TestRegression_OverlayNotSilentNoop is the regression guard: verifies that
// setting overlayActive=true via OverlayOpenMsg actually changes View() output.
func TestRegression_OverlayNotSilentNoop(t *testing.T) {
	m := initModel(t, 80, 24)
	viewBefore := m.View()

	// Send OverlayOpenMsg with "help" kind directly (simulates what /help
	// command does internally).
	m2, _ := m.Update(tui.OverlayOpenMsg{Kind: "help"})
	m = m2.(tui.Model)

	if !m.OverlayActive() {
		t.Fatal("overlayActive must be true after OverlayOpenMsg")
	}

	viewAfter := m.View()

	// This is the exact regression: before the fix, viewAfter == viewBefore
	// because View() never checked overlayActive.
	if viewAfter == viewBefore {
		t.Error("REGRESSION: View() output did not change after overlayActive=true — overlay is still a silent no-op")
	}
}

// TestOverlay_StatsEscapeRestoresViewport verifies stats overlay close via Escape.
func TestOverlay_StatsEscapeRestoresViewport(t *testing.T) {
	m := initModel(t, 80, 24)

	m = sendSlashCommand(m, "/stats")
	if !m.OverlayActive() {
		t.Fatal("overlayActive must be true after /stats")
	}

	viewWithStats := m.View()

	m, _ = sendEscape(m)

	if m.OverlayActive() {
		t.Error("overlayActive must be false after Escape from stats overlay")
	}

	viewAfterClose := m.View()
	if viewAfterClose == viewWithStats {
		t.Error("View() after closing stats overlay must differ from the stats view")
	}
}

// TestOverlay_OverlayCloseMsg verifies OverlayCloseMsg closes the overlay.
func TestOverlay_OverlayCloseMsg(t *testing.T) {
	m := initModel(t, 80, 24)

	m = sendSlashCommand(m, "/help")
	if !m.OverlayActive() {
		t.Fatal("overlayActive must be true after /help")
	}

	m2, _ := m.Update(tui.OverlayCloseMsg{})
	m = m2.(tui.Model)

	if m.OverlayActive() {
		t.Error("overlayActive must be false after OverlayCloseMsg")
	}
	if m.ActiveOverlay() != "" {
		t.Errorf("activeOverlay must be empty after OverlayCloseMsg, got %q", m.ActiveOverlay())
	}

	// View() must revert to viewport (no overlay content).
	view := m.View()
	_ = view // no panic, returns valid string
}

// TestOverlay_SwitchBetweenOverlays verifies switching from one overlay to another.
func TestOverlay_SwitchBetweenOverlays(t *testing.T) {
	m := initModel(t, 80, 24)

	// Open help.
	m = sendSlashCommand(m, "/help")
	if m.ActiveOverlay() != "help" {
		t.Fatalf("want activeOverlay=help, got %q", m.ActiveOverlay())
	}

	// Close and open stats.
	m, _ = sendEscape(m)
	m = sendSlashCommand(m, "/stats")
	if m.ActiveOverlay() != "stats" {
		t.Fatalf("want activeOverlay=stats, got %q", m.ActiveOverlay())
	}

	viewStats := m.View()
	if !strings.Contains(viewStats, "Activity") {
		t.Errorf("stats overlay view must contain 'Activity'; got:\n%s", viewStats)
	}
}

// TestOverlay_LargeTerminal verifies overlays render correctly at 120x40.
func TestOverlay_LargeTerminal(t *testing.T) {
	m := initModel(t, 120, 40)

	m = sendSlashCommand(m, "/help")
	view := m.View()

	if !strings.Contains(view, "Commands") {
		t.Errorf("help overlay at 120x40 must contain 'Commands'; got:\n%s", view)
	}
}

// TestOverlay_ReadyGate verifies overlays are not rendered before WindowSizeMsg.
func TestOverlay_ReadyGate(t *testing.T) {
	// Model not yet ready (no WindowSizeMsg sent).
	m := tui.New(tui.DefaultTUIConfig())

	// Even if we inject OverlayOpenMsg, View() should return "Initializing...".
	m2, _ := m.Update(tui.OverlayOpenMsg{Kind: "help"})
	view := m2.(tui.Model).View()
	if !strings.Contains(view, "Initializing") {
		t.Errorf("View() before ready must return Initializing; got %q", view)
	}
}

// TestHelp_OverlayViewDiffersFromViewport verifies that View() with help open
// produces output different from View() with help closed at the same size.
// This is the canonical BUG-2 regression test.
func TestHelp_OverlayViewDiffersFromViewport(t *testing.T) {
	for _, size := range []struct{ w, h int }{{80, 24}, {120, 40}, {200, 50}} {
		t.Run("", func(t *testing.T) {
			m := initModel(t, size.w, size.h)
			viewportView := m.View()

			m = sendSlashCommand(m, "/help")
			overlayView := m.View()

			if overlayView == viewportView {
				t.Errorf("BUG-2 regression at %dx%d: View() with /help open is identical to viewport view — overlay is still a no-op", size.w, size.h)
			}
		})
	}
}

// TestStats_OverlayViewDiffersFromViewport verifies stats overlay changes View().
func TestStats_OverlayViewDiffersFromViewport(t *testing.T) {
	for _, size := range []struct{ w, h int }{{80, 24}, {120, 40}} {
		t.Run("", func(t *testing.T) {
			m := initModel(t, size.w, size.h)
			viewportView := m.View()

			m = sendSlashCommand(m, "/stats")
			overlayView := m.View()

			if overlayView == viewportView {
				t.Errorf("BUG-2 regression at %dx%d: View() with /stats open is identical to viewport view — overlay is still a no-op", size.w, size.h)
			}
		})
	}
}

// TestContext_OverlayViewDiffersFromViewport verifies context overlay changes View().
func TestContext_OverlayViewDiffersFromViewport(t *testing.T) {
	m := initModel(t, 80, 24)
	viewportView := m.View()

	m = sendSlashCommand(m, "/context")
	overlayView := m.View()

	if overlayView == viewportView {
		t.Error("BUG-2 regression: View() with /context open is identical to viewport view — overlay is still a no-op")
	}
}

// TestOverlay_ConcurrentAccess verifies no race condition when multiple goroutines
// hold their own model copy with overlays active.
func TestOverlay_ConcurrentAccess(t *testing.T) {
	base := initModel(t, 80, 24)
	base = sendSlashCommand(base, "/help")

	done := make(chan struct{}, 10)
	for i := 0; i < 10; i++ {
		go func() {
			m := base // each goroutine has its own copy
			_ = m.View()
			_ = m.OverlayActive()
			_ = m.ActiveOverlay()
			m, _ = sendEscape(m)
			_ = m.OverlayActive()
			done <- struct{}{}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

// Verify that tea.WindowSizeMsg is handled correctly — included here so the
// helper function sendSlashCommand is only defined once.
var _ tea.Msg = tea.WindowSizeMsg{}
