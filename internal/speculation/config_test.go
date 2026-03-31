package speculation_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"

	"go-agent-harness/internal/speculation"
)

// TestDefaultSpeculationConfig_DisabledByDefault verifies speculation is off by default.
func TestDefaultSpeculationConfig_DisabledByDefault(t *testing.T) {
	cfg := speculation.DefaultSpeculationConfig()
	if cfg.Enabled {
		t.Error("DefaultSpeculationConfig().Enabled: got true, want false (must be disabled by default)")
	}
}

// TestDefaultSpeculationConfig_Limits verifies MaxTurns=20 and MaxMessages=100.
func TestDefaultSpeculationConfig_Limits(t *testing.T) {
	cfg := speculation.DefaultSpeculationConfig()
	if cfg.MaxTurns != 20 {
		t.Errorf("MaxTurns: got %d, want 20", cfg.MaxTurns)
	}
	if cfg.MaxMessages != 100 {
		t.Errorf("MaxMessages: got %d, want 100", cfg.MaxMessages)
	}
}

// TestDefaultSpeculationConfig_StopOnWrite verifies StopOnWrite=true by default.
func TestDefaultSpeculationConfig_StopOnWrite(t *testing.T) {
	cfg := speculation.DefaultSpeculationConfig()
	if !cfg.StopOnWrite {
		t.Error("StopOnWrite: got false, want true")
	}
}

// TestDefaultSpeculationConfig_AllowedTools verifies default allowed tools are set.
func TestDefaultSpeculationConfig_AllowedTools(t *testing.T) {
	cfg := speculation.DefaultSpeculationConfig()
	if len(cfg.AllowedTools) == 0 {
		t.Fatal("AllowedTools: got empty slice, want at least one tool")
	}
	// Must contain "read"
	found := false
	for _, tool := range cfg.AllowedTools {
		if tool == "read" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("AllowedTools: want 'read' in list, got %v", cfg.AllowedTools)
	}
}

// TestSpeculationConfig_FromTOML verifies the [speculation] TOML section is parsed correctly.
func TestSpeculationConfig_FromTOML(t *testing.T) {
	tomlContent := `
[speculation]
enabled = true
max_turns = 10
max_messages = 50
overlay_dir = "/tmp/my-spec"
stop_on_write = false
allowed_tools = ["read", "grep"]
`
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(cfgFile, []byte(tomlContent), 0600); err != nil {
		t.Fatal(err)
	}

	// Use the TOMLWrapper to parse the [speculation] section
	var wrapper struct {
		Speculation speculation.SpeculationConfig `toml:"speculation"`
	}
	f, err := os.Open(cfgFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := toml.NewDecoder(f).Decode(&wrapper); err != nil {
		t.Fatalf("TOML decode error: %v", err)
	}
	cfg := wrapper.Speculation

	if !cfg.Enabled {
		t.Error("Enabled: got false, want true")
	}
	if cfg.MaxTurns != 10 {
		t.Errorf("MaxTurns: got %d, want 10", cfg.MaxTurns)
	}
	if cfg.MaxMessages != 50 {
		t.Errorf("MaxMessages: got %d, want 50", cfg.MaxMessages)
	}
	if cfg.OverlayDir != "/tmp/my-spec" {
		t.Errorf("OverlayDir: got %q, want %q", cfg.OverlayDir, "/tmp/my-spec")
	}
	if cfg.StopOnWrite {
		t.Error("StopOnWrite: got true, want false")
	}
	if len(cfg.AllowedTools) != 2 {
		t.Errorf("AllowedTools: got %d entries, want 2", len(cfg.AllowedTools))
	}
}
