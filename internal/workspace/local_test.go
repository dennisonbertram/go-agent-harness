package workspace_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go-agent-harness/internal/workspace"
)

// Compile-time interface compliance check.
var _ workspace.Workspace = (*workspace.LocalWorkspace)(nil)

// TestLocalWorkspace_ImplementsWorkspace is the runtime counterpart to the
// compile-time check above.
func TestLocalWorkspace_ImplementsWorkspace(t *testing.T) {
	var _ workspace.Workspace = (*workspace.LocalWorkspace)(nil)
}

func TestLocalWorkspace_Provision(t *testing.T) {
	base := t.TempDir()
	ws := workspace.NewLocal("http://localhost:8080", base)

	opts := workspace.Options{ID: "test-provision"}
	if err := ws.Provision(context.Background(), opts); err != nil {
		t.Fatalf("Provision: unexpected error: %v", err)
	}

	want := filepath.Join(base, "test-provision")
	if got := ws.WorkspacePath(); got != want {
		t.Errorf("WorkspacePath = %q, want %q", got, want)
	}

	// Directory must exist.
	if _, err := os.Stat(want); os.IsNotExist(err) {
		t.Errorf("directory %q was not created", want)
	}
}

func TestLocalWorkspace_Provision_EmptyID(t *testing.T) {
	base := t.TempDir()
	ws := workspace.NewLocal("http://localhost:8080", base)

	err := ws.Provision(context.Background(), workspace.Options{ID: ""})
	if err == nil {
		t.Fatal("Provision with empty ID: expected error, got nil")
	}
}

func TestLocalWorkspace_HarnessURL_Default(t *testing.T) {
	// Zero-value LocalWorkspace (as returned by the factory) should return the
	// default URL even before Provision is called.
	ws := workspace.NewLocal("", "")
	if got := ws.HarnessURL(); got != "http://localhost:8080" {
		t.Errorf("HarnessURL (pre-provision, no URL set) = %q, want %q", got, "http://localhost:8080")
	}
}

func TestLocalWorkspace_HarnessURL_FromEnv(t *testing.T) {
	base := t.TempDir()
	ws := workspace.NewLocal("", base)

	opts := workspace.Options{
		ID:  "env-url-test",
		Env: map[string]string{"HARNESS_URL": "http://custom-harness:9090"},
	}
	if err := ws.Provision(context.Background(), opts); err != nil {
		t.Fatalf("Provision: %v", err)
	}

	if got := ws.HarnessURL(); got != "http://custom-harness:9090" {
		t.Errorf("HarnessURL = %q, want %q", got, "http://custom-harness:9090")
	}
}

func TestLocalWorkspace_WorkspacePath_BeforeProvision(t *testing.T) {
	ws := workspace.NewLocal("http://localhost:8080", t.TempDir())
	if got := ws.WorkspacePath(); got != "" {
		t.Errorf("WorkspacePath before Provision = %q, want empty string", got)
	}
}

func TestLocalWorkspace_WorkspacePath_AfterProvision(t *testing.T) {
	base := t.TempDir()
	ws := workspace.NewLocal("http://localhost:8080", base)

	id := "ws-after-provision"
	if err := ws.Provision(context.Background(), workspace.Options{ID: id}); err != nil {
		t.Fatalf("Provision: %v", err)
	}

	want := filepath.Join(base, id)
	if got := ws.WorkspacePath(); got != want {
		t.Errorf("WorkspacePath = %q, want %q", got, want)
	}
}

func TestLocalWorkspace_Destroy(t *testing.T) {
	base := t.TempDir()
	ws := workspace.NewLocal("http://localhost:8080", base)

	id := "destroy-test"
	if err := ws.Provision(context.Background(), workspace.Options{ID: id}); err != nil {
		t.Fatalf("Provision: %v", err)
	}

	dir := ws.WorkspacePath()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Fatalf("directory %q does not exist after Provision", dir)
	}

	if err := ws.Destroy(context.Background()); err != nil {
		t.Fatalf("Destroy: unexpected error: %v", err)
	}

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("directory %q still exists after Destroy", dir)
	}
}

func TestLocalWorkspace_Destroy_NotProvisioned(t *testing.T) {
	ws := workspace.NewLocal("http://localhost:8080", t.TempDir())
	if err := ws.Destroy(context.Background()); err != nil {
		t.Errorf("Destroy (not provisioned): expected nil error, got %v", err)
	}
}

func TestLocalWorkspace_FullLifecycle(t *testing.T) {
	base := t.TempDir()
	ws := workspace.NewLocal("http://harness:8080", base)

	// Before Provision.
	if got := ws.WorkspacePath(); got != "" {
		t.Errorf("pre-provision WorkspacePath = %q, want empty", got)
	}

	opts := workspace.Options{ID: "lifecycle-test"}
	if err := ws.Provision(context.Background(), opts); err != nil {
		t.Fatalf("Provision: %v", err)
	}

	// After Provision.
	wantPath := filepath.Join(base, "lifecycle-test")
	if got := ws.WorkspacePath(); got != wantPath {
		t.Errorf("WorkspacePath = %q, want %q", got, wantPath)
	}
	if got := ws.HarnessURL(); got != "http://harness:8080" {
		t.Errorf("HarnessURL = %q, want %q", got, "http://harness:8080")
	}

	// Write a file inside to confirm it's a real directory.
	marker := filepath.Join(wantPath, "marker.txt")
	if err := os.WriteFile(marker, []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Destroy cleans everything up.
	if err := ws.Destroy(context.Background()); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
	if _, err := os.Stat(wantPath); !os.IsNotExist(err) {
		t.Errorf("directory %q still exists after Destroy", wantPath)
	}
}

func TestLocalWorkspace_BaseDir_Default(t *testing.T) {
	// When NewLocal is given an empty basePath and opts.BaseDir is also empty,
	// it should fall back to os.TempDir().
	ws := workspace.NewLocal("", "")

	id := "base-default-test"
	if err := ws.Provision(context.Background(), workspace.Options{ID: id}); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	t.Cleanup(func() { _ = ws.Destroy(context.Background()) })

	want := filepath.Join(os.TempDir(), id)
	if got := ws.WorkspacePath(); got != want {
		t.Errorf("WorkspacePath = %q, want %q", got, want)
	}
	if _, err := os.Stat(want); os.IsNotExist(err) {
		t.Errorf("directory %q was not created", want)
	}
}

func TestLocalWorkspace_CustomBaseDir(t *testing.T) {
	customBase := t.TempDir()
	ws := workspace.NewLocal("http://localhost:8080", "")

	opts := workspace.Options{
		ID:      "custom-base",
		BaseDir: customBase,
	}
	if err := ws.Provision(context.Background(), opts); err != nil {
		t.Fatalf("Provision: %v", err)
	}

	want := filepath.Join(customBase, "custom-base")
	if got := ws.WorkspacePath(); got != want {
		t.Errorf("WorkspacePath = %q, want %q", got, want)
	}
	if _, err := os.Stat(want); os.IsNotExist(err) {
		t.Errorf("directory %q was not created", want)
	}
}

// TestLocalWorkspace_RegisteredInDefaultRegistry verifies the init() function
// registered "local" in the package-level default registry.
func TestLocalWorkspace_RegisteredInDefaultRegistry(t *testing.T) {
	found := false
	for _, n := range workspace.List() {
		if n == "local" {
			found = true
			break
		}
	}
	if !found {
		t.Error(`"local" is not registered in the default workspace registry`)
	}
}
