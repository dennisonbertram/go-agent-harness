package harness

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"

	htools "go-agent-harness/internal/harness/tools"
)

func registerDeferredTaskComplete(t *testing.T, registry *Registry) {
	t.Helper()
	err := registry.RegisterWithOptions(ToolDefinition{Name: "task_complete", Description: "finish child", Parameters: map[string]any{"type": "object"}}, func(ctx context.Context, raw json.RawMessage) (string, error) {
		if htools.ForkDepthFromContext(ctx) == 0 {
			return "", errTaskCompleteRoot
		}
		var args struct {
			Summary string `json:"summary"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", err
		}
		return mustJSON(map[string]any{"_task_complete": true, "status": "completed", "summary": args.Summary, "findings": []any{}}), nil
	}, RegisterOptions{Tier: htools.TierDeferred})
	if err != nil {
		t.Fatalf("register task_complete: %v", err)
	}
}

var errTaskCompleteRoot = &taskCompleteRootError{}

type taskCompleteRootError struct{}

func (*taskCompleteRootError) Error() string { return "task_complete is only available to subagents" }

func TestRunForkedSkill_MandatoryTaskCompleteActivatesAndTerminatesWithoutAnotherProviderTurn(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()
	registerDeferredTaskComplete(t, registry)
	provider := &capturingProvider{turns: []CompletionResult{{ToolCalls: []ToolCall{{ID: "complete", Name: "task_complete", Arguments: `{"summary":"child finished"}`}}}, {Content: "must not be requested"}}}
	activations := NewActivationTracker()
	runner := NewRunner(provider, registry, RunnerConfig{DefaultModel: "gpt-4.1-mini", MaxSteps: 3, Activations: activations})

	result, err := runner.RunForkedSkill(htools.WithForkDepth(context.Background(), 1), htools.ForkConfig{Prompt: "finish", AllowedTools: []string{"read_file"}})
	if err != nil {
		t.Fatalf("RunForkedSkill: %v", err)
	}
	if result.Output == "" || !containsTaskCompleteSentinel(result.Output) {
		t.Fatalf("child output = %q, want validated task_complete payload", result.Output)
	}
	if len(provider.calls) != 1 {
		t.Fatalf("provider calls = %d, want exactly 1", len(provider.calls))
	}
	if !toolDefinitionsContain(provider.calls[0].Tools, "task_complete") {
		t.Fatalf("child tools = %v, want mandatory task_complete", toolDefinitionNames(provider.calls[0].Tools))
	}
	if len(activations.ActiveTools(singleRunID(t, runner))) != 0 {
		t.Fatal("terminal child activation state was not cleaned up")
	}
}

func TestRootTaskCompleteRemainsUnavailable(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()
	registerDeferredTaskComplete(t, registry)
	provider := &capturingProvider{turns: []CompletionResult{{ToolCalls: []ToolCall{{ID: "root-complete", Name: "task_complete", Arguments: `{"summary":"no"}`}}}, {Content: "root final"}}}
	runner := NewRunner(provider, registry, RunnerConfig{DefaultModel: "gpt-4.1-mini", MaxSteps: 2})
	run, err := runner.StartRun(RunRequest{Prompt: "root", AllowedTools: []string{"task_complete"}})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	_, err = collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if toolDefinitionsContain(provider.calls[0].Tools, "task_complete") {
		t.Fatal("root was offered task_complete")
	}
}

func TestTaskCompleteMixedWithSiblingRejectsBeforeSiblingMutation(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()
	registerDeferredTaskComplete(t, registry)
	var mutations atomic.Int32
	if err := registry.Register(ToolDefinition{Name: "mutate", Description: "mutates", Parameters: map[string]any{"type": "object"}, Mutating: true}, func(context.Context, json.RawMessage) (string, error) { mutations.Add(1); return "{}", nil }); err != nil {
		t.Fatal(err)
	}
	provider := &capturingProvider{turns: []CompletionResult{{ToolCalls: []ToolCall{{ID: "complete", Name: "task_complete", Arguments: `{"summary":"child finished"}`}, {ID: "mutate", Name: "mutate", Arguments: `{}`}}}, {Content: "repair"}}}
	runner := NewRunner(provider, registry, RunnerConfig{DefaultModel: "gpt-4.1-mini", MaxSteps: 2})
	_, err := runner.RunForkedSkill(htools.WithForkDepth(context.Background(), 1), htools.ForkConfig{Prompt: "finish", AllowedTools: []string{"mutate"}})
	if err != nil {
		t.Fatalf("RunForkedSkill: %v", err)
	}
	if got := mutations.Load(); got != 0 {
		t.Fatalf("sibling mutation calls = %d, want 0", got)
	}
}

func TestTaskCompleteMalformedOutputDoesNotTerminateChild(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()
	if err := registry.RegisterWithOptions(ToolDefinition{Name: "task_complete", Description: "finish child", Parameters: map[string]any{"type": "object"}}, func(context.Context, json.RawMessage) (string, error) {
		return `{"_task_complete":true,"status":"completed"}`, nil // missing required summary
	}, RegisterOptions{Tier: htools.TierDeferred}); err != nil {
		t.Fatal(err)
	}
	provider := &capturingProvider{turns: []CompletionResult{{ToolCalls: []ToolCall{{ID: "bad", Name: "task_complete", Arguments: `{}`}}}, {Content: "child continued"}}}
	runner := NewRunner(provider, registry, RunnerConfig{DefaultModel: "gpt-4.1-mini", MaxSteps: 2})
	result, err := runner.RunForkedSkill(htools.WithForkDepth(context.Background(), 1), htools.ForkConfig{Prompt: "finish"})
	if err != nil {
		t.Fatalf("RunForkedSkill: %v", err)
	}
	if result.Output != "child continued" {
		t.Fatalf("output = %q, want child continuation", result.Output)
	}
	if len(provider.calls) != 2 {
		t.Fatalf("provider calls = %d, want 2", len(provider.calls))
	}
}

func containsTaskCompleteSentinel(output string) bool {
	var v struct {
		Complete bool `json:"_task_complete"`
	}
	return json.Unmarshal([]byte(output), &v) == nil && v.Complete
}
func toolDefinitionsContain(defs []ToolDefinition, name string) bool {
	for _, def := range defs {
		if def.Name == name {
			return true
		}
	}
	return false
}
func toolDefinitionNames(defs []ToolDefinition) []string {
	names := make([]string, 0, len(defs))
	for _, def := range defs {
		names = append(names, def.Name)
	}
	return names
}
