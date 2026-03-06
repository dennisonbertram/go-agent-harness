package systemprompt

import (
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

func TestResolveAddsWarningForReservedSkillsField(t *testing.T) {
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
		t.Fatalf("expected warning for reserved skills field")
	}
	if out.Warnings[0].Code != "skills_reserved_noop" {
		t.Fatalf("unexpected warning code: %+v", out.Warnings[0])
	}
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
