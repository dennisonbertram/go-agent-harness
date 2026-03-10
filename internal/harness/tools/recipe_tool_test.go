package tools_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tools "go-agent-harness/internal/harness/tools"
)

// writeRecipeFile writes a recipe YAML file to dir.
func writeRecipeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("writeRecipeFile %s: %v", name, err)
	}
}

// buildCatalogWithRecipes creates a minimal catalog with recipes enabled.
func buildCatalogWithRecipes(t *testing.T, recipesDir string) []tools.Tool {
	t.Helper()
	cat, err := tools.BuildCatalog(tools.BuildOptions{
		WorkspaceRoot: t.TempDir(),
		EnableRecipes: true,
		RecipesDir:    recipesDir,
	})
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	return cat
}

// findTool finds a tool by name in the catalog.
func findToolInCatalog(cat []tools.Tool, name string) (tools.Tool, bool) {
	for _, t := range cat {
		if t.Definition.Name == name {
			return t, true
		}
	}
	return tools.Tool{}, false
}

// ---------------------------------------------------------------------------
// run_recipe tool tests
// ---------------------------------------------------------------------------

func TestRunRecipeTool_RegisteredInCatalog(t *testing.T) {
	dir := t.TempDir()
	writeRecipeFile(t, dir, "hello.yaml", `
name: hello
description: "Say hello"
steps:
  - name: greet
    tool: bash
    args:
      command: "echo hello"
    capture: result
`)

	cat := buildCatalogWithRecipes(t, dir)
	_, ok := findToolInCatalog(cat, "run_recipe")
	if !ok {
		t.Error("expected run_recipe tool to be registered in catalog")
	}
}

func TestRunRecipeTool_IsDeferred(t *testing.T) {
	dir := t.TempDir()
	writeRecipeFile(t, dir, "hello.yaml", `
name: hello
description: "Say hello"
steps:
  - name: greet
    tool: bash
    args:
      command: "echo hello"
`)

	cat := buildCatalogWithRecipes(t, dir)
	tool, ok := findToolInCatalog(cat, "run_recipe")
	if !ok {
		t.Fatal("run_recipe not found in catalog")
	}
	if tool.Definition.Tier != tools.TierDeferred {
		t.Errorf("expected TierDeferred, got %q", tool.Definition.Tier)
	}
}

func TestRunRecipeTool_NotRegisteredWhenNoRecipes(t *testing.T) {
	dir := t.TempDir() // empty dir — no recipes

	cat := buildCatalogWithRecipes(t, dir)
	_, ok := findToolInCatalog(cat, "run_recipe")
	if ok {
		t.Error("expected run_recipe NOT to be registered when no recipes are loaded")
	}
}

func TestRunRecipeTool_NotRegisteredWhenDisabled(t *testing.T) {
	dir := t.TempDir()
	writeRecipeFile(t, dir, "hello.yaml", `
name: hello
description: "Say hello"
steps:
  - name: greet
    tool: bash
    args:
      command: "echo hello"
`)

	cat, err := tools.BuildCatalog(tools.BuildOptions{
		WorkspaceRoot: t.TempDir(),
		EnableRecipes: false, // disabled
		RecipesDir:    dir,
	})
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	_, ok := findToolInCatalog(cat, "run_recipe")
	if ok {
		t.Error("expected run_recipe NOT to be registered when EnableRecipes=false")
	}
}

