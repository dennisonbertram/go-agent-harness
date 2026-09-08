package tui

import (
	"testing"

	harnessconfig "go-agent-harness/cmd/harnesscli/config"
)

// TestRememberedModelAppliedAtStartup pins issue #1424: a model chosen in a
// previous session is active on the next start, with no user action.
//
// It asserts the same three fields a live selection sets, because the status
// bar, the switcher's highlight, the context window and the submitted run all
// read from them — restoring only selectedModel would leave the UI agreeing
// with itself while runs went somewhere else.
func TestRememberedModelAppliedAtStartup(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := harnessconfig.Save(&harnessconfig.Config{
		Model:           "gpt-4.1-mini",
		Provider:        "openai",
		ReasoningEffort: "medium",
	}); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	m := New(TUIConfig{}) // no explicit model requested

	if m.selectedModel != "gpt-4.1-mini" {
		t.Errorf("selectedModel = %q, want the remembered %q", m.selectedModel, "gpt-4.1-mini")
	}
	if m.selectedProvider != "openai" {
		t.Errorf("selectedProvider = %q, want %q", m.selectedProvider, "openai")
	}
	if m.selectedReasoningEffort != "medium" {
		t.Errorf("selectedReasoningEffort = %q, want %q", m.selectedReasoningEffort, "medium")
	}
}

// TestExplicitModelBeatsRememberedModel guards the precedence rule: remembering
// a preference must never override an instruction. Today the TUI has no wired
// -model flag, so this is the guard that keeps precedence correct when one is
// added rather than a live path.
func TestExplicitModelBeatsRememberedModel(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := harnessconfig.Save(&harnessconfig.Config{
		Model:    "remembered-model",
		Provider: "openai",
	}); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	m := New(TUIConfig{Model: "explicitly-requested"})

	if m.selectedModel != "explicitly-requested" {
		t.Errorf("selectedModel = %q, want the explicitly requested %q; a remembered "+
			"preference must not override an instruction", m.selectedModel, "explicitly-requested")
	}
}

// TestNoRememberedModelLeavesDaemonDefault is the false-positive control. An
// empty selectedModel means "let the daemon choose"; if this test could pass
// with some value invented here, the feature would be indistinguishable from
// hardcoding a default nobody asked for.
func TestNoRememberedModelLeavesDaemonDefault(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	m := New(TUIConfig{})

	if m.selectedModel != "" {
		t.Errorf("selectedModel = %q, want empty so the daemon default applies", m.selectedModel)
	}
}

// TestModelSelectionIsPersisted pins the write half: choosing a model in the
// TUI stores it, or there is nothing to remember next time.
func TestModelSelectionIsPersisted(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	m := New(TUIConfig{})
	updated, _ := m.Update(ModelSelectedMsg{
		ModelID:         "claude-sonnet-5",
		Provider:        "anthropic",
		ReasoningEffort: "high",
	})
	_ = updated

	stored, err := harnessconfig.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if stored.Model != "claude-sonnet-5" {
		t.Errorf("stored Model = %q, want %q", stored.Model, "claude-sonnet-5")
	}
	if stored.Provider != "anthropic" {
		t.Errorf("stored Provider = %q, want %q", stored.Provider, "anthropic")
	}
	if stored.ReasoningEffort != "high" {
		t.Errorf("stored ReasoningEffort = %q, want %q", stored.ReasoningEffort, "high")
	}
}
