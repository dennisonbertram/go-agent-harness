package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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

// --- mock AgentRunner types ---

type mockBasicRunner struct {
	output string
	err    error
}

func (m *mockBasicRunner) RunPrompt(ctx context.Context, prompt string) (string, error) {
	return m.output, m.err
}

type mockForkedRunner struct {
	runPromptOutput string
	runPromptErr    error
	forkOutput      tools.ForkResult
	forkErr         error
	lastForkConfig  tools.ForkConfig
}

func (m *mockForkedRunner) RunPrompt(ctx context.Context, prompt string) (string, error) {
	return m.runPromptOutput, m.runPromptErr
}

func (m *mockForkedRunner) RunForkedSkill(ctx context.Context, config tools.ForkConfig) (tools.ForkResult, error) {
	m.lastForkConfig = config
	return m.forkOutput, m.forkErr
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

	if strings.Contains(desc, `<script>`) {
		t.Fatal("XSS: raw <script> tag found in description -- XML escaping failed")
	}
	if strings.Contains(desc, `"dangerous"`) && !strings.Contains(desc, `&quot;`) && !strings.Contains(desc, `&#34;`) {
		t.Fatal("unescaped double quotes in XML attribute value")
	}
	if !strings.Contains(desc, "<available_skills>") {
		t.Fatal("expected <available_skills> block")
	}
}

func TestBuildSkillDescription_ForkContextAttribute(t *testing.T) {
	t.Parallel()
	lister := &mockSkillLister{
		skills: map[string]tools.SkillInfo{
			"research": {
				Name:        "research",
				Description: "Research a topic",
				Source:      "global",
				Context:     "fork",
			},
		},
		bodies: map[string]string{},
	}
	desc := buildSkillDescription(lister)
	if !strings.Contains(desc, `context="fork"`) {
		t.Fatal("expected context=fork attribute in skill XML for fork skills")
	}
}

// ---------- SkillTool definition tests ----------

func TestSkillTool_Definition(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := SkillTool(lister, nil)

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
	tool := SkillTool(lister, nil)

	if !strings.Contains(tool.Definition.Description, "<available_skills>") {
		t.Fatal("description should contain available_skills XML block")
	}
}

// ---------- handler tests (conversation path) ----------

func TestSkillTool_Handler_ApplyValid(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := SkillTool(lister, nil)

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
	if _, hasInstructions := result["instructions"]; hasInstructions {
		t.Fatal("instructions should not be in the tool output; they belong in meta-messages")
	}
	allowed := result["allowed_tools"].([]any)
	if len(allowed) != 2 {
		t.Fatalf("expected 2 allowed tools, got %d", len(allowed))
	}

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
	tool := SkillTool(lister, nil)

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
	tool := SkillTool(lister, nil)

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
	tool := SkillTool(lister, nil)

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
	tool := SkillTool(lister, nil)

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
	tool := SkillTool(lister, nil)

	_, err := tool.Handler(context.Background(), json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestSkillTool_Handler_WithRunMetadata(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := SkillTool(lister, nil)

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
	tool := SkillTool(lister, nil)

	out, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"review"}`))
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	result, _ := unwrapSkillResult(t, out)

	if result["allowed_tools"] != nil {
		t.Fatalf("expected nil allowed_tools for review skill, got %v", result["allowed_tools"])
	}
}

func TestSkillTool_Handler_CommandNoArgs(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := SkillTool(lister, nil)

	out, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"review"}`))
	if err != nil {
		t.Fatalf("command with no args failed: %v", err)
	}

	result, metaMsgs := unwrapSkillResult(t, out)

	if result["skill"].(string) != "review" {
		t.Fatalf("expected skill=review, got %v", result["skill"])
	}
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
	tool := SkillTool(lister, nil)

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
	tool := SkillTool(lister, nil)

	out, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"deploy staging us-east-1"}`))
	if err != nil {
		t.Fatalf("command with multi-word args failed: %v", err)
	}

	result, metaMsgs := unwrapSkillResult(t, out)

	if result["skill"].(string) != "deploy" {
		t.Fatalf("expected skill=deploy, got %v", result["skill"])
	}
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
	tool := SkillTool(lister, nil)

	out, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"deploy"}`))
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	tr, ok := tools.UnwrapToolResult(out)
	if !ok {
		t.Fatal("expected enriched tool result from skill handler")
	}

	if strings.Contains(tr.Output, "Run deploy steps") {
		t.Fatal("tool output should not contain full instructions (those belong in meta-messages)")
	}

	if len(tr.MetaMessages) == 0 {
		t.Fatal("expected at least one meta-message with skill instructions")
	}
}

