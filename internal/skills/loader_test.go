package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSkillFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	skillDir := filepath.Join(dir, name)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const validSkillMD = `---
name: my-skill
description: "A test skill. Trigger: do my thing"
version: 1
---
# My Skill Body

Use $ARGUMENTS here.
`

func TestLoaderLoad_ValidSkill(t *testing.T) {
	dir := t.TempDir()
	writeSkillFile(t, dir, "my-skill", validSkillMD)

	loader := NewLoader(LoaderConfig{GlobalDir: dir})
	skills, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("Load() returned %d skills, want 1", len(skills))
	}

	s := skills[0]
	if s.Name != "my-skill" {
		t.Errorf("Name = %q, want %q", s.Name, "my-skill")
	}
	if s.Description != "A test skill. Trigger: do my thing" {
		t.Errorf("Description = %q", s.Description)
	}
	if s.Version != 1 {
		t.Errorf("Version = %d, want 1", s.Version)
	}
	if !s.AutoInvoke {
		t.Error("AutoInvoke should default to true")
	}
	if s.Source != SourceGlobal {
		t.Errorf("Source = %q, want %q", s.Source, SourceGlobal)
	}
	if len(s.Triggers) != 1 || s.Triggers[0] != "do my thing" {
		t.Errorf("Triggers = %v, want [do my thing]", s.Triggers)
	}
	if s.Body == "" {
		t.Error("Body should not be empty")
	}
}

func TestLoaderLoad_AutoInvokeFalse(t *testing.T) {
	dir := t.TempDir()
	content := `---
name: no-auto
description: "No auto invoke skill"
version: 1
auto-invoke: false
---
Body here.
`
	writeSkillFile(t, dir, "no-auto", content)

	loader := NewLoader(LoaderConfig{GlobalDir: dir})
	skills, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("got %d skills", len(skills))
	}
	if skills[0].AutoInvoke != false {
		t.Error("AutoInvoke should be false")
	}
}

func TestLoaderLoad_AllowedTools(t *testing.T) {
	dir := t.TempDir()
	content := `---
name: limited
description: "Limited tools skill"
version: 1
allowed-tools:
  - bash
  - read
---
Body.
`
	writeSkillFile(t, dir, "limited", content)

	loader := NewLoader(LoaderConfig{GlobalDir: dir})
	skills, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(skills[0].AllowedTools) != 2 {
		t.Errorf("AllowedTools = %v, want [bash read]", skills[0].AllowedTools)
	}
}

func TestLoaderLoad_MissingName(t *testing.T) {
	dir := t.TempDir()
	content := `---
description: "No name"
version: 1
---
Body.
`
	writeSkillFile(t, dir, "no-name", content)

	loader := NewLoader(LoaderConfig{GlobalDir: dir})
	_, err := loader.Load()
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestLoaderLoad_MissingDescription(t *testing.T) {
	dir := t.TempDir()
	content := `---
name: no-desc
version: 1
---
Body.
`
	writeSkillFile(t, dir, "no-desc", content)

	loader := NewLoader(LoaderConfig{GlobalDir: dir})
	_, err := loader.Load()
	if err == nil {
		t.Fatal("expected error for missing description")
	}
}

func TestLoaderLoad_InvalidVersion(t *testing.T) {
	dir := t.TempDir()
	content := `---
name: bad-ver
description: "Bad version"
version: 2
---
Body.
`
	writeSkillFile(t, dir, "bad-ver", content)

	loader := NewLoader(LoaderConfig{GlobalDir: dir})
	_, err := loader.Load()
	if err == nil {
		t.Fatal("expected error for invalid version")
	}
}

func TestLoaderLoad_NotKebabCase(t *testing.T) {
	dir := t.TempDir()
	content := `---
name: NotKebab
description: "Bad name"
version: 1
---
Body.
`
	writeSkillFile(t, dir, "NotKebab", content)

	loader := NewLoader(LoaderConfig{GlobalDir: dir})
	_, err := loader.Load()
	if err == nil {
		t.Fatal("expected error for non-kebab-case name")
	}
}

func TestLoaderLoad_NameDirMismatch(t *testing.T) {
	dir := t.TempDir()
	content := `---
name: wrong-name
description: "Name doesn't match dir"
version: 1
---
Body.
`
	writeSkillFile(t, dir, "actual-dir", content)

	loader := NewLoader(LoaderConfig{GlobalDir: dir})
	_, err := loader.Load()
	if err == nil {
		t.Fatal("expected error for name/dir mismatch")
	}
}

func TestLoaderLoad_MissingDirectory(t *testing.T) {
	loader := NewLoader(LoaderConfig{GlobalDir: "/nonexistent/path/skills"})
	skills, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v (should skip missing dirs)", err)
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}
}

func TestLoaderLoad_EmptyDirs(t *testing.T) {
	loader := NewLoader(LoaderConfig{})
	skills, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}
}

func TestLoaderLoad_DirWithoutSkillMD(t *testing.T) {
	dir := t.TempDir()
	// Create a directory but no SKILL.md inside
	if err := os.MkdirAll(filepath.Join(dir, "some-dir"), 0o755); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(LoaderConfig{GlobalDir: dir})
	skills, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}
}

func TestLoaderLoad_BothDirs(t *testing.T) {
	globalDir := t.TempDir()
	localDir := t.TempDir()

	writeSkillFile(t, globalDir, "my-skill", validSkillMD)

	localContent := `---
name: local-skill
description: "A local skill"
version: 1
---
Local body.
`
	writeSkillFile(t, localDir, "local-skill", localContent)

	loader := NewLoader(LoaderConfig{GlobalDir: globalDir, WorkspaceDir: localDir})
	skills, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}

	// Check sources
	sourceMap := map[string]SkillSource{}
	for _, s := range skills {
		sourceMap[s.Name] = s.Source
	}
	if sourceMap["my-skill"] != SourceGlobal {
		t.Errorf("my-skill source = %q, want global", sourceMap["my-skill"])
	}
	if sourceMap["local-skill"] != SourceLocal {
		t.Errorf("local-skill source = %q, want local", sourceMap["local-skill"])
	}
}

func TestLoaderLoad_MissingFrontmatter(t *testing.T) {
	dir := t.TempDir()
	content := `# No frontmatter here
Just markdown.
`
	writeSkillFile(t, dir, "bad-skill", content)

	loader := NewLoader(LoaderConfig{GlobalDir: dir})
	_, err := loader.Load()
	if err == nil {
		t.Fatal("expected error for missing frontmatter")
	}
}

func TestLoaderLoad_ArgumentHint(t *testing.T) {
	dir := t.TempDir()
	content := `---
name: with-hint
description: "Has argument hint"
version: 1
argument-hint: "<filename> [options]"
---
Body.
`
	writeSkillFile(t, dir, "with-hint", content)

	loader := NewLoader(LoaderConfig{GlobalDir: dir})
	skills, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if skills[0].ArgumentHint != "<filename> [options]" {
		t.Errorf("ArgumentHint = %q", skills[0].ArgumentHint)
	}
}
