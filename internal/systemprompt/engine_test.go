package systemprompt

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestResolveComposesPromptSectionsInOrder(t *testing.T) {
	t.Parallel()
	root := makePromptFixture(t)
	engine, err := NewFileEngine(root)
	if err != nil {
		t.Fatalf("new file engine: %v", err)
	}

	out, err := engine.Resolve(ResolveRequest{
		Model:       "gpt-5-nano",
		AgentIntent: "code_review",
		TaskContext: "Review PR #42 for regressions",
		Extensions: Extensions{
			Behaviors: []string{"precise"},
			Talents:   []string{"ui"},
			Custom:    "CUSTOM_TEXT",
		},
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	mustContainOrdered(t, out.StaticPrompt,
		"BASE_PROMPT",
		"INTENT_CODE_REVIEW",
		"MODEL_GPT5",
		"BEHAVIOR_PRECISE",
		"TALENT_UI",
		"CUSTOM_TEXT",
	)
	if !strings.Contains(out.StaticPrompt, "Review PR #42 for regressions") {
		t.Fatalf("expected task context in static prompt: %q", out.StaticPrompt)
	}
	if out.ResolvedIntent != "code_review" {
		t.Fatalf("expected code_review, got %q", out.ResolvedIntent)
	}
	if len(out.Behaviors) != 1 || out.Behaviors[0] != "precise" {
		t.Fatalf("unexpected behavior selection: %+v", out.Behaviors)
	}
}

func TestResolveRejectsUnknownExtensions(t *testing.T) {
	t.Parallel()
	root := makePromptFixture(t)
	engine, err := NewFileEngine(root)
	if err != nil {
		t.Fatalf("new file engine: %v", err)
	}

	_, err = engine.Resolve(ResolveRequest{Model: "gpt-5-nano", Extensions: Extensions{Behaviors: []string{"missing"}}})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestResolveSkillsNoResolverProducesWarning(t *testing.T) {
	t.Parallel()
	root := makePromptFixture(t)
	engine, err := NewFileEngine(root)
	if err != nil {
		t.Fatalf("new file engine: %v", err)
	}

	out, err := engine.Resolve(ResolveRequest{Model: "gpt-5-nano", Extensions: Extensions{Skills: []string{"foo"}}})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(out.Warnings) == 0 {
		t.Fatalf("expected warning when no skill resolver is configured")
	}
	if out.Warnings[0].Code != "skills_no_resolver" {
		t.Fatalf("expected skills_no_resolver warning, got %+v", out.Warnings[0])
	}
}

func TestResolveSkillsWithResolverSuccess(t *testing.T) {
	t.Parallel()
	root := makePromptFixture(t)
	engine, err := NewFileEngine(root)
	if err != nil {
		t.Fatalf("new file engine: %v", err)
	}

	mock := &mockSkillResolver{
		skills: map[string]string{
			"deploy": "SKILL_DEPLOY_CONTENT",
			"test":   "SKILL_TEST_CONTENT",
		},
	}
	engine.SetSkillResolver(mock)

	out, err := engine.Resolve(ResolveRequest{
		Model: "gpt-5-nano",
		Extensions: Extensions{
			Skills: []string{"deploy", "test"},
		},
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	// Skills should appear in the prompt between talents and custom
	if !strings.Contains(out.StaticPrompt, "[SECTION SKILL:deploy]") {
		t.Fatalf("expected SKILL:deploy section header in prompt")
	}
	if !strings.Contains(out.StaticPrompt, "SKILL_DEPLOY_CONTENT") {
		t.Fatalf("expected deploy skill content in prompt")
	}
	if !strings.Contains(out.StaticPrompt, "[SECTION SKILL:test]") {
		t.Fatalf("expected SKILL:test section header in prompt")
	}
	if !strings.Contains(out.StaticPrompt, "SKILL_TEST_CONTENT") {
		t.Fatalf("expected test skill content in prompt")
	}

	// Skills field should be populated
	if len(out.Skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(out.Skills))
	}
	if out.Skills[0] != "deploy" || out.Skills[1] != "test" {
		t.Fatalf("unexpected skills: %+v", out.Skills)
	}

	// No warnings expected
	if len(out.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %+v", out.Warnings)
	}
}

func TestResolveSkillsFailedResolutionProducesWarning(t *testing.T) {
	t.Parallel()
	root := makePromptFixture(t)
	engine, err := NewFileEngine(root)
	if err != nil {
		t.Fatalf("new file engine: %v", err)
	}

	mock := &mockSkillResolver{
		skills: map[string]string{
			"deploy": "SKILL_DEPLOY_CONTENT",
		},
	}
	engine.SetSkillResolver(mock)

	out, err := engine.Resolve(ResolveRequest{
		Model: "gpt-5-nano",
		Extensions: Extensions{
			Skills: []string{"deploy", "nonexistent"},
		},
	})
	if err != nil {
		t.Fatalf("resolve should not fail entirely: %v", err)
	}

	// The successful skill should still be in the prompt
	if !strings.Contains(out.StaticPrompt, "SKILL_DEPLOY_CONTENT") {
		t.Fatalf("expected deploy skill content in prompt")
	}

	// Only the successful skill should be in the Skills list
	if len(out.Skills) != 1 || out.Skills[0] != "deploy" {
		t.Fatalf("expected only deploy in skills list, got %+v", out.Skills)
	}

	// Should have a warning for the failed skill
	if len(out.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %+v", len(out.Warnings), out.Warnings)
	}
	if out.Warnings[0].Code != "skill_resolve_failed" {
		t.Fatalf("expected skill_resolve_failed warning, got %+v", out.Warnings[0])
	}
	if !strings.Contains(out.Warnings[0].Message, "nonexistent") {
		t.Fatalf("warning should mention the failed skill name: %s", out.Warnings[0].Message)
	}
}

func TestResolveSkillsSectionOrderBetweenTalentsAndCustom(t *testing.T) {
	t.Parallel()
	root := makePromptFixture(t)
	engine, err := NewFileEngine(root)
	if err != nil {
		t.Fatalf("new file engine: %v", err)
	}

	mock := &mockSkillResolver{
		skills: map[string]string{"deploy": "SKILL_DEPLOY_CONTENT"},
	}
	engine.SetSkillResolver(mock)

	out, err := engine.Resolve(ResolveRequest{
		Model: "gpt-5-nano",
		Extensions: Extensions{
			Talents: []string{"ui"},
			Skills:  []string{"deploy"},
			Custom:  "CUSTOM_TEXT",
		},
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	// Verify ordering: TALENT section before SKILL section before CUSTOM section
	mustContainOrdered(t, out.StaticPrompt,
		"[SECTION TALENT:ui]",
		"TALENT_UI",
		"[SECTION SKILL:deploy]",
		"SKILL_DEPLOY_CONTENT",
		"[SECTION CUSTOM]",
		"CUSTOM_TEXT",
	)
}

func TestResolveSkillsEmptyNameSkipped(t *testing.T) {
	t.Parallel()
	root := makePromptFixture(t)
	engine, err := NewFileEngine(root)
	if err != nil {
		t.Fatalf("new file engine: %v", err)
	}

	mock := &mockSkillResolver{
		skills: map[string]string{"deploy": "SKILL_DEPLOY_CONTENT"},
	}
	engine.SetSkillResolver(mock)

	out, err := engine.Resolve(ResolveRequest{
		Model: "gpt-5-nano",
		Extensions: Extensions{
			Skills: []string{"", "  ", "deploy"},
		},
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	// Only deploy should be resolved
	if len(out.Skills) != 1 || out.Skills[0] != "deploy" {
		t.Fatalf("expected only deploy, got %+v", out.Skills)
	}
	if len(out.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %+v", out.Warnings)
	}
}

func TestResolveSkillsNoSkillsRequestedNoWarning(t *testing.T) {
	t.Parallel()
	root := makePromptFixture(t)
	engine, err := NewFileEngine(root)
	if err != nil {
		t.Fatalf("new file engine: %v", err)
	}

	out, err := engine.Resolve(ResolveRequest{Model: "gpt-5-nano"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if len(out.Warnings) != 0 {
		t.Fatalf("no warnings expected when no skills requested: %+v", out.Warnings)
	}
	if out.Skills != nil {
		t.Fatalf("expected nil skills, got %+v", out.Skills)
	}
}

type mockSkillResolver struct {
	skills map[string]string
}

func (m *mockSkillResolver) ResolveSkill(_ context.Context, name, args, workspace string) (string, error) {
	content, ok := m.skills[name]
	if !ok {
		return "", fmt.Errorf("skill not found: %s", name)
	}
	return content, nil
}

func TestRuntimeContextUsesFixedFormat(t *testing.T) {
	t.Parallel()
	root := makePromptFixture(t)
	engine, err := NewFileEngine(root)
	if err != nil {
		t.Fatalf("new file engine: %v", err)
	}

	startedAt := time.Date(2026, 3, 5, 15, 0, 0, 0, time.UTC)
	now := startedAt.Add(130 * time.Second)
	ctx := engine.RuntimeContext(RuntimeContextInput{RunStartedAt: startedAt, Now: now, Step: 2})
	mustContainOrdered(t, ctx,
		"<runtime_context>",
		"run_started_at_utc: 2026-03-05T15:00:00Z",
		"current_time_utc: 2026-03-05T15:02:10Z",
		"elapsed_seconds: 130",
		"step: 2",
		"prompt_tokens_total: 0",
		"completion_tokens_total: 0",
		"total_tokens: 0",
		"last_turn_tokens: 0",
		"cost_usd_total: 0.000000",
		"last_turn_cost_usd: 0.000000",
		"cost_status: pending",
		"</runtime_context>",
	)
}

func mustContainOrdered(t *testing.T, text string, parts ...string) {
	t.Helper()
	pos := 0
	for _, part := range parts {
		i := strings.Index(text[pos:], part)
		if i < 0 {
			t.Fatalf("missing part %q in text: %q", part, text)
		}
		pos += i + len(part)
	}
}
