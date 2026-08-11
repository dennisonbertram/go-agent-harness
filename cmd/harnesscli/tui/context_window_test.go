package tui

import (
	"encoding/json"
	"testing"

	"go-agent-harness/cmd/harnesscli/tui/components/modelswitcher"
)

// TestContextWindowTotalUsesModelWindow is the regression test for issue #1306:
// the meter must divide by the selected model's real context window, not by a
// hardcoded constant.
func TestContextWindowTotalUsesModelWindow(t *testing.T) {
	m := Model{
		selectedModel:    "openrouter/pareto-code",
		selectedProvider: "openrouter",
		serverModels: []modelswitcher.ServerModelEntry{
			{ID: "openrouter/pareto-code", Provider: "openrouter", ContextWindow: 2000000},
		},
	}
	m.applyContextWindowForSelectedModel()

	if got := m.contextWindowTotal(); got != 2000000 {
		t.Errorf("contextWindowTotal() = %d, want 2000000 (the model's declared window)", got)
	}
}

// TestContextWindowTotalFallsBackWhenUnknown is the false-positive control: the
// fix must not be achievable by hardcoding a different constant.
func TestContextWindowTotalFallsBackWhenUnknown(t *testing.T) {
	for _, tc := range []struct {
		name    string
		entries []modelswitcher.ServerModelEntry
	}{
		{"model absent from the list", nil},
		{
			"model present but declares no window",
			[]modelswitcher.ServerModelEntry{{ID: "some/model", Provider: "p"}},
		},
		{
			"model declares a zero window",
			[]modelswitcher.ServerModelEntry{{ID: "some/model", Provider: "p", ContextWindow: 0}},
		},
		{
			"model declares a negative window",
			[]modelswitcher.ServerModelEntry{{ID: "some/model", Provider: "p", ContextWindow: -1}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := Model{selectedModel: "some/model", selectedProvider: "p", serverModels: tc.entries}
			m.applyContextWindowForSelectedModel()

			if got := m.contextWindowTotal(); got != defaultContextWindowTokens {
				t.Errorf("contextWindowTotal() = %d, want the %d fallback", got, defaultContextWindowTokens)
			}
		})
	}
}

// TestContextWindowMatchesProviderNotJustID guards against picking a window from
// a same-named model belonging to a different provider.
func TestContextWindowMatchesProviderNotJustID(t *testing.T) {
	m := Model{
		selectedModel:    "shared-id",
		selectedProvider: "wanted",
		serverModels: []modelswitcher.ServerModelEntry{
			{ID: "shared-id", Provider: "other", ContextWindow: 999},
			{ID: "shared-id", Provider: "wanted", ContextWindow: 555000},
		},
	}
	m.applyContextWindowForSelectedModel()

	if got := m.contextWindowTotal(); got != 555000 {
		t.Errorf("contextWindowTotal() = %d, want 555000 from the matching provider", got)
	}
}

// TestModelsResponseDecodesContextWindow pins the wire contract: the server's
// field name and the client's tag must agree, or the window silently stays zero
// and every model falls back to the 200K default (issue #1306).
func TestModelsResponseDecodesContextWindow(t *testing.T) {
	const body = `{"models":[{"id":"openrouter/pareto-code","provider":"openrouter","context_window":2000000},{"id":"no/window","provider":"p"}]}`

	var mr modelsResponse
	if err := json.Unmarshal([]byte(body), &mr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(mr.Models) != 2 {
		t.Fatalf("decoded %d models, want 2", len(mr.Models))
	}
	if got := mr.Models[0].ContextWindow; got != 2000000 {
		t.Errorf("ContextWindow = %d, want 2000000", got)
	}
	// A model with no declared window must decode to zero, not to a default.
	if got := mr.Models[1].ContextWindow; got != 0 {
		t.Errorf("undeclared ContextWindow = %d, want 0", got)
	}
}
