package workspace_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go-agent-harness/internal/workspace"
)

// TestLocalWorkspace_WritesConfigTOML verifies that when opts.ConfigTOML is
// non-empty, Provision() writes harness.toml to the workspace root.
func TestLocalWorkspace_WritesConfigTOML(t *testing.T) {
	base := t.TempDir()
	ws := workspace.NewLocal("http://localhost:8080", base)

	const tomlContent = `model = "gpt-4.1"
auto_compact_enabled = true
trace_tool_decisions = true
`

	opts := workspace.Options{
		ID:         "config-toml-test",
		ConfigTOML: tomlContent,
	}
	if err := ws.Provision(context.Background(), opts); err != nil {
		t.Fatalf("Provision: unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Destroy(context.Background()) })

	// harness.toml must exist in the workspace root.
	cfgPath := filepath.Join(ws.WorkspacePath(), "harness.toml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("harness.toml not written: %v", err)
	}
	if string(data) != tomlContent {
		t.Errorf("harness.toml content mismatch:\ngot:  %q\nwant: %q", string(data), tomlContent)
	}
}

// TestLocalWorkspace_NoConfigTOML verifies that when opts.ConfigTOML is empty,
// Provision() does NOT write harness.toml to the workspace root.
func TestLocalWorkspace_NoConfigTOML(t *testing.T) {
	base := t.TempDir()
	ws := workspace.NewLocal("http://localhost:8080", base)

	opts := workspace.Options{
		ID:         "no-config-toml-test",
		ConfigTOML: "", // empty: no file should be written
	}
	if err := ws.Provision(context.Background(), opts); err != nil {
		t.Fatalf("Provision: unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Destroy(context.Background()) })

	cfgPath := filepath.Join(ws.WorkspacePath(), "harness.toml")
	if _, err := os.Stat(cfgPath); !os.IsNotExist(err) {
		t.Errorf("harness.toml unexpectedly exists at %q (should not be written when ConfigTOML is empty)", cfgPath)
	}
}

// TestLocalWorkspace_ConfigTOMLPermissions verifies that the written harness.toml
// has restrictive permissions (0600 — not world-readable).
func TestLocalWorkspace_ConfigTOMLPermissions(t *testing.T) {
	base := t.TempDir()
	ws := workspace.NewLocal("http://localhost:8080", base)

	opts := workspace.Options{
		ID:         "perm-test",
		ConfigTOML: `model = "gpt-4o"`,
	}
	if err := ws.Provision(context.Background(), opts); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	t.Cleanup(func() { _ = ws.Destroy(context.Background()) })

	cfgPath := filepath.Join(ws.WorkspacePath(), "harness.toml")
	info, err := os.Stat(cfgPath)
	if err != nil {
		t.Fatalf("Stat(%q): %v", cfgPath, err)
	}
	perm := info.Mode().Perm()
	// Must be 0600 or more restrictive (never world-readable).
	if perm&0o044 != 0 {
		t.Errorf("harness.toml has unsafe permissions: %o (group/other readable bits set)", perm)
	}
}

// TestOptionsConfigTOMLField verifies that workspace.Options has the ConfigTOML
// field and it defaults to the zero value (empty string).
func TestOptionsConfigTOMLField(t *testing.T) {
	opts := workspace.Options{ID: "test"}
	if opts.ConfigTOML != "" {
		t.Errorf("Options.ConfigTOML default: got %q, want empty string", opts.ConfigTOML)
	}
}
