package skills

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestRegistryGetEmpty(t *testing.T) {
	r := NewRegistry()
	_, ok := r.Get("nonexistent")
	if ok {
		t.Error("Get should return false for empty registry")
	}
}

func TestRegistryListEmpty(t *testing.T) {
	r := NewRegistry()
	list := r.List()
	if len(list) != 0 {
		t.Errorf("List() returned %d items, want 0", len(list))
	}
}

func TestRegistryLoad(t *testing.T) {
	dir := t.TempDir()
	writeSkillFile(t, dir, "my-skill", validSkillMD)

	r := NewRegistry()
	loader := NewLoader(LoaderConfig{GlobalDir: dir})
	if err := r.Load(loader); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	s, ok := r.Get("my-skill")
	if !ok {
		t.Fatal("expected to find my-skill")
	}
	if s.Name != "my-skill" {
		t.Errorf("Name = %q", s.Name)
	}
}

func TestRegistryLocalOverridesGlobal(t *testing.T) {
	globalDir := t.TempDir()
	localDir := t.TempDir()

	globalContent := `---
name: my-skill
description: "Global version"
version: 1
---
Global body.
`
	localContent := `---
name: my-skill
description: "Local version"
version: 1
---
Local body.
`
	writeSkillFile(t, globalDir, "my-skill", globalContent)
	writeSkillFile(t, localDir, "my-skill", localContent)

	r := NewRegistry()
	loader := NewLoader(LoaderConfig{GlobalDir: globalDir, WorkspaceDir: localDir})
	if err := r.Load(loader); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	s, ok := r.Get("my-skill")
	if !ok {
		t.Fatal("expected to find my-skill")
	}
	if s.Source != SourceLocal {
		t.Errorf("Source = %q, want local (local should override global)", s.Source)
	}
	if s.Description != "Local version" {
		t.Errorf("Description = %q, want 'Local version'", s.Description)
	}
}

func TestRegistryListSorted(t *testing.T) {
	globalDir := t.TempDir()

	skills := []struct {
		name    string
		content string
	}{
		{"charlie", `---
name: charlie
description: "C skill"
version: 1
---
C`},
		{"alpha", `---
name: alpha
description: "A skill"
version: 1
---
A`},
		{"bravo", `---
name: bravo
description: "B skill"
version: 1
---
B`},
	}

	for _, s := range skills {
		writeSkillFile(t, globalDir, s.name, s.content)
	}

	r := NewRegistry()
	loader := NewLoader(LoaderConfig{GlobalDir: globalDir})
	if err := r.Load(loader); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	list := r.List()
	if len(list) != 3 {
		t.Fatalf("List() returned %d items, want 3", len(list))
	}
	expected := []string{"alpha", "bravo", "charlie"}
	for i, s := range list {
		if s.Name != expected[i] {
			t.Errorf("List()[%d].Name = %q, want %q", i, s.Name, expected[i])
		}
	}
}

func TestRegistryMatchTriggers(t *testing.T) {
	globalDir := t.TempDir()

	content1 := `---
name: deploy-skill
description: "Deploy things. Trigger: deploy my app"
version: 1
---
Deploy body.
`
	content2 := `---
name: test-skill
description: "Run tests. Trigger: run tests, test suite"
version: 1
---
Test body.
`
	content3 := `---
name: no-trigger
description: "No trigger here"
version: 1
---
Plain body.
`
	writeSkillFile(t, globalDir, "deploy-skill", content1)
	writeSkillFile(t, globalDir, "test-skill", content2)
	writeSkillFile(t, globalDir, "no-trigger", content3)

	r := NewRegistry()
	loader := NewLoader(LoaderConfig{GlobalDir: globalDir})
	if err := r.Load(loader); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	matches := r.MatchTriggers("please deploy my app")
	if len(matches) != 1 {
		t.Fatalf("MatchTriggers returned %d, want 1", len(matches))
	}
	if matches[0].Name != "deploy-skill" {
		t.Errorf("matched %q, want deploy-skill", matches[0].Name)
	}

	matches = r.MatchTriggers("run tests now")
	if len(matches) != 1 {
		t.Fatalf("MatchTriggers returned %d, want 1", len(matches))
	}
	if matches[0].Name != "test-skill" {
		t.Errorf("matched %q, want test-skill", matches[0].Name)
	}

	matches = r.MatchTriggers("something unrelated")
	if len(matches) != 0 {
		t.Errorf("MatchTriggers returned %d, want 0", len(matches))
	}
}

