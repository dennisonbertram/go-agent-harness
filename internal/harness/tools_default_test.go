package harness

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	htools "go-agent-harness/internal/harness/tools"
)

type staticPolicy struct {
	decision ToolPolicyDecision
	err      error
}

func (s staticPolicy) Allow(_ context.Context, _ ToolPolicyInput) (ToolPolicyDecision, error) {
	return s.decision, s.err
}

func TestToolPolicyAdapterAndDangerousWrapper(t *testing.T) {
	t.Parallel()

	adapter := toolPolicyAdapter{policy: staticPolicy{decision: ToolPolicyDecision{Allow: true, Reason: "ok"}}}
	decision, err := adapter.Allow(context.Background(), htools.PolicyInput{ToolName: "bash", Action: htools.ActionExecute})
	if err != nil {
		t.Fatalf("allow adapter returned error: %v", err)
	}
	if !decision.Allow || decision.Reason != "ok" {
		t.Fatalf("unexpected decision: %+v", decision)
	}

	errAdapter := toolPolicyAdapter{policy: staticPolicy{err: errors.New("boom")}}
	if _, err := errAdapter.Allow(context.Background(), htools.PolicyInput{}); err == nil {
		t.Fatalf("expected adapter error")
	}

	nilAdapter := toolPolicyAdapter{}
	decision, err = nilAdapter.Allow(context.Background(), htools.PolicyInput{})
	if err != nil {
		t.Fatalf("nil adapter should not error: %v", err)
	}
	if decision.Allow {
		t.Fatalf("expected zero decision for nil policy")
	}

	if !isDangerousCommand("rm -rf /") {
		t.Fatalf("expected dangerous wrapper detection")
	}
}

func TestNewDefaultRegistryWithPolicyIncludesAskUserQuestion(t *testing.T) {
	t.Parallel()

	registry := NewDefaultRegistryWithPolicy(t.TempDir(), ToolApprovalModeFullAuto, nil)
	defs := registry.Definitions()
	foundAskUser := false
	foundObsMemory := false
	for _, def := range defs {
		if def.Name == "AskUserQuestion" {
			foundAskUser = true
		}
		if def.Name == "observational_memory" {
			foundObsMemory = true
		}
	}
	if !foundAskUser {
		t.Fatalf("expected AskUserQuestion in default registry")
	}
	if !foundObsMemory {
		t.Fatalf("expected observational_memory in default registry")
	}
}

func TestDefaultRegistry_RecipesDir_RegistersRunRecipe(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	recipeYAML := `
name: greet
description: "Say hello"
steps:
  - name: s1
    tool: bash
    args:
      command: "echo {{name}}"
    capture: out
`
	if err := os.WriteFile(filepath.Join(dir, "greet.yaml"), []byte(recipeYAML), 0644); err != nil {
		t.Fatal(err)
	}

	registry := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		RecipesDir: dir,
	})
	defs := registry.DeferredDefinitions()
	found := false
	for _, def := range defs {
		if def.Name == "run_recipe" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected run_recipe to be registered when RecipesDir is set with recipes")
	}
}

func TestDefaultRegistry_RecipesDir_Empty_NoRunRecipe(t *testing.T) {
	t.Parallel()

	dir := t.TempDir() // empty — no recipe files

	registry := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		RecipesDir: dir,
	})
	defs := registry.DeferredDefinitions()
	for _, def := range defs {
		if def.Name == "run_recipe" {
			t.Error("expected run_recipe NOT to be registered for empty recipes dir")
			return
		}
	}
}

func TestDefaultRegistry_RecipesDir_Missing_NoRunRecipe(t *testing.T) {
	t.Parallel()

	registry := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		RecipesDir: "/tmp/nonexistent-recipes-for-test-xyz",
	})
	defs := registry.DeferredDefinitions()
	for _, def := range defs {
		if def.Name == "run_recipe" {
			t.Error("expected run_recipe NOT to be registered for missing recipes dir")
			return
		}
	}
}

func TestDefaultRegistry_RecipesDir_Empty_NoRegistry(t *testing.T) {
	t.Parallel()

	// No RecipesDir set — run_recipe should not appear
	registry := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{})
	defs := registry.DeferredDefinitions()
	for _, def := range defs {
		if def.Name == "run_recipe" {
			t.Error("expected run_recipe NOT to be registered when RecipesDir is empty")
			return
		}
	}
}
