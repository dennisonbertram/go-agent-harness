package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAndRegisterPlugins_RegistersPromptPlugin(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pluginJSON := `{
		"name": "summarize",
		"description": "Summarize the current topic",
		"handler": "prompt",
		"prompt_template": "Summarize: {args}"
	}`
	if err := os.WriteFile(filepath.Join(dir, "summarize.json"), []byte(pluginJSON), 0o644); err != nil {
		t.Fatalf("write plugin file: %v", err)
	}

	registry := NewCommandRegistry()
	warnings := LoadAndRegisterPlugins(registry, dir)
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	entry, ok := registry.Lookup("summarize")
	if !ok {
		t.Fatal("expected summarize plugin command to be registered")
	}
	result := entry.Handler(Command{Name: "summarize", Args: []string{"release", "notes"}})
	if result.Status != CmdOK {
		t.Fatalf("expected CmdOK, got %v with output %q", result.Status, result.Output)
	}
	if result.Output != "Summarize: release notes" {
		t.Fatalf("unexpected plugin output: %q", result.Output)
	}
}

func TestLoadAndRegisterPlugins_SkipsCommandCollisions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pluginJSON := `{
		"name": "help",
		"description": "Overrides built-in help",
		"handler": "prompt",
		"prompt_template": "nope"
	}`
	if err := os.WriteFile(filepath.Join(dir, "help.json"), []byte(pluginJSON), 0o644); err != nil {
		t.Fatalf("write plugin file: %v", err)
	}

	registry := NewCommandRegistry()
	warnings := LoadAndRegisterPlugins(registry, dir)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %v", warnings)
	}
	if !strings.Contains(warnings[0], "already registered") {
		t.Fatalf("expected collision warning, got %q", warnings[0])
	}

	entry, ok := registry.Lookup("help")
	if !ok {
		t.Fatal("expected built-in help command to remain registered")
	}
	result := entry.Handler(Command{Name: "help"})
	if result.Status != CmdOK {
		t.Fatalf("expected built-in help command to remain intact, got status %v", result.Status)
	}
}
