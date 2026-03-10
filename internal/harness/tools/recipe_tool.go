package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go-agent-harness/internal/harness/tools/descriptions"
	"go-agent-harness/internal/harness/tools/recipe"
)

// runRecipeArgs are the arguments accepted by the run_recipe tool.
type runRecipeArgs struct {
	// Name is the recipe name to execute.
	Name string `json:"name"`
	// Args are key-value pairs substituted into the recipe's step templates.
	Args map[string]string `json:"args,omitempty"`
}

// runRecipeTool creates the run_recipe deferred tool.
// handlers is a map of tool-name → handler used to execute recipe steps.
// recipes is the list of available Recipe definitions.
func runRecipeTool(handlers recipe.HandlerMap, recipes []recipe.Recipe) Tool {
	// Build a name → recipe index for O(1) lookup.
	index := make(map[string]recipe.Recipe, len(recipes))
	for _, r := range recipes {
		index[r.Name] = r
	}

	exec := recipe.NewExecutor(handlers)

	def := Definition{
		Name:         "run_recipe",
		Description:  descriptions.Load("run_recipe"),
		Action:       ActionExecute,
		Mutating:     true,
		ParallelSafe: false,
		Tier:         TierDeferred,
		Tags:         append([]string{"recipe", "macro", "workflow", "compose"}, recipeNameList(recipes)...),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "The name of the recipe to run. " + recipeListHint(recipes),
				},
				"args": map[string]any{
					"type":                 "object",
					"description":          "Key-value arguments substituted into recipe step templates ({{key}} placeholders).",
					"additionalProperties": map[string]any{"type": "string"},
				},
			},
			"required":             []string{"name"},
			"additionalProperties": false,
		},
	}

	handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args runRecipeArgs
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse run_recipe args: %w", err)
		}
		name := strings.TrimSpace(args.Name)
		if name == "" {
			return "", fmt.Errorf("name is required")
		}
		r, ok := index[name]
		if !ok {
			return "", fmt.Errorf("recipe %q not found; available: %s", name, strings.Join(recipeNameList(recipes), ", "))
		}
		return exec.Execute(ctx, r, args.Args)
	}

	return Tool{Definition: def, Handler: handler}
}

// recipeNameList returns the names of all recipes, for use in tags/hints.
func recipeNameList(recipes []recipe.Recipe) []string {
	names := make([]string, len(recipes))
	for i, r := range recipes {
		names[i] = r.Name
	}
	return names
}

// recipeListHint builds a short one-line hint listing available recipe names.
func recipeListHint(recipes []recipe.Recipe) string {
	if len(recipes) == 0 {
		return "No recipes loaded."
	}
	names := recipeNameList(recipes)
	return "Available: " + strings.Join(names, ", ") + "."
}
