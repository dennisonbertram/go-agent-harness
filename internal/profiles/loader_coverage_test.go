package profiles

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTestProfile(t *testing.T, dir, name, description, model string, tools []string) {
	t.Helper()
	content := "[meta]\n" +
		"name = \"" + name + "\"\n" +
		"description = \"" + description + "\"\n" +
		"version = 1\n" +
		"created_at = \"2026-03-25\"\n" +
		"created_by = \"test\"\n" +
		"review_eligible = false\n\n" +
		"[runner]\n" +
		"model = \"" + model + "\"\n\n" +
		"[tools]\n" +
		"allow = [\"" + tools[0] + "\"]\n"
	if err := os.WriteFile(filepath.Join(dir, name+".toml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write profile %s: %v", name, err)
	}
}

func TestListProfileSummariesPrefersHigherPriorityDirs(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	userDir := t.TempDir()

	writeTestProfile(t, projectDir, "shared", "project profile", "gpt-4.1", []string{"bash"})
	writeTestProfile(t, userDir, "shared", "user profile", "gpt-4.1-mini", []string{"read"})
	writeTestProfile(t, userDir, "user-only", "user only profile", "gpt-4.1-nano", []string{"edit"})

	summaries, err := ListProfileSummariesFromDirs(projectDir, userDir)
	if err != nil {
		t.Fatalf("ListProfileSummariesFromDirs: %v", err)
	}

	byName := make(map[string]ProfileSummary, len(summaries))
	hasBuiltin := false
	for _, summary := range summaries {
		byName[summary.Name] = summary
		if summary.SourceTier == "built-in" {
			hasBuiltin = true
		}
	}

	shared, ok := byName["shared"]
	if !ok {
		t.Fatal("shared profile missing from summaries")
	}
	if shared.SourceTier != "project" {
		t.Fatalf("shared.SourceTier = %q, want project", shared.SourceTier)
	}
	if shared.Description != "project profile" {
		t.Fatalf("shared.Description = %q, want project profile", shared.Description)
	}
	if shared.AllowedToolCount != 1 || len(shared.AllowedTools) != 1 || shared.AllowedTools[0] != "bash" {
		t.Fatalf("shared allowed tools = %#v (count=%d), want [bash]", shared.AllowedTools, shared.AllowedToolCount)
	}

	userOnly, ok := byName["user-only"]
	if !ok {
		t.Fatal("user-only profile missing from summaries")
	}
	if userOnly.SourceTier != "user" {
		t.Fatalf("userOnly.SourceTier = %q, want user", userOnly.SourceTier)
	}

	if !hasBuiltin {
		t.Fatal("expected at least one built-in profile summary")
	}
}

func TestListProfileSummariesUsesDefaultDirs(t *testing.T) {
	projectRoot := t.TempDir()
	projectProfilesDir := filepath.Join(projectRoot, ".harness", "profiles")
	if err := os.MkdirAll(projectProfilesDir, 0o755); err != nil {
		t.Fatalf("mkdir project profiles: %v", err)
	}

	homeDir := t.TempDir()
	userProfilesDir := filepath.Join(homeDir, ".harness", "profiles")
	if err := os.MkdirAll(userProfilesDir, 0o755); err != nil {
		t.Fatalf("mkdir user profiles: %v", err)
	}

	writeTestProfile(t, projectProfilesDir, "wrapper-project", "wrapper project profile", "gpt-4.1", []string{"bash"})
	writeTestProfile(t, userProfilesDir, "wrapper-user", "wrapper user profile", "gpt-4.1-mini", []string{"read"})

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(projectRoot); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() {
		_ = os.Chdir(cwd)
	}()
	t.Setenv("HOME", homeDir)

	summaries, err := ListProfileSummaries()
	if err != nil {
		t.Fatalf("ListProfileSummaries: %v", err)
	}

	byName := make(map[string]ProfileSummary, len(summaries))
	for _, summary := range summaries {
		byName[summary.Name] = summary
	}

	projectSummary, ok := byName["wrapper-project"]
	if !ok {
		t.Fatal("wrapper-project summary missing")
	}
	if projectSummary.SourceTier != "project" {
		t.Fatalf("wrapper-project SourceTier = %q, want project", projectSummary.SourceTier)
	}

	userSummary, ok := byName["wrapper-user"]
	if !ok {
		t.Fatal("wrapper-user summary missing")
	}
	if userSummary.SourceTier != "user" {
		t.Fatalf("wrapper-user SourceTier = %q, want user", userSummary.SourceTier)
	}
}

