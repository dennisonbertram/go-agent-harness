package deferred

import (
	"context"
	"encoding/json"
	"testing"

	tools "go-agent-harness/internal/harness/tools"
)

// TestGetProfileTool_Definition verifies the get_profile tool constructor.
func TestGetProfileTool_Definition(t *testing.T) {
	tool := GetProfileTool("")
	assertToolDef(t, tool, "get_profile", tools.TierDeferred)
	assertHasTags(t, tool, "profiles", "agent")
}

// TestGetProfileTool_ReturnsProfileDetails verifies that getting a specific profile returns details.
func TestGetProfileTool_ReturnsProfileDetails(t *testing.T) {
	t.Parallel()

	// "full" is a built-in profile that always exists.
	tool := GetProfileTool("")
	result, err := tool.Handler(context.Background(), json.RawMessage(`{"name":"full"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if out["name"] != "full" {
		t.Errorf("expected name 'full', got %v", out["name"])
	}

	// source_tier must be present.
	tier, ok := out["source_tier"].(string)
	if !ok || tier == "" {
		t.Errorf("expected non-empty source_tier, got %v", out["source_tier"])
	}

	// model field must be present (even if empty string).
	if _, exists := out["model"]; !exists {
		t.Error("expected 'model' field in profile response")
	}
}

// TestGetProfileTool_UnknownProfileReturnsError verifies error for an unknown profile name.
func TestGetProfileTool_UnknownProfileReturnsError(t *testing.T) {
	t.Parallel()

	tool := GetProfileTool("")
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"name":"no-such-profile-xyz"}`))
	if err == nil {
		t.Fatal("expected error for unknown profile name")
	}
}

// TestGetProfileTool_MissingNameReturnsError verifies error when name is not provided.
func TestGetProfileTool_MissingNameReturnsError(t *testing.T) {
	t.Parallel()

	tool := GetProfileTool("")
	_, err := tool.Handler(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error when name is missing")
	}
}
