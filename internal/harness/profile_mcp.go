package harness

import (
	"os"
	"path/filepath"
	"strings"

	"go-agent-harness/internal/config"
	"go-agent-harness/internal/profiles"
)

// loadProfileMCPServers loads a user-tier profile's MCP servers. It retains
// the legacy missing-profile-as-nonfatal contract for callers without a
// project profile directory.
func loadProfileMCPServers(profilesDir, profileName string) (map[string]config.MCPServerConfig, error) {
	if err := config.ValidateProfileName(profileName); err != nil {
		return nil, err
	}
	profilePath := filepath.Join(profilesDir, profileName+".toml")
	if _, statErr := os.Stat(profilePath); os.IsNotExist(statErr) {
		return nil, nil
	}
	cfg, err := config.Load(config.LoadOptions{ProfilesDir: profilesDir, ProfileName: profileName})
	if err != nil {
		return nil, err
	}
	return cfg.MCPServers, nil
}

// loadProfileMCPServersWithDirs uses the same project > user > built-in
// resolution order as the rest of named-profile execution. A missing profile
// remains non-fatal because MCP activation is optional for a run.
func loadProfileMCPServersWithDirs(projectDir, userDir, profileName string) (map[string]config.MCPServerConfig, error) {
	if projectDir == "" {
		return loadProfileMCPServers(userDir, profileName)
	}
	if err := config.ValidateProfileName(profileName); err != nil {
		return nil, err
	}
	p, err := profiles.LoadProfileWithDirs(profileName, projectDir, userDir)
	if err != nil {
		if os.IsNotExist(err) || strings.Contains(err.Error(), "not found") {
			return nil, nil
		}
		return nil, err
	}
	if p == nil {
		return nil, nil
	}
	return p.MCPServers, nil
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
