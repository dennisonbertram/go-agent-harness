package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	tui "go-agent-harness/cmd/harnesscli/tui"
	"go-agent-harness/cmd/harnesscli/tui/components/inputarea"
)

// ─── /provider command tests ──────────────────────────────────────────────────

// TestProviderCommand_OpensOverlay verifies /provider opens the provider overlay.
func TestProviderCommand_OpensOverlay(t *testing.T) {
	m := initModel(t, 80, 24)
	m = sendSlashCommand(m, "/provider")

	if !m.OverlayActive() {
		t.Fatal("OverlayActive() must be true after /provider")
	}
	if m.ActiveOverlay() != "provider" {
		t.Errorf("ActiveOverlay(): want %q, got %q", "provider", m.ActiveOverlay())
	}
}

// TestProviderOverlay_ContainsExpectedContent verifies the overlay view shows
// the title and both gateway options.
func TestProviderOverlay_ContainsExpectedContent(t *testing.T) {
	m := initModel(t, 80, 24)
	m = sendSlashCommand(m, "/provider")

	v := m.View()
	if !strings.Contains(v, "Routing Gateway") {
		t.Errorf("View() with provider overlay must contain 'Routing Gateway'; got:\n%s", v)
	}
	if !strings.Contains(v, "Direct") {
		t.Errorf("View() with provider overlay must contain 'Direct'; got:\n%s", v)
	}
	if !strings.Contains(v, "OpenRouter") {
		t.Errorf("View() with provider overlay must contain 'OpenRouter'; got:\n%s", v)
	}
}

// TestProviderOverlay_Navigation verifies Up/Down moves the cursor.
func TestProviderOverlay_Navigation(t *testing.T) {
	m := initModel(t, 80, 24)
	m = sendSlashCommand(m, "/provider")

	// Default cursor is at index 0 (Direct). Press Down to move to OpenRouter.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = m2.(tui.Model)

	// Press Enter to confirm and capture the msg.
	m3, cmds := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m3.(tui.Model)

	if cmds == nil {
		t.Fatal("expected cmd from Enter on provider overlay")
	}
	msg := cmds()
	gw, ok := msg.(tui.GatewaySelectedMsg)
	if !ok {
		t.Fatalf("expected GatewaySelectedMsg, got %T", msg)
	}
	if gw.Gateway != "openrouter" {
		t.Errorf("GatewaySelectedMsg.Gateway = %q, want %q", gw.Gateway, "openrouter")
	}
}

// TestProviderOverlay_NavigationWrap verifies cursor wraps around.
func TestProviderOverlay_NavigationWrap(t *testing.T) {
	m := initModel(t, 80, 24)
	m = sendSlashCommand(m, "/provider")

	// At index 0 (Direct), press Up to wrap to last (OpenRouter).
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = m2.(tui.Model)

	// Press Enter to confirm.
	m3, cmds := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m3.(tui.Model)

	if cmds == nil {
		t.Fatal("expected cmd from Enter")
	}
	msg := cmds()
	gw, ok := msg.(tui.GatewaySelectedMsg)
	if !ok {
		t.Fatalf("expected GatewaySelectedMsg, got %T", msg)
	}
	if gw.Gateway != "openrouter" {
		t.Errorf("GatewaySelectedMsg.Gateway = %q, want %q after Up wrap", gw.Gateway, "openrouter")
	}
}

// TestProviderOverlay_EscapeClosesWithoutChange verifies Escape closes the overlay
// without changing the selected gateway.
func TestProviderOverlay_EscapeClosesWithoutChange(t *testing.T) {
	m := initModel(t, 80, 24)
	// Ensure gateway is "" (direct) initially.
	if m.SelectedGateway() != "" {
		t.Fatalf("precondition: SelectedGateway() = %q, want empty", m.SelectedGateway())
	}

	m = sendSlashCommand(m, "/provider")

	// Navigate to OpenRouter.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = m2.(tui.Model)

	// Press Escape — should close without emitting GatewaySelectedMsg.
	m, _ = sendEscape(m)

	if m.OverlayActive() {
		t.Error("OverlayActive() must be false after Escape")
	}
	if m.ActiveOverlay() != "" {
		t.Errorf("ActiveOverlay() must be empty after Escape, got %q", m.ActiveOverlay())
	}
	// Gateway must not have changed.
	if m.SelectedGateway() != "" {
		t.Errorf("SelectedGateway() must still be empty after Escape, got %q", m.SelectedGateway())
	}
}

// TestProviderOverlay_EnterEmitsMsg verifies Enter on OpenRouter emits GatewaySelectedMsg.
func TestProviderOverlay_EnterEmitsMsg(t *testing.T) {
	m := initModel(t, 80, 24)
	m = sendSlashCommand(m, "/provider")

	// Navigate to OpenRouter (index 1).
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = m2.(tui.Model)

	// Press Enter.
	m3, cmds := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m3.(tui.Model)

	// Overlay should be closed.
	if m.OverlayActive() {
		t.Error("overlay must be closed after Enter")
	}

	if cmds == nil {
		t.Fatal("expected cmd from Enter on provider overlay")
	}
	msg := cmds()
	gw, ok := msg.(tui.GatewaySelectedMsg)
	if !ok {
		t.Fatalf("expected GatewaySelectedMsg, got %T", msg)
	}
	if gw.Gateway != "openrouter" {
		t.Errorf("GatewaySelectedMsg.Gateway = %q, want %q", gw.Gateway, "openrouter")
	}
}

