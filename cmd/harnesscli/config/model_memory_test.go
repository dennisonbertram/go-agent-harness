package config

import (
	"testing"
)

// TestConfigRoundTripsModelSelection pins issue #1424: the model a user picks
// must survive a restart, together with the provider and reasoning effort that
// were chosen in the same moment and mean nothing apart from it.
func TestConfigRoundTripsModelSelection(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := Save(&Config{
		Model:           "gpt-4.1-mini",
		Provider:        "openai",
		ReasoningEffort: "medium",
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Model != "gpt-4.1-mini" {
		t.Errorf("Model = %q, want %q", got.Model, "gpt-4.1-mini")
	}
	if got.Provider != "openai" {
		t.Errorf("Provider = %q, want %q", got.Provider, "openai")
	}
	if got.ReasoningEffort != "medium" {
		t.Errorf("ReasoningEffort = %q, want %q", got.ReasoningEffort, "medium")
	}
}

// TestConfigWithNoModelRemembersNothing is the false-positive control: an empty
// store must stay empty, so "remembering" cannot be faked by defaulting to some
// model nobody chose.
func TestConfigWithNoModelRemembersNothing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Model != "" || got.Provider != "" || got.ReasoningEffort != "" {
		t.Errorf("empty store should remember nothing, got model=%q provider=%q effort=%q",
			got.Model, got.Provider, got.ReasoningEffort)
	}
}
