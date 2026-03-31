package memory

import (
	"testing"
)

func TestAllMemoryTypes_ReturnsFour(t *testing.T) {
	t.Parallel()
	types := AllMemoryTypes()
	if len(types) != 4 {
		t.Errorf("AllMemoryTypes() returned %d types, want 4", len(types))
	}
}

func TestIsValidMemoryType_Valid(t *testing.T) {
	t.Parallel()
	valid := []MemoryType{
		MemoryTypeUser,
		MemoryTypeFeedback,
		MemoryTypeProject,
		MemoryTypeReference,
	}
	for _, mt := range valid {
		mt := mt
		t.Run(string(mt), func(t *testing.T) {
			t.Parallel()
			if !IsValidMemoryType(mt) {
				t.Errorf("IsValidMemoryType(%q) = false, want true", mt)
			}
		})
	}
}

func TestIsValidMemoryType_Invalid(t *testing.T) {
	t.Parallel()
	if IsValidMemoryType("custom") {
		t.Error("IsValidMemoryType(\"custom\") = true, want false")
	}
}

func TestDefaultMemoryConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultMemoryConfig()

	if !cfg.Enabled {
		t.Error("DefaultMemoryConfig().Enabled = false, want true")
	}
	if cfg.IndexMaxLines != 200 {
		t.Errorf("DefaultMemoryConfig().IndexMaxLines = %d, want 200", cfg.IndexMaxLines)
	}
	if cfg.IndexMaxBytes != 25600 {
		t.Errorf("DefaultMemoryConfig().IndexMaxBytes = %d, want 25600", cfg.IndexMaxBytes)
	}
	if cfg.MaxTopicFiles != 50 {
		t.Errorf("DefaultMemoryConfig().MaxTopicFiles = %d, want 50", cfg.MaxTopicFiles)
	}
	if cfg.RelevanceSelectorTopK != 5 {
		t.Errorf("DefaultMemoryConfig().RelevanceSelectorTopK = %d, want 5", cfg.RelevanceSelectorTopK)
	}
	if cfg.MemoryDir != ".harness/memory" {
		t.Errorf("DefaultMemoryConfig().MemoryDir = %q, want %q", cfg.MemoryDir, ".harness/memory")
	}
	if !cfg.SaveValidations {
		t.Error("DefaultMemoryConfig().SaveValidations = false, want true")
	}
	if !cfg.DriftProtectionEnabled {
		t.Error("DefaultMemoryConfig().DriftProtectionEnabled = false, want true")
	}
}
