package deferred

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"go-agent-harness/internal/harness/tools/recipe"
)

// ---------------------------------------------------------------------------
// RunRecipeTool tests (deferred package integration)
// ---------------------------------------------------------------------------

func TestRunRecipeTool_ToolDefinition(t *testing.T) {
	recipes := []recipe.Recipe{
		{
			Name:        "greet",
			Description: "Say hello",
			Steps: []recipe.Step{
				{Name: "s1", Tool: "bash", Args: map[string]any{"command": "echo hi"}},
			},
		},
	}
	handlers := recipe.HandlerMap{
		"bash": func(_ context.Context, _ json.RawMessage) (string, error) {
			return "hi", nil
		},
	}

	tool := RunRecipeTool(handlers, recipes)

	if tool.Definition.Name != "run_recipe" {
		t.Errorf("expected name 'run_recipe', got %q", tool.Definition.Name)
	}
	if tool.Definition.Tier != "deferred" {
		t.Errorf("expected TierDeferred, got %q", tool.Definition.Tier)
	}
	if !tool.Definition.Mutating {
		t.Error("expected Mutating=true")
	}
}

func TestRunRecipeTool_TagsIncludeRecipeName(t *testing.T) {
	recipes := []recipe.Recipe{
		{
			Name:        "lint_and_fix",
			Description: "Lint and fix",
			Steps: []recipe.Step{
				{Name: "s1", Tool: "bash", Args: map[string]any{"command": "echo lint"}},
			},
		},
	}
	handlers := recipe.HandlerMap{
		"bash": func(_ context.Context, _ json.RawMessage) (string, error) {
			return "ok", nil
		},
	}

	tool := RunRecipeTool(handlers, recipes)

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
		t.Errorf("expected 'lint_and_fix' name tag, got %v", tool.Definition.Tags)
	}
}

func TestRunRecipeTool_Execute_HappyPath(t *testing.T) {
	recipes := []recipe.Recipe{
		{
			Name:        "greet",
			Description: "Say hello",
			Steps: []recipe.Step{
				{Name: "s1", Tool: "bash", Args: map[string]any{"command": "echo {{name}}"}, Capture: "out"},
			},
		},
	}
	handlers := recipe.HandlerMap{
		"bash": func(_ context.Context, args json.RawMessage) (string, error) {
			return "hello world", nil
		},
	}

	tool := RunRecipeTool(handlers, recipes)

	rawArgs, _ := json.Marshal(map[string]any{
		"name": "greet",
		"args": map[string]string{"name": "world"},
	})
	out, err := tool.Handler(context.Background(), rawArgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "" {
		t.Error("expected non-empty output")
	}
}

func TestRunRecipeTool_Execute_UnknownRecipe(t *testing.T) {
	recipes := []recipe.Recipe{}
	handlers := recipe.HandlerMap{}

	tool := RunRecipeTool(handlers, recipes)

	rawArgs, _ := json.Marshal(map[string]any{"name": "nonexistent"})
	_, err := tool.Handler(context.Background(), rawArgs)
	if err == nil {
		t.Error("expected error for unknown recipe, got nil")
	}
}

func TestRunRecipeTool_Execute_MissingName(t *testing.T) {
	tool := RunRecipeTool(recipe.HandlerMap{}, nil)

	rawArgs, _ := json.Marshal(map[string]any{})
	_, err := tool.Handler(context.Background(), rawArgs)
	if err == nil {
		t.Error("expected error for missing name, got nil")
	}
}

func TestRunRecipeTool_Execute_InvalidJSON(t *testing.T) {
	tool := RunRecipeTool(recipe.HandlerMap{}, nil)

	_, err := tool.Handler(context.Background(), json.RawMessage("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestRunRecipeTool_DescriptionContainsAvailableRecipes(t *testing.T) {
	recipes := []recipe.Recipe{
		{
			Name:        "alpha",
			Description: "Alpha recipe",
			Steps: []recipe.Step{
				{Name: "s1", Tool: "bash", Args: map[string]any{}},
			},
		},
		{
			Name:        "beta",
			Description: "Beta recipe",
			Steps: []recipe.Step{
				{Name: "s1", Tool: "bash", Args: map[string]any{}},
			},
		},
	}
	tool := RunRecipeTool(recipe.HandlerMap{}, recipes)

	// The name parameter description should list available recipes.
	params, ok := tool.Definition.Parameters["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties map in parameters")
	}
	nameParam, ok := params["name"].(map[string]any)
	if !ok {
		t.Fatal("expected name param")
	}
	desc, _ := nameParam["description"].(string)
	if !strings.Contains(desc, "alpha") {
		t.Errorf("expected 'alpha' in name description, got %q", desc)
	}
}

func TestRunRecipeTool_EmptyRecipeList_StillWorks(t *testing.T) {
	tool := RunRecipeTool(recipe.HandlerMap{}, nil)

	if tool.Definition.Name != "run_recipe" {
		t.Errorf("expected name 'run_recipe', got %q", tool.Definition.Name)
	}
}
