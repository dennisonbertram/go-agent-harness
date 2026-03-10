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

func (m *mockSkillLister) ResolveSkill(_ context.Context, name, args, workspace string) (string, error) {
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

// unwrapSkillResult is a test helper that unwraps an enriched tool result
// and returns the parsed output JSON and meta-messages.
func unwrapSkillResult(t *testing.T, raw string) (map[string]any, []tools.MetaMessage) {
	t.Helper()
	tr, ok := tools.UnwrapToolResult(raw)
	if !ok {
		t.Fatalf("expected enriched tool result, got plain string: %s", raw)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(tr.Output), &result); err != nil {
		t.Fatalf("unmarshal unwrapped output: %v", err)
	}
	return result, tr.MetaMessages
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
		t.Fatal("XSS: raw <script> tag found in description -- XML escaping failed")
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

	out, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"deploy staging"}`))
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	result, metaMsgs := unwrapSkillResult(t, out)

	if result["skill"].(string) != "deploy" {
		t.Fatalf("expected skill=deploy, got %v", result["skill"])
	}
	if result["status"].(string) != "activated" {
		t.Fatalf("expected status=activated, got %v", result["status"])
	}
	// Instructions should NOT be in the tool output (they are in meta-messages now)
	if _, hasInstructions := result["instructions"]; hasInstructions {
		t.Fatal("instructions should not be in the tool output; they belong in meta-messages")
	}
	allowed := result["allowed_tools"].([]any)
	if len(allowed) != 2 {
		t.Fatalf("expected 2 allowed tools, got %d", len(allowed))
	}

	// Verify meta-messages contain the skill instructions
	if len(metaMsgs) != 1 {
		t.Fatalf("expected 1 meta-message, got %d", len(metaMsgs))
	}
	if !strings.Contains(metaMsgs[0].Content, "Run deploy steps for the given environment.") {
		t.Fatalf("meta-message should contain skill instructions, got: %s", metaMsgs[0].Content)
	}
	if !strings.Contains(metaMsgs[0].Content, `<skill name="deploy">`) {
		t.Fatalf("meta-message should contain skill XML tag, got: %s", metaMsgs[0].Content)
	}
}

func TestSkillTool_Handler_MissingCommand(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := SkillTool(lister)

	_, err := tool.Handler(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for missing command")
	}
	if !strings.Contains(err.Error(), "command is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSkillTool_Handler_EmptyCommand(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := SkillTool(lister)

	_, err := tool.Handler(context.Background(), json.RawMessage(`{"command":""}`))
	if err == nil {
		t.Fatal("expected error for blank command")
	}
	if !strings.Contains(err.Error(), "command is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSkillTool_Handler_WhitespaceOnlyCommand(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := SkillTool(lister)

	_, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"  "}`))
	if err == nil {
		t.Fatal("expected error for whitespace-only command")
	}
	if !strings.Contains(err.Error(), "command is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSkillTool_Handler_UnknownSkill(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := SkillTool(lister)

	_, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"nonexistent"}`))
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

	out, err := tool.Handler(ctx, json.RawMessage(`{"command":"review"}`))
	if err != nil {
		t.Fatalf("apply with metadata failed: %v", err)
	}

	result, _ := unwrapSkillResult(t, out)

	if result["skill"].(string) != "review" {
		t.Fatalf("expected skill=review, got %v", result["skill"])
	}
	if result["status"].(string) != "activated" {
		t.Fatalf("expected status=activated, got %v", result["status"])
	}
}

func TestSkillTool_Handler_NoAllowedTools(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := SkillTool(lister)

	out, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"review"}`))
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	result, _ := unwrapSkillResult(t, out)

	// review skill has no allowed_tools -- should be nil/null in JSON
	if result["allowed_tools"] != nil {
		t.Fatalf("expected nil allowed_tools for review skill, got %v", result["allowed_tools"])
	}
}

func TestSkillTool_Handler_CommandNoArgs(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := SkillTool(lister)

	out, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"review"}`))
	if err != nil {
		t.Fatalf("command with no args failed: %v", err)
	}

	result, metaMsgs := unwrapSkillResult(t, out)

	if result["skill"].(string) != "review" {
		t.Fatalf("expected skill=review, got %v", result["skill"])
	}
	// Instructions are now in meta-messages, not in the output
	if len(metaMsgs) != 1 {
		t.Fatalf("expected 1 meta-message, got %d", len(metaMsgs))
	}
	if !strings.Contains(metaMsgs[0].Content, "Review the code carefully.") {
		t.Fatalf("meta-message should contain skill instructions, got: %s", metaMsgs[0].Content)
	}
}

func TestSkillTool_Handler_CommandExtraWhitespace(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := SkillTool(lister)

	out, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"  deploy   staging  "}`))
	if err != nil {
		t.Fatalf("command with extra whitespace failed: %v", err)
	}

	result, _ := unwrapSkillResult(t, out)

	if result["skill"].(string) != "deploy" {
		t.Fatalf("expected skill=deploy, got %v", result["skill"])
	}
}

func TestSkillTool_Handler_CommandMultiWordArgs(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := SkillTool(lister)

	out, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"deploy staging us-east-1"}`))
	if err != nil {
		t.Fatalf("command with multi-word args failed: %v", err)
	}

	result, metaMsgs := unwrapSkillResult(t, out)

	if result["skill"].(string) != "deploy" {
		t.Fatalf("expected skill=deploy, got %v", result["skill"])
	}
	// Instructions are in meta-messages
	if len(metaMsgs) != 1 {
		t.Fatalf("expected 1 meta-message, got %d", len(metaMsgs))
	}
	if !strings.Contains(metaMsgs[0].Content, "Run deploy steps for the given environment.") {
		t.Fatalf("meta-message should contain skill instructions, got: %s", metaMsgs[0].Content)
	}
}

// ---------- meta-message specific tests ----------

func TestSkillTool_Handler_ReturnsEnrichedResult(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := SkillTool(lister)

	out, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"deploy"}`))
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	// The raw output should be an enriched result (contains __tool_result__ sentinel)
	tr, ok := tools.UnwrapToolResult(out)
	if !ok {
		t.Fatal("expected enriched tool result from skill handler")
	}

	// Output should be a concise activation acknowledgment, not the full instructions
	if strings.Contains(tr.Output, "Run deploy steps") {
		t.Fatal("tool output should not contain full instructions (those belong in meta-messages)")
	}

	// Meta-messages should contain the instructions
	if len(tr.MetaMessages) == 0 {
		t.Fatal("expected at least one meta-message with skill instructions")
	}
}

func TestSkillTool_Handler_MetaMessageContainsInstructions(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := SkillTool(lister)

	out, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"deploy production"}`))
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	tr, ok := tools.UnwrapToolResult(out)
	if !ok {
		t.Fatal("expected enriched tool result")
	}

	if len(tr.MetaMessages) != 1 {
		t.Fatalf("expected exactly 1 meta-message, got %d", len(tr.MetaMessages))
	}

	meta := tr.MetaMessages[0]
	if !strings.Contains(meta.Content, "Run deploy steps for the given environment.") {
		t.Fatalf("meta-message should contain the resolved skill instructions, got: %s", meta.Content)
	}
	if !strings.Contains(meta.Content, `<skill name="deploy">`) {
		t.Fatalf("meta-message should be wrapped in <skill> XML tags, got: %s", meta.Content)
	}
	if !strings.Contains(meta.Content, "</skill>") {
		t.Fatalf("meta-message should have closing </skill> tag, got: %s", meta.Content)
	}
}
