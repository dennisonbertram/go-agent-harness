package recipe_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"go-agent-harness/internal/harness/tools/recipe"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeFile %s: %v", name, err)
	}
	return path
}

// mockTool returns a handler that echoes its JSON args as a string.
func mockTool(name string) func(ctx context.Context, args json.RawMessage) (string, error) {
	return func(ctx context.Context, args json.RawMessage) (string, error) {
		return fmt.Sprintf("tool=%s args=%s", name, string(args)), nil
	}
}

// errorTool always returns an error.
func errorTool(msg string) func(ctx context.Context, args json.RawMessage) (string, error) {
	return func(ctx context.Context, args json.RawMessage) (string, error) {
		return "", fmt.Errorf("%s", msg)
	}
}

// ---------------------------------------------------------------------------
// Recipe loading tests
// ---------------------------------------------------------------------------

func TestLoadRecipe_HappyPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "greet.yaml", `
name: greet
description: "Say hello"
parameters:
  name:
    type: string
    description: "Name to greet"
steps:
  - name: say_hello
    tool: bash
    args:
      command: "echo hello {{name}}"
`)
	r, err := recipe.LoadRecipe(filepath.Join(dir, "greet.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Name != "greet" {
		t.Errorf("expected name 'greet', got %q", r.Name)
	}
	if r.Description != "Say hello" {
		t.Errorf("expected description 'Say hello', got %q", r.Description)
	}
	if len(r.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(r.Steps))
	}
	if r.Steps[0].Tool != "bash" {
		t.Errorf("expected step tool 'bash', got %q", r.Steps[0].Tool)
	}
}

func TestLoadRecipe_MultipleSteps(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "multi.yaml", `
name: multi
description: "Multiple steps"
steps:
  - name: step1
    tool: read
    args:
      path: "/tmp/foo"
  - name: step2
    tool: write
    args:
      path: "/tmp/bar"
      content: "hello"
  - name: step3
    tool: bash
    args:
      command: "echo done"
`)
	r, err := recipe.LoadRecipe(filepath.Join(dir, "multi.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.Steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(r.Steps))
	}
}

func TestLoadRecipe_MissingName(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "bad.yaml", `
description: "No name"
steps:
  - name: s1
    tool: bash
    args:
      command: "echo hi"
`)
	_, err := recipe.LoadRecipe(filepath.Join(dir, "bad.yaml"))
	if err == nil {
		t.Error("expected error for missing name, got nil")
	}
}

func TestLoadRecipe_MissingDescription(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "bad.yaml", `
name: nodesc
steps:
  - name: s1
    tool: bash
    args:
      command: "echo hi"
`)
	_, err := recipe.LoadRecipe(filepath.Join(dir, "bad.yaml"))
	if err == nil {
		t.Error("expected error for missing description, got nil")
	}
}

func TestLoadRecipe_EmptySteps(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "empty.yaml", `
name: empty
description: "No steps"
steps: []
`)
	_, err := recipe.LoadRecipe(filepath.Join(dir, "empty.yaml"))
	if err == nil {
		t.Error("expected error for empty steps, got nil")
	}
}

func TestLoadRecipe_MissingFile(t *testing.T) {
	_, err := recipe.LoadRecipe("/tmp/nonexistent-recipe-xyz.yaml")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestLoadRecipe_MalformedYAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "bad.yaml", `
name: [not a string
description: "broken
`)
	_, err := recipe.LoadRecipe(filepath.Join(dir, "bad.yaml"))
	if err == nil {
		t.Error("expected error for malformed YAML, got nil")
	}
}

func TestLoadRecipe_StepMissingTool(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "bad.yaml", `
name: badstep
description: "Step missing tool"
steps:
  - name: s1
    args:
      command: "echo hi"
`)
	_, err := recipe.LoadRecipe(filepath.Join(dir, "bad.yaml"))
	if err == nil {
		t.Error("expected error for step missing tool, got nil")
	}
}

