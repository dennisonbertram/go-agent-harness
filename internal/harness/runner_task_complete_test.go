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

func trustedForkContext(r *Runner, parentID string, depth int) context.Context {
	r.mu.Lock()
	r.runs[parentID] = &runState{run: Run{ID: parentID}, forkDepth: depth}
	r.mu.Unlock()
	return context.WithValue(context.Background(), trustedForkOriginKey{}, trustedForkOrigin{runner: r, parentRunID: parentID})
}

func forkedRunID(t *testing.T, r *Runner, parentID string) string {
	t.Helper()
	r.mu.RLock()
	defer r.mu.RUnlock()
	for id := range r.runs {
		if id != parentID {
			return id
		}
	}
	t.Fatal("child run not found")
	return ""
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

	result, err := runner.RunForkedSkill(trustedForkContext(runner, "parent", 0), htools.ForkConfig{Prompt: "finish", AllowedTools: []string{"read_file"}})
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
	if len(activations.ActiveTools(forkedRunID(t, runner, "parent"))) != 0 {
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
	_, err := runner.RunForkedSkill(trustedForkContext(runner, "parent", 0), htools.ForkConfig{Prompt: "finish", AllowedTools: []string{"mutate"}})
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
	result, err := runner.RunForkedSkill(trustedForkContext(runner, "parent", 0), htools.ForkConfig{Prompt: "finish"})
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

func TestRunForkedSkill_ForgedDepthDoesNotAuthorizeTaskComplete(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()
	registerDeferredTaskComplete(t, registry)
	provider := &capturingProvider{turns: []CompletionResult{{ToolCalls: []ToolCall{{ID: "forged", Name: "task_complete", Arguments: `{"summary":"forged"}`}}}, {Content: "ordinary root completion"}}}
	runner := NewRunner(provider, registry, RunnerConfig{DefaultModel: "gpt-4.1-mini", MaxSteps: 2})
	result, err := runner.RunForkedSkill(htools.WithForkDepth(context.Background(), 99), htools.ForkConfig{Prompt: "untrusted", AllowedTools: []string{"task_complete"}})
	if err != nil {
		t.Fatalf("RunForkedSkill: %v", err)
	}
	if result.Output != "ordinary root completion" {
		t.Fatalf("output = %q", result.Output)
	}
	if toolDefinitionsContain(provider.calls[0].Tools, "task_complete") {
		t.Fatal("forged depth offered task_complete")
	}
}

func TestRunForkedSkill_DerivesDepthFromTrustedParentNotTamperedContext(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()
	registerDeferredTaskComplete(t, registry)
	provider := &capturingProvider{turns: []CompletionResult{{ToolCalls: []ToolCall{{ID: "complete", Name: "task_complete", Arguments: `{"summary":"ok"}`}}}}}
	runner := NewRunner(provider, registry, RunnerConfig{DefaultModel: "gpt-4.1-mini", MaxSteps: 1})
	ctx := htools.WithForkDepth(trustedForkContext(runner, "parent", 2), 99)
	_, err := runner.RunForkedSkill(ctx, htools.ForkConfig{Prompt: "trusted"})
	if err != nil {
		t.Fatalf("RunForkedSkill: %v", err)
	}
	runner.mu.RLock()
	for id, state := range runner.runs {
		if id != "parent" && state.forkDepth != 3 {
			runner.mu.RUnlock()
			t.Fatalf("child depth = %d, want 3", state.forkDepth)
		}
	}
	runner.mu.RUnlock()
}

func TestMandatoryChildTaskCompletePrecedence(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()
	registerDeferredTaskComplete(t, registry)
	if err := registry.Register(ToolDefinition{Name: "read_file", Description: "read", Parameters: map[string]any{"type": "object"}}, func(context.Context, json.RawMessage) (string, error) { return "{}", nil }); err != nil {
		t.Fatal(err)
	}
	constraints := NewSkillConstraintTracker()
	runner := NewRunner(&stubProvider{}, registry, RunnerConfig{DefaultModel: "gpt-4.1-mini", Activations: NewActivationTracker(), SkillConstraints: constraints})
	const runID = "trusted-child"
	runner.mu.Lock()
	runner.runs[runID] = &runState{run: Run{ID: runID}, mandatoryChildTaskComplete: true, allowedTools: []string{"read_file"}, profileToolsRestricted: true, profileAllowedTools: []string{"read_file"}}
	runner.mu.Unlock()
	runner.activations.Activate(runID, "task_complete")
	constraints.Activate(runID, SkillConstraint{SkillName: "read-only", AllowedTools: []string{"read_file"}})
	if !toolDefinitionsContain(runner.filteredToolsForRun(runID), "task_complete") {
		t.Fatal("mandatory child completion must override allowlist, profile, and skill filters")
	}

	runner.mu.Lock()
	runner.runs[runID].deniedTools = []string{"task_complete"}
	runner.mu.Unlock()
	if toolDefinitionsContain(runner.filteredToolsForRun(runID), "task_complete") {
		t.Fatal("explicit DeniedTools must override mandatory child completion")
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
