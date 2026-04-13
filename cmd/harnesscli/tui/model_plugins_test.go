package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWithPluginsDirLoadsPluginsAndExposesWarnings(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()
	summarizePlugin := `{
		"name": "summarize",
		"description": "Summarize the current topic",
		"handler": "prompt",
		"prompt_template": "Summarize: {args}"
	}`
	if err := os.WriteFile(filepath.Join(dir, "summarize.json"), []byte(summarizePlugin), 0o644); err != nil {
		t.Fatalf("write summarize plugin: %v", err)
	}

	collisionPlugin := `{
		"name": "help",
		"description": "Overrides built-in help",
		"handler": "prompt",
		"prompt_template": "nope"
	}`
	if err := os.WriteFile(filepath.Join(dir, "help.json"), []byte(collisionPlugin), 0o644); err != nil {
		t.Fatalf("write collision plugin: %v", err)
	}

	model := New(DefaultTUIConfig()).WithPluginsDir(dir)

	if model.pluginsDir != dir {
		t.Fatalf("expected pluginsDir %q, got %q", dir, model.pluginsDir)
	}

	warnings := model.PluginWarnings()
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %v", warnings)
	}
	if !strings.Contains(warnings[0], "already registered") {
		t.Fatalf("expected collision warning, got %q", warnings[0])
	}

	entry, ok := model.commandRegistry.Lookup("summarize")
	if !ok {
		t.Fatal("expected summarize command to be registered")
	}
	result := entry.Handler(Command{Name: "summarize", Args: []string{"release", "notes"}})
	if result.Status != CmdOK {
		t.Fatalf("expected CmdOK, got %v with output %q", result.Status, result.Output)
	}
	if result.Output != "Summarize: release notes" {
		t.Fatalf("unexpected plugin output: %q", result.Output)
	}
}
