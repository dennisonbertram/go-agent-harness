package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	tui "go-agent-harness/cmd/harnesscli/tui"
	"go-agent-harness/cmd/harnesscli/tui/components/inputarea"
)

// ─── /model command tests ─────────────────────────────────────────────────────

// TestTUI137_ModelCommandOpensOverlay verifies /model opens the model overlay.
func TestTUI137_ModelCommandOpensOverlay(t *testing.T) {
	m := initModel(t, 80, 24)
	m = sendSlashCommand(m, "/model")

	if !m.OverlayActive() {
		t.Fatal("OverlayActive() must be true after /model")
	}
	if m.ActiveOverlay() != "model" {
		t.Errorf("ActiveOverlay(): want %q, got %q", "model", m.ActiveOverlay())
	}
}

// TestTUI137_ModelOverlayEscapeLevel0ClosesOverlay verifies Escape at Level-0 closes the overlay.
func TestTUI137_ModelOverlayEscapeLevel0ClosesOverlay(t *testing.T) {
	m := initModel(t, 80, 24)
	m = sendSlashCommand(m, "/model")
	if !m.OverlayActive() {
		t.Fatal("pre-condition: overlay must be open")
	}

	m, _ = sendEscape(m)

	if m.OverlayActive() {
		t.Error("OverlayActive() must be false after Escape at Level-0")
	}
	if m.ActiveOverlay() != "" {
		t.Errorf("ActiveOverlay() must be '' after Escape, got %q", m.ActiveOverlay())
	}
}

// TestTUI137_ModelOverlayEscapeLevel1ReturnsToLevel0 verifies Escape at Level-1
// goes back to Level-0 without closing the overlay.
func TestTUI137_ModelOverlayEscapeLevel1ReturnsToLevel0(t *testing.T) {
	m := initModel(t, 80, 24)
	m = sendSlashCommand(m, "/model")

	// Navigate to deepseek-reasoner (which has ReasoningMode=true). It's at index 8.
	// Navigate down 8 times from first entry (gpt-4.1 at 0) to reach deepseek-reasoner (index 8).
	for i := 0; i < 8; i++ {
		m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = m2.(tui.Model)
	}

	// Press Enter to enter Level-1 (reasoning effort selection).
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m2.(tui.Model)

	// Overlay should still be active and now showing Level-1.
	if !m.OverlayActive() {
		t.Fatal("overlay must still be active after entering reasoning mode")
	}
	v := m.View()
	if !strings.Contains(v, "Reasoning Effort") {
		t.Errorf("view must contain 'Reasoning Effort' at Level-1:\n%s", v)
	}

	// Escape from Level-1: should go back to Level-0 (overlay still open).
	m, _ = sendEscape(m)
	if !m.OverlayActive() {
		t.Error("overlay must remain active after Escape at Level-1")
	}
	if m.ActiveOverlay() != "model" {
		t.Errorf("ActiveOverlay() must be 'model' after Escape from Level-1, got %q", m.ActiveOverlay())
	}

	// View should now show Level-0 again (no "Reasoning Effort").
	v2 := m.View()
	if strings.Contains(v2, "Reasoning Effort") {
		t.Errorf("view must not contain 'Reasoning Effort' after returning to Level-0:\n%s", v2)
	}
}

