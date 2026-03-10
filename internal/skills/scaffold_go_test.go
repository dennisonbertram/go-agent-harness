package skills

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// skillsRepoRoot returns the repository root by walking up from this test file.
func skillsRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// file is .../internal/skills/scaffold_go_test.go
	// Walk up two directories to get the repo root.
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// TestScaffoldGoSkillExists verifies the scaffold-go SKILL.md file is present on disk.
func TestScaffoldGoSkillExists(t *testing.T) {
	root := skillsRepoRoot(t)
	skillPath := filepath.Join(root, "skills", "scaffold-go", "SKILL.md")

	if _, err := os.Stat(skillPath); os.IsNotExist(err) {
		t.Fatalf("skills/scaffold-go/SKILL.md does not exist at %s", skillPath)
	}
}

// TestScaffoldGoSkillParses verifies the scaffold-go SKILL.md parses without error
// and that all required fields are present and valid.
func TestScaffoldGoSkillParses(t *testing.T) {
	root := skillsRepoRoot(t)
	skillsDir := filepath.Join(root, "skills")

	loader := NewLoader(LoaderConfig{GlobalDir: skillsDir})
	skills, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	var found *Skill
	for i := range skills {
		if skills[i].Name == "scaffold-go" {
			found = &skills[i]
			break
		}
	}
	if found == nil {
		t.Fatal("scaffold-go skill not found after loading skills/")
	}

	t.Run("name", func(t *testing.T) {
		if found.Name != "scaffold-go" {
			t.Errorf("Name = %q, want %q", found.Name, "scaffold-go")
		}
	})

	t.Run("description_nonempty", func(t *testing.T) {
		if found.Description == "" {
			t.Error("Description must not be empty")
		}
	})

	t.Run("version_is_1", func(t *testing.T) {
		if found.Version != 1 {
			t.Errorf("Version = %d, want 1", found.Version)
		}
	})

	t.Run("body_nonempty", func(t *testing.T) {
		if strings.TrimSpace(found.Body) == "" {
			t.Error("Body must not be empty")
		}
	})

	t.Run("has_triggers", func(t *testing.T) {
		if len(found.Triggers) == 0 {
			t.Error("Triggers must not be empty — description must contain 'Trigger:' phrase(s)")
		}
	})

	t.Run("trigger_covers_go_project", func(t *testing.T) {
		// At least one trigger must contain "go" to be useful for Go scaffolding.
		hasTrigger := false
		for _, tr := range found.Triggers {
			if strings.Contains(strings.ToLower(tr), "go") {
				hasTrigger = true
				break
			}
		}
		if !hasTrigger {
			t.Error("at least one trigger must reference 'go' (e.g. 'create a new Go project')")
		}
	})

	t.Run("argument_hint_set", func(t *testing.T) {
		if found.ArgumentHint == "" {
			t.Error("argument-hint should be set to guide users on providing a module path")
		}
	})

	t.Run("body_contains_go_mod_init", func(t *testing.T) {
		if !strings.Contains(found.Body, "go mod init") {
			t.Error("Body must contain 'go mod init' instruction")
		}
	})

	t.Run("body_contains_makefile", func(t *testing.T) {
		if !strings.Contains(found.Body, "Makefile") {
			t.Error("Body must reference a Makefile")
		}
	})

	t.Run("body_contains_gitignore", func(t *testing.T) {
		if !strings.Contains(found.Body, ".gitignore") {
			t.Error("Body must reference .gitignore")
		}
	})

	t.Run("body_contains_dockerfile", func(t *testing.T) {
		if !strings.Contains(found.Body, "Dockerfile") {
			t.Error("Body must reference a Dockerfile")
		}
	})

	t.Run("body_contains_github_actions", func(t *testing.T) {
		if !strings.Contains(found.Body, ".github/workflows") {
			t.Error("Body must reference .github/workflows for CI")
		}
	})

	t.Run("body_contains_internal_layout", func(t *testing.T) {
		if !strings.Contains(found.Body, "internal/") {
			t.Error("Body must reference the internal/ directory layout")
		}
	})

	t.Run("body_contains_cmd_layout", func(t *testing.T) {
		if !strings.Contains(found.Body, "cmd/") {
			t.Error("Body must reference the cmd/ directory layout")
		}
	})

	t.Run("file_path_set", func(t *testing.T) {
		if found.FilePath == "" {
			t.Error("FilePath must be set by the loader")
		}
	})
}

// TestScaffoldGoSkillFrontmatter verifies the raw frontmatter parses correctly
// without going through the full loader validation.
func TestScaffoldGoSkillFrontmatter(t *testing.T) {
	root := skillsRepoRoot(t)
	skillPath := filepath.Join(root, "skills", "scaffold-go", "SKILL.md")

	data, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", skillPath, err)
	}

	content := string(data)

	t.Run("starts_with_frontmatter_delimiter", func(t *testing.T) {
		if !strings.HasPrefix(strings.TrimSpace(content), "---") {
			t.Error("SKILL.md must start with --- frontmatter delimiter")
		}
	})

	t.Run("has_closing_frontmatter_delimiter", func(t *testing.T) {
		// Must have at least two --- delimiters.
		count := strings.Count(content, "---")
		if count < 2 {
			t.Errorf("SKILL.md must have opening and closing --- delimiters, found %d occurrences", count)
		}
	})

	t.Run("name_field_present", func(t *testing.T) {
		if !strings.Contains(content, "name:") {
			t.Error("SKILL.md frontmatter must contain 'name:' field")
		}
	})

	t.Run("description_field_present", func(t *testing.T) {
		if !strings.Contains(content, "description:") {
			t.Error("SKILL.md frontmatter must contain 'description:' field")
		}
	})

	t.Run("version_field_present", func(t *testing.T) {
		if !strings.Contains(content, "version:") {
			t.Error("SKILL.md frontmatter must contain 'version:' field")
		}
	})
}
