package memory

import (
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestDriftProtectionText_Enabled(t *testing.T) {
	t.Parallel()
	cfg := DefaultMemoryConfig()
	cfg.DriftProtectionEnabled = true

	text := DriftProtectionText(cfg)
	if text == "" {
		t.Error("DriftProtectionText() = empty string, want non-empty when drift protection enabled")
	}
}

func TestDriftProtectionText_Disabled(t *testing.T) {
	t.Parallel()
	cfg := DefaultMemoryConfig()
	cfg.DriftProtectionEnabled = false

	text := DriftProtectionText(cfg)
	if text != "" {
		t.Errorf("DriftProtectionText() = %q, want empty string when drift protection disabled", text)
	}
}

func TestDriftProtectionText_IsSection(t *testing.T) {
	t.Parallel()
	cfg := DefaultMemoryConfig()
	cfg.DriftProtectionEnabled = true

	text := DriftProtectionText(cfg)
	if !strings.HasPrefix(text, "## ") {
		t.Errorf("DriftProtectionText() does not start with '## ': got prefix %q", text[:min(len(text), 10)])
	}
}

func TestDriftProtectionText_ContainsVerification(t *testing.T) {
	t.Parallel()
	cfg := DefaultMemoryConfig()
	cfg.DriftProtectionEnabled = true

	text := DriftProtectionText(cfg)
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "verify") && !strings.Contains(lower, "verification") {
		t.Error("DriftProtectionText() does not mention 'verify' or 'verification'")
	}
}

// tomlMemoryConfig is a minimal TOML struct for config parsing tests.
type tomlMemoryConfig struct {
	Memory struct {
		Enabled               bool   `toml:"enabled"`
		IndexMaxLines         int    `toml:"index_max_lines"`
		IndexMaxBytes         int    `toml:"index_max_bytes"`
		MaxTopicFiles         int    `toml:"max_topic_files"`
		RelevanceSelectorTopK int    `toml:"relevance_selector_top_k"`
		MemoryDir             string `toml:"memory_dir"`
		SaveValidations       bool   `toml:"save_validations"`
		DriftProtection       bool   `toml:"drift_protection_enabled"`
	} `toml:"memory"`
}

func TestMemoryConfig_FromTOML(t *testing.T) {
	t.Parallel()
	raw := `
[memory]
enabled = true
index_max_lines = 150
index_max_bytes = 20000
max_topic_files = 30
relevance_selector_top_k = 3
memory_dir = ".harness/custom-memory"
save_validations = false
drift_protection_enabled = true
`
	var parsed tomlMemoryConfig
	if _, err := toml.Decode(raw, &parsed); err != nil {
		t.Fatalf("toml.Decode() error = %v", err)
	}
	if !parsed.Memory.Enabled {
		t.Error("parsed.Memory.Enabled = false, want true")
	}
	if parsed.Memory.IndexMaxLines != 150 {
		t.Errorf("IndexMaxLines = %d, want 150", parsed.Memory.IndexMaxLines)
	}
	if parsed.Memory.IndexMaxBytes != 20000 {
		t.Errorf("IndexMaxBytes = %d, want 20000", parsed.Memory.IndexMaxBytes)
	}
	if parsed.Memory.MaxTopicFiles != 30 {
		t.Errorf("MaxTopicFiles = %d, want 30", parsed.Memory.MaxTopicFiles)
	}
	if parsed.Memory.RelevanceSelectorTopK != 3 {
		t.Errorf("RelevanceSelectorTopK = %d, want 3", parsed.Memory.RelevanceSelectorTopK)
	}
	if parsed.Memory.MemoryDir != ".harness/custom-memory" {
		t.Errorf("MemoryDir = %q, want %q", parsed.Memory.MemoryDir, ".harness/custom-memory")
	}
	if parsed.Memory.SaveValidations {
		t.Error("SaveValidations = true, want false (overridden in TOML)")
	}
	if !parsed.Memory.DriftProtection {
		t.Error("DriftProtection = false, want true")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
