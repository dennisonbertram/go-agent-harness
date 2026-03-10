package deferred

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-agent-harness/internal/skills/packs"
)

// writeTestPack creates a test pack directory with manifest and instructions.
func writeTestPack(t *testing.T, dir, name, manifest, instructions string) {
	t.Helper()
	packDir := filepath.Join(dir, name)
	if err := os.MkdirAll(packDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packDir, name+".yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packDir, name+".md"), []byte(instructions), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newTestRegistry(t *testing.T) (*packs.PackRegistry, string) {
	t.Helper()
	dir := t.TempDir()

	manifest1 := `name: dev-tools
display_name: "Development Tools"
category: development
description: "Common development workflow helpers"
version: 1
instructions: dev-tools.md
tags:
  - dev
  - workflow
`
	manifest2 := `name: deploy-pack
display_name: "Deployment Pack"
category: deployment
description: "Deployment workflows for cloud platforms"
version: 1
instructions: deploy-pack.md
allowed_tools:
  - bash
  - read
tags:
  - deploy
  - cloud
`
	writeTestPack(t, dir, "dev-tools", manifest1, "# Dev Tools\n\nDev workflow instructions.")
	writeTestPack(t, dir, "deploy-pack", manifest2, "# Deploy Pack\n\nDeployment instructions.")

	r, err := packs.NewPackRegistry(dir)
	if err != nil {
		t.Fatalf("NewPackRegistry() error = %v", err)
	}
	return r, dir
}

func callTool(t *testing.T, registry *packs.PackRegistry, args map[string]any) (string, error) {
	t.Helper()
	tool := ManageSkillPacksTool(registry)
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	return tool.Handler(context.Background(), raw)
}

func TestManageSkillPacksTool_List(t *testing.T) {
	registry, _ := newTestRegistry(t)

	out, err := callTool(t, registry, map[string]any{"action": "list"})
	if err != nil {
		t.Fatalf("list error: %v", err)
	}
	if !strings.Contains(out, "dev-tools") {
		t.Errorf("output should contain dev-tools: %s", out)
	}
	if !strings.Contains(out, "deploy-pack") {
		t.Errorf("output should contain deploy-pack: %s", out)
	}
}

func TestManageSkillPacksTool_Search_Found(t *testing.T) {
	registry, _ := newTestRegistry(t)

	out, err := callTool(t, registry, map[string]any{"action": "search", "query": "deploy"})
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if !strings.Contains(out, "deploy-pack") {
		t.Errorf("output should contain deploy-pack: %s", out)
	}
}

func TestManageSkillPacksTool_Search_NotFound(t *testing.T) {
	registry, _ := newTestRegistry(t)

	out, err := callTool(t, registry, map[string]any{"action": "search", "query": "xyzzy-no-match"})
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if !strings.Contains(out, `"count"`) {
		t.Errorf("output should contain count field: %s", out)
	}
}

func TestManageSkillPacksTool_Search_EmptyQuery(t *testing.T) {
	registry, _ := newTestRegistry(t)

	_, err := callTool(t, registry, map[string]any{"action": "search", "query": ""})
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestManageSkillPacksTool_Activate_Valid(t *testing.T) {
	registry, _ := newTestRegistry(t)

	out, err := callTool(t, registry, map[string]any{"action": "activate", "name": "dev-tools"})
	if err != nil {
		t.Fatalf("activate error: %v", err)
	}
	// Should be a wrapped result with meta-message
	if !strings.Contains(out, "__tool_result__") {
		t.Errorf("activate should return a wrapped tool result: %s", out)
	}
	if !strings.Contains(out, "activated") {
		t.Errorf("output should contain 'activated': %s", out)
	}
}

func TestManageSkillPacksTool_Activate_ContainsMetaMessage(t *testing.T) {
	registry, _ := newTestRegistry(t)

	out, err := callTool(t, registry, map[string]any{"action": "activate", "name": "deploy-pack"})
	if err != nil {
		t.Fatalf("activate error: %v", err)
	}
	if !strings.Contains(out, "meta_messages") {
		t.Errorf("activate should include meta_messages: %s", out)
	}
	if !strings.Contains(out, "skill_pack") {
		t.Errorf("meta message should contain skill_pack tag: %s", out)
	}
	if !strings.Contains(out, "Deployment instructions") {
		t.Errorf("meta message should contain instructions text: %s", out)
	}
}

func TestManageSkillPacksTool_Activate_WithAllowedTools(t *testing.T) {
	registry, _ := newTestRegistry(t)

	out, err := callTool(t, registry, map[string]any{"action": "activate", "name": "deploy-pack"})
	if err != nil {
		t.Fatalf("activate error: %v", err)
	}
	if !strings.Contains(out, "allowed_tools") {
		t.Errorf("output should contain allowed_tools: %s", out)
	}
}

func TestManageSkillPacksTool_Activate_NotFound(t *testing.T) {
	registry, _ := newTestRegistry(t)

	_, err := callTool(t, registry, map[string]any{"action": "activate", "name": "nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent pack")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should mention pack name: %v", err)
	}
}

func TestManageSkillPacksTool_Activate_EmptyName(t *testing.T) {
	registry, _ := newTestRegistry(t)

	_, err := callTool(t, registry, map[string]any{"action": "activate", "name": ""})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestManageSkillPacksTool_UnknownAction(t *testing.T) {
	registry, _ := newTestRegistry(t)

	_, err := callTool(t, registry, map[string]any{"action": "unknown"})
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Errorf("error should mention unknown action: %v", err)
	}
}

func TestManageSkillPacksTool_BadJSON(t *testing.T) {
	registry, _ := newTestRegistry(t)
	tool := ManageSkillPacksTool(registry)
	_, err := tool.Handler(context.Background(), []byte("{bad json"))
	if err == nil {
		t.Fatal("expected error for bad JSON")
	}
}

func TestManageSkillPacksTool_Definition(t *testing.T) {
	registry, _ := newTestRegistry(t)
	tool := ManageSkillPacksTool(registry)

	if tool.Definition.Name != "manage_skill_packs" {
		t.Errorf("Name = %q", tool.Definition.Name)
	}
	if tool.Definition.Description == "" {
		t.Error("Description should not be empty")
	}
	if tool.Handler == nil {
		t.Error("Handler should not be nil")
	}
}
