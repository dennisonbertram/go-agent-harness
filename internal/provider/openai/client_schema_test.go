package openai

import "testing"

func TestInjectAdditionalPropertiesFalseDeepCopiesNestedSchemas(t *testing.T) {
	t.Parallel()

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"config": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"enabled": map[string]any{"type": "boolean"},
				},
			},
			"items": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id": map[string]any{"type": "string"},
					},
				},
			},
		},
	}

	got := injectAdditionalPropertiesFalse(schema)

	if got["additionalProperties"] != false {
		t.Fatalf("expected top-level additionalProperties=false, got %#v", got["additionalProperties"])
	}

	configSchema := got["properties"].(map[string]any)["config"].(map[string]any)
	if configSchema["additionalProperties"] != false {
		t.Fatalf("expected nested object additionalProperties=false, got %#v", configSchema["additionalProperties"])
	}

	itemSchema := got["properties"].(map[string]any)["items"].(map[string]any)["items"].(map[string]any)
	if itemSchema["additionalProperties"] != false {
		t.Fatalf("expected array item object additionalProperties=false, got %#v", itemSchema["additionalProperties"])
	}

	originalConfig := schema["properties"].(map[string]any)["config"].(map[string]any)
	if _, exists := originalConfig["additionalProperties"]; exists {
		t.Fatal("expected original schema to remain unmodified")
	}
}