// TestTUI137_ModelOverlayEnterNonReasoningEmitsMsg verifies that for a non-reasoning
// model (gpt-4.1), Enter at Level-0 opens the config panel, and Enter at the config
// panel emits ModelSelectedMsg and GatewaySelectedMsg, closing the overlay.
func TestTUI137_ModelOverlayEnterNonReasoningEmitsMsg(t *testing.T) {
	m := initModel(t, 80, 24)
	m = sendSlashCommand(m, "/model")

	// gpt-4.1 is at index 0 (first entry) — should be already selected.
	// Press Enter to enter the config panel.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m2.(tui.Model)

	// Overlay must still be active (now showing config panel).
	if !m.OverlayActive() {
		t.Fatal("overlay must remain active after Enter at Level-0 (config panel opened)")
	}
	if !m.ModelConfigMode() {
		t.Fatal("ModelConfigMode() must be true after Enter at Level-0")
	}
	if m.ModelConfigEntry().ID != "gpt-4.1" {
		t.Errorf("ModelConfigEntry().ID = %q, want %q", m.ModelConfigEntry().ID, "gpt-4.1")
	}

	// Press Enter at the config panel to confirm and close.
	m3, cmds := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m3.(tui.Model)

	// Overlay should now be closed.
	if m.OverlayActive() {
		t.Error("overlay must be closed after Enter in config panel")
	}

	// The returned batch must contain a ModelSelectedMsg.
	if cmds == nil {
		t.Fatal("expected cmd from Enter in config panel")
	}
	batchMsg := cmds()
	// BubbleTea Batch returns a tea.BatchMsg (slice of tea.Msg).
	batch, ok := batchMsg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected tea.BatchMsg from config panel Enter, got %T", batchMsg)
	}
	var foundSelected *tui.ModelSelectedMsg
	for _, cmdFn := range batch {
		if cmdFn == nil {
			continue
		}
		inner := cmdFn()
		if sel, ok2 := inner.(tui.ModelSelectedMsg); ok2 {
			sel := sel
			foundSelected = &sel
		}
	}
	if foundSelected == nil {
		t.Fatal("batch must contain a ModelSelectedMsg")
	}
	if foundSelected.ModelID != "gpt-4.1" {
		t.Errorf("ModelSelectedMsg.ModelID = %q, want %q", foundSelected.ModelID, "gpt-4.1")
	}
	if foundSelected.ReasoningEffort != "" {
		t.Errorf("ModelSelectedMsg.ReasoningEffort = %q, want empty", foundSelected.ReasoningEffort)
	}
}

// TestTUI137_ModelOverlayEnterReasoningModelEntersLevel1 verifies Enter on a reasoning
// model enters Level-1 without closing the overlay.
func TestTUI137_ModelOverlayEnterReasoningModelEntersLevel1(t *testing.T) {
	m := initModel(t, 80, 24)
	m = sendSlashCommand(m, "/model")

	// Navigate down to deepseek-reasoner (index 8).
	for i := 0; i < 8; i++ {
		m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = m2.(tui.Model)
	}

	// Press Enter — should enter Level-1.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m2.(tui.Model)

	if !m.OverlayActive() {
		t.Error("overlay must remain active after Enter on reasoning model")
	}
	v := m.View()
	if !strings.Contains(v, "Reasoning Effort") {
		t.Errorf("view must show 'Reasoning Effort' after entering Level-1:\n%s", v)
	}
}

// TestTUI137_ModelOverlayEnterAtConfigPanelClosesAndSetsModel verifies that:
// - Enter at Level-0 on a reasoning model (deepseek-reasoner) opens the config panel.
// - Navigating to the Reasoning section and selecting "low" effort.
// - Enter at the config panel closes the overlay and emits ModelSelectedMsg with
//   ReasoningEffort set correctly.
func TestTUI137_ModelOverlayEnterAtConfigPanelClosesAndSetsModel(t *testing.T) {
	m := initModel(t, 80, 24)
	m = sendSlashCommand(m, "/model")

	// Navigate down to deepseek-reasoner (index 8).
	for i := 0; i < 8; i++ {
		m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = m2.(tui.Model)
	}

	// Enter config panel for deepseek-reasoner.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m2.(tui.Model)

	// Must be in config panel mode.
	if !m.ModelConfigMode() {
		t.Fatal("ModelConfigMode() must be true after Enter on deepseek-reasoner")
	}
	if m.ModelConfigEntry().ID != "deepseek-reasoner" {
		t.Errorf("ModelConfigEntry().ID = %q, want %q", m.ModelConfigEntry().ID, "deepseek-reasoner")
	}

	// Navigate to Reasoning section: press j twice (section 0 → 1 → 2).
	for i := 0; i < 2; i++ {
		m3, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
		m = m3.(tui.Model)
	}
	if m.ModelConfigSection() != 2 {
		t.Errorf("ModelConfigSection() = %d, want 2 (reasoning)", m.ModelConfigSection())
	}

	// Navigate down to "low" (index 1 in ReasoningLevels) using j.
	m4, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = m4.(tui.Model)
	if m.ModelConfigReasoningCursor() != 1 {
		t.Errorf("ModelConfigReasoningCursor() = %d, want 1 (low)", m.ModelConfigReasoningCursor())
	}

	// Press Enter at config panel to confirm and close.
	m5, cmds := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m5.(tui.Model)

	// Overlay should be closed.
	if m.OverlayActive() {
		t.Error("overlay must be closed after Enter in config panel")
	}

	// Returned batch must contain a ModelSelectedMsg with ReasoningEffort="low".
	if cmds == nil {
		t.Fatal("expected cmd from Enter in config panel")
	}
	batchMsg := cmds()
	batch, ok := batchMsg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected tea.BatchMsg from config panel Enter, got %T", batchMsg)
	}
	var foundSelected *tui.ModelSelectedMsg
	for _, cmdFn := range batch {
		if cmdFn == nil {
			continue
		}
		inner := cmdFn()
		if sel, ok2 := inner.(tui.ModelSelectedMsg); ok2 {
			sel := sel
			foundSelected = &sel
		}
	}
	if foundSelected == nil {
		t.Fatal("batch must contain a ModelSelectedMsg")
	}
	if foundSelected.ReasoningEffort != "low" {
		t.Errorf("ModelSelectedMsg.ReasoningEffort = %q, want %q", foundSelected.ReasoningEffort, "low")
	}

	// Apply ModelSelectedMsg to update model state.
	m6, _ := m.Update(*foundSelected)
	m = m6.(tui.Model)

	if m.SelectedModel() != "deepseek-reasoner" {
		t.Errorf("SelectedModel() = %q, want %q", m.SelectedModel(), "deepseek-reasoner")
	}
	if m.SelectedReasoningEffort() != "low" {
		t.Errorf("SelectedReasoningEffort() = %q, want %q", m.SelectedReasoningEffort(), "low")
	}
}

