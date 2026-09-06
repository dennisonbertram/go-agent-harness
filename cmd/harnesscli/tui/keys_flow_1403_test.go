package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"go-agent-harness/cmd/harnesscli/tui"
)

// Issue #1403: a chat message must never end up saved as an API key.

func keysOverlay(t *testing.T, w, h int, providers []tui.ProviderInfo) tui.Model {
	t.Helper()
	m := initModel(t, w, h)
	m = sendSlashCommand(m, "/keys")
	m2, _ := m.Update(tui.ProvidersLoadedMsg{Providers: providers})
	return m2.(tui.Model)
}

// Printable keys typed while the keys overlay is open (list mode) must not
// leak into the chat input.
func TestOverlay_TypedRunesDoNotReachInput(t *testing.T) {
	m := keysOverlay(t, 120, 40, []tui.ProviderInfo{{Name: "openai", APIKeyEnv: "OPENAI_API_KEY"}})
	m = typeIntoModel(m, "hello there")
	if m.Input() != "" {
		t.Fatalf("typed text leaked into the chat input while the keys overlay was open: %q", m.Input())
	}
	if !m.OverlayActive() {
		t.Fatalf("overlay must stay open")
	}
}

// The key form rejects values that cannot be API keys and stays in edit mode.
func TestAPIKeys_RejectsImplausibleKey(t *testing.T) {
	for _, bad := range []string{"/model", "hello world", "   "} {
		m := keysOverlay(t, 120, 40, []tui.ProviderInfo{{Name: "openai", APIKeyEnv: "OPENAI_API_KEY"}})
		m = sendKey(m, tea.KeyEnter) // edit the highlighted provider
		if !m.APIKeyInputMode() {
			t.Fatalf("Enter must open the key form")
		}
		m = typeIntoModel(m, bad)
		m = sendKey(m, tea.KeyEnter)
		if !m.APIKeyInputMode() {
			t.Errorf("value %q must be rejected and keep the form open", bad)
		}
		if !strings.Contains(strings.ToLower(m.StatusMsg()), "api key") {
			t.Errorf("value %q: status must explain the rejection, got %q", bad, m.StatusMsg())
		}
	}
}

// Selecting an unavailable model must say why the keys screen opened.
func TestModelPicker_UnavailableSelectionExplains(t *testing.T) {
	providers := []tui.ProviderInfo{
		{Name: "groq", Configured: false, APIKeyEnv: "GROQ_API_KEY"},
		{Name: "anthropic", Configured: true, APIKeyEnv: "ANTHROPIC_API_KEY"},
	}
	t.Setenv("GROQ_API_KEY", "")
	m := openModelOverlayWithProviders(t, providers)
	m = navigateToModelByID(m, "llama-3.3-70b-versatile")
	// Walk down until the highlight sits on a model whose provider is not configured.
	found := false
	for i := 0; i < 80; i++ {
		if entry, ok := m.ModelSwitcher().Accept(); ok && !entry.Available && m.ModelSwitcher().AvailabilityKnown() {
			found = true
			break
		}
		m = sendKey(m, tea.KeyDown)
	}
	if !found {
		t.Skip("no unavailable model reachable in the fixture")
	}
	m = sendKey(m, tea.KeyEnter)
	view := m.View()
	if !strings.Contains(view, "GROQ_API_KEY") || !strings.Contains(view, "not set up") {
		t.Fatalf("keys screen must explain the redirect (provider not set up, which key), view:\n%s", view)
	}
}

// Keys rows must fit inside the box at 120 columns, and subscription labels
// must name their own product.
func TestAPIKeys_RowsFitBoxAndLabels(t *testing.T) {
	m := keysOverlay(t, 120, 40, []tui.ProviderInfo{
		{Name: "codex-subscription", AuthType: "subscription"},
		{Name: "kimi-subscription", AuthType: "subscription", Configured: true},
		{Name: "openrouter", APIKeyEnv: "OPENROUTER_API_KEY", Configured: true},
		{Name: "anthropic", APIKeyEnv: "ANTHROPIC_API_KEY"},
	})
	view := m.View()
	for _, line := range strings.Split(view, "\n") {
		if w := lipgloss.Width(line); w > 120 {
			t.Errorf("row wider than the terminal (%d): %q", w, line)
		}
	}
	// A wrapped row shows the status on a line of its own.
	for _, line := range strings.Split(view, "\n") {
		trimmed := strings.TrimSpace(strings.Trim(strings.TrimSpace(line), "│"))
		if trimmed == "not connected" || trimmed == "connected" || trimmed == "(env)" {
			t.Errorf("status wrapped onto its own line: %q", line)
		}
	}
	if strings.Contains(view, "kimi-subscription ChatGPT") || (strings.Contains(view, "kimi-subscription") && !strings.Contains(view, "Kimi subscription")) {
		t.Errorf("kimi-subscription must be labelled as a Kimi subscription, view:\n%s", view)
	}
}