func TestLoadProfile_SingleInheritance(t *testing.T) {
	projectDir := t.TempDir()

	baseProfile := `
[meta]
name = "base"
description = "Base profile"
version = 1
created_at = "2026-03-25"
created_by = "test"
review_eligible = false

[runner]
model = "gpt-4.1-mini"
max_steps = 20
max_cost_usd = 0.5
system_prompt = "Base system prompt"
reasoning_effort = "low"

[tools]
allow = ["read", "grep", "bash"]
`
	childProfile := `
extends = "base"

[meta]
name = "child"
description = "Child profile"
created_at = "2026-03-25"
created_by = "test"
review_eligible = false
`

	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "base.toml"), []byte(baseProfile), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "child.toml"), []byte(childProfile), 0o644))

	p, err := loadProfileWithDirs("child", projectDir, "")
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, "child", p.Meta.Name, "child name should remain child")
	assert.Equal(t, "Base system prompt", p.Runner.SystemPrompt)
	assert.Equal(t, "gpt-4.1-mini", p.Runner.Model)
	assert.Equal(t, 20, p.Runner.MaxSteps)
	assert.Equal(t, 0.5, p.Runner.MaxCostUSD)
	assert.Equal(t, []string{"read", "grep", "bash"}, p.Tools.Allow)
}

func TestLoadProfile_MultiFieldOverridePrecedenceAndTierPriority(t *testing.T) {
	projectDir := t.TempDir()
	userDir := t.TempDir()

	projectBaseProfile := `
[meta]
name = "base"
description = "Project base profile"
version = 1
created_at = "2026-03-25"
created_by = "test"
review_eligible = false

[runner]
model = "project-model"
max_steps = 10
max_cost_usd = 0.75
system_prompt = "Project base prompt"
reasoning_effort = "low"

[tools]
allow = ["read"]
`
	userBaseProfile := `
[meta]
name = "base"
description = "User base profile"
version = 1
created_at = "2026-03-25"
created_by = "user"
review_eligible = false

[runner]
model = "user-model"
max_steps = 5
max_cost_usd = 0.25
system_prompt = "User base prompt"
reasoning_effort = "high"

[tools]
allow = ["write"]
`
	childProfile := `
extends = "base"

[meta]
name = "child"
description = "Child profile"
created_at = "2026-03-25"
created_by = "test"
review_eligible = false

[runner]
system_prompt = "Child prompt"
max_steps = 99
reasoning_effort = "high"

[tools]
allow = ["child-only"]
`
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "base.toml"), []byte(projectBaseProfile), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(userDir, "base.toml"), []byte(userBaseProfile), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(userDir, "child.toml"), []byte(childProfile), 0o644))

	p, err := loadProfileWithDirs("child", projectDir, userDir)
	require.NoError(t, err)
	require.NotNil(t, p)

	// Tool list is explicit child override (replace semantics), not merged union.
	assert.Equal(t, []string{"child-only"}, p.Tools.Allow)

	// Child overrides prompt/runtime fields where explicitly set.
	assert.Equal(t, "Child prompt", p.Runner.SystemPrompt)
	assert.Equal(t, "high", p.Runner.ReasoningEffort)
	assert.Equal(t, 99, p.Runner.MaxSteps)

	// Unset child runtime fields inherit from the resolved base.
	assert.Equal(t, "project-model", p.Runner.Model, "project-level base should win over user-level base")
	assert.Equal(t, 0.75, p.Runner.MaxCostUSD)
}

func TestLoadProfile_ExtendsDetectsCycle(t *testing.T) {
	projectDir := t.TempDir()

	first := `
extends = "b"
[meta]
name = "a"
description = "first"
created_at = "2026-03-25"
created_by = "test"
review_eligible = false
`
	second := `
extends = "a"
[meta]
name = "b"
description = "second"
created_at = "2026-03-25"
created_by = "test"
review_eligible = false
`
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "a.toml"), []byte(first), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "b.toml"), []byte(second), 0o644))

	_, err := loadProfileWithDirs("a", projectDir, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cycle detected in profile inheritance")
	assert.Contains(t, err.Error(), "a -> b -> a")
}

func TestLoadProfile_ExtendsMissingBase(t *testing.T) {
	projectDir := t.TempDir()

	child := `
extends = "missing-base"
[meta]
name = "child"
description = "child with bad extends"
created_at = "2026-03-25"
created_by = "test"
review_eligible = false
`
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "child.toml"), []byte(child), 0o644))

	_, err := loadProfileWithDirs("child", projectDir, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "extends missing base profile")
	assert.Contains(t, err.Error(), "missing-base")
	assert.Contains(t, err.Error(), "not found")
}