func TestSkillTool_Handler_MetaMessageContainsInstructions(t *testing.T) {
	t.Parallel()
	lister := newMockSkillLister()
	tool := SkillTool(lister, nil)

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

// ---------- fork dispatch tests ----------

func newForkSkillLister() *mockSkillLister {
	return &mockSkillLister{
		skills: map[string]tools.SkillInfo{
			"research": {
				Name:         "research",
				Description:  "Deep research",
				AllowedTools: []string{"read", "grep"},
				Source:       "global",
				Context:      "fork",
				Agent:        "Explore",
			},
			"deploy": {
				Name:         "deploy",
				Description:  "Deploy to production",
				AllowedTools: []string{"bash", "read"},
				Source:       "project",
				Context:      "conversation",
			},
		},
		bodies: map[string]string{
			"research": "Research the topic thoroughly.",
			"deploy":   "Run deploy steps.",
		},
	}
}

func TestSkillTool_Handler_ForkWithForkedRunner(t *testing.T) {
	t.Parallel()
	lister := newForkSkillLister()
	runner := &mockForkedRunner{
		forkOutput: tools.ForkResult{
			Output:  "Full research output",
			Summary: "Summary of research",
		},
	}
	tool := SkillTool(lister, runner)

	out, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"research OAuth2"}`))
	if err != nil {
		t.Fatalf("fork failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if result["skill"] != "research" {
		t.Fatalf("expected skill=research, got %v", result["skill"])
	}
	if result["status"] != "completed" {
		t.Fatalf("expected status=completed, got %v", result["status"])
	}
	if result["context"] != "fork" {
		t.Fatalf("expected context=fork, got %v", result["context"])
	}
	// Should use summary (preferred over output)
	if result["result"] != "Summary of research" {
		t.Fatalf("expected result=summary, got %v", result["result"])
	}

	// Verify fork config was passed correctly
	if runner.lastForkConfig.SkillName != "research" {
		t.Fatalf("ForkConfig.SkillName = %q, want %q", runner.lastForkConfig.SkillName, "research")
	}
	if runner.lastForkConfig.Agent != "Explore" {
		t.Fatalf("ForkConfig.Agent = %q, want %q", runner.lastForkConfig.Agent, "Explore")
	}
	if len(runner.lastForkConfig.AllowedTools) != 2 {
		t.Fatalf("ForkConfig.AllowedTools = %v, want [read grep]", runner.lastForkConfig.AllowedTools)
	}
}

func TestSkillTool_Handler_ForkWithBasicRunner(t *testing.T) {
	t.Parallel()
	lister := newForkSkillLister()
	runner := &mockBasicRunner{output: "Basic runner result"}
	tool := SkillTool(lister, runner)

	out, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"research OAuth2"}`))
	if err != nil {
		t.Fatalf("fork with basic runner failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if result["result"] != "Basic runner result" {
		t.Fatalf("expected result from RunPrompt fallback, got %v", result["result"])
	}
	if result["context"] != "fork" {
		t.Fatalf("expected context=fork, got %v", result["context"])
	}
}

