package descriptions

import (
	"embed"
	"strings"

	"go-agent-harness/internal/harness/tools/behavioral_specs"
)

//go:embed *.md
var FS embed.FS

// Load reads a tool description from the embedded filesystem.
// The filename should match the tool name (e.g., "cron_create.md").
func Load(name string) string {
	data, err := FS.ReadFile(name + ".md")
	if err != nil {
		panic("missing tool description: " + name + ".md")
	}
	return strings.TrimSpace(string(data))
}

// LoadWithSpec reads a tool description and, if enabled, appends its behavioral spec.
// When cfg.Enabled is false or no spec exists for the tool, returns the plain description.
func LoadWithSpec(name string, cfg behavioral_specs.BehavioralSpecConfig) string {
	base := Load(name)
	if !cfg.Enabled {
		return base
	}
	spec, err := behavioral_specs.LoadSpec(name)
	if err != nil || spec == nil {
		return base
	}
	formatted := behavioral_specs.FormatSpec(spec, cfg)
	if formatted == "" {
		return base
	}
	return base + "\n\n" + formatted
}