func TestRegistryConcurrentAccess(t *testing.T) {
	globalDir := t.TempDir()
	writeSkillFile(t, globalDir, "my-skill", validSkillMD)

	r := NewRegistry()
	loader := NewLoader(LoaderConfig{GlobalDir: globalDir})
	if err := r.Load(loader); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			r.Get("my-skill")
		}()
		go func() {
			defer wg.Done()
			r.List()
		}()
		go func() {
			defer wg.Done()
			r.MatchTriggers("do my thing")
		}()
	}
	wg.Wait()
}

const skillOneMD = `---
name: skill-one
description: first skill for reload test
version: 1
---
Do something one`

const skillTwoMD = `---
name: skill-two
description: second skill for reload test
version: 1
---
Do something two`

// TestRegistry_Reload_ReplacesAll verifies that Reload fully replaces the
// existing skill set, including removing skills that are no longer on disk.
func TestRegistry_Reload_ReplacesAll(t *testing.T) {
	dir := t.TempDir()
	writeSkillFile(t, dir, "skill-one", skillOneMD)
	writeSkillFile(t, dir, "skill-two", skillTwoMD)

	r := NewRegistry()
	loader := NewLoader(LoaderConfig{GlobalDir: dir})
	if err := r.Load(loader); err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if n := len(r.List()); n != 2 {
		t.Fatalf("expected 2 skills after initial Load, got %d", n)
	}

	// Remove skill-two from disk then Reload
	if err := removeSkillDir(t, dir, "skill-two"); err != nil {
		t.Fatalf("removeSkillDir: %v", err)
	}

	if err := r.Reload(loader); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	skills := r.List()
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill after Reload, got %d: %v", len(skills), skills)
	}
	if skills[0].Name != "skill-one" {
		t.Errorf("expected skill %q, got %q", "skill-one", skills[0].Name)
	}
}

// TestRegistry_Reload_AddNewSkill verifies that a newly-created skill on disk
// appears after Reload.
func TestRegistry_Reload_AddNewSkill(t *testing.T) {
	dir := t.TempDir()
	writeSkillFile(t, dir, "my-skill", validSkillMD)

	r := NewRegistry()
	loader := NewLoader(LoaderConfig{GlobalDir: dir})
	if err := r.Load(loader); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Write a new skill to disk
	writeSkillFile(t, dir, "brand-new", `---
name: brand-new
description: a brand new skill
version: 1
---
Brand new body`)

	if err := r.Reload(loader); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	skills := r.List()
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills after Reload, got %d", len(skills))
	}

	names := map[string]bool{}
	for _, s := range skills {
		names[s.Name] = true
	}
	if !names["my-skill"] || !names["brand-new"] {
		t.Errorf("unexpected skill names: %v", names)
	}
}

// TestRegistry_Reload_Concurrent verifies Reload is safe under concurrent reads.
func TestRegistry_Reload_Concurrent(t *testing.T) {
	dir := t.TempDir()
	writeSkillFile(t, dir, "my-skill", validSkillMD)

	r := NewRegistry()
	loader := NewLoader(LoaderConfig{GlobalDir: dir})
	if err := r.Load(loader); err != nil {
		t.Fatalf("Load: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = r.Reload(loader)
		}()
		go func() {
			defer wg.Done()
			_ = r.List()
		}()
	}
	wg.Wait()
}

// removeSkillDir removes a skill subdirectory from a parent directory.
func removeSkillDir(t *testing.T, parentDir, skillName string) error {
	t.Helper()
	return os.RemoveAll(filepath.Join(parentDir, skillName))
}