func TestSkillTool_Handler_ForkNoRunner(t *testing.T) {
	t.Parallel()
	lister := newForkSkillLister()
	tool := SkillTool(lister, nil) // nil runner

	_, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"research OAuth2"}`))
	if err == nil {
		t.Fatal("expected error when AgentRunner is nil for fork skill")
	}
	if !strings.Contains(err.Error(), "no AgentRunner is configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSkillTool_Handler_ForkNestedPrevention(t *testing.T) {
	t.Parallel()
	lister := newForkSkillLister()
	runner := &mockForkedRunner{
		forkOutput: tools.ForkResult{Output: "result"},
	}
	tool := SkillTool(lister, runner)

	// Simulate being at maximum fork depth — further forking must be rejected.
	ctx := tools.WithForkDepth(context.Background(), tools.DefaultMaxForkDepth)

	_, err := tool.Handler(ctx, json.RawMessage(`{"command":"research OAuth2"}`))
	if err == nil {
		t.Fatal("expected error at max fork depth")
	}
	if !strings.Contains(err.Error(), "max recursion depth") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSkillTool_Handler_ForkRunnerError(t *testing.T) {
	t.Parallel()
	lister := newForkSkillLister()
	runner := &mockForkedRunner{
		forkErr: fmt.Errorf("subagent timeout"),
	}
	tool := SkillTool(lister, runner)

	_, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"research OAuth2"}`))
	if err == nil {
		t.Fatal("expected error when runner fails")
	}
	if !strings.Contains(err.Error(), "forked skill") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "subagent timeout") {
		t.Fatalf("expected original error in message: %v", err)
	}
}

func TestSkillTool_Handler_ForkResultError(t *testing.T) {
	t.Parallel()
	lister := newForkSkillLister()
	runner := &mockForkedRunner{
		forkOutput: tools.ForkResult{Error: "child run failed"},
	}
	tool := SkillTool(lister, runner)

	_, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"research OAuth2"}`))
	if err == nil {
		t.Fatal("expected error when child run reports failure")
	}
	if !strings.Contains(err.Error(), "child run failed") {
		t.Fatalf("expected child failure in error, got %v", err)
	}
}

func TestSkillTool_Handler_ForkSummaryPreference(t *testing.T) {
	t.Parallel()
	lister := newForkSkillLister()
	runner := &mockForkedRunner{
		forkOutput: tools.ForkResult{
			Output:  "Very long output...",
			Summary: "Concise summary",
		},
	}
	tool := SkillTool(lister, runner)

	out, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"research topic"}`))
	if err != nil {
		t.Fatalf("fork failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["result"] != "Concise summary" {
		t.Fatalf("expected Summary to be preferred, got %v", result["result"])
	}
}

func TestSkillTool_Handler_ForkEmptySummaryFallback(t *testing.T) {
	t.Parallel()
	lister := newForkSkillLister()
	runner := &mockForkedRunner{
		forkOutput: tools.ForkResult{
			Output:  "Full output here",
			Summary: "", // empty summary
		},
	}
	tool := SkillTool(lister, runner)

	out, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"research topic"}`))
	if err != nil {
		t.Fatalf("fork failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["result"] != "Full output here" {
		t.Fatalf("expected fallback to Output when Summary is empty, got %v", result["result"])
	}
}

func TestSkillTool_Handler_ConversationUnchanged(t *testing.T) {
	t.Parallel()
	lister := newForkSkillLister()
	runner := &mockForkedRunner{
		forkOutput: tools.ForkResult{Output: "should not be used"},
	}
	tool := SkillTool(lister, runner)

	// deploy has context=conversation, so it should follow the normal path
	out, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"deploy staging"}`))
	if err != nil {
		t.Fatalf("conversation skill failed: %v", err)
	}

	result, metaMsgs := unwrapSkillResult(t, out)

	if result["skill"].(string) != "deploy" {
		t.Fatalf("expected skill=deploy, got %v", result["skill"])
	}
	if result["status"].(string) != "activated" {
		t.Fatalf("expected status=activated, got %v", result["status"])
	}
	if len(metaMsgs) != 1 {
		t.Fatalf("expected 1 meta-message for conversation skill, got %d", len(metaMsgs))
	}
}

func TestSkillTool_Handler_ForkPassesAllowedTools(t *testing.T) {
	t.Parallel()
	lister := newForkSkillLister()
	runner := &mockForkedRunner{
		forkOutput: tools.ForkResult{Output: "done"},
	}
	tool := SkillTool(lister, runner)

	_, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"research topic"}`))
	if err != nil {
		t.Fatalf("fork failed: %v", err)
	}

	if len(runner.lastForkConfig.AllowedTools) != 2 {
		t.Fatalf("AllowedTools = %v, want [read grep]", runner.lastForkConfig.AllowedTools)
	}
}

func TestSkillTool_Handler_ForkPassesAgent(t *testing.T) {
	t.Parallel()
	lister := newForkSkillLister()
	runner := &mockForkedRunner{
		forkOutput: tools.ForkResult{Output: "done"},
	}
	tool := SkillTool(lister, runner)

	_, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"research topic"}`))
	if err != nil {
		t.Fatalf("fork failed: %v", err)
	}

	if runner.lastForkConfig.Agent != "Explore" {
		t.Fatalf("Agent = %q, want %q", runner.lastForkConfig.Agent, "Explore")
	}
}

