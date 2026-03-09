package descriptions

import (
	"embed"
	"strings"
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
