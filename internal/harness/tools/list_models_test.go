package tools

import (
	"context"
	"encoding/json"
	"testing"

	"go-agent-harness/internal/provider/catalog"
)

func testModelCatalog() *catalog.Catalog {
	return &catalog.Catalog{
		CatalogVersion: "1.0",
		Providers: map[string]catalog.ProviderEntry{
			"openai": {
				DisplayName: "OpenAI",
				BaseURL:     "https://api.openai.com/v1",
				APIKeyEnv:   "OPENAI_API_KEY",
				Protocol:    "openai",
				Models: map[string]catalog.Model{
					"gpt-4o": {
						DisplayName:   "GPT-4o",
						Description:   "Flagship model",
						ContextWindow: 128000,
						Modalities:    []string{"text", "vision"},
						ToolCalling:   true,
						Streaming:     true,
						Strengths:     []string{"general", "code"},
						BestFor:       []string{"code-generation"},
						SpeedTier:     "fast",
						CostTier:      "standard",
					},
				},
			},
			"deepseek": {
				DisplayName: "DeepSeek",
				BaseURL:     "https://api.deepseek.com/v1",
				APIKeyEnv:   "DEEPSEEK_API_KEY",
				Protocol:    "openai-compatible",
				Models: map[string]catalog.Model{
					"deepseek-chat": {
						DisplayName:   "DeepSeek Chat",
						Description:   "Budget chat model",
						ContextWindow: 64000,
						Modalities:    []string{"text"},
						ToolCalling:   true,
						Streaming:     true,
						Strengths:     []string{"code"},
						BestFor:       []string{"code-generation"},
						SpeedTier:     "ultra-fast",
						CostTier:      "budget",
					},
				},
			},
		},
	}
}

func TestListModelsToolRegistered(t *testing.T) {
	t.Parallel()
	cat := testModelCatalog()
	list, err := BuildCatalog(BuildOptions{
		WorkspaceRoot: t.TempDir(),
		ModelCatalog:  cat,
	})
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	found := false
	for _, tool := range list {
		if tool.Definition.Name == "list_models" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("list_models tool not found in catalog")
	}
}

func TestListModelsToolNotRegisteredWithoutCatalog(t *testing.T) {
	t.Parallel()
	list, err := BuildCatalog(BuildOptions{
		WorkspaceRoot: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	for _, tool := range list {
		if tool.Definition.Name == "list_models" {
			t.Fatalf("list_models tool should not be registered without ModelCatalog")
		}
	}
}

func TestListModelsToolListAction(t *testing.T) {
	t.Parallel()
	cat := testModelCatalog()
	tool := listModelsTool(cat)
	out, err := tool.Handler(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("list_models list: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["action"] != "list" {
		t.Fatalf("expected action=list, got %v", result["action"])
	}
	count := result["count"].(float64)
	if count != 2 {
		t.Fatalf("expected 2 models, got %v", count)
	}
}

func TestListModelsToolListWithFilter(t *testing.T) {
	t.Parallel()
	cat := testModelCatalog()
	tool := listModelsTool(cat)
	out, err := tool.Handler(context.Background(), json.RawMessage(`{"cost_tier":"budget"}`))
	if err != nil {
		t.Fatalf("list_models filter: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	count := result["count"].(float64)
	if count != 1 {
		t.Fatalf("expected 1 budget model, got %v", count)
	}
}

func TestListModelsToolInfoAction(t *testing.T) {
	t.Parallel()
	cat := testModelCatalog()
	tool := listModelsTool(cat)
	out, err := tool.Handler(context.Background(), json.RawMessage(`{"action":"info","provider":"openai","model_id":"gpt-4o"}`))
	if err != nil {
		t.Fatalf("list_models info: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["action"] != "info" {
		t.Fatalf("expected action=info, got %v", result["action"])
	}
	model := result["model"].(map[string]any)
	if model["model_id"] != "gpt-4o" {
		t.Fatalf("expected model_id=gpt-4o, got %v", model["model_id"])
	}
}

func TestListModelsToolInfoNotFound(t *testing.T) {
	t.Parallel()
	cat := testModelCatalog()
	tool := listModelsTool(cat)
	out, err := tool.Handler(context.Background(), json.RawMessage(`{"action":"info","provider":"openai","model_id":"nonexistent"}`))
	if err != nil {
		t.Fatalf("list_models info not found: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["error"] == nil {
		t.Fatalf("expected error field for missing model")
	}
}

func TestListModelsToolInfoMissingParams(t *testing.T) {
	t.Parallel()
	cat := testModelCatalog()
	tool := listModelsTool(cat)
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"action":"info"}`))
	if err == nil {
		t.Fatalf("expected error for missing provider/model_id")
	}
}

func TestListModelsToolProvidersAction(t *testing.T) {
	t.Parallel()
	cat := testModelCatalog()
	tool := listModelsTool(cat)
	out, err := tool.Handler(context.Background(), json.RawMessage(`{"action":"providers"}`))
	if err != nil {
		t.Fatalf("list_models providers: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["action"] != "providers" {
		t.Fatalf("expected action=providers, got %v", result["action"])
	}
	providers := result["providers"].([]any)
	if len(providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(providers))
	}
}

func TestListModelsToolUnknownAction(t *testing.T) {
	t.Parallel()
	cat := testModelCatalog()
	tool := listModelsTool(cat)
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"action":"invalid"}`))
	if err == nil {
		t.Fatalf("expected error for unknown action")
	}
}

func TestListModelsToolInvalidJSON(t *testing.T) {
	t.Parallel()
	cat := testModelCatalog()
	tool := listModelsTool(cat)
	_, err := tool.Handler(context.Background(), json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatalf("expected error for invalid JSON")
	}
}
