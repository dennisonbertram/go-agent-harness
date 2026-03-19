package tui_test

// Tests for GitHub issue #315 — provider auth TUI enhancements:
//
//   Gap 1: Selecting an unavailable (greyed-out) model opens /keys overlay
//           pre-positioned on the provider for that model.
//
//   Gap 2: When no providers are configured, a hint is shown in the status bar.
//
//   Gap 3: Selecting a codex model while OpenAI is unconfigured shows a
//           special instructional message.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	tui "go-agent-harness/cmd/harnesscli/tui"
	"go-agent-harness/cmd/harnesscli/tui/components/modelswitcher"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// openModelOverlayWithProviders opens the /model overlay, injects providers
// and sets availability on the model switcher.
func openModelOverlayWithProviders(t *testing.T, providers []tui.ProviderInfo) tui.Model {
	t.Helper()
	m := initModel(t, 80, 24)
	m = sendSlashCommand(m, "/model")
	// Inject providers so availability is set.
	m2, _ := m.Update(tui.ProvidersLoadedMsg{Providers: providers})
	m = m2.(tui.Model)
	return m
}

// navigateToModelByID navigates to the given model ID. With the two-level hierarchy:
// 1. Find which provider the target model belongs to (by looking at DefaultModels).
// 2. Navigate the provider cursor to that provider at level 0.
// 3. Press Enter to drill into the provider (level 1).
// 4. Navigate the model cursor to the target model within that provider.
func navigateToModelByID(m tui.Model, targetID string) tui.Model {
	// Find the target model's provider label.
	targetProviderLabel := ""
	for _, dm := range modelswitcher.DefaultModels {
		if dm.ID == targetID {
			targetProviderLabel = dm.ProviderLabel
			break
		}
	}
	if targetProviderLabel == "" {
		// Unknown model — try navigating directly within current level.
		for range modelswitcher.DefaultModels {
			entry, _ := m.ModelSwitcher().Accept()
			if entry.ID == targetID {
				return m
			}
			m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
			m = m2.(tui.Model)
		}
		return m
	}

	// Navigate to the provider at level 0.
	for i := 0; i < len(m.ModelSwitcher().Providers()); i++ {
		if m.ModelSwitcher().Providers()[m.ModelSwitcher().ProviderCursorIndex()].Label == targetProviderLabel {
			break
		}
		m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = m2.(tui.Model)
	}

	// Drill into provider (Enter at level 0).
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m2.(tui.Model)

	// Now at level 1 — navigate to the target model.
	for i := 0; i < 30; i++ {
		entry, _ := m.ModelSwitcher().Accept()
		if entry.ID == targetID {
			return m
		}
		m3, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = m3.(tui.Model)
	}
	return m
}

// ─── Gap 1: unavailable model → /keys overlay ────────────────────────────────

// TestTUI315_UnavailableModelEnterOpensKeysOverlay verifies that pressing Enter
// on a greyed-out (unavailable) model opens the /keys overlay.
func TestTUI315_UnavailableModelEnterOpensKeysOverlay(t *testing.T) {
	providers := []tui.ProviderInfo{
		{Name: "openai", Configured: false, APIKeyEnv: "OPENAI_API_KEY"},
		{Name: "anthropic", Configured: true, APIKeyEnv: "ANTHROPIC_API_KEY"},
	}
	m := openModelOverlayWithProviders(t, providers)
	// Navigate to a model whose provider is openai (unavailable).
	m = navigateToModelByID(m, "gpt-4.1")

	// Confirm the entry is actually unavailable before pressing Enter.
	entry, _ := m.ModelSwitcher().Accept()
	if entry.Available {
		t.Skip("model is Available — skip; test requires openai to be unconfigured")
	}

	// Press Enter on the unavailable model.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m2.(tui.Model)

	if !m.OverlayActive() {
		t.Fatal("OverlayActive() must be true after selecting unavailable model")
	}
	if m.ActiveOverlay() != "apikeys" {
		t.Errorf("ActiveOverlay(): want %q, got %q", "apikeys", m.ActiveOverlay())
	}
}

