package skills

import (
	"testing"
)

func TestSkillSourceConstants(t *testing.T) {
	if SourceGlobal != "global" {
		t.Errorf("SourceGlobal = %q, want %q", SourceGlobal, "global")
	}
	if SourceLocal != "local" {
		t.Errorf("SourceLocal = %q, want %q", SourceLocal, "local")
	}
}

func TestSkillStruct(t *testing.T) {
	s := Skill{
		Name:         "test-skill",
		Description:  "A test skill",
		Body:         "# Hello",
		FilePath:     "/tmp/skills/test-skill/SKILL.md",
		Version:      1,
		AutoInvoke:   true,
		AllowedTools: []string{"bash", "read"},
		ArgumentHint: "<filename>",
		Source:       SourceLocal,
		Triggers:     []string{"do the thing"},
	}

	if s.Name != "test-skill" {
		t.Errorf("Name = %q, want %q", s.Name, "test-skill")
	}
	if s.Source != SourceLocal {
		t.Errorf("Source = %q, want %q", s.Source, SourceLocal)
	}
	if len(s.AllowedTools) != 2 {
		t.Errorf("AllowedTools len = %d, want 2", len(s.AllowedTools))
	}
	if len(s.Triggers) != 1 {
		t.Errorf("Triggers len = %d, want 1", len(s.Triggers))
	}
}

func TestFrontmatterStruct(t *testing.T) {
	boolVal := false
	fm := frontmatter{
		Name:         "my-skill",
		Description:  "desc",
		Version:      1,
		AutoInvoke:   &boolVal,
		AllowedTools: []string{"bash"},
		ArgumentHint: "hint",
	}

	if fm.Name != "my-skill" {
		t.Errorf("Name = %q, want %q", fm.Name, "my-skill")
	}
	if *fm.AutoInvoke != false {
		t.Error("AutoInvoke should be false")
	}
}

func TestLoaderConfig(t *testing.T) {
	cfg := LoaderConfig{
		GlobalDir:    "/home/user/.go-harness/skills",
		WorkspaceDir: "/project/.go-harness/skills",
	}

	if cfg.GlobalDir != "/home/user/.go-harness/skills" {
		t.Errorf("GlobalDir = %q", cfg.GlobalDir)
	}
	if cfg.WorkspaceDir != "/project/.go-harness/skills" {
		t.Errorf("WorkspaceDir = %q", cfg.WorkspaceDir)
	}
}
