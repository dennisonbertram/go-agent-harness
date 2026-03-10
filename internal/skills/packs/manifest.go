// Package packs provides infrastructure for loading and activating skill packs
// from a filesystem registry. A skill pack is a named directory containing a
// YAML manifest and a markdown instructions file.
package packs

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// SkillManifest describes a skill pack — a collection of prompt instructions
// combined with prerequisite declarations and tool constraints.
type SkillManifest struct {
	Name         string   `yaml:"name"`
	DisplayName  string   `yaml:"display_name"`
	Category     string   `yaml:"category"`
	Description  string   `yaml:"description"`
	Version      int      `yaml:"version"`
	Tools        []string `yaml:"tools"`
	RequiresCLI  []string `yaml:"requires_cli"`
	RequiresEnv  []string `yaml:"requires_env"`
	Instructions string   `yaml:"instructions"` // filename relative to pack directory
	Tags         []string `yaml:"tags"`
	AllowedTools []string `yaml:"allowed_tools"`
}

// ParseManifest parses a SkillManifest from raw YAML bytes and validates
// that required fields are present.
func ParseManifest(data []byte) (*SkillManifest, error) {
	var m SkillManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest YAML: %w", err)
	}
	if err := validateManifest(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

// LoadManifestFromFile reads and parses a manifest YAML file from disk.
func LoadManifestFromFile(path string) (*SkillManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading manifest file %s: %w", path, err)
	}
	return ParseManifest(data)
}

func validateManifest(m *SkillManifest) error {
	if m.Name == "" {
		return fmt.Errorf("manifest missing required field: name")
	}
	if m.Description == "" {
		return fmt.Errorf("manifest %q missing required field: description", m.Name)
	}
	if m.Instructions == "" {
		return fmt.Errorf("manifest %q missing required field: instructions", m.Name)
	}
	if m.Version == 0 {
		return fmt.Errorf("manifest %q missing required field: version (must be >= 1)", m.Name)
	}
	return nil
}
