package profiles

import (
	"os"
	"path/filepath"
	"testing"
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
