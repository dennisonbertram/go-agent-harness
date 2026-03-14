package symphd

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"

	"go-agent-harness/internal/config"
)

// mergeProfileMCPIntoTOML merges profile MCP servers into an existing TOML
// config string. Profile entries override existing mcp_servers entries with
// the same name; unique names are added. Returns the merged TOML string.
//
// An empty existingTOML is valid (starts from scratch). A nil or empty
// profileMCPServers map returns existingTOML unchanged.
func mergeProfileMCPIntoTOML(existingTOML string, profileMCPServers map[string]config.MCPServerConfig) (string, error) {
	if len(profileMCPServers) == 0 {
		return existingTOML, nil
	}

	// Parse existingTOML into a generic map to preserve unknown keys.
	var raw map[string]any
	if strings.TrimSpace(existingTOML) != "" {
		if _, err := toml.Decode(existingTOML, &raw); err != nil {
			return "", fmt.Errorf("mergeProfileMCPIntoTOML: parse existing TOML: %w", err)
		}
	}
	if raw == nil {
		raw = make(map[string]any)
	}

	// Get or create the mcp_servers table.
	mcpRaw, _ := raw["mcp_servers"].(map[string]any)
	if mcpRaw == nil {
		mcpRaw = make(map[string]any)
	}

	// Merge profile MCP entries (profile wins on name collision).
	for name, srv := range profileMCPServers {
		entry := make(map[string]any)
		if srv.Transport != "" {
			entry["transport"] = srv.Transport
		}
		if srv.Command != "" {
			entry["command"] = srv.Command
		}
		if len(srv.Args) > 0 {
			entry["args"] = srv.Args
		}
		if srv.URL != "" {
			entry["url"] = srv.URL
		}
		mcpRaw[name] = entry
	}

	raw["mcp_servers"] = mcpRaw

	// Re-encode to TOML.
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(raw); err != nil {
		return "", fmt.Errorf("mergeProfileMCPIntoTOML: encode TOML: %w", err)
	}
	return buf.String(), nil
}