// TestTUI137_ModelOverlayEnterAtLevel1ClosesAndSetsModel is kept for backward compatibility.
// It tests the config panel approach with deepseek-reasoner.
func TestTUI137_ModelOverlayEnterAtLevel1ClosesAndSetsModel(t *testing.T) {
	m := initModel(t, 80, 24)
	m = sendSlashCommand(m, "/model")

	// Navigate down to deepseek-reasoner (index 8).
	for i := 0; i < 8; i++ {
		m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = m2.(tui.Model)
	}

	// Enter config panel.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m2.(tui.Model)

	// Navigate to Reasoning section (section 2) via j twice.
	for i := 0; i < 2; i++ {
		m3, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
		m = m3.(tui.Model)
	}

	// Navigate down to "low" in the reasoning cursor (j navigates cursor when in reasoning section).
	m4, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = m4.(tui.Model)

	// Press Enter at config panel to confirm.
	m5, cmds := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m5.(tui.Model)

	// Overlay should be closed.
	if m.OverlayActive() {
		t.Error("overlay must be closed after Enter at config panel")
	}

	// Execute returned batch to get ModelSelectedMsg.
	if cmds == nil {
		t.Fatal("expected cmd from Enter at config panel")
	}
	batchMsg := cmds()
	batch, ok := batchMsg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected tea.BatchMsg, got %T", batchMsg)
	}
	var selected *tui.ModelSelectedMsg
	for _, cmdFn := range batch {
		if cmdFn == nil {
			continue
		}
		if sel, ok2 := cmdFn().(tui.ModelSelectedMsg); ok2 {
			sel := sel
			selected = &sel
		}
	}
	if selected == nil {
		t.Fatal("batch must contain a ModelSelectedMsg")
	}
	if selected.ReasoningEffort != "low" {
		t.Errorf("ModelSelectedMsg.ReasoningEffort = %q, want %q", selected.ReasoningEffort, "low")
	}

	// Apply ModelSelectedMsg to update model state.
	m6, _ := m.Update(*selected)
	m = m6.(tui.Model)

	if m.SelectedModel() != "deepseek-reasoner" {
		t.Errorf("SelectedModel() = %q, want %q", m.SelectedModel(), "deepseek-reasoner")
	}
	if m.SelectedReasoningEffort() != "low" {
		t.Errorf("SelectedReasoningEffort() = %q, want %q", m.SelectedReasoningEffort(), "low")
	}
}

// TestTUI137_ModelAppearsInHelpCommand verifies /model appears in /help output.
func TestTUI137_ModelAppearsInHelpCommand(t *testing.T) {
	m := initModel(t, 80, 24)
	m = sendSlashCommand(m, "/help")

	v := m.View()
	if !strings.Contains(v, "model") {
		t.Errorf("/model must appear in /help view:\n%s", v)
	}
}

