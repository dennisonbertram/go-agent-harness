package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

type mockSkillLister struct {
	skills map[string]SkillInfo
	bodies map[string]string
}

func (m *mockSkillLister) GetSkill(name string) (SkillInfo, bool) {
	s, ok := m.skills[name]
	return s, ok
}

func (m *mockSkillLister) ListSkills() []SkillInfo {
	result := make([]SkillInfo, 0, len(m.skills))
	for _, s := range m.skills {
		result = append(result, s)
	}
	return result
}

func (m *mockSkillLister) ResolveSkill(name, args, workspace string) (string, error) {
	body, ok := m.bodies[name]
	if !ok {
		return "", fmt.Errorf("skill not found: %s", name)
	}
	return body, nil
}

func newMockSkillLister() *mockSkillLister {
	return &mockSkillLister{
		skills: map[string]SkillInfo{
			"deploy": {
				Name:         "deploy",
				Description:  "Deploy to production",
				ArgumentHint: "<env>",
				AllowedTools: []string{"bash", "read"},
				Source:       "project",
			},
			"review": {
				Name:        "review",
				Description: "Code review helper",
				Source:      "user",
			},
		},
		bodies: map[string]string{
			"deploy": "Run deploy steps for the given environment.",
			"review": "Review the code carefully.",
		},
	}
}

func TestSkillToolApplyValidSkill(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := skillTool(lister)

	out, err := tool.Handler(context.Background(), json.RawMessage(`{"name":"deploy","arguments":"staging"}`))
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal apply result: %v", err)
	}

	if result["skill"].(string) != "deploy" {
		t.Fatalf("expected skill=deploy, got %v", result["skill"])
	}
	if result["instructions"].(string) != "Run deploy steps for the given environment." {
		t.Fatalf("unexpected instructions: %v", result["instructions"])
	}
	allowed := result["allowed_tools"].([]any)
	if len(allowed) != 2 {
		t.Fatalf("expected 2 allowed tools, got %d", len(allowed))
	}
}

func TestSkillToolApplyMissingName(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := skillTool(lister)

	_, err := tool.Handler(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatalf("expected error for apply without name")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSkillToolApplyEmptyName(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := skillTool(lister)

	_, err := tool.Handler(context.Background(), json.RawMessage(`{"name":"  "}`))
	if err == nil {
		t.Fatalf("expected error for apply with blank name")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSkillToolApplyUnknownSkill(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := skillTool(lister)

	_, err := tool.Handler(context.Background(), json.RawMessage(`{"name":"nonexistent"}`))
	if err == nil {
		t.Fatalf("expected error for unknown skill")
	}
	if !strings.Contains(err.Error(), "skill not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSkillToolRegisteredWhenEnabled(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	list, err := BuildCatalog(BuildOptions{
		WorkspaceRoot: t.TempDir(),
		EnableSkills:  true,
		SkillLister:   lister,
	})
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}

	found := false
	for _, tool := range list {
		if tool.Definition.Name == "skill" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected skill tool in catalog when enabled")
	}
}

func TestSkillToolNotRegisteredWhenDisabled(t *testing.T) {
	t.Parallel()
	list, err := BuildCatalog(BuildOptions{
		WorkspaceRoot: t.TempDir(),
		EnableSkills:  false,
	})
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}

	for _, tool := range list {
		if tool.Definition.Name == "skill" {
			t.Fatalf("skill tool should not be in catalog when disabled")
		}
	}
}

func TestSkillToolNotRegisteredWhenNilLister(t *testing.T) {
	t.Parallel()
	list, err := BuildCatalog(BuildOptions{
		WorkspaceRoot: t.TempDir(),
		EnableSkills:  true,
		SkillLister:   nil,
	})
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}

	for _, tool := range list {
		if tool.Definition.Name == "skill" {
			t.Fatalf("skill tool should not be in catalog when lister is nil")
		}
	}
}

func TestSkillToolDefinition(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := skillTool(lister)

	if tool.Definition.Name != "skill" {
		t.Fatalf("expected name=skill, got %s", tool.Definition.Name)
	}
	if tool.Definition.Action != ActionRead {
		t.Fatalf("expected action=read, got %s", tool.Definition.Action)
	}
	if tool.Definition.Mutating {
		t.Fatalf("expected mutating=false")
	}
	if !tool.Definition.ParallelSafe {
		t.Fatalf("expected parallel_safe=true")
	}
}

func TestSkillToolInvalidJSON(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := skillTool(lister)

	_, err := tool.Handler(context.Background(), json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatalf("expected error for invalid JSON")
	}
}
