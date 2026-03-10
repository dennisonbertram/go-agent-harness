// Package recipe implements a tool recipe system for the agent harness.
// A recipe is a named sequence of tool calls (steps) that compose existing
// tools into higher-order, reusable operations. Recipes are stored as YAML
// files and exposed via the run_recipe deferred tool.
package recipe

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Recipe is a named sequence of tool calls.
type Recipe struct {
	// Name is the recipe identifier (used as the value of "name" in run_recipe calls).
	Name string `yaml:"name"`

	// Description is a human-readable summary shown in find_tool.
	Description string `yaml:"description"`

	// Tags are optional search tags for tool discovery.
	Tags []string `yaml:"tags"`

	// Parameters declares the input parameters accepted by this recipe.
	// Each key is a parameter name; the value is an object describing
	// the parameter (type, description, etc.). This mirrors the JSON Schema
	// "properties" shape used by tool definitions.
	Parameters map[string]ParameterDef `yaml:"parameters"`

	// Steps is the ordered list of tool calls to execute.
	Steps []Step `yaml:"steps"`
}

// ParameterDef describes a single recipe parameter.
type ParameterDef struct {
	Type        string `yaml:"type"`
	Description string `yaml:"description"`
}

// Step is a single tool call within a recipe.
type Step struct {
	// Name is an optional human-readable label for the step.
	Name string `yaml:"name"`

	// Tool is the name of the tool to invoke.
	Tool string `yaml:"tool"`

	// Args are the static arguments passed to the tool. String values
	// may contain {{variable}} placeholders that are substituted at
	// execution time using input parameters and prior step captures.
	Args map[string]any `yaml:"args"`

	// Capture is the variable name under which the step's output is stored.
	// Subsequent steps can reference it via {{capture_name}}.
	// If empty, the output is still included in the aggregate result but
	// is not addressable by name in later steps.
	Capture string `yaml:"capture"`
}

// LoadRecipe reads and parses a single recipe YAML file.
func LoadRecipe(path string) (Recipe, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Recipe{}, fmt.Errorf("reading recipe file %s: %w", path, err)
	}

	var r Recipe
	if err := yaml.Unmarshal(data, &r); err != nil {
		return Recipe{}, fmt.Errorf("parsing recipe YAML %s: %w", path, err)
	}

	if err := r.validate(); err != nil {
		return Recipe{}, fmt.Errorf("invalid recipe %s: %w", path, err)
	}
	return r, nil
}

// LoadRecipes discovers and loads all *.yaml files in dir as recipes.
// If dir does not exist, an empty slice is returned without error.
// If a YAML file fails to parse or validate, an error is returned.
func LoadRecipes(dir string) ([]Recipe, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading recipes directory %s: %w", dir, err)
	}

	var recipes []Recipe
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		r, err := LoadRecipe(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		recipes = append(recipes, r)
	}
	return recipes, nil
}

// validate checks that the recipe has all required fields.
func (r *Recipe) validate() error {
	if r.Name == "" {
		return fmt.Errorf("name is required")
	}
	if r.Description == "" {
		return fmt.Errorf("description is required")
	}
	if len(r.Steps) == 0 {
		return fmt.Errorf("at least one step is required")
	}
	for i, s := range r.Steps {
		if s.Tool == "" {
			return fmt.Errorf("step %d: tool is required", i)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Template substitution
// ---------------------------------------------------------------------------

// Substitute replaces all {{key}} placeholders in s with the corresponding
// value from vars. Missing keys are replaced with an empty string.
// Substitution is one-pass only — substituted values are not re-evaluated,
// meaning {{...}} sequences that appear inside substituted values are
// preserved as literal text.
func Substitute(s string, vars map[string]string) string {
	if !strings.Contains(s, "{{") {
		return s
	}
	return substituteOnce(s, vars)
}

// substituteOnce scans s for {{key}} patterns and replaces them with the
// corresponding value from vars (or empty string for unknown keys).
// It processes the string left-to-right in a single pass so that values
// introduced by substitution are never re-evaluated.
func substituteOnce(s string, vars map[string]string) string {
	var b strings.Builder
	b.Grow(len(s))

	for len(s) > 0 {
		start := strings.Index(s, "{{")
		if start < 0 {
			// No more placeholders; write the rest of the string.
			b.WriteString(s)
			break
		}

		// Write everything before the opening "{{".
		b.WriteString(s[:start])
		s = s[start+2:] // skip past "{{"

		// Find the closing "}}".
		end := strings.Index(s, "}}")
		if end < 0 {
			// Unclosed placeholder — write the "{{" back and continue.
			b.WriteString("{{")
			continue
		}

		key := s[:end]
		s = s[end+2:] // skip past "}}"

		// Look up key in vars; empty string for unknown keys.
		b.WriteString(vars[key])
	}

	return b.String()
}

// SubstituteArgs applies Substitute to all string values in an args map.
// Non-string values are passed through unchanged.
func SubstituteArgs(args map[string]any, vars map[string]string) map[string]any {
	if len(args) == 0 {
		return args
	}
	result := make(map[string]any, len(args))
	for k, v := range args {
		if str, ok := v.(string); ok {
			result[k] = Substitute(str, vars)
		} else {
			result[k] = v
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// Executor
// ---------------------------------------------------------------------------

// HandlerMap maps tool names to their handler functions.
type HandlerMap map[string]func(context.Context, json.RawMessage) (string, error)

// Executor runs recipes by dispatching steps to their handlers.
type Executor struct {
	handlers HandlerMap
}

// NewExecutor creates an Executor with the given handler map.
func NewExecutor(handlers HandlerMap) *Executor {
	return &Executor{handlers: handlers}
}

// stepResult holds the output of a completed step.
type stepResult struct {
	StepName string `json:"step_name"`
	Tool     string `json:"tool"`
	Output   string `json:"output"`
}

// Execute runs all steps of r in order, substituting vars into step args.
// Returns a JSON-encoded summary of all step outputs.
func (e *Executor) Execute(ctx context.Context, r Recipe, params map[string]string) (string, error) {
	// Check context before starting
	if err := ctx.Err(); err != nil {
		return "", err
	}

	// vars accumulates both input params and captured step outputs.
	vars := make(map[string]string, len(params))
	for k, v := range params {
		vars[k] = v
	}

	results := make([]stepResult, 0, len(r.Steps))

	for _, step := range r.Steps {
		// Check for cancellation before each step.
		if err := ctx.Err(); err != nil {
			return "", err
		}

		handler, ok := e.handlers[step.Tool]
		if !ok {
			return "", fmt.Errorf("step %q: tool %q not found", step.Name, step.Tool)
		}

		// Substitute variables into step args.
		substituted := SubstituteArgs(step.Args, vars)

		// Marshal the substituted args to JSON for the handler.
		rawArgs, err := json.Marshal(substituted)
		if err != nil {
			return "", fmt.Errorf("step %q: marshal args: %w", step.Name, err)
		}

		output, err := handler(ctx, rawArgs)
		if err != nil {
			return "", fmt.Errorf("step %q (tool=%s): %w", step.Name, step.Tool, err)
		}

		// Store capture for subsequent steps.
		if step.Capture != "" {
			vars[step.Capture] = output
		}

		results = append(results, stepResult{
			StepName: step.Name,
			Tool:     step.Tool,
			Output:   output,
		})
	}

	// Build final output as JSON summary.
	summary := map[string]any{
		"recipe":  r.Name,
		"steps":   len(results),
		"results": results,
	}
	data, err := json.Marshal(summary)
	if err != nil {
		return "", fmt.Errorf("marshal recipe output: %w", err)
	}
	return string(data), nil
}
