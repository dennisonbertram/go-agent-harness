package deferred

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	tools "go-agent-harness/internal/harness/tools"
)

// --- mock runners ---

type mockSpawnRunner struct {
	output string
	err    error
}

func (m *mockSpawnRunner) RunPrompt(ctx context.Context, prompt string) (string, error) {
	return m.output, m.err
}

type mockSpawnForkedRunner struct {
	output    tools.ForkResult
	err       error
	lastConfig tools.ForkConfig
	lastCtx   context.Context
}

func (m *mockSpawnForkedRunner) RunPrompt(ctx context.Context, prompt string) (string, error) {
	return m.output.Output, m.err
}

func (m *mockSpawnForkedRunner) RunForkedSkill(ctx context.Context, config tools.ForkConfig) (tools.ForkResult, error) {
	m.lastConfig = config
	m.lastCtx = ctx
	return m.output, m.err
}

// --- SpawnAgentTool tests ---

func TestSpawnAgentTool_Definition(t *testing.T) {
	t.Parallel()
	tool := SpawnAgentTool(nil)

	if tool.Definition.Name != "spawn_agent" {
		t.Fatalf("expected name=spawn_agent, got %s", tool.Definition.Name)
	}
	if tool.Definition.Tier != tools.TierDeferred {
		t.Fatalf("expected tier=deferred, got %s", tool.Definition.Tier)
	}
	if tool.Definition.Action != tools.ActionExecute {
		t.Fatalf("expected action=execute, got %s", tool.Definition.Action)
	}
	if !tool.Definition.Mutating {
		t.Fatal("expected mutating=true")
	}
	if tool.Handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestSpawnAgentTool_RequiresTask(t *testing.T) {
	t.Parallel()
	tool := SpawnAgentTool(&mockSpawnRunner{output: "done"})

	_, err := tool.Handler(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for missing task")
	}
	if !strings.Contains(err.Error(), "task is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSpawnAgentTool_EmptyTask(t *testing.T) {
	t.Parallel()
	tool := SpawnAgentTool(&mockSpawnRunner{output: "done"})

	_, err := tool.Handler(context.Background(), json.RawMessage(`{"task":""}`))
	if err == nil {
		t.Fatal("expected error for empty task")
	}
	if !strings.Contains(err.Error(), "task is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSpawnAgentTool_NilRunner(t *testing.T) {
	t.Parallel()
	tool := SpawnAgentTool(nil)

	_, err := tool.Handler(context.Background(), json.RawMessage(`{"task":"do something"}`))
	if err == nil {
		t.Fatal("expected error for nil runner")
	}
	if !strings.Contains(err.Error(), "no AgentRunner configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSpawnAgentTool_EnforcesDepthLimit(t *testing.T) {
	t.Parallel()
	runner := &mockSpawnForkedRunner{
		output: tools.ForkResult{Output: "done"},
	}
	tool := SpawnAgentTool(runner)

	// Simulate being at max depth.
	ctx := tools.WithForkDepth(context.Background(), tools.DefaultMaxForkDepth)

	_, err := tool.Handler(ctx, json.RawMessage(`{"task":"do something"}`))
	if err == nil {
		t.Fatal("expected error at max fork depth")
	}
	if !strings.Contains(err.Error(), "max recursion depth") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSpawnAgentTool_SuccessWithForkedRunner(t *testing.T) {
	t.Parallel()
	runner := &mockSpawnForkedRunner{
		output: tools.ForkResult{
			Output:  "Child completed successfully",
			Summary: "Done",
		},
	}
	tool := SpawnAgentTool(runner)

	// Depth 0 → child gets depth 1.
	out, err := tool.Handler(context.Background(), json.RawMessage(`{"task":"implement the feature"}`))
	if err != nil {
		t.Fatalf("spawn_agent failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result["status"] != "completed" {
		t.Fatalf("expected status=completed, got %v", result["status"])
	}
	if result["summary"] == nil {
		t.Fatal("expected summary in result")
	}
	if result["jsonl"] == nil {
		t.Fatal("expected jsonl in result")
	}
}

func TestSpawnAgentTool_PropagatesDepthToChild(t *testing.T) {
	t.Parallel()
	runner := &mockSpawnForkedRunner{
		output: tools.ForkResult{Output: "done"},
	}
	tool := SpawnAgentTool(runner)

	// Spawn from depth 2 → child should be at depth 3.
	ctx := tools.WithForkDepth(context.Background(), 2)
	_, err := tool.Handler(ctx, json.RawMessage(`{"task":"do something"}`))
	if err != nil {
		t.Fatalf("spawn_agent failed: %v", err)
	}

	// The child context should have depth 3.
	childDepth := tools.ForkDepthFromContext(runner.lastCtx)
	if childDepth != 3 {
		t.Fatalf("expected child depth 3, got %d", childDepth)
	}
}

func TestSpawnAgentTool_ChildRunnerError(t *testing.T) {
	t.Parallel()
	import_err := "child run timed out"
	runner := &mockSpawnForkedRunner{}
	runner.err = makeError(import_err)
	tool := SpawnAgentTool(runner)

	_, err := tool.Handler(context.Background(), json.RawMessage(`{"task":"do something"}`))
	if err == nil {
		t.Fatal("expected error when child run fails")
	}
	if !strings.Contains(err.Error(), "child run failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSpawnAgentTool_WithStructuredTaskCompleteResult(t *testing.T) {
	t.Parallel()
	// Simulate child that called task_complete and returned structured JSON.
	taskCompleteOutput := `{"_task_complete":true,"status":"completed","summary":"Auth module done","findings":[{"type":"test_result","content":"14 passed"}]}`
	runner := &mockSpawnForkedRunner{
		output: tools.ForkResult{Output: taskCompleteOutput},
	}
	tool := SpawnAgentTool(runner)

	out, err := tool.Handler(context.Background(), json.RawMessage(`{"task":"implement auth"}`))
	if err != nil {
		t.Fatalf("spawn_agent failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result["status"] != "completed" {
		t.Fatalf("expected status=completed, got %v", result["status"])
	}
	if result["summary"] != "Auth module done" {
		t.Fatalf("expected summary='Auth module done', got %v", result["summary"])
	}
	jsonl, ok := result["jsonl"].([]any)
	if !ok {
		t.Fatalf("expected jsonl to be array, got %T", result["jsonl"])
	}
	if len(jsonl) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(jsonl))
	}
}

func TestSpawnAgentTool_DefaultMaxSteps(t *testing.T) {
	t.Parallel()
	runner := &mockSpawnForkedRunner{
		output: tools.ForkResult{Output: "done"},
	}
	tool := SpawnAgentTool(runner)

	// No max_steps specified → should default to 30.
	out, err := tool.Handler(context.Background(), json.RawMessage(`{"task":"do something"}`))
	if err != nil {
		t.Fatalf("spawn_agent failed: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty output")
	}
}

func TestSpawnAgentTool_AllowedToolsForwarded(t *testing.T) {
	t.Parallel()
	runner := &mockSpawnForkedRunner{
		output: tools.ForkResult{Output: "done"},
	}
	tool := SpawnAgentTool(runner)

	_, err := tool.Handler(context.Background(), json.RawMessage(`{"task":"do something","allowed_tools":["bash","read"]}`))
	if err != nil {
		t.Fatalf("spawn_agent failed: %v", err)
	}

	if len(runner.lastConfig.AllowedTools) != 2 {
		t.Fatalf("expected 2 allowed tools, got %d: %v", len(runner.lastConfig.AllowedTools), runner.lastConfig.AllowedTools)
	}
}

// makeError creates a simple error for testing.
func makeError(msg string) error {
	return fmt.Errorf("%s", msg)
}
