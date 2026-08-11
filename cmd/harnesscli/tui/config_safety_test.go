package tui

import (
	"os"
	"path/filepath"
	"testing"

	harnessconfig "go-agent-harness/cmd/harnesscli/config"
)

// TestSaveSitesRefuseToWriteBackAFailedLoad is the client half of issue #1300.
//
// Load returns an empty Config on a read or parse failure. A save site that
// ignores that error mutates the empty value and writes it back, destroying every
// stored field — which is how a valid OpenRouter key was lost.
func TestSaveSitesRefuseToWriteBackAFailedLoad(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, ".config", "harnesscli")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.json")
	// A file Load cannot parse: the exact condition a torn read produced.
	if err := os.WriteFile(path, []byte(`{"api_keys":{"openrouter":"sk-keep-me"`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := persistConfigField(func(c *harnessconfig.Config) { c.Gateway = "openrouter" }); err == nil {
		t.Error("persisting over an unreadable config must fail rather than overwrite it")
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) == "" {
		t.Fatal("config file was emptied")
	}
	if !contains(string(after), "sk-keep-me") {
		t.Errorf("stored key was destroyed by a failed load; file now: %s", after)
	}
}

// TestPersistConfigFieldWritesWhenLoadSucceeds is the false-positive control:
// refusing on error must not mean refusing always.
func TestPersistConfigFieldWritesWhenLoadSucceeds(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := harnessconfig.Save(&harnessconfig.Config{
		APIKeys: map[string]string{"openrouter": "sk-existing"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := persistConfigField(func(c *harnessconfig.Config) { c.Gateway = "openrouter" }); err != nil {
		t.Fatalf("persistConfigField: %v", err)
	}

	got, err := harnessconfig.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Gateway != "openrouter" {
		t.Errorf("Gateway = %q, want openrouter", got.Gateway)
	}
	// The mutation must not have dropped the neighbouring field.
	if got.APIKeys["openrouter"] != "sk-existing" {
		t.Errorf("existing key lost: %v", got.APIKeys)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
