package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"go-agent-harness/cmd/harnesscli/tui"
	"go-agent-harness/cmd/harnesscli/tui/components/modelswitcher"
)

// Issue #1403 (routing): a model that only exists on OpenRouter must be sent to
// OpenRouter with its own id, whatever the "gateway" setting says. Before the
// fix, the direct gateway rewrote "deepseek/deepseek-v4-pro" to "deepseek-v4"
// and still sent it to OpenRouter, which rejected it.
func TestOpenRouterModel_DirectGatewayKeepsSlugAndProvider(t *testing.T) {
	m := initModel(t, 120, 40)
	m2, _ := m.Update(tui.GatewaySelectedMsg{Gateway: ""})
	m = m2.(tui.Model)
	m3, _ := m.Update(tui.ModelSelectedMsg{ModelID: "deepseek/deepseek-v4-pro", Provider: "openrouter"})
	m = m3.(tui.Model)
	model, provider := m.EffectiveModelAndProvider()
	if model != "deepseek/deepseek-v4-pro" || provider != "openrouter" {
		t.Fatalf("want (deepseek/deepseek-v4-pro, openrouter), got (%s, %s)", model, provider)
	}
}

// The configuration panel must not offer a "Direct" gateway for such a model;
// it says the model is served by OpenRouter.
func TestOpenRouterModel_ConfigPanelExplainsRouting(t *testing.T) {
	providers := []tui.ProviderInfo{{Name: "openrouter", Configured: true, APIKeyEnv: "OPENROUTER_API_KEY"}}
	m := openModelOverlayWithProviders(t, providers)
	m2, _ := m.Update(tui.ModelsFetchedMsg{Models: []modelswitcher.ServerModelEntry{{ID: "deepseek/deepseek-v4-pro", Provider: "openrouter"}}})
	m = m2.(tui.Model)
	// Reach the model through the picker's search, as a user would.
	m = typeIntoModel(m, "/deepseek/deepseek-v4-pro")
	if entry, ok := m.ModelSwitcher().Accept(); !ok || entry.ID != "deepseek/deepseek-v4-pro" {
		t.Fatalf("search did not land on the model, got %+v", entry)
	}
	m = sendKey(m, tea.KeyEnter)
	view := m.View()
	if !strings.Contains(view, "served only by OpenRouter") {
		t.Fatalf("config panel must explain that the model is served by OpenRouter, view:\n%s", view)
	}
	if strings.Contains(view, "Use each model's native provider") {
		t.Fatalf("config panel must not offer the Direct gateway for an OpenRouter-only model")
	}
}
