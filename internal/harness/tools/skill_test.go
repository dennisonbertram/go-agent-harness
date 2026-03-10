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

func (m *mockSkillLister) ResolveSkill(_ context.Context, name, args, workspace string) (string, error) {
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
	tool := skillTool(lister, nil)

	out, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"deploy staging"}`))
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

func TestSkillToolApplyMissingCommand(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := skillTool(lister, nil)

	_, err := tool.Handler(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatalf("expected error for apply without command")
	}
	if !strings.Contains(err.Error(), "command is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSkillToolApplyEmptyCommand(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := skillTool(lister, nil)

	_, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"  "}`))
	if err == nil {
		t.Fatalf("expected error for apply with blank command")
	}
	if !strings.Contains(err.Error(), "command is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSkillToolApplyUnknownSkill(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := skillTool(lister, nil)

	_, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"nonexistent"}`))
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
	tool := skillTool(lister, nil)

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
	tool := skillTool(lister, nil)

	_, err := tool.Handler(context.Background(), json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatalf("expected error for invalid JSON")
	}
}

func TestSkillToolCommandNoArgs(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := skillTool(lister, nil)

	out, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"review"}`))
	if err != nil {
		t.Fatalf("command with no args failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result["skill"].(string) != "review" {
		t.Fatalf("expected skill=review, got %v", result["skill"])
	}
	if result["instructions"].(string) != "Review the code carefully." {
		t.Fatalf("unexpected instructions: %v", result["instructions"])
	}
}

func TestSkillToolCommandExtraWhitespace(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := skillTool(lister, nil)

	out, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"  deploy   staging  "}`))
	if err != nil {
		t.Fatalf("command with extra whitespace failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result["skill"].(string) != "deploy" {
		t.Fatalf("expected skill=deploy, got %v", result["skill"])
	}
}

func TestSkillToolCommandMultiWordArgs(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := skillTool(lister, nil)

	out, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"deploy staging us-east-1"}`))
	if err != nil {
		t.Fatalf("command with multi-word args failed: %v", err)
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
}

// ---------------------------------------------------------------------------
// flatSkillFork tests
// ---------------------------------------------------------------------------

type mockAgentRunner struct {
	output string
	err    error
}

func (m *mockAgentRunner) RunPrompt(_ context.Context, prompt string) (string, error) {
	return m.output, m.err
}

type mockForkedAgentRunner struct {
	result ForkResult
	err    error
}

func (m *mockForkedAgentRunner) RunPrompt(_ context.Context, prompt string) (string, error) {
	return m.result.Output, m.err
}

func (m *mockForkedAgentRunner) RunForkedSkill(_ context.Context, config ForkConfig) (ForkResult, error) {
	return m.result, m.err
}

func TestFlatSkillForkBasicRunPrompt(t *testing.T) {
	t.Parallel()
	runner := &mockAgentRunner{output: "fork output"}
	info := SkillInfo{Name: "test-fork", Context: "fork"}

	out, err := flatSkillFork(context.Background(), runner, info, "do the thing")
	if err != nil {
		t.Fatalf("flatSkillFork: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["skill"].(string) != "test-fork" {
		t.Fatalf("expected skill=test-fork, got %v", result["skill"])
	}
	if result["status"].(string) != "completed" {
		t.Fatalf("expected status=completed, got %v", result["status"])
	}
	if result["result"].(string) != "fork output" {
		t.Fatalf("expected result='fork output', got %v", result["result"])
	}
	if result["context"].(string) != "fork" {
		t.Fatalf("expected context=fork, got %v", result["context"])
	}
}

func TestFlatSkillForkForkedAgentRunner(t *testing.T) {
	t.Parallel()
	runner := &mockForkedAgentRunner{
		result: ForkResult{Output: "raw output", Summary: "summary output"},
	}
	info := SkillInfo{
		Name:         "rich-fork",
		Context:      "fork",
		Agent:        "Explore",
		AllowedTools: []string{"bash"},
	}

	out, err := flatSkillFork(context.Background(), runner, info, "explore code")
	if err != nil {
		t.Fatalf("flatSkillFork: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Summary should be preferred over raw output
	if result["result"].(string) != "summary output" {
		t.Fatalf("expected summary to be used, got %v", result["result"])
	}
}

func TestFlatSkillForkForkedAgentRunnerFallbackToOutput(t *testing.T) {
	t.Parallel()
	runner := &mockForkedAgentRunner{
		result: ForkResult{Output: "raw output", Summary: ""},
	}
	info := SkillInfo{Name: "no-summary", Context: "fork"}

	out, err := flatSkillFork(context.Background(), runner, info, "prompt")
	if err != nil {
		t.Fatalf("flatSkillFork: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["result"].(string) != "raw output" {
		t.Fatalf("expected raw output fallback, got %v", result["result"])
	}
}

func TestFlatSkillForkNilRunner(t *testing.T) {
	t.Parallel()
	info := SkillInfo{Name: "needs-runner", Context: "fork"}

	_, err := flatSkillFork(context.Background(), nil, info, "prompt")
	if err == nil {
		t.Fatalf("expected error for nil runner")
	}
	if !strings.Contains(err.Error(), "no AgentRunner is configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFlatSkillForkNestedPrevention(t *testing.T) {
	t.Parallel()
	runner := &mockAgentRunner{output: "ok"}
	info := SkillInfo{Name: "inner", Context: "fork"}

	// Simulate already being inside a forked skill
	ctx := context.WithValue(context.Background(), ContextKeyForkedSkill, "outer")
	_, err := flatSkillFork(ctx, runner, info, "prompt")
	if err == nil {
		t.Fatalf("expected error for nested fork")
	}
	if !strings.Contains(err.Error(), "nested skill forking is not supported") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFlatSkillForkRunPromptError(t *testing.T) {
	t.Parallel()
	runner := &mockAgentRunner{err: fmt.Errorf("agent crashed")}
	info := SkillInfo{Name: "failing", Context: "fork"}

	_, err := flatSkillFork(context.Background(), runner, info, "prompt")
	if err == nil {
		t.Fatalf("expected error from RunPrompt failure")
	}
	if !strings.Contains(err.Error(), "forked skill") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFlatSkillForkForkedAgentRunnerError(t *testing.T) {
	t.Parallel()
	runner := &mockForkedAgentRunner{err: fmt.Errorf("fork failed")}
	info := SkillInfo{Name: "failing-fork", Context: "fork", Agent: "Code"}

	_, err := flatSkillFork(context.Background(), runner, info, "prompt")
	if err == nil {
		t.Fatalf("expected error from RunForkedSkill failure")
	}
	if !strings.Contains(err.Error(), "forked skill") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFlatSkillForkViaSkillToolHandler(t *testing.T) {
	t.Parallel()
	lister := &mockSkillLister{
		skills: map[string]SkillInfo{
			"fork-skill": {
				Name:    "fork-skill",
				Context: "fork",
			},
		},
		bodies: map[string]string{
			"fork-skill": "do the forked thing",
		},
	}
	runner := &mockAgentRunner{output: "forked result"}
	tool := skillTool(lister, runner)

	out, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"fork-skill"}`))
	if err != nil {
		t.Fatalf("skill tool handler fork: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["context"].(string) != "fork" {
		t.Fatalf("expected context=fork, got %v", result["context"])
	}
}
