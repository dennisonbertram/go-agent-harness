package harness

import (
	"context"
	"fmt"
	"testing"

	htools "go-agent-harness/internal/harness/tools"
)

func TestDefaultRegistryToolContract(t *testing.T) {
	t.Parallel()

	registry := NewDefaultRegistry(t.TempDir())
	defs := registry.Definitions()

	names := make([]string, 0, len(defs))
	for _, def := range defs {
		names = append(names, def.Name)
		if def.Parameters == nil {
			t.Fatalf("tool %q missing parameters schema", def.Name)
		}
	}

	expected := []string{
		"AskUserQuestion",
		"apply_patch",
		"bash",
		"edit",
		"job_kill",
		"job_output",
		"observational_memory",
		"read",
		"todos",
		"write",
	}
	if len(names) != len(expected) {
		t.Fatalf("expected %d tools, got %d (%v)", len(expected), len(names), names)
	}
	for i := range expected {
		if names[i] != expected[i] {
			t.Fatalf("unexpected tools order/value. got=%v want=%v", names, expected)
		}
	}
}

// ---------- mock SkillLister for contract tests ----------

type contractMockSkillLister struct {
	skills map[string]htools.SkillInfo
}

func (m *contractMockSkillLister) GetSkill(name string) (htools.SkillInfo, bool) {
	s, ok := m.skills[name]
	return s, ok
}

func (m *contractMockSkillLister) ListSkills() []htools.SkillInfo {
	result := make([]htools.SkillInfo, 0, len(m.skills))
	for _, s := range m.skills {
		result = append(result, s)
	}
	return result
}

func (m *contractMockSkillLister) ResolveSkill(_ context.Context, name, args, workspace string) (string, error) {
	if _, ok := m.skills[name]; !ok {
		return "", fmt.Errorf("skill not found: %s", name)
	}
	return "instructions for " + name, nil
}

func TestDefaultRegistryToolContractWithSkills(t *testing.T) {
	t.Parallel()

	lister := &contractMockSkillLister{
		skills: map[string]htools.SkillInfo{
			"deploy": {
				Name:        "deploy",
				Description: "Deploy to production",
				Source:      "project",
			},
		},
	}

	registry := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode: ToolApprovalModeFullAuto,
		SkillLister:  lister,
	})
	defs := registry.Definitions()

	// With skills enabled, the skill tool should appear as a core tool
	found := false
	for _, def := range defs {
		if def.Name == "skill" {
			found = true
			break
		}
	}
	if !found {
		names := make([]string, 0, len(defs))
		for _, def := range defs {
			names = append(names, def.Name)
		}
		t.Fatalf("expected 'skill' tool in registry with skills enabled, got: %v", names)
	}
}

func TestDefaultRegistryToolContractWithSkills_ZeroSkills(t *testing.T) {
	t.Parallel()

	lister := &contractMockSkillLister{
		skills: map[string]htools.SkillInfo{},
	}

	registry := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode: ToolApprovalModeFullAuto,
		SkillLister:  lister,
	})
	defs := registry.Definitions()

	// With zero skills, the skill tool should NOT appear
	for _, def := range defs {
		if def.Name == "skill" {
			t.Fatal("skill tool should not be in registry when lister returns zero skills")
		}
	}
}
