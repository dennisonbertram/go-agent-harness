package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	tools "go-agent-harness/internal/harness/tools"
)

// ---------- mock types ----------

type mockSkillLister struct {
	skills map[string]tools.SkillInfo
	bodies map[string]string
}

func (m *mockSkillLister) GetSkill(name string) (tools.SkillInfo, bool) {
	s, ok := m.skills[name]
	return s, ok
}

func (m *mockSkillLister) ListSkills() []tools.SkillInfo {
	result := make([]tools.SkillInfo, 0, len(m.skills))
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
		skills: map[string]tools.SkillInfo{
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

func newEmptySkillLister() *mockSkillLister {
	return &mockSkillLister{
		skills: map[string]tools.SkillInfo{},
		bodies: map[string]string{},
	}
}

// ---------- buildSkillDescription tests ----------

func TestBuildSkillDescription_WithSkills(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	desc := buildSkillDescription(lister)

	if !strings.Contains(desc, "<available_skills>") {
		t.Fatal("expected <available_skills> XML block in description")
	}
	if !strings.Contains(desc, "</available_skills>") {
		t.Fatal("expected </available_skills> closing tag in description")
	}
	if !strings.Contains(desc, "deploy") {
		t.Fatal("expected 'deploy' skill in description")
	}
	if !strings.Contains(desc, "review") {
		t.Fatal("expected 'review' skill in description")
	}
	if !strings.Contains(desc, "Deploy to production") {
		t.Fatal("expected deploy description in XML")
	}
	if !strings.Contains(desc, `argument_hint=`) {
		t.Fatal("expected argument_hint attribute for deploy skill")
	}
}

func TestBuildSkillDescription_NoSkills(t *testing.T) {
	t.Parallel()
	lister := newEmptySkillLister()
	desc := buildSkillDescription(lister)

	if strings.Contains(desc, "<available_skills>") {
		t.Fatal("should not contain <available_skills> when no skills registered")
	}
	// Should just be the base description
	if desc == "" {
		t.Fatal("expected non-empty base description")
	}
}

func TestBuildSkillDescription_XMLEscaping(t *testing.T) {
	t.Parallel()
	lister := &mockSkillLister{
		skills: map[string]tools.SkillInfo{
			"xss": {
				Name:         `<script>alert("xss")</script>`,
				Description:  `A "dangerous" skill with <html> & entities`,
				ArgumentHint: `<arg>`,
				Source:       "test",
			},
		},
		bodies: map[string]string{},
	}
	desc := buildSkillDescription(lister)

	// Must not contain raw < or > from the skill name/description (except the XML tags themselves)
	if strings.Contains(desc, `<script>`) {
		t.Fatal("XSS: raw <script> tag found in description — XML escaping failed")
	}
	if strings.Contains(desc, `"dangerous"`) && !strings.Contains(desc, `&quot;`) && !strings.Contains(desc, `&#34;`) {
		// html.EscapeString escapes " to &#34;
		t.Fatal("unescaped double quotes in XML attribute value")
	}
	if !strings.Contains(desc, "<available_skills>") {
		t.Fatal("expected <available_skills> block")
	}
}

// ---------- SkillTool definition tests ----------

func TestSkillTool_Definition(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := SkillTool(lister)

	if tool.Definition.Name != "skill" {
		t.Fatalf("expected name=skill, got %s", tool.Definition.Name)
	}
	if tool.Definition.Tier != tools.TierCore {
		t.Fatalf("expected tier=core, got %s", tool.Definition.Tier)
	}
	if tool.Definition.Action != tools.ActionRead {
		t.Fatalf("expected action=read, got %s", tool.Definition.Action)
	}
	if tool.Definition.Mutating {
		t.Fatal("expected mutating=false")
	}
	if !tool.Definition.ParallelSafe {
		t.Fatal("expected parallel_safe=true")
	}
	if tool.Handler == nil {
		t.Fatal("handler is nil")
	}
	if tool.Definition.Parameters == nil {
		t.Fatal("parameters is nil")
	}
}

func TestSkillTool_DescriptionContainsSkills(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := SkillTool(lister)

	if !strings.Contains(tool.Definition.Description, "<available_skills>") {
		t.Fatal("description should contain available_skills XML block")
	}
}

// ---------- handler tests ----------

func TestSkillTool_Handler_ApplyValid(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := SkillTool(lister)

	out, err := tool.Handler(context.Background(), json.RawMessage(`{"name":"deploy","arguments":"staging"}`))
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
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

func TestSkillTool_Handler_MissingName(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := SkillTool(lister)

	_, err := tool.Handler(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSkillTool_Handler_EmptyName(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := SkillTool(lister)

	_, err := tool.Handler(context.Background(), json.RawMessage(`{"name":"  "}`))
	if err == nil {
		t.Fatal("expected error for blank name")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSkillTool_Handler_UnknownSkill(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := SkillTool(lister)

	_, err := tool.Handler(context.Background(), json.RawMessage(`{"name":"nonexistent"}`))
	if err == nil {
		t.Fatal("expected error for unknown skill")
	}
	if !strings.Contains(err.Error(), "skill not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSkillTool_Handler_InvalidJSON(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := SkillTool(lister)

	_, err := tool.Handler(context.Background(), json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestSkillTool_Handler_WithRunMetadata(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := SkillTool(lister)

	ctx := context.WithValue(context.Background(), tools.ContextKeyRunMetadata, tools.RunMetadata{
		RunID: "test-run-123",
	})

	out, err := tool.Handler(ctx, json.RawMessage(`{"name":"review"}`))
	if err != nil {
		t.Fatalf("apply with metadata failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result["skill"].(string) != "review" {
		t.Fatalf("expected skill=review, got %v", result["skill"])
	}
}

func TestSkillTool_Handler_NoAllowedTools(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := SkillTool(lister)

	out, err := tool.Handler(context.Background(), json.RawMessage(`{"name":"review"}`))
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	// review skill has no allowed_tools — should be nil/null in JSON
	if result["allowed_tools"] != nil {
		t.Fatalf("expected nil allowed_tools for review skill, got %v", result["allowed_tools"])
	}
}
