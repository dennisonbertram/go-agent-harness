package workspace

import (
	"context"
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
