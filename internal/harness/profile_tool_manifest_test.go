package harness

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	htools "go-agent-harness/internal/harness/tools"
)

type manifestTestMCPRegistry struct{}

func (m *manifestTestMCPRegistry) ListResources(_ context.Context, _ string) ([]htools.MCPResource, error) {
	return nil, nil
}

func (m *manifestTestMCPRegistry) ReadResource(_ context.Context, _, _ string) (string, error) {
	return "", nil
}

func (m *manifestTestMCPRegistry) ListTools(_ context.Context) (map[string][]htools.MCPToolDefinition, error) {
	return map[string][]htools.MCPToolDefinition{
		"demo": {
			{
				Name:        "Search",
				Description: "Search demo resources",
				Parameters:  map[string]any{"type": "object"},
			},
		},
	}, nil
}

func (m *manifestTestMCPRegistry) CallTool(_ context.Context, _, _ string, _ json.RawMessage) (string, error) {
	return `{"ok":true}`, nil
}

func writeManifestTestProfile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir profile dir: %v", err)
	}
	path := filepath.Join(dir, name+".toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write profile %s: %v", name, err)
	}
}

func writeManifestTestScriptTool(t *testing.T, toolsDir, name string) {
	t.Helper()
	toolDir := filepath.Join(toolsDir, name)
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatalf("mkdir script tool dir: %v", err)
	}
	manifest := `{
  "name": "hello_script",
  "description": "Test script tool",
  "parameters": {"type":"object","properties":{}}
}`
	if err := os.WriteFile(filepath.Join(toolDir, "tool.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write tool manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(toolDir, "run.sh"), []byte("#!/bin/sh\nprintf '{}'\n"), 0o755); err != nil {
		t.Fatalf("write run.sh: %v", err)
	}
}

func findManifestEntry(entries []ToolManifestEntry, name string) (ToolManifestEntry, bool) {
	for _, entry := range entries {
		if entry.Name == name {
			return entry, true
		}
	}
	return ToolManifestEntry{}, false
}

func TestBuildProfileToolManifest_UnrestrictedBuiltInProfile(t *testing.T) {
	t.Parallel()

	manifest, err := BuildProfileToolManifest(t.TempDir(), "", "", "full", DefaultRegistryOptions{
		ApprovalMode: ToolApprovalModeFullAuto,
	})
	if err != nil {
		t.Fatalf("BuildProfileToolManifest: %v", err)
	}

	if manifest.ProfileName != "full" {
		t.Fatalf("profile_name = %q, want full", manifest.ProfileName)
	}
	if manifest.ProfileSourceTier != "built-in" {
		t.Fatalf("profile_source_tier = %q, want built-in", manifest.ProfileSourceTier)
	}
	if manifest.AllowedToolsRestricted {
		t.Fatal("expected unrestricted built-in profile")
	}

	readEntry, ok := findManifestEntry(manifest.VisibleTools, "read")
	if !ok {
		t.Fatal("expected read in visible tools")
	}
	if readEntry.Tier != htools.TierCore {
		t.Fatalf("read tier = %q, want %q", readEntry.Tier, htools.TierCore)
	}
	if !readEntry.VisibleByDefault {
		t.Fatal("expected read to be visible by default")
	}

	profileEntry, ok := findManifestEntry(manifest.DeferredTools, "get_profile")
	if !ok {
		t.Fatal("expected get_profile in deferred tools")
	}
	if profileEntry.Tier != htools.TierDeferred {
		t.Fatalf("get_profile tier = %q, want %q", profileEntry.Tier, htools.TierDeferred)
	}
	if profileEntry.Source != "built_in" {
		t.Fatalf("get_profile source = %q, want built_in", profileEntry.Source)
	}
}

