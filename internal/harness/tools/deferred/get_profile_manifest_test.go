package deferred

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	tools "go-agent-harness/internal/harness/tools"
)

func TestGetProfileManifestTool_Definition(t *testing.T) {
	tool := GetProfileManifestTool(func(profileName string) (map[string]any, error) {
		return map[string]any{"profile_name": profileName}, nil
	})

	assertToolDef(t, tool, "get_profile_manifest", tools.TierDeferred)
	assertHasTags(t, tool, "profiles", "tools", "manifest")
}

func TestGetProfileManifestTool_Handler_Success(t *testing.T) {
	tool := GetProfileManifestTool(func(profileName string) (map[string]any, error) {
		return map[string]any{
			"profile_name": profileName,
			"visible_tools": []map[string]any{
				{"name": "read", "tier": "core"},
			},
		}, nil
	})

	result, err := tool.Handler(context.Background(), json.RawMessage(`{"profile_name":"full"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if payload["profile_name"] != "full" {
		t.Fatalf("profile_name = %v, want full", payload["profile_name"])
	}
}

func TestGetProfileManifestTool_Handler_RequiresProfileName(t *testing.T) {
	tool := GetProfileManifestTool(func(string) (map[string]any, error) {
		return nil, nil
	})

	_, err := tool.Handler(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected missing profile_name error")
	}
	if !strings.Contains(err.Error(), "profile_name is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}
