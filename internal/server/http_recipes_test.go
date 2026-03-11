package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-agent-harness/internal/harness"
	"go-agent-harness/internal/harness/tools/recipe"
)

func newRecipeTestServer(t *testing.T, recipes []recipe.Recipe) *httptest.Server {
	t.Helper()
	registry := harness.NewRegistry()
	runner := harness.NewRunner(
		&staticProvider{result: harness.CompletionResult{Content: "done"}},
		registry,
		harness.RunnerConfig{
			DefaultModel:        "gpt-4.1-mini",
			DefaultSystemPrompt: "You are helpful.",
			MaxSteps:            1,
		},
	)
	handler := NewWithOptions(ServerOptions{Runner: runner, Recipes: recipes})
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return ts
}

func testRecipes() []recipe.Recipe {
	return []recipe.Recipe{
		{
			Name:        "greet",
			Description: "A simple greeting recipe",
			Tags:        []string{"hello", "demo"},
			Parameters: map[string]recipe.ParameterDef{
				"name": {Type: "string", Description: "Who to greet"},
			},
			Steps: []recipe.Step{
				{Name: "say_hello", Tool: "echo", Args: map[string]any{"message": "hello {{name}}"}},
			},
		},
		{
			Name:        "inspect",
			Description: "Inspect a file",
			Tags:        []string{"file"},
			Steps: []recipe.Step{
				{Tool: "read", Args: map[string]any{"path": "{{file}}"}},
			},
		},
	}
}

func TestRecipeListAll(t *testing.T) {
	t.Parallel()
	ts := newRecipeTestServer(t, testRecipes())

	resp, err := http.Get(ts.URL + "/v1/recipes")
	if err != nil {
		t.Fatalf("GET /v1/recipes: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Count   int           `json:"count"`
		Recipes []recipeEntry `json:"recipes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Count != 2 {
		t.Errorf("count = %d, want 2", body.Count)
	}
	if len(body.Recipes) != 2 {
		t.Fatalf("len(recipes) = %d, want 2", len(body.Recipes))
	}
	if body.Recipes[0].Name != "greet" {
		t.Errorf("recipes[0].Name = %q", body.Recipes[0].Name)
	}
}

func TestRecipeListEmpty(t *testing.T) {
	t.Parallel()
	ts := newRecipeTestServer(t, nil)

	resp, err := http.Get(ts.URL + "/v1/recipes")
	if err != nil {
		t.Fatalf("GET /v1/recipes: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Count   int           `json:"count"`
		Recipes []recipeEntry `json:"recipes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Count != 0 {
		t.Errorf("expected count=0, got %d", body.Count)
	}
}

func TestRecipeGetByName(t *testing.T) {
	t.Parallel()
	ts := newRecipeTestServer(t, testRecipes())

	resp, err := http.Get(ts.URL + "/v1/recipes/greet")
	if err != nil {
		t.Fatalf("GET /v1/recipes/greet: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var entry recipeEntry
	if err := json.NewDecoder(resp.Body).Decode(&entry); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if entry.Name != "greet" {
		t.Errorf("name = %q, want greet", entry.Name)
	}
	if entry.Description != "A simple greeting recipe" {
		t.Errorf("description = %q", entry.Description)
	}
	if len(entry.Tags) != 2 {
		t.Errorf("len(tags) = %d, want 2", len(entry.Tags))
	}
}

func TestRecipeGetNotFound(t *testing.T) {
	t.Parallel()
	ts := newRecipeTestServer(t, testRecipes())

	resp, err := http.Get(ts.URL + "/v1/recipes/nonexistent")
	if err != nil {
		t.Fatalf("GET /v1/recipes/nonexistent: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestRecipeSchema(t *testing.T) {
	t.Parallel()
	ts := newRecipeTestServer(t, testRecipes())

	resp, err := http.Get(ts.URL + "/v1/recipes/greet/schema")
	if err != nil {
		t.Fatalf("GET /v1/recipes/greet/schema: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Parameters map[string]recipe.ParameterDef `json:"parameters"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := body.Parameters["name"]; !ok {
		t.Error("expected 'name' parameter in schema")
	}
}

func TestRecipeSchemaNotFound(t *testing.T) {
	t.Parallel()
	ts := newRecipeTestServer(t, testRecipes())

	resp, err := http.Get(ts.URL + "/v1/recipes/missing/schema")
	if err != nil {
		t.Fatalf("GET /v1/recipes/missing/schema: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestRecipeMethodNotAllowed(t *testing.T) {
	t.Parallel()
	ts := newRecipeTestServer(t, testRecipes())

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/recipes", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /v1/recipes: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
}
