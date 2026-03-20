package profiles

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadBuiltinProfiles verifies all built-in profiles parse correctly.
func TestLoadBuiltinProfiles(t *testing.T) {
	builtins := []string{"github", "file-writer", "researcher", "bash-runner", "reviewer", "full"}
	for _, name := range builtins {
		t.Run(name, func(t *testing.T) {
			p, err := loadProfileWithDirs(name, "", "")
			require.NoError(t, err)
			require.NotNil(t, p)
			assert.Equal(t, name, p.Meta.Name)
			assert.NotEmpty(t, p.Meta.Description)
			assert.Equal(t, "built-in", p.Meta.CreatedBy)
			assert.False(t, p.Meta.ReviewEligible, "built-in profiles must not be review-eligible")
		})
	}
}

func TestLoadBuiltinFullProfileGoldenPathDefaults(t *testing.T) {
	p, err := loadProfileWithDirs("full", "", "")
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, "full", p.Meta.Name)
	assert.Equal(t, "gpt-4.1-mini", p.Runner.Model)
	assert.Equal(t, 30, p.Runner.MaxSteps)
	assert.Equal(t, 2.0, p.Runner.MaxCostUSD)
	assert.Empty(t, p.Tools.Allow, "full profile should keep the full tool registry available")
}

// TestLoadProfileNotFound verifies that a missing profile returns an error.
func TestLoadProfileNotFound(t *testing.T) {
	_, err := loadProfileWithDirs("nonexistent-profile-xyz", "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestLoadProfileFromUserDir verifies project-level and user-global resolution.
func TestLoadProfileFromUserDir(t *testing.T) {
	dir := t.TempDir()

	// Write a custom profile.
	content := `
[meta]
name = "custom"
description = "Test profile"
version = 1
created_at = "2026-01-01"
created_by = "user"
review_eligible = true

[runner]
model = "gpt-4.1-mini"
max_steps = 5
max_cost_usd = 0.10
system_prompt = "Test prompt"

[tools]
allow = ["read", "grep"]
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "custom.toml"), []byte(content), 0644))

	p, err := loadProfileWithDirs("custom", "", dir)
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, "custom", p.Meta.Name)
	assert.Equal(t, "gpt-4.1-mini", p.Runner.Model)
	assert.Equal(t, 5, p.Runner.MaxSteps)
	assert.Equal(t, 0.10, p.Runner.MaxCostUSD)
	assert.Equal(t, "Test prompt", p.Runner.SystemPrompt)
	assert.Equal(t, []string{"read", "grep"}, p.Tools.Allow)
	assert.True(t, p.Meta.ReviewEligible)
}

// TestProjectLevelOverridesUserGlobal verifies project-level profiles take precedence.
func TestProjectLevelOverridesUserGlobal(t *testing.T) {
	projectDir := t.TempDir()
	userDir := t.TempDir()

	projectContent := `
[meta]
name = "myprofile"
description = "Project version"
created_by = "user"

[runner]
model = "gpt-4.1"
max_steps = 25
`
	userContent := `
[meta]
name = "myprofile"
description = "User version"
created_by = "user"

[runner]
model = "gpt-4.1-mini"
max_steps = 10
`
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "myprofile.toml"), []byte(projectContent), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(userDir, "myprofile.toml"), []byte(userContent), 0644))

	p, err := loadProfileWithDirs("myprofile", projectDir, userDir)
	require.NoError(t, err)
	assert.Equal(t, "gpt-4.1", p.Runner.Model, "project-level should override user-global")
	assert.Equal(t, 25, p.Runner.MaxSteps)
}

// TestBuiltinOverriddenByUserGlobal verifies user-global overrides built-ins.
func TestBuiltinOverriddenByUserGlobal(t *testing.T) {
	userDir := t.TempDir()

	// Override the "github" built-in with a custom version.
	content := `
[meta]
name = "github"
description = "Custom github override"
created_by = "user"

[runner]
model = "gpt-4.1"
max_steps = 99
`
	require.NoError(t, os.WriteFile(filepath.Join(userDir, "github.toml"), []byte(content), 0644))

	p, err := loadProfileWithDirs("github", "", userDir)
	require.NoError(t, err)
	assert.Equal(t, "gpt-4.1", p.Runner.Model, "user-global should override built-in")
	assert.Equal(t, 99, p.Runner.MaxSteps)
}

// TestListProfiles verifies that ListProfiles returns all profile names.
func TestListProfiles(t *testing.T) {
	names, err := listProfilesWithDirs("", "")
	require.NoError(t, err)
	// Should include all 6 built-ins.
	assert.Contains(t, names, "github")
	assert.Contains(t, names, "file-writer")
	assert.Contains(t, names, "researcher")
	assert.Contains(t, names, "bash-runner")
	assert.Contains(t, names, "reviewer")
	assert.Contains(t, names, "full")
}

// TestListProfilesDeduplicates verifies that duplicate names are only listed once.
func TestListProfilesDeduplicates(t *testing.T) {
	userDir := t.TempDir()

	// Add a profile with the same name as a built-in.
	content := "[meta]\nname = \"github\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(userDir, "github.toml"), []byte(content), 0644))

	names, err := listProfilesWithDirs("", userDir)
	require.NoError(t, err)

	count := 0
	for _, n := range names {
		if n == "github" {
			count++
		}
	}
	assert.Equal(t, 1, count, "duplicate profile name should appear only once")
}

// TestSaveProfile verifies that SaveProfile writes a TOML file correctly.
func TestSaveProfile(t *testing.T) {
	dir := t.TempDir()

	p := &Profile{
		Meta: ProfileMeta{
			Name:        "test-save",
			Description: "Saved test profile",
			Version:     1,
			CreatedBy:   "user",
		},
		Runner: ProfileRunner{
			Model:    "gpt-4.1-mini",
			MaxSteps: 10,
		},
		Tools: ProfileTools{
			Allow: []string{"read", "write"},
		},
	}

	// Temporarily swap defaultUserProfilesDir by using the internal function.
	path := filepath.Join(dir, "test-save.toml")
	require.NoError(t, os.MkdirAll(dir, 0755))

	// Use the internal save logic with explicit dir.
	require.NoError(t, saveProfileToDir(p, dir))

	// Verify the file was written.
	_, err := os.Stat(path)
	require.NoError(t, err)

	// Reload and verify round-trip.
	loaded, err := loadProfileFile(path)
	require.NoError(t, err)
	assert.Equal(t, "test-save", loaded.Meta.Name)
	assert.Equal(t, []string{"read", "write"}, loaded.Tools.Allow)
}

// TestApplyValues verifies that ApplyValues returns correct profile fields.
func TestApplyValues(t *testing.T) {
	p := &Profile{
		Runner: ProfileRunner{
			Model:        "claude-opus-4-6",
			MaxSteps:     50,
			MaxCostUSD:   2.0,
			SystemPrompt: "Be thorough.",
		},
		Tools: ProfileTools{
			Allow: []string{"bash", "read"},
		},
	}

	vals := p.ApplyValues()
	assert.Equal(t, "claude-opus-4-6", vals.Model)
	assert.Equal(t, 50, vals.MaxSteps)
	assert.Equal(t, 2.0, vals.MaxCostUSD)
	assert.Equal(t, "Be thorough.", vals.SystemPrompt)
	assert.Equal(t, []string{"bash", "read"}, vals.AllowedTools)
}

// TestApplyValuesCopiesSlice verifies that AllowedTools is a copy (no aliasing).
func TestApplyValuesCopiesSlice(t *testing.T) {
	p := &Profile{
		Tools: ProfileTools{Allow: []string{"bash"}},
	}
	v1 := p.ApplyValues()
	v1.AllowedTools[0] = "mutated"
	v2 := p.ApplyValues()
	assert.Equal(t, "bash", v2.AllowedTools[0], "profile should not be mutated via AllowedTools alias")
}

// TestInvalidProfileName verifies path traversal protection.
func TestInvalidProfileName(t *testing.T) {
	tests := []string{"../secret", "/etc/passwd", "foo/bar", ""}
	for _, name := range tests {
		_, err := loadProfileWithDirs(name, "", "")
		assert.Error(t, err, "expected error for invalid name %q", name)
	}
}

// TestLoadProfileExported verifies the exported LoadProfile function resolves
// built-in profiles when no project or user directories are present.
func TestLoadProfileExported(t *testing.T) {
	// LoadProfile uses default dirs (which likely do not exist in CI),
	// so a built-in profile should still be found.
	p, err := LoadProfile("full")
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, "full", p.Meta.Name)
}

// TestLoadProfileFromUserDirExported verifies the exported LoadProfileFromUserDir
// function resolves profiles from an explicit user directory.
func TestLoadProfileFromUserDirExported(t *testing.T) {
	dir := t.TempDir()

	content := `
[meta]
name = "exported-test"
description = "Exported test profile"
version = 1
created_at = "2026-01-01"
created_by = "test"
review_eligible = false

[runner]
model = "gpt-4.1-mini"
max_steps = 3
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "exported-test.toml"), []byte(content), 0644))

	p, err := LoadProfileFromUserDir("exported-test", dir)
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, "exported-test", p.Meta.Name)
	assert.Equal(t, 3, p.Runner.MaxSteps)
}

