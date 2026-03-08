package skills

import (
	"strings"
	"testing"
)

func setupResolverRegistry(t *testing.T) *Registry {
	t.Helper()
	dir := t.TempDir()
	content := `---
name: greet
description: "Greeting skill"
version: 1
---
Hello $1! You said: $ARGUMENTS
Workspace: $WORKSPACE
Skill dir: $SKILL_DIR
`
	writeSkillFile(t, dir, "greet", content)

	r := NewRegistry()
	loader := NewLoader(LoaderConfig{GlobalDir: dir})
	if err := r.Load(loader); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestResolverResolveSkill_HappyPath(t *testing.T) {
	reg := setupResolverRegistry(t)
	resolver := NewResolver(reg)

	result, err := resolver.ResolveSkill("greet", "world today", "/my/workspace")
	if err != nil {
		t.Fatalf("ResolveSkill() error = %v", err)
	}

	if !strings.Contains(result, "Hello world!") {
		t.Errorf("expected $1=world, got: %s", result)
	}
	if !strings.Contains(result, "You said: world today") {
		t.Errorf("expected $ARGUMENTS=world today, got: %s", result)
	}
	if !strings.Contains(result, "Workspace: /my/workspace") {
		t.Errorf("expected $WORKSPACE=/my/workspace, got: %s", result)
	}
	if !strings.Contains(result, "Skill dir:") {
		t.Errorf("expected $SKILL_DIR to be set, got: %s", result)
	}
}

func TestResolverResolveSkill_NotFound(t *testing.T) {
	reg := NewRegistry()
	resolver := NewResolver(reg)

	_, err := resolver.ResolveSkill("nonexistent", "", "/ws")
	if err == nil {
		t.Fatal("expected error for nonexistent skill")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %v, want 'not found'", err)
	}
}

func TestResolverResolveSkill_EmptyArgs(t *testing.T) {
	reg := setupResolverRegistry(t)
	resolver := NewResolver(reg)

	result, err := resolver.ResolveSkill("greet", "", "/ws")
	if err != nil {
		t.Fatalf("ResolveSkill() error = %v", err)
	}

	// $1 should be empty, $ARGUMENTS should be empty
	if !strings.Contains(result, "Hello !") {
		t.Errorf("expected empty $1, got: %s", result)
	}
	if !strings.Contains(result, "You said: \n") {
		t.Errorf("expected empty $ARGUMENTS, got: %s", result)
	}
}

func TestResolverResolveSkill_ManyArgs(t *testing.T) {
	dir := t.TempDir()
	content := `---
name: multi
description: "Multi arg skill"
version: 1
---
$1 $2 $3 $4 $5 $6 $7 $8 $9
`
	writeSkillFile(t, dir, "multi", content)

	reg := NewRegistry()
	loader := NewLoader(LoaderConfig{GlobalDir: dir})
	if err := reg.Load(loader); err != nil {
		t.Fatal(err)
	}

	resolver := NewResolver(reg)
	result, err := resolver.ResolveSkill("multi", "a b c d e f g h i", "/ws")
	if err != nil {
		t.Fatalf("error = %v", err)
	}

	expected := "a b c d e f g h i"
	result = strings.TrimSpace(result)
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestResolverImplementsInterface(t *testing.T) {
	reg := NewRegistry()
	var _ SkillResolver = NewResolver(reg)
}
