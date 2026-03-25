package harness

import (
	"fmt"
	"strings"

	htools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/profiles"
)

type ProfileToolManifest struct {
	ProfileName            string              `json:"profile_name"`
	ProfileSourceTier      string              `json:"profile_source_tier"`
	DeclaredAllowedTools   []string            `json:"declared_allowed_tools,omitempty"`
	AllowedToolsRestricted bool                `json:"allowed_tools_restricted"`
	VisibleTools           []ToolManifestEntry `json:"visible_tools"`
	DeferredTools          []ToolManifestEntry `json:"deferred_tools"`
	ResolvedTools          []ToolManifestEntry `json:"resolved_tools"`
}

type ToolManifestEntry struct {
	Name             string          `json:"name"`
	Description      string          `json:"description"`
	Tier             htools.ToolTier `json:"tier"`
	Source           string          `json:"source"`
	Tags             []string        `json:"tags,omitempty"`
	VisibleByDefault bool            `json:"visible_by_default"`
}

// BuildProfileToolManifest resolves a profile and returns the declared
// allowlist together with the effective visible and deferred tools that a
// profile-backed run can access from the default registry.
func BuildProfileToolManifest(workspaceRoot, projectDir, userDir, profileName string, opts DefaultRegistryOptions) (*ProfileToolManifest, error) {
	profileName = strings.TrimSpace(profileName)
	if profileName == "" {
		return nil, fmt.Errorf("profile name is required")
	}

	var (
		profile *profiles.Profile
		err     error
	)
	switch {
	case projectDir != "" || userDir != "":
		profile, err = profiles.LoadProfileWithDirs(profileName, projectDir, userDir)
	case userDir != "":
		profile, err = profiles.LoadProfileFromUserDir(profileName, userDir)
	default:
		profile, err = profiles.LoadProfile(profileName)
	}
	if err != nil {
		return nil, err
	}

	registry := NewDefaultRegistryWithOptions(workspaceRoot, opts)
	entries := filterManifestEntries(registry.DefinitionsWithMetadata(), profile.Tools.Allow)

	manifest := &ProfileToolManifest{
		ProfileName:            profile.Meta.Name,
		ProfileSourceTier:      resolveManifestProfileSourceTier(profileName, projectDir, userDir),
		DeclaredAllowedTools:   append([]string(nil), profile.Tools.Allow...),
		AllowedToolsRestricted: len(profile.Tools.Allow) > 0,
		VisibleTools:           make([]ToolManifestEntry, 0),
		DeferredTools:          make([]ToolManifestEntry, 0),
		ResolvedTools:          make([]ToolManifestEntry, 0, len(entries)),
	}

	for _, entry := range entries {
		manifest.ResolvedTools = append(manifest.ResolvedTools, entry)
		if entry.VisibleByDefault {
			manifest.VisibleTools = append(manifest.VisibleTools, entry)
			continue
		}
		manifest.DeferredTools = append(manifest.DeferredTools, entry)
	}

	return manifest, nil
}

func filterManifestEntries(entries []ToolMetadata, allowedTools []string) []ToolManifestEntry {
	allowed := make(map[string]bool, len(allowedTools)+len(AlwaysAvailableTools))
	restricted := len(allowedTools) > 0
	if restricted {
		for _, name := range allowedTools {
			allowed[name] = true
		}
		for name := range AlwaysAvailableTools {
			allowed[name] = true
		}
	}

	result := make([]ToolManifestEntry, 0, len(entries))
	for _, entry := range entries {
		if restricted && !allowed[entry.Definition.Name] {
			continue
		}
		result = append(result, ToolManifestEntry{
			Name:             entry.Definition.Name,
			Description:      entry.Definition.Description,
			Tier:             entry.Tier,
			Source:           inferToolManifestSource(entry),
			Tags:             copyStrings(entry.Tags),
			VisibleByDefault: entry.Tier == htools.TierCore,
		})
	}
	return result
}

func inferToolManifestSource(entry ToolMetadata) string {
	if strings.HasPrefix(entry.Definition.Name, "mcp_") || hasTag(entry.Tags, "mcp") {
		return "mcp"
	}
	if hasTag(entry.Tags, "script") {
		return "script"
	}
	if entry.Definition.Name == "run_recipe" {
		return "recipe"
	}
	return "built_in"
}

func hasTag(tags []string, want string) bool {
	for _, tag := range tags {
		if tag == want {
			return true
		}
	}
	return false
}

func resolveManifestProfileSourceTier(name, projectDir, userDir string) string {
	if projectDir != "" {
		if _, err := profiles.LoadProfileWithDirs(name, projectDir, ""); err == nil {
			return "project"
		}
	}
	if userDir != "" {
		if _, err := profiles.LoadProfileWithDirs(name, "", userDir); err == nil {
			return "user"
		}
	}
	return "built-in"
}