func TestBuildProfileToolManifest_RestrictedProfileIncludesScriptAndMCP(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	userDir := filepath.Join(t.TempDir(), "profiles")
	scriptToolsDir := filepath.Join(t.TempDir(), "script-tools")
	writeManifestTestScriptTool(t, scriptToolsDir, "hello_script")

	writeManifestTestProfile(t, userDir, "tool-inspector", `
[meta]
name = "tool-inspector"
description = "Inspect a restricted manifest"
created_by = "user"

[runner]
model = "gpt-4.1-mini"
max_steps = 4

[tools]
allow = ["read", "get_profile", "mcp_demo_search", "hello_script"]
`)

	manifest, err := BuildProfileToolManifest(workspaceRoot, "", userDir, "tool-inspector", DefaultRegistryOptions{
		ApprovalMode:   ToolApprovalModeFullAuto,
		ProfilesDir:    userDir,
		MCPRegistry:    &manifestTestMCPRegistry{},
		ScriptToolsDir: scriptToolsDir,
	})
	if err != nil {
		t.Fatalf("BuildProfileToolManifest: %v", err)
	}

	if !manifest.AllowedToolsRestricted {
		t.Fatal("expected restricted profile")
	}
	if len(manifest.DeclaredAllowedTools) != 4 {
		t.Fatalf("declared allowed tools = %v, want 4 items", manifest.DeclaredAllowedTools)
	}

	if _, ok := findManifestEntry(manifest.VisibleTools, "read"); !ok {
		t.Fatal("expected read in visible tools")
	}
	if _, ok := findManifestEntry(manifest.VisibleTools, "find_tool"); !ok {
		t.Fatal("expected find_tool in visible tools as always-available infrastructure")
	}
	if _, ok := findManifestEntry(manifest.VisibleTools, "bash"); ok {
		t.Fatal("did not expect bash in restricted visible tools")
	}

	scriptEntry, ok := findManifestEntry(manifest.DeferredTools, "hello_script")
	if !ok {
		t.Fatal("expected hello_script in deferred tools")
	}
	if scriptEntry.Source != "script" {
		t.Fatalf("hello_script source = %q, want script", scriptEntry.Source)
	}

	mcpEntry, ok := findManifestEntry(manifest.DeferredTools, "mcp_demo_search")
	if !ok {
		t.Fatal("expected mcp_demo_search in deferred tools")
	}
	if mcpEntry.Source != "mcp" {
		t.Fatalf("mcp_demo_search source = %q, want mcp", mcpEntry.Source)
	}

	profileEntry, ok := findManifestEntry(manifest.DeferredTools, "get_profile")
	if !ok {
		t.Fatal("expected get_profile in deferred tools")
	}
	if profileEntry.Source != "built_in" {
		t.Fatalf("get_profile source = %q, want built_in", profileEntry.Source)
	}
}

func TestBuildProfileToolManifestWithRegistry_UsesDynamicMCPRegistrations(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	userDir := filepath.Join(t.TempDir(), "profiles")
	writeManifestTestProfile(t, userDir, "dynamic-mcp", `
[meta]
name = "dynamic-mcp"
description = "Inspect live MCP registrations"
created_by = "user"

[runner]
model = "gpt-4.1-mini"
max_steps = 4

[tools]
allow = ["mcp_demo_search"]
`)

	registry := NewDefaultRegistryWithOptions(workspaceRoot, DefaultRegistryOptions{
		ApprovalMode: ToolApprovalModeFullAuto,
		ProfilesDir:  userDir,
	})

	_, err := registry.RegisterMCPTools("demo", []htools.MCPToolDefinition{
		{
			Name:        "Search",
			Description: "Search demo resources",
			Parameters:  map[string]any{"type": "object"},
		},
	}, &manifestTestMCPRegistry{})
	if err != nil {
		t.Fatalf("RegisterMCPTools: %v", err)
	}

	manifest, err := BuildProfileToolManifestWithRegistry("", userDir, "dynamic-mcp", registry)
	if err != nil {
		t.Fatalf("BuildProfileToolManifestWithRegistry: %v", err)
	}

	entry, ok := findManifestEntry(manifest.DeferredTools, "mcp_demo_search")
	if !ok {
		t.Fatal("expected dynamically registered MCP tool in deferred tools")
	}
	if entry.Source != "mcp" {
		t.Fatalf("mcp_demo_search source = %q, want mcp", entry.Source)
	}
}

func TestDefaultRegistry_GetProfileManifestDiscoverableViaFindTool(t *testing.T) {
	t.Parallel()

	activations := NewActivationTracker()
	registry := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode: ToolApprovalModeFullAuto,
		Activations:  activations,
	})

	findToolDef, ok := findManifestEntry(filterManifestEntries(registry.DefinitionsWithMetadata(), nil), "find_tool")
	if !ok {
		t.Fatal("expected find_tool in resolved registry metadata")
	}
	if !strings.Contains(findToolDef.Description, "get_profile_manifest") {
		t.Fatalf("find_tool description does not advertise get_profile_manifest: %q", findToolDef.Description)
	}

	ctx := context.WithValue(context.Background(), htools.ContextKeyRunID, "run-1")
	if _, err := registry.Execute(ctx, "find_tool", json.RawMessage(`{"query":"select:get_profile_manifest"}`)); err != nil {
		t.Fatalf("find_tool select get_profile_manifest: %v", err)
	}

	defs := registry.DefinitionsForRun("run-1", activations)
	found := false
	for _, def := range defs {
		if def.Name == "get_profile_manifest" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected get_profile_manifest to be activated into the run tool set")
	}
}
