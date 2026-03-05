package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"go-agent-harness/internal/systemprompt"
)

type promptEngineStub struct {
	resolveCalls int
	resolveReqs  []systemprompt.ResolveRequest
	resolved     systemprompt.ResolvedPrompt
	resolveErr   error

	runtimeCalls []systemprompt.RuntimeContextInput
}

func (p *promptEngineStub) Resolve(req systemprompt.ResolveRequest) (systemprompt.ResolvedPrompt, error) {
	p.resolveCalls++
	p.resolveReqs = append(p.resolveReqs, req)
	if p.resolveErr != nil {
		return systemprompt.ResolvedPrompt{}, p.resolveErr
	}
	return p.resolved, nil
}

func (p *promptEngineStub) RuntimeContext(in systemprompt.RuntimeContextInput) string {
	p.runtimeCalls = append(p.runtimeCalls, in)
	return fmt.Sprintf("runtime-step-%d", in.Step)
}

func TestRunnerSystemPromptOverrideBypassesPromptEngine(t *testing.T) {
	t.Parallel()

	provider := &capturingProvider{turns: []CompletionResult{{Content: "done"}}}
	engine := &promptEngineStub{resolved: systemprompt.ResolvedPrompt{StaticPrompt: "SHOULD_NOT_USE"}}
	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel:       "gpt-5-nano",
		MaxSteps:           2,
		PromptEngine:       engine,
		DefaultAgentIntent: "general",
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello", SystemPrompt: "EXPLICIT_SYSTEM"})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	_, err = collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect events: %v", err)
	}

	if engine.resolveCalls != 0 {
		t.Fatalf("expected prompt engine resolve not called, got %d", engine.resolveCalls)
	}
	if len(engine.runtimeCalls) != 0 {
		t.Fatalf("expected prompt engine runtime not called, got %d", len(engine.runtimeCalls))
	}
	if len(provider.calls) != 1 {
		t.Fatalf("expected one provider call, got %d", len(provider.calls))
	}
	if len(provider.calls[0].Messages) < 2 {
		t.Fatalf("expected at least two messages, got %+v", provider.calls[0].Messages)
	}
	if provider.calls[0].Messages[0].Role != "system" || provider.calls[0].Messages[0].Content != "EXPLICIT_SYSTEM" {
		t.Fatalf("expected explicit system prompt first, got %+v", provider.calls[0].Messages)
	}
}

func TestRunnerBuildsEphemeralRuntimeContextPerTurn(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	if err := registry.Register(ToolDefinition{
		Name:        "nop",
		Description: "no-op",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		return `{}`, nil
	}); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	provider := &capturingProvider{turns: []CompletionResult{
		{ToolCalls: []ToolCall{{ID: "c1", Name: "nop", Arguments: `{}`}}},
		{Content: "done"},
	}}
	engine := &promptEngineStub{resolved: systemprompt.ResolvedPrompt{
		StaticPrompt:         "STATIC_PROMPT",
		ResolvedIntent:       "general",
		ResolvedModelProfile: "default",
	}}
	runner := NewRunner(provider, registry, RunnerConfig{
		DefaultModel:       "gpt-5-nano",
		DefaultAgentIntent: "general",
		PromptEngine:       engine,
		MaxSteps:           4,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	_, err = collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect events: %v", err)
	}

	if len(provider.calls) != 2 {
		t.Fatalf("expected 2 provider calls, got %d", len(provider.calls))
	}
	if len(engine.runtimeCalls) != 2 {
		t.Fatalf("expected runtime context called twice, got %d", len(engine.runtimeCalls))
	}
	if !engine.runtimeCalls[0].RunStartedAt.Equal(engine.runtimeCalls[1].RunStartedAt) {
		t.Fatalf("expected run_started_at to be stable across turns")
	}

	first := provider.calls[0].Messages
	second := provider.calls[1].Messages
	if !containsMessage(first, "system", "runtime-step-1") {
		t.Fatalf("first turn missing runtime step 1 message: %+v", first)
	}
	if !containsMessage(second, "system", "runtime-step-2") {
		t.Fatalf("second turn missing runtime step 2 message: %+v", second)
	}
	if containsMessage(second, "system", "runtime-step-1") {
		t.Fatalf("runtime step 1 should not persist into second turn: %+v", second)
	}
}

func TestRunnerEmitsPromptResolvedWithModelFallback(t *testing.T) {
	t.Parallel()
	root := makeRunnerPromptFixture(t)
	engine, err := systemprompt.NewFileEngine(root)
	if err != nil {
		t.Fatalf("new file engine: %v", err)
	}

	provider := &stubProvider{turns: []CompletionResult{{Content: "done"}}}
	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel:       "unknown-model",
		DefaultAgentIntent: "general",
		PromptEngine:       engine,
		MaxSteps:           2,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello", Model: "unmatched"})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect events: %v", err)
	}

	var found bool
	for _, ev := range events {
		if ev.Type != "prompt.resolved" {
			continue
		}
		found = true
		fallback, _ := ev.Payload["model_fallback"].(bool)
		if !fallback {
			t.Fatalf("expected model_fallback true in prompt.resolved payload: %+v", ev.Payload)
		}
	}
	if !found {
		t.Fatalf("expected prompt.resolved event in %+v", eventTypes(events))
	}
}

func containsMessage(messages []Message, role, content string) bool {
	for _, msg := range messages {
		if msg.Role == role && msg.Content == content {
			return true
		}
	}
	return false
}

func makeRunnerPromptFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	write("catalog.yaml", `version: 1
defaults:
  intent: general
  model_profile: default
intents:
  general: intents/general.md
model_profiles:
  - name: openai_gpt5
    match: gpt-5-*
    file: models/openai_gpt5.md
  - name: default
    match: "*"
    file: models/default.md
extensions:
  behaviors_dir: extensions/behaviors
  talents_dir: extensions/talents
`)
	write("base/main.md", "BASE")
	write("intents/general.md", "INTENT")
	write("models/default.md", "MODEL_DEFAULT")
	write("models/openai_gpt5.md", "MODEL_GPT5")
	write("extensions/behaviors/.keep", "")
	write("extensions/talents/.keep", "")
	return root
}
