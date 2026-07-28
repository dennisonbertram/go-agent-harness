package deferred_test

// Behaviour tests for list_models that existed only in the deleted duplicate
// tool package: the filter, info-action, and malformed-input cases. The
// definition/list/providers/unknown-action cases are already covered by
// deferred_test.go and were not duplicated here.

import (
	"context"
	"encoding/json"
	"testing"

	"go-agent-harness/internal/harness/tools/deferred"
	"go-agent-harness/internal/provider/catalog"
)

func movedTestModelCatalog() *catalog.Catalog {
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

func TestListModelsToolListWithFilter(t *testing.T) {
	t.Parallel()
	cat := movedTestModelCatalog()
	tool := deferred.ListModelsTool(cat)
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
	cat := movedTestModelCatalog()
	tool := deferred.ListModelsTool(cat)
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
	cat := movedTestModelCatalog()
	tool := deferred.ListModelsTool(cat)
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
	cat := movedTestModelCatalog()
	tool := deferred.ListModelsTool(cat)
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"action":"info"}`))
	if err == nil {
		t.Fatalf("expected error for missing provider/model_id")
	}
}

func TestListModelsToolInvalidJSON(t *testing.T) {
	t.Parallel()
	cat := movedTestModelCatalog()
	tool := deferred.ListModelsTool(cat)
	_, err := tool.Handler(context.Background(), json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatalf("expected error for invalid JSON")
	}
}