// TestTUI315_UnavailableModelKeysOverlayPrePositioned verifies the /keys overlay
// cursor is pre-positioned on the provider of the unavailable model.
func TestTUI315_UnavailableModelKeysOverlayPrePositioned(t *testing.T) {
	providers := []tui.ProviderInfo{
		{Name: "anthropic", Configured: false, APIKeyEnv: "ANTHROPIC_API_KEY"},
		{Name: "openai", Configured: true, APIKeyEnv: "OPENAI_API_KEY"},
	}
	m := openModelOverlayWithProviders(t, providers)
	// Navigate to a Claude model (anthropic provider, unconfigured).
	m = navigateToModelByID(m, "claude-sonnet-4-6")

	entry, _ := m.ModelSwitcher().Accept()
	if entry.Available {
		t.Skip("model is Available — skip; test requires anthropic to be unconfigured")
	}

	// Press Enter.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m2.(tui.Model)

	if m.ActiveOverlay() != "apikeys" {
		t.Fatalf("expected apikeys overlay, got %q", m.ActiveOverlay())
	}

	// The cursor should be on "anthropic" (index 0 in the providers list).
	cursor := m.APIKeyCursor()
	providers2 := m.APIKeyProviders()
	if cursor < 0 || cursor >= len(providers2) {
		t.Fatalf("cursor %d out of range [0, %d)", cursor, len(providers2))
	}
	if providers2[cursor].Name != "anthropic" {
		t.Errorf("cursor positioned on %q, want %q", providers2[cursor].Name, "anthropic")
	}
}

// TestTUI315_AvailableModelEnterOpensConfigPanel verifies that pressing Enter
// on an available model still opens the config panel (existing behaviour is preserved).
func TestTUI315_AvailableModelEnterOpensConfigPanel(t *testing.T) {
	providers := []tui.ProviderInfo{
		{Name: "openai", Configured: true, APIKeyEnv: "OPENAI_API_KEY"},
	}
	m := openModelOverlayWithProviders(t, providers)
	m = navigateToModelByID(m, "gpt-4.1")

	entry, _ := m.ModelSwitcher().Accept()
	if !entry.Available {
		t.Skip("model is not Available — skip; test requires openai to be configured")
	}

	// Press Enter.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m2.(tui.Model)

	// Should open config panel, NOT the /keys overlay.
	if m.ActiveOverlay() == "apikeys" {
		t.Error("should NOT open apikeys overlay for an available model")
	}
	if !m.ModelConfigMode() {
		t.Error("ModelConfigMode() must be true after Enter on an available model")
	}
}

// ─── Gap 2: empty-state status bar prompt ─────────────────────────────────────

// TestTUI315_EmptyStateHintWhenNoProvidersConfigured verifies that when
// ProvidersLoadedMsg arrives with all providers unconfigured, a hint appears
// in the status bar.
func TestTUI315_EmptyStateHintWhenNoProvidersConfigured(t *testing.T) {
	m := initModel(t, 80, 24)

	// Inject all-unconfigured providers.
	m2, _ := m.Update(tui.ProvidersLoadedMsg{
		Providers: []tui.ProviderInfo{
			{Name: "openai", Configured: false, APIKeyEnv: "OPENAI_API_KEY"},
			{Name: "anthropic", Configured: false, APIKeyEnv: "ANTHROPIC_API_KEY"},
		},
	})
	m = m2.(tui.Model)

	statusMsg := m.StatusMsg()
	if statusMsg == "" {
		t.Fatal("StatusMsg() must be non-empty when no providers are configured")
	}
	if !strings.Contains(strings.ToLower(statusMsg), "keys") {
		t.Errorf("StatusMsg() %q should mention 'keys'", statusMsg)
	}
}

// TestTUI315_NoEmptyStateHintWhenProviderConfigured verifies that when at
// least one provider is configured, no empty-state hint is emitted.
func TestTUI315_NoEmptyStateHintWhenProviderConfigured(t *testing.T) {
	m := initModel(t, 80, 24)

	// Inject with one configured provider.
	m2, _ := m.Update(tui.ProvidersLoadedMsg{
		Providers: []tui.ProviderInfo{
			{Name: "openai", Configured: true, APIKeyEnv: "OPENAI_API_KEY"},
			{Name: "anthropic", Configured: false, APIKeyEnv: "ANTHROPIC_API_KEY"},
		},
	})
	m = m2.(tui.Model)

	statusMsg := m.StatusMsg()
	// Must NOT show the "no providers" hint.
	if strings.Contains(strings.ToLower(statusMsg), "no providers") {
		t.Errorf("StatusMsg() %q must not show 'no providers' hint when some are configured", statusMsg)
	}
}