// TestGatewaySelectedMsg_SetsGateway verifies GatewaySelectedMsg updates the gateway state.
func TestGatewaySelectedMsg_SetsGateway(t *testing.T) {
	m := initModel(t, 80, 24)

	// Set to openrouter.
	m2, _ := m.Update(tui.GatewaySelectedMsg{Gateway: "openrouter"})
	m = m2.(tui.Model)

	if m.SelectedGateway() != "openrouter" {
		t.Errorf("SelectedGateway() = %q, want %q", m.SelectedGateway(), "openrouter")
	}

	// Set back to direct.
	m3, _ := m.Update(tui.GatewaySelectedMsg{Gateway: ""})
	m = m3.(tui.Model)

	if m.SelectedGateway() != "" {
		t.Errorf("SelectedGateway() = %q, want empty", m.SelectedGateway())
	}
}

// TestGatewaySelectedMsg_SetsStatusMsg verifies GatewaySelectedMsg sets the status bar message.
func TestGatewaySelectedMsg_SetsStatusMsg(t *testing.T) {
	m := initModel(t, 80, 24)

	m2, _ := m.Update(tui.GatewaySelectedMsg{Gateway: "openrouter"})
	m = m2.(tui.Model)
	if !strings.Contains(m.StatusMsg(), "Gateway: OpenRouter") {
		t.Errorf("StatusMsg() = %q, want containing 'Gateway: OpenRouter'", m.StatusMsg())
	}

	m3, _ := m.Update(tui.GatewaySelectedMsg{Gateway: ""})
	m = m3.(tui.Model)
	if !strings.Contains(m.StatusMsg(), "Gateway: Direct") {
		t.Errorf("StatusMsg() = %q, want containing 'Gateway: Direct'", m.StatusMsg())
	}
}

// TestProviderCommand_InHelpList verifies /provider appears in /help output.
func TestProviderCommand_InHelpList(t *testing.T) {
	m := initModel(t, 80, 24)
	m = sendSlashCommand(m, "/help")

	v := m.View()
	if !strings.Contains(v, "provider") {
		t.Errorf("/provider must appear in /help view:\n%s", v)
	}
}

// TestProviderOverlay_SubmitViaCommandSubmittedMsg verifies CommandSubmittedMsg{/provider}
// also opens the overlay.
func TestProviderOverlay_SubmitViaCommandSubmittedMsg(t *testing.T) {
	m := initModel(t, 80, 24)
	m2, _ := m.Update(inputarea.CommandSubmittedMsg{Value: "/provider"})
	m = m2.(tui.Model)

	if !m.OverlayActive() {
		t.Fatal("OverlayActive() must be true after CommandSubmittedMsg{/provider}")
	}
	if m.ActiveOverlay() != "provider" {
		t.Errorf("ActiveOverlay() = %q, want %q", m.ActiveOverlay(), "provider")
	}
}

// TestProviderOverlay_ViewDiffersFromViewport verifies that the provider overlay
// produces different View() output than the normal viewport.
func TestProviderOverlay_ViewDiffersFromViewport(t *testing.T) {
	m := initModel(t, 80, 24)
	viewBefore := m.View()

	m = sendSlashCommand(m, "/provider")
	viewAfter := m.View()

	if viewAfter == viewBefore {
		t.Error("View() must change when provider overlay is active")
	}
}

// TestProviderOverlay_ConcurrentAccess verifies no race condition with value-type copies.
func TestProviderOverlay_ConcurrentAccess(t *testing.T) {
	base := initModel(t, 80, 24)
	base = sendSlashCommand(base, "/provider")

	done := make(chan struct{}, 10)
	for i := 0; i < 10; i++ {
		go func() {
			m := base
			_ = m.View()
			_ = m.OverlayActive()
			_ = m.ActiveOverlay()
			_ = m.SelectedGateway()
			m, _ = sendEscape(m)
			_ = m.OverlayActive()
			done <- struct{}{}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestProviderOverlay_EnterOnDirectEmitsEmptyGateway verifies Enter on Direct
// (index 0) emits GatewaySelectedMsg with empty gateway.
func TestProviderOverlay_EnterOnDirectEmitsEmptyGateway(t *testing.T) {
	m := initModel(t, 80, 24)
	m = sendSlashCommand(m, "/provider")

	// Index 0 is already Direct. Press Enter.
	m2, cmds := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m2.(tui.Model)

	if m.OverlayActive() {
		t.Error("overlay must be closed after Enter")
	}
	if cmds == nil {
		t.Fatal("expected cmd from Enter")
	}
	msg := cmds()
	gw, ok := msg.(tui.GatewaySelectedMsg)
	if !ok {
		t.Fatalf("expected GatewaySelectedMsg, got %T", msg)
	}
	if gw.Gateway != "" {
		t.Errorf("GatewaySelectedMsg.Gateway = %q, want empty for Direct", gw.Gateway)
	}
}

// TestProviderCommand_InSlashCompleteDropdown verifies /provider appears in the
// slash-complete suggestions when typing "/p".
func TestProviderCommand_InSlashCompleteDropdown(t *testing.T) {
	m := initModel(t, 80, 24)
	m = typeIntoModel(m, "/p")

	v := m.View()
	if !strings.Contains(v, "provider") {
		t.Errorf("slash-complete dropdown must contain 'provider' when typing '/p':\n%s", v)
	}
}
