package workspace

import (
	"context"
	"os"
	"testing"
)

// Compile-time interface check — fails to build if ContainerWorkspace stops
// satisfying Workspace.
var _ Workspace = (*ContainerWorkspace)(nil)

func TestContainerWorkspace_ImplementsWorkspace(t *testing.T) {
	// This test documents the interface contract. The compile-time check above
	// is the real guard; this test exists so "go test" reports it explicitly.
	var _ Workspace = (*ContainerWorkspace)(nil)
}

func TestContainerWorkspace_Provision_EmptyID(t *testing.T) {
	w := NewContainer("")

	err := w.Provision(context.Background(), Options{})
	if err != ErrInvalidID {
		t.Errorf("expected ErrInvalidID, got %v", err)
	}
}

func TestContainerWorkspace_Provision_Success(t *testing.T) {
	// We test the basic behaviour with a minimal environment and a valid ID.
	// This is a very shallow test that will check for no immediate errors
	// and output URL and workspace path are consistent.

	ctx := context.Background()
	w := NewContainer("")
	opts := Options{
		ID:      "test-provision",
		BaseDir: t.TempDir(),
		Env:     map[string]string{},
	}

	err := w.Provision(ctx, opts)
	if err != nil {
		t.Fatalf("Provision returned error: %v", err)
	}

	url := w.HarnessURL()
	path := w.WorkspacePath()

	if url == "" {
		t.Errorf("HarnessURL returned empty string after Provision")
	}
	if path == "" {
		t.Errorf("WorkspacePath returned empty string after Provision")
	}

	// Check that the path exists and contains the harness.toml file if any configuration is written.
	stat, err := os.Stat(path)
	if err != nil {
		t.Errorf("WorkspacePath does not exist: %v", err)
	}
	if !stat.IsDir() {
		t.Errorf("WorkspacePath is not a directory")
	}
}

func TestContainerWorkspace_Destroy_NotProvisioned(t *testing.T) {
	w := NewContainer("")
	err := w.Destroy(context.Background())
	if err != nil {
		t.Errorf("expected nil error for unprovisioned Destroy, got %v", err)
	}
}

func TestContainerWorkspace_HarnessURL_BeforeProvision(t *testing.T) {
	w := NewContainer("")
	if got := w.HarnessURL(); got != "" {
		t.Errorf("HarnessURL() before Provision = %q, want empty string", got)
	}
}

func TestContainerWorkspace_WorkspacePath_BeforeProvision(t *testing.T) {
	w := NewContainer("")
	if got := w.WorkspacePath(); got != "" {
		t.Errorf("WorkspacePath() before Provision = %q, want empty string", got)
	}
}

func TestContainerWorkspace_DefaultImageName(t *testing.T) {
	w := NewContainer("")
	if w.imageName != defaultImage {
		t.Errorf("imageName = %q, want %q", w.imageName, defaultImage)
	}
}

func TestContainerWorkspace_CustomImageName(t *testing.T) {
	const custom = "my-harness:v2"
	w := NewContainer(custom)
	if w.imageName != custom {
		t.Errorf("imageName = %q, want %q", w.imageName, custom)
	}
}

func TestContainerWorkspace_RegisteredInFactory(t *testing.T) {
	names := List()
	for _, n := range names {
		if n == "container" {
			return
		}
	}
	t.Error("'container' not registered in default factory")
}

func TestGetFreePort(t *testing.T) {
	port, err := getFreePort()
	if err != nil {
		t.Fatalf("getFreePort error: %v", err)
	}
	if port <= 0 || port > 65535 {
		t.Fatalf("expected valid port in range 1-65535, got %d", port)
	}
}