// TestTUI315_NoEmptyStateHintWhenProvidersListEmpty verifies that when the
// providers list is completely empty (server returned nothing), no empty-state
// hint is shown — we can't be sure providers are really unconfigured.
func TestTUI315_NoEmptyStateHintWhenProvidersListEmpty(t *testing.T) {
	m := initModel(t, 80, 24)

	m2, _ := m.Update(tui.ProvidersLoadedMsg{
		Providers: []tui.ProviderInfo{},
	})
	m = m2.(tui.Model)

	statusMsg := m.StatusMsg()
	if strings.Contains(strings.ToLower(statusMsg), "no providers") {
		t.Errorf("StatusMsg() %q must not show 'no providers' for empty list", statusMsg)
	}
}

// ─── Gap 3: Codex model special instruction message ───────────────────────────

// TestTUI315_CodexModelUnconfiguredShowsInstruction verifies that when a codex
// model is selected while OpenAI is unconfigured, a special instructional
// message is shown explaining how to configure it.
func TestTUI315_CodexModelUnconfiguredShowsInstruction(t *testing.T) {
	// Clear env vars so the env-bootstrap in New() doesn't mark openai as available.
	t.Setenv("OPENAI_API_KEY", "")
	providers := []tui.ProviderInfo{
		{Name: "openai", Configured: false, APIKeyEnv: "OPENAI_API_KEY"},
	}
	// Synthesise a ModelSelectedMsg for a codex model.
	m := initModel(t, 80, 24)
	// Inject provider list so the model knows OpenAI is unconfigured.
	m2, _ := m.Update(tui.ProvidersLoadedMsg{Providers: providers})
	m = m2.(tui.Model)

	// Fire ModelSelectedMsg for a codex model.
	m3, _ := m.Update(tui.ModelSelectedMsg{
		ModelID:  "gpt-5.1-codex-mini",
		Provider: "openai",
	})
	m = m3.(tui.Model)

	statusMsg := m.StatusMsg()
	if statusMsg == "" {
		t.Fatal("StatusMsg() must be non-empty when selecting unconfigured codex model")
	}
	// The message should contain something about the codex/openai key.
	lower := strings.ToLower(statusMsg)
	if !strings.Contains(lower, "codex") && !strings.Contains(lower, "openai") {
		t.Errorf("StatusMsg() %q should mention 'codex' or 'openai'", statusMsg)
	}
	if !strings.Contains(lower, "key") && !strings.Contains(lower, "openai_api_key") {
		t.Errorf("StatusMsg() %q should mention 'key' or 'OPENAI_API_KEY'", statusMsg)
	}
}

// TestTUI315_CodexModelConfiguredDoesNotShowInstruction verifies that when
// OpenAI IS configured, selecting a codex model does NOT show the special
// instruction (only shows the normal "Model: ..." status).
func TestTUI315_CodexModelConfiguredDoesNotShowInstruction(t *testing.T) {
	providers := []tui.ProviderInfo{
		{Name: "openai", Configured: true, APIKeyEnv: "OPENAI_API_KEY"},
	}
	m := initModel(t, 80, 24)
	m2, _ := m.Update(tui.ProvidersLoadedMsg{Providers: providers})
	m = m2.(tui.Model)

	m3, _ := m.Update(tui.ModelSelectedMsg{
		ModelID:  "gpt-5.1-codex-mini",
		Provider: "openai",
	})
	m = m3.(tui.Model)

	statusMsg := m.StatusMsg()
	lower := strings.ToLower(statusMsg)
	if strings.Contains(lower, "set openai_api_key") || strings.Contains(lower, "codex uses") {
		t.Errorf("StatusMsg() %q should NOT show codex instruction when OpenAI is configured", statusMsg)
	}
}

// TestTUI315_NonCodexUnconfiguredModelDoesNotShowCodexInstruction verifies
// that a regular unavailable model does NOT show the codex-specific message.
func TestTUI315_NonCodexUnconfiguredModelDoesNotShowCodexInstruction(t *testing.T) {
	providers := []tui.ProviderInfo{
		{Name: "anthropic", Configured: false, APIKeyEnv: "ANTHROPIC_API_KEY"},
	}
	m := initModel(t, 80, 24)
	m2, _ := m.Update(tui.ProvidersLoadedMsg{Providers: providers})
	m = m2.(tui.Model)

	m3, _ := m.Update(tui.ModelSelectedMsg{
		ModelID:  "claude-sonnet-4-6",
		Provider: "anthropic",
	})
	m = m3.(tui.Model)

	statusMsg := m.StatusMsg()
	lower := strings.ToLower(statusMsg)
	if strings.Contains(lower, "codex") {
		t.Errorf("StatusMsg() %q should not mention 'codex' for non-codex model", statusMsg)
	}
}