func TestLoadRecipes_Directory(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.yaml", `
name: alpha
description: "Alpha recipe"
steps:
  - name: s1
    tool: bash
    args:
      command: "echo alpha"
`)
	writeFile(t, dir, "b.yaml", `
name: beta
description: "Beta recipe"
steps:
  - name: s1
    tool: bash
    args:
      command: "echo beta"
`)
	writeFile(t, dir, "not-a-recipe.txt", "ignored")

	recipes, err := recipe.LoadRecipes(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recipes) != 2 {
		t.Errorf("expected 2 recipes, got %d", len(recipes))
	}
}

func TestLoadRecipes_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	recipes, err := recipe.LoadRecipes(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recipes) != 0 {
		t.Errorf("expected 0 recipes, got %d", len(recipes))
	}
}

func TestLoadRecipes_MissingDirectory(t *testing.T) {
	recipes, err := recipe.LoadRecipes("/tmp/nonexistent-recipes-dir-xyz")
	if err != nil {
		t.Fatalf("missing dir should not error, got: %v", err)
	}
	if len(recipes) != 0 {
		t.Errorf("expected 0 recipes for missing dir, got %d", len(recipes))
	}
}

func TestLoadRecipes_SkipsBadRecipes(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "good.yaml", `
name: good
description: "Good recipe"
steps:
  - name: s1
    tool: bash
    args:
      command: "echo good"
`)
	// A bad recipe file causes an error — directory loading returns error on parse failure
	writeFile(t, dir, "bad.yaml", `
name: [broken
`)
	_, err := recipe.LoadRecipes(dir)
	// Bad files should cause errors (to prevent silent misconfigurations)
	if err == nil {
		t.Error("expected error when a recipe file is malformed, got nil")
	}
}

// ---------------------------------------------------------------------------
// Template substitution tests
// ---------------------------------------------------------------------------

func TestSubstitute_BasicParam(t *testing.T) {
	vars := map[string]string{
		"name": "World",
	}
	result := recipe.Substitute("Hello {{name}}!", vars)
	if result != "Hello World!" {
		t.Errorf("expected 'Hello World!', got %q", result)
	}
}

func TestSubstitute_MultipleParams(t *testing.T) {
	vars := map[string]string{
		"a": "foo",
		"b": "bar",
	}
	result := recipe.Substitute("{{a}} and {{b}}", vars)
	if result != "foo and bar" {
		t.Errorf("expected 'foo and bar', got %q", result)
	}
}

func TestSubstitute_MissingParam(t *testing.T) {
	vars := map[string]string{}
	result := recipe.Substitute("Hello {{name}}!", vars)
	// Missing params are replaced with empty string
	if result != "Hello !" {
		t.Errorf("expected 'Hello !', got %q", result)
	}
}

func TestSubstitute_NoPlaceholders(t *testing.T) {
	vars := map[string]string{"x": "y"}
	result := recipe.Substitute("no placeholders here", vars)
	if result != "no placeholders here" {
		t.Errorf("expected unchanged string, got %q", result)
	}
}

func TestSubstitute_EmptyString(t *testing.T) {
	result := recipe.Substitute("", map[string]string{"x": "y"})
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestSubstitute_CapturedOutputNotReEvaluated(t *testing.T) {
	// Template injection: captured output containing {{ should not be re-evaluated
	vars := map[string]string{
		"output": "{{injected}}",
	}
	result := recipe.Substitute("Result: {{output}}", vars)
	// The substituted value should be literal "{{injected}}", not further evaluated
	if result != "Result: {{injected}}" {
		t.Errorf("expected literal '{{injected}}' in result, got %q", result)
	}
}

// ---------------------------------------------------------------------------
// Step args substitution tests
// ---------------------------------------------------------------------------

func TestSubstituteArgs_StringValues(t *testing.T) {
	args := map[string]any{
		"command": "echo {{message}}",
		"path":    "/tmp/{{filename}}",
	}
	vars := map[string]string{
		"message":  "hello world",
		"filename": "output.txt",
	}
	result := recipe.SubstituteArgs(args, vars)
	if result["command"] != "echo hello world" {
		t.Errorf("expected 'echo hello world', got %q", result["command"])
	}
	if result["path"] != "/tmp/output.txt" {
		t.Errorf("expected '/tmp/output.txt', got %q", result["path"])
	}
}

func TestSubstituteArgs_NonStringValues(t *testing.T) {
	args := map[string]any{
		"count":  42,
		"flag":   true,
		"nested": map[string]any{"key": "val"},
	}
	vars := map[string]string{}
	result := recipe.SubstituteArgs(args, vars)
	if result["count"] != 42 {
		t.Errorf("expected 42, got %v", result["count"])
	}
	if result["flag"] != true {
		t.Errorf("expected true, got %v", result["flag"])
	}
}

// ---------------------------------------------------------------------------
// Executor tests
// ---------------------------------------------------------------------------

func makeTestHandlers() recipe.HandlerMap {
	return recipe.HandlerMap{
		"bash":  mockTool("bash"),
		"read":  mockTool("read"),
		"write": mockTool("write"),
	}
}

func TestExecute_SingleStep(t *testing.T) {
	r := recipe.Recipe{
		Name:        "single",
		Description: "Single step recipe",
		Steps: []recipe.Step{
			{
				Name: "step1",
				Tool: "bash",
				Args: map[string]any{
					"command": "echo hello {{target}}",
				},
				Capture: "step1_output",
			},
		},
	}

	exec := recipe.NewExecutor(makeTestHandlers())
	out, err := exec.Execute(context.Background(), r, map[string]string{"target": "world"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "bash") {
		t.Errorf("expected output to contain 'bash', got %q", out)
	}
}

func TestExecute_MultipleSteps_Sequential(t *testing.T) {
	var callOrder []string
	var mu sync.Mutex

	handlers := recipe.HandlerMap{
		"step_a": func(ctx context.Context, args json.RawMessage) (string, error) {
			mu.Lock()
			callOrder = append(callOrder, "a")
			mu.Unlock()
			return "output_a", nil
		},
		"step_b": func(ctx context.Context, args json.RawMessage) (string, error) {
			mu.Lock()
			callOrder = append(callOrder, "b")
			mu.Unlock()
			return "output_b", nil
		},
	}

	r := recipe.Recipe{
		Name:        "seq",
		Description: "Sequential steps",
		Steps: []recipe.Step{
			{Name: "a", Tool: "step_a", Args: map[string]any{}, Capture: "out_a"},
			{Name: "b", Tool: "step_b", Args: map[string]any{}, Capture: "out_b"},
		},
	}

	exec := recipe.NewExecutor(handlers)
	_, err := exec.Execute(context.Background(), r, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(callOrder) != 2 || callOrder[0] != "a" || callOrder[1] != "b" {
		t.Errorf("expected ['a','b'] call order, got %v", callOrder)
	}
}

func TestExecute_CapturePassedToNextStep(t *testing.T) {
	var receivedArgs json.RawMessage
	handlers := recipe.HandlerMap{
		"producer": func(ctx context.Context, args json.RawMessage) (string, error) {
			return "produced_value", nil
		},
		"consumer": func(ctx context.Context, args json.RawMessage) (string, error) {
			receivedArgs = args
			return "consumed", nil
		},
	}

	r := recipe.Recipe{
		Name:        "chain",
		Description: "Chain output to next step",
		Steps: []recipe.Step{
			{Name: "s1", Tool: "producer", Args: map[string]any{}, Capture: "first_out"},
			{Name: "s2", Tool: "consumer", Args: map[string]any{
				"input": "{{first_out}}",
			}, Capture: "second_out"},
		},
	}

	exec := recipe.NewExecutor(handlers)
	_, err := exec.Execute(context.Background(), r, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(receivedArgs, &parsed); err != nil {
		t.Fatalf("parse args: %v", err)
	}
	if parsed["input"] != "produced_value" {
		t.Errorf("expected input='produced_value', got %v", parsed["input"])
	}
}

func TestExecute_StepFailure_ReturnsError(t *testing.T) {
	handlers := recipe.HandlerMap{
		"bash": errorTool("step failed"),
	}

	r := recipe.Recipe{
		Name:        "failing",
		Description: "Has a failing step",
		Steps: []recipe.Step{
			{Name: "s1", Tool: "bash", Args: map[string]any{"command": "fail"}},
		},
	}

	exec := recipe.NewExecutor(handlers)
	_, err := exec.Execute(context.Background(), r, nil)
	if err == nil {
		t.Error("expected error from failing step, got nil")
	}
	if !strings.Contains(err.Error(), "step failed") {
		t.Errorf("expected 'step failed' in error, got %q", err.Error())
	}
}

func TestExecute_UnknownTool_ReturnsError(t *testing.T) {
	r := recipe.Recipe{
		Name:        "unknown",
		Description: "References nonexistent tool",
		Steps: []recipe.Step{
			{Name: "s1", Tool: "nonexistent_tool", Args: map[string]any{}},
		},
	}

	exec := recipe.NewExecutor(makeTestHandlers())
	_, err := exec.Execute(context.Background(), r, nil)
	if err == nil {
		t.Error("expected error for unknown tool, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent_tool") {
		t.Errorf("expected tool name in error, got %q", err.Error())
	}
}

func TestExecute_StepWithoutCapture(t *testing.T) {
	// Steps without Capture field should work fine — output discarded
	r := recipe.Recipe{
		Name:        "nocapture",
		Description: "Steps without capture",
		Steps: []recipe.Step{
			{Name: "s1", Tool: "bash", Args: map[string]any{"command": "echo hi"}},
		},
	}

	exec := recipe.NewExecutor(makeTestHandlers())
	_, err := exec.Execute(context.Background(), r, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecute_ParametersSubstitutedInSteps(t *testing.T) {
	var capturedCommand string
	handlers := recipe.HandlerMap{
		"bash": func(ctx context.Context, args json.RawMessage) (string, error) {
			var a map[string]any
			_ = json.Unmarshal(args, &a)
			capturedCommand, _ = a["command"].(string)
			return "ok", nil
		},
	}

	r := recipe.Recipe{
		Name:        "paramtest",
		Description: "Test param substitution",
		Steps: []recipe.Step{
			{Name: "s1", Tool: "bash", Args: map[string]any{"command": "run {{action}} on {{target}}"}},
		},
	}

	exec := recipe.NewExecutor(handlers)
	_, err := exec.Execute(context.Background(), r, map[string]string{
		"action": "lint",
		"target": "./...",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedCommand != "run lint on ./..." {
		t.Errorf("expected 'run lint on ./...', got %q", capturedCommand)
	}
}

func TestExecute_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	handlers := recipe.HandlerMap{
		"bash": func(ctx context.Context, args json.RawMessage) (string, error) {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			return "ok", nil
		},
	}

	r := recipe.Recipe{
		Name:        "cancelled",
		Description: "Should handle cancellation",
		Steps: []recipe.Step{
			{Name: "s1", Tool: "bash", Args: map[string]any{"command": "echo hi"}},
		},
	}

	exec := recipe.NewExecutor(handlers)
	_, err := exec.Execute(ctx, r, nil)
	if err == nil {
		t.Error("expected error from cancelled context, got nil")
	}
}

// ---------------------------------------------------------------------------
// Concurrent execution tests (race safety)
// ---------------------------------------------------------------------------

func TestExecute_ConcurrentSameRecipe(t *testing.T) {
	// Same recipe executed concurrently — must be safe under -race
	r := recipe.Recipe{
		Name:        "concurrent",
		Description: "Concurrent test",
		Steps: []recipe.Step{
			{Name: "s1", Tool: "bash", Args: map[string]any{"command": "echo {{n}}"}, Capture: "out"},
		},
	}

	handlers := makeTestHandlers()
	exec := recipe.NewExecutor(handlers)

	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_, err := exec.Execute(context.Background(), r, map[string]string{
				"n": fmt.Sprintf("%d", n),
			})
			if err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent execution error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Large output handling
// ---------------------------------------------------------------------------

func TestExecute_LargeOutput(t *testing.T) {
	largeOutput := strings.Repeat("x", 1024*1024) // 1MB
	handlers := recipe.HandlerMap{
		"bash": func(ctx context.Context, args json.RawMessage) (string, error) {
			return largeOutput, nil
		},
	}

	r := recipe.Recipe{
		Name:        "large",
		Description: "Large output test",
		Steps: []recipe.Step{
			{Name: "s1", Tool: "bash", Args: map[string]any{"command": "cat big_file"}, Capture: "out"},
		},
	}

	exec := recipe.NewExecutor(handlers)
	out, err := exec.Execute(context.Background(), r, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Output should be non-empty (truncated or full — implementation choice)
	if out == "" {
		t.Error("expected non-empty output for large step result")
	}
}