func TestSkillTool_Handler_ForkContextCanceled(t *testing.T) {
	t.Parallel()
	lister := newForkSkillLister()
	runner := &mockBasicRunner{
		err: context.Canceled,
	}
	tool := SkillTool(lister, runner)

	_, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"research topic"}`))
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------- list action tests ----------

func newVerificationSkillLister() *mockSkillLister {
	return &mockSkillLister{
		skills: map[string]tools.SkillInfo{
			"verified-skill": {
				Name:        "verified-skill",
				Description: "A verified skill",
				Source:      "global",
				Verified:    true,
				VerifiedAt:  "2026-03-09T12:00:00Z",
				VerifiedBy:  "dennisonbertram",
			},
			"unverified-skill": {
				Name:        "unverified-skill",
				Description: "An unverified skill",
				Source:      "global",
				Verified:    false,
			},
		},
		bodies: map[string]string{
			"verified-skill":   "Do the verified thing.",
			"unverified-skill": "Do the unverified thing.",
		},
	}
}

func TestSkillTool_Handler_ListShowsVerifiedStatus(t *testing.T) {
	t.Parallel()
	lister := newVerificationSkillLister()
	tool := SkillTool(lister, nil)

	out, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"list"}`))
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	if !strings.Contains(out, "[verified]") {
		t.Fatalf("list output should contain [verified], got: %s", out)
	}
	if !strings.Contains(out, "[unverified]") {
		t.Fatalf("list output should contain [unverified], got: %s", out)
	}
	if !strings.Contains(out, "verified-skill") {
		t.Fatalf("list output should contain 'verified-skill', got: %s", out)
	}
	if !strings.Contains(out, "unverified-skill") {
		t.Fatalf("list output should contain 'unverified-skill', got: %s", out)
	}
}

func TestSkillTool_Handler_ListEmptySkills(t *testing.T) {
	t.Parallel()
	lister := newEmptySkillLister()
	tool := SkillTool(lister, nil)

	out, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"list"}`))
	if err != nil {
		t.Fatalf("list with no skills failed: %v", err)
	}

	if !strings.Contains(out, "No skills registered") {
		t.Fatalf("list output should say no skills, got: %s", out)
	}
}

// ---------- apply unverified warning tests ----------

func TestSkillTool_Handler_ApplyUnverifiedPrependsWarning(t *testing.T) {
	t.Parallel()
	lister := newVerificationSkillLister()
	tool := SkillTool(lister, nil)

	out, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"unverified-skill"}`))
	if err != nil {
		t.Fatalf("apply unverified skill failed: %v", err)
	}

	tr, ok := tools.UnwrapToolResult(out)
	if !ok {
		t.Fatal("expected enriched tool result")
	}
	if len(tr.MetaMessages) != 1 {
		t.Fatalf("expected 1 meta-message, got %d", len(tr.MetaMessages))
	}
	if !strings.Contains(tr.MetaMessages[0].Content, "WARNING: skill is unverified") {
		t.Fatalf("meta-message should contain unverified warning, got: %s", tr.MetaMessages[0].Content)
	}
}