// TestTUI137_ModelSelectedMsgUpdatesState verifies ModelSelectedMsg handler
// updates selectedModel, selectedProvider, and selectedReasoningEffort.
func TestTUI137_ModelSelectedMsgUpdatesState(t *testing.T) {
	m := initModel(t, 80, 24)

	msg := tui.ModelSelectedMsg{
		ModelID:         "o4-mini",
		Provider:        "openai",
		ReasoningEffort: "medium",
	}
	m2, _ := m.Update(msg)
	m = m2.(tui.Model)

	if m.SelectedModel() != "o4-mini" {
		t.Errorf("SelectedModel() = %q, want %q", m.SelectedModel(), "o4-mini")
	}
	if m.SelectedReasoningEffort() != "medium" {
		t.Errorf("SelectedReasoningEffort() = %q, want %q", m.SelectedReasoningEffort(), "medium")
	}
}

// TestTUI137_ModelSelectedMsgSetsStatusMsg verifies ModelSelectedMsg sets the status bar message.
func TestTUI137_ModelSelectedMsgSetsStatusMsg(t *testing.T) {
	m := initModel(t, 80, 24)

	msg := tui.ModelSelectedMsg{
		ModelID:         "o3",
		Provider:        "openai",
		ReasoningEffort: "high",
	}
	m2, _ := m.Update(msg)
	m = m2.(tui.Model)

	if !strings.Contains(m.StatusMsg(), "Model:") {
		t.Errorf("StatusMsg() must contain 'Model:' after ModelSelectedMsg, got %q", m.StatusMsg())
	}
	if !strings.Contains(m.StatusMsg(), "high") {
		t.Errorf("StatusMsg() must contain reasoning effort 'high', got %q", m.StatusMsg())
	}
}

// TestTUI137_ModelOverlayUpDownNavigates verifies Up/Down keys navigate the model list.
func TestTUI137_ModelOverlayUpDownNavigates(t *testing.T) {
	m := initModel(t, 80, 24)
	m = sendSlashCommand(m, "/model")

	// gpt-4.1 is at index 0. Press Down to move to gpt-4.1-mini.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = m2.(tui.Model)

	// The view should now show gpt-4.1-mini highlighted.
	v := m.View()
	if !strings.Contains(v, "GPT-4.1 Mini") {
		t.Errorf("view must still contain 'GPT-4.1 Mini' after Down:\n%s", v)
	}

	// Press Up to go back to gpt-4.1.
	m3, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = m3.(tui.Model)

	// Accept to confirm gpt-4.1 is selected.
	_, cmds := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmds != nil {
		msg := cmds()
		if sel, ok := msg.(tui.ModelSelectedMsg); ok {
			if sel.ModelID != "gpt-4.1" {
				t.Errorf("after Down+Up: selected model = %q, want %q", sel.ModelID, "gpt-4.1")
			}
		}
	}
}

// TestTUI137_ModelCommandInSlashCompleteDropdown verifies /model appears in the
// slash-complete suggestions when typing "/".
func TestTUI137_ModelCommandInSlashCompleteDropdown(t *testing.T) {
	m := initModel(t, 80, 24)
	// Type "/m" to trigger autocomplete.
	m = typeIntoModel(m, "/m")

	v := m.View()
	// The slash-complete dropdown should contain "model".
	if !strings.Contains(v, "model") {
		t.Errorf("slash-complete dropdown must contain 'model' when typing '/m':\n%s", v)
	}
}

// TestTUI137_SelectedModelInitialisedFromConfig verifies that SelectedModel()
// is initialised from TUIConfig.Model.
func TestTUI137_SelectedModelInitialisedFromConfig(t *testing.T) {
	cfg := tui.DefaultTUIConfig()
	cfg.Model = "gpt-4.1-mini"
	m := tui.New(cfg)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = m2.(tui.Model)

	if m.SelectedModel() != "gpt-4.1-mini" {
		t.Errorf("SelectedModel() = %q, want %q", m.SelectedModel(), "gpt-4.1-mini")
	}
}

// TestTUI137_ModelOverlaySubmitViaCommandSubmittedMsg verifies that dispatching
// CommandSubmittedMsg{Value:"/model"} also opens the overlay.
func TestTUI137_ModelOverlaySubmitViaCommandSubmittedMsg(t *testing.T) {
	m := initModel(t, 80, 24)
	m2, _ := m.Update(inputarea.CommandSubmittedMsg{Value: "/model"})
	m = m2.(tui.Model)

	if !m.OverlayActive() {
		t.Fatal("OverlayActive() must be true after CommandSubmittedMsg{/model}")
	}
	if m.ActiveOverlay() != "model" {
		t.Errorf("ActiveOverlay() = %q, want %q", m.ActiveOverlay(), "model")
	}
}
