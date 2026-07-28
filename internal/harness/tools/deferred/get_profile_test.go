package deferred

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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

// TestGetProfileTool_InvalidJSONReturnsError verifies get_profile rejects
// malformed JSON args instead of treating them as an empty/default request.
func TestGetProfileTool_InvalidJSONReturnsError(t *testing.T) {
	t.Parallel()

	tool := GetProfileTool("")
	_, err := tool.Handler(context.Background(), json.RawMessage(`{not-json`))
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

// writeMinimalProfile writes a minimal valid profile TOML file for tier
// resolution tests.
func writeMinimalProfile(t *testing.T, dir, name, model string) {
	t.Helper()
	content := "[meta]\nname = \"" + name + "\"\ndescription = \"tier test profile\"\n\n" +
		"[runner]\nmodel = \"" + model + "\"\nmax_steps = 3\n"
	if err := os.WriteFile(filepath.Join(dir, name+".toml"), []byte(content), 0644); err != nil {
		t.Fatalf("write profile toml: %v", err)
	}
}

// TestGetProfileToolWithDirs_ProjectTier verifies a profile found in the
// project directory is loaded with source_tier "project" and its fields
// (not the built-in defaults) are returned.
func TestGetProfileToolWithDirs_ProjectTier(t *testing.T) {
	t.Parallel()

	const name = "zz-custom-project-profile"
	projectDir := t.TempDir()
	writeMinimalProfile(t, projectDir, name, "proj-model")

	tool := GetProfileToolWithDirs(projectDir, "")
	raw, _ := json.Marshal(map[string]string{"name": name})
	result, err := tool.Handler(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["model"] != "proj-model" {
		t.Errorf("expected model 'proj-model', got %v", out["model"])
	}
	if out["source_tier"] != "project" {
		t.Errorf("expected source_tier 'project', got %v", out["source_tier"])
	}
}

// TestGetProfileToolWithDirs_UserTierFallback verifies that when a profile
// is absent from the project directory but present in the user directory,
// it resolves with source_tier "user" and the user-tier field values.
func TestGetProfileToolWithDirs_UserTierFallback(t *testing.T) {
	t.Parallel()

	const name = "zz-custom-user-profile"
	projectDir := t.TempDir() // no matching file here
	userDir := t.TempDir()
	writeMinimalProfile(t, userDir, name, "user-model")

	tool := GetProfileToolWithDirs(projectDir, userDir)
	raw, _ := json.Marshal(map[string]string{"name": name})
	result, err := tool.Handler(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["model"] != "user-model" {
		t.Errorf("expected model 'user-model', got %v", out["model"])
	}
	if out["source_tier"] != "user" {
		t.Errorf("expected source_tier 'user', got %v", out["source_tier"])
	}
}

// TestResolveSourceTier_DirectProjectAndUser exercises resolveSourceTier
// directly (it is unexported but same-package) to pin down its two success
// branches independent of the get_profile handler plumbing.
func TestResolveSourceTier_DirectProjectAndUser(t *testing.T) {
	t.Parallel()

	const name = "zz-custom-resolve-profile"
	projectDir := t.TempDir()
	userDir := t.TempDir()
	writeMinimalProfile(t, projectDir, name, "proj-model")

	if got := resolveSourceTier(name, projectDir, userDir); got != "project" {
		t.Errorf("expected 'project' when the profile exists in projectDir, got %q", got)
	}

	// Move it conceptually to user tier: a fresh empty project dir, profile
	// only in userDir.
	emptyProjectDir := t.TempDir()
	if got := resolveSourceTier(name, emptyProjectDir, userDir); got != "built-in" {
		// The profile is not yet in userDir at this point.
		t.Errorf("expected 'built-in' before the user-dir file exists, got %q", got)
	}
	writeMinimalProfile(t, userDir, name, "user-model")
	if got := resolveSourceTier(name, emptyProjectDir, userDir); got != "user" {
		t.Errorf("expected 'user' once the profile exists only in userDir, got %q", got)
	}
}
