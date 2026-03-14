package harness

import (
	"os"
	"path/filepath"

	"go-agent-harness/internal/config"
)

// loadProfileMCPServers loads the mcp_servers section from a named profile TOML
// file located in profilesDir. It returns the server map from the profile layer only.
//
// Returns nil, nil when the profile file does not exist (non-fatal: the profile
// may simply not define any MCP servers, or the profile itself may not exist).
//
// Returns a non-nil error only for:
//   - Invalid profile names (path traversal, empty name).
//   - TOML parse errors in the profile file.
func loadProfileMCPServers(profilesDir, profileName string) (map[string]config.MCPServerConfig, error) {
	if err := config.ValidateProfileName(profileName); err != nil {
		return nil, err
	}

	// Check if the profile file exists before attempting to load.
	// config.Load treats a missing profile as a hard error ("profile not found"),
	// but for MCP server activation the profile not existing is non-fatal
	// (the profile may not define any MCP servers).
	profilePath := filepath.Join(profilesDir, profileName+".toml")
	if _, statErr := os.Stat(profilePath); os.IsNotExist(statErr) {
		return nil, nil
	}

	opts := config.LoadOptions{
		ProfilesDir: profilesDir,
		ProfileName: profileName,
		// No UserConfigPath, no ProjectConfigPath — we only want the profile layer.
	}
	cfg, err := config.Load(opts)
	if err != nil {
		return nil, err
	}
	return cfg.MCPServers, nil
}

// defaultProfilesDir returns the default profiles directory (~/.harness/profiles/).
// Returns an empty string if the user home directory cannot be determined.
func defaultProfilesDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".harness", "profiles")
}
