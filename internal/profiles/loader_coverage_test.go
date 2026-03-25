package profiles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListProfileSummariesPrefersHigherPriorityDirs(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	userDir := t.TempDir()

	writeProfile := func(dir, name, description, model string, tools []string) {
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

	writeProfile(projectDir, "shared", "project profile", "gpt-4.1", []string{"bash"})
	writeProfile(userDir, "shared", "user profile", "gpt-4.1-mini", []string{"read"})
	writeProfile(userDir, "user-only", "user only profile", "gpt-4.1-nano", []string{"edit"})

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