func TestSkillTool_Handler_ApplyVerifiedNoWarning(t *testing.T) {
	t.Parallel()
	lister := newVerificationSkillLister()
	tool := SkillTool(lister, nil)

	out, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"verified-skill"}`))
	if err != nil {
		t.Fatalf("apply verified skill failed: %v", err)
	}

	tr, ok := tools.UnwrapToolResult(out)
	if !ok {
		t.Fatal("expected enriched tool result")
	}
	if len(tr.MetaMessages) != 1 {
		t.Fatalf("expected 1 meta-message, got %d", len(tr.MetaMessages))
	}
	if strings.Contains(tr.MetaMessages[0].Content, "WARNING") {
		t.Fatalf("meta-message should NOT contain warning for verified skill, got: %s", tr.MetaMessages[0].Content)
	}
}

// ---------- verify action tests ----------

func TestSkillTool_Handler_VerifyAction(t *testing.T) {
	t.Parallel()

	// Create a real skill file on disk
	dir := t.TempDir()
	skillContent := "---\nname: my-skill\ndescription: \"A test skill. Trigger: do my thing\"\nversion: 1\n---\n# My Skill Body\n\nUse $ARGUMENTS here.\n"
	skillDir := dir + "/my-skill"
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillFile := skillDir + "/SKILL.md"
	if err := os.WriteFile(skillFile, []byte(skillContent), 0o644); err != nil {
		t.Fatal(err)
	}

	lister := &mockSkillLister{
		skills: map[string]tools.SkillInfo{
			"my-skill": {
				Name:        "my-skill",
				Description: "A test skill",
				Source:      "global",
				Verified:    false,
				FilePath:    skillFile,
			},
		},
		bodies: map[string]string{
			"my-skill": "Use $ARGUMENTS here.",
		},
	}
	tool := SkillTool(lister, nil)

	out, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"verify my-skill dennisonbertram"}`))
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}

	if !strings.Contains(out, "my-skill") {
		t.Fatalf("verify output should mention skill name, got: %s", out)
	}
	if !strings.Contains(out, "verified") {
		t.Fatalf("verify output should confirm verification, got: %s", out)
	}

	// Re-read the file and verify it was updated
	data, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatalf("reading updated skill file: %v", err)
	}
	updated := string(data)
	if !strings.Contains(updated, "verified: true") {
		t.Fatalf("skill file should contain 'verified: true', got:\n%s", updated)
	}
	if !strings.Contains(updated, "verified_by: dennisonbertram") {
		t.Fatalf("skill file should contain 'verified_by: dennisonbertram', got:\n%s", updated)
	}
	if !strings.Contains(updated, "verified_at:") {
		t.Fatalf("skill file should contain 'verified_at:', got:\n%s", updated)
	}
}

func TestSkillTool_Handler_VerifyDefaultVerifiedBy(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	skillContent := "---\nname: basic\ndescription: \"Basic skill\"\nversion: 1\n---\nBody.\n"
	skillDir := dir + "/basic"
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillFile := skillDir + "/SKILL.md"
	if err := os.WriteFile(skillFile, []byte(skillContent), 0o644); err != nil {
		t.Fatal(err)
	}

	lister := &mockSkillLister{
		skills: map[string]tools.SkillInfo{
			"basic": {
				Name:     "basic",
				FilePath: skillFile,
			},
		},
		bodies: map[string]string{"basic": "Body."},
	}
	tool := SkillTool(lister, nil)

	// verify without a verified_by arg — should default to "agent"
	out, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"verify basic"}`))
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}

	if !strings.Contains(out, "agent") {
		t.Fatalf("verify output should mention default verifier 'agent', got: %s", out)
	}

	data, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatalf("reading updated skill file: %v", err)
	}
	if !strings.Contains(string(data), "verified_by: agent") {
		t.Fatalf("skill file should contain 'verified_by: agent', got:\n%s", string(data))
	}
}

func TestSkillTool_Handler_VerifyNonexistentSkill(t *testing.T) {
	t.Parallel()
	lister := newVerificationSkillLister()
	tool := SkillTool(lister, nil)

	_, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"verify nonexistent"}`))
	if err == nil {
		t.Fatal("expected error for nonexistent skill")
	}
	if !strings.Contains(err.Error(), "skill not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSkillTool_Handler_VerifyMissingSkillName(t *testing.T) {
	t.Parallel()
	lister := newVerificationSkillLister()
	tool := SkillTool(lister, nil)

	_, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"verify"}`))
	if err == nil {
		t.Fatal("expected error for verify without skill name")
	}
	if !strings.Contains(err.Error(), "verify requires a skill name") {
		t.Fatalf("unexpected error: %v", err)
	}
}
