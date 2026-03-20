package deferred

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	tools "go-agent-harness/internal/harness/tools"
)

// TestListProfilesTool_Definition verifies the list_profiles tool constructor.
func TestListProfilesTool_Definition(t *testing.T) {
	tool := ListProfilesTool("")
	assertToolDef(t, tool, "list_profiles", tools.TierDeferred)
	assertHasTags(t, tool, "profiles", "agent")
}

// TestListProfilesTool_ReturnsBuiltinProfiles verifies built-ins are always included.
func TestListProfilesTool_ReturnsBuiltinProfiles(t *testing.T) {
	t.Parallel()

	// No user/project dir — only built-ins should appear.
	tool := ListProfilesTool("")
	result, err := tool.Handler(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	profiles, ok := out["profiles"].([]any)
	if !ok {
		t.Fatalf("expected 'profiles' array in result, got %T", out["profiles"])
	}
	if len(profiles) == 0 {
		t.Fatal("expected at least one built-in profile")
	}
}

// TestListProfilesTool_IncludesSourceTier verifies source_tier field is present in each profile.
func TestListProfilesTool_IncludesSourceTier(t *testing.T) {
	t.Parallel()

	tool := ListProfilesTool("")
	result, err := tool.Handler(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	profiles, ok := out["profiles"].([]any)
	if !ok {
		t.Fatalf("expected 'profiles' array")
	}

	for i, raw := range profiles {
		p, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("profile[%d] is not an object", i)
		}
		tier, ok := p["source_tier"].(string)
		if !ok || tier == "" {
			t.Errorf("profile[%d] missing or empty source_tier", i)
		}
	}
}

// TestListProfilesTool_ProjectOverridesBuiltin verifies project tier takes precedence.
func TestListProfilesTool_ProjectOverridesBuiltin(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Write a project-level profile with name "full" (same as built-in).
	content := `
[meta]
name = "full"
description = "project override"

[runner]
model = "project-model"
max_steps = 5
`
	if err := os.WriteFile(filepath.Join(dir, "full.toml"), []byte(content), 0644); err != nil {
		t.Fatalf("write toml: %v", err)
	}

	tool := ListProfilesToolWithDirs(dir, "")
	result, err := tool.Handler(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	profiles, ok := out["profiles"].([]any)
	if !ok {
		t.Fatalf("expected 'profiles' array")
	}

	// Find the "full" profile.
	var found map[string]any
	for _, raw := range profiles {
		p, _ := raw.(map[string]any)
		if p["name"] == "full" {
			found = p
			break
		}
	}
	if found == nil {
		t.Fatal("expected 'full' profile in list")
	}
	if found["source_tier"] != "project" {
		t.Errorf("expected source_tier 'project', got %q", found["source_tier"])
	}
	if found["model"] != "project-model" {
		t.Errorf("expected model 'project-model', got %q", found["model"])
	}
}