func TestRunRecipeTool_InvokesRecipe(t *testing.T) {
	dir := t.TempDir()
	writeRecipeFile(t, dir, "greeter.yaml", `
name: greeter
description: "Greet someone"
steps:
  - name: say_hi
    tool: bash
    args:
      command: "echo hi {{target}}"
    capture: greeting
`)

	cat := buildCatalogWithRecipes(t, dir)
	tool, ok := findToolInCatalog(cat, "run_recipe")
	if !ok {
		t.Fatal("run_recipe not found")
	}

	args, _ := json.Marshal(map[string]any{
		"name": "greeter",
		"args": map[string]string{"target": "world"},
	})

	ctx := context.WithValue(context.Background(), tools.ContextKeyRunID, "test-run")
	out, err := tool.Handler(ctx, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "" {
		t.Error("expected non-empty output")
	}
}

func TestRunRecipeTool_UnknownRecipeError(t *testing.T) {
	dir := t.TempDir()
	writeRecipeFile(t, dir, "hello.yaml", `
name: hello
description: "Hello"
steps:
  - name: s1
    tool: bash
    args:
      command: "echo hi"
`)

	cat := buildCatalogWithRecipes(t, dir)
	tool, ok := findToolInCatalog(cat, "run_recipe")
	if !ok {
		t.Fatal("run_recipe not found")
	}

	args, _ := json.Marshal(map[string]any{"name": "nonexistent"})
	_, err := tool.Handler(context.Background(), args)
	if err == nil {
		t.Error("expected error for unknown recipe, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("expected 'nonexistent' in error, got %q", err.Error())
	}
}

func TestRunRecipeTool_MissingNameError(t *testing.T) {
	dir := t.TempDir()
	writeRecipeFile(t, dir, "hello.yaml", `
name: hello
description: "Hello"
steps:
  - name: s1
    tool: bash
    args:
      command: "echo hi"
`)

	cat := buildCatalogWithRecipes(t, dir)
	tool, _ := findToolInCatalog(cat, "run_recipe")

	args, _ := json.Marshal(map[string]any{})
	_, err := tool.Handler(context.Background(), args)
	if err == nil {
		t.Error("expected error for missing name, got nil")
	}
}

func TestRunRecipeTool_MissingDirectory(t *testing.T) {
	// A missing recipes dir should not cause an error — just no recipes loaded
	cat, err := tools.BuildCatalog(tools.BuildOptions{
		WorkspaceRoot: t.TempDir(),
		EnableRecipes: true,
		RecipesDir:    "/tmp/no-such-recipes-dir-xyz",
	})
	if err != nil {
		t.Fatalf("expected no error for missing recipes dir, got: %v", err)
	}
	_, ok := findToolInCatalog(cat, "run_recipe")
	if ok {
		t.Error("expected run_recipe NOT to be registered for missing dir")
	}
}

func TestRunRecipeTool_HasRecipeTags(t *testing.T) {
	dir := t.TempDir()
	writeRecipeFile(t, dir, "lint.yaml", `
name: lint_and_fix
description: "Lint and fix"
steps:
  - name: s1
    tool: bash
    args:
      command: "echo lint"
`)

	cat := buildCatalogWithRecipes(t, dir)
	tool, ok := findToolInCatalog(cat, "run_recipe")
	if !ok {
		t.Fatal("run_recipe not found")
	}

	// Should have "recipe" tag and the recipe name as a tag
	hasRecipeTag := false
	hasNameTag := false
	for _, tag := range tool.Definition.Tags {
		if tag == "recipe" {
			hasRecipeTag = true
		}
		if tag == "lint_and_fix" {
			hasNameTag = true
		}
	}
	if !hasRecipeTag {
		t.Errorf("expected 'recipe' tag, got %v", tool.Definition.Tags)
	}
	if !hasNameTag {
		t.Errorf("expected 'lint_and_fix' tag, got %v", tool.Definition.Tags)
	}
}

func TestRunRecipeTool_InvalidJSONArgs(t *testing.T) {
	dir := t.TempDir()
	writeRecipeFile(t, dir, "hello.yaml", `
name: hello
description: "Hello"
steps:
  - name: s1
    tool: bash
    args:
      command: "echo hi"
`)

	cat := buildCatalogWithRecipes(t, dir)
	tool, _ := findToolInCatalog(cat, "run_recipe")

	_, err := tool.Handler(context.Background(), json.RawMessage("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON args, got nil")
	}
}

func TestBuildCatalog_RecipeLoadError(t *testing.T) {
	dir := t.TempDir()
	// Write a malformed YAML file
	if err := os.WriteFile(filepath.Join(dir, "broken.yaml"), []byte("name: [broken"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := tools.BuildCatalog(tools.BuildOptions{
		WorkspaceRoot: t.TempDir(),
		EnableRecipes: true,
		RecipesDir:    dir,
	})
	if err == nil {
		t.Error("expected BuildCatalog to return error for malformed recipe, got nil")
	}
}