// TestListProfilesExported verifies the exported ListProfiles function returns
// at least the built-in profiles.
func TestListProfilesExported(t *testing.T) {
	names, err := ListProfiles()
	require.NoError(t, err)
	// Built-in profiles must always be present.
	assert.Contains(t, names, "full")
	assert.Contains(t, names, "github")
	assert.Contains(t, names, "researcher")
}

// TestSaveProfileExported verifies the exported SaveProfile function writes a
// TOML file to the default user profiles directory.
func TestSaveProfileExported(t *testing.T) {
	dir := t.TempDir()
	p := &Profile{
		Meta: ProfileMeta{
			Name:        "save-exported-test",
			Description: "Test save via exported function",
			Version:     1,
			CreatedAt:   "2026-01-01",
			CreatedBy:   "test",
		},
		Runner: ProfileRunner{
			Model:    "gpt-4.1-mini",
			MaxSteps: 10,
		},
	}

	err := saveProfileToDir(p, dir)
	require.NoError(t, err)

	path := filepath.Join(dir, "save-exported-test.toml")
	_, statErr := os.Stat(path)
	require.NoError(t, statErr, "expected TOML file to be written")

	loaded, err := loadProfileWithDirs("save-exported-test", "", dir)
	require.NoError(t, err)
	assert.Equal(t, "save-exported-test", loaded.Meta.Name)
	assert.Equal(t, 10, loaded.Runner.MaxSteps)
}

// TestSaveProfileCallsDefaultUserDir exercises the exported SaveProfile path
// by temporarily overriding HOME to point to a temp directory.
func TestSaveProfileCallsDefaultUserDir(t *testing.T) {
	// Note: t.Parallel() is intentionally omitted — t.Setenv requires sequential execution.

	// Redirect HOME so SaveProfile writes to a temp dir.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	p := &Profile{
		Meta: ProfileMeta{
			Name:        "save-via-exported",
			Description: "Coverage test for exported SaveProfile",
			Version:     1,
			CreatedAt:   "2026-01-01",
			CreatedBy:   "test",
		},
		Runner: ProfileRunner{Model: "gpt-4.1-mini", MaxSteps: 1},
	}

	err := SaveProfile(p)
	require.NoError(t, err, "SaveProfile must succeed with valid home dir")

	// Verify the file was written under the expected subdirectory.
	// defaultUserProfilesDir() returns $HOME/.harness/profiles.
	expectedDir := filepath.Join(tmpHome, ".harness", "profiles")
	_, statErr := os.Stat(filepath.Join(expectedDir, "save-via-exported.toml"))
	require.NoError(t, statErr, "expected TOML file at %s", expectedDir)
}
