package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
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

func TestContainerWorkspace_Provision_TestIDUniquePerCall(t *testing.T) {
	first := containerWorkspaceTestID(t, "test-provision")
	second := containerWorkspaceTestID(t, "test-provision")

	if first == second {
		t.Fatalf("containerWorkspaceTestID returned duplicate ID %q", first)
	}
	if !strings.HasPrefix(first, "test-provision-") {
		t.Fatalf("containerWorkspaceTestID() = %q, want readable test-provision prefix", first)
	}
	if strings.ContainsAny(first+second, "/ \t\n") {
		t.Fatalf("containerWorkspaceTestID returned Docker-hostile IDs %q and %q", first, second)
	}
}

func TestContainerWorkspace_Provision_ConflictIsNotSkipped(t *testing.T) {
	err := errors.New(`workspace: container create: Conflict. The container name "/workspace-test-provision" is already in use`)
	if isContainerWorkspaceProvisionEnvironmentUnavailable(err) {
		t.Fatal("container name conflicts must remain test failures")
	}
}

var containerWorkspaceTestIDSeq atomic.Uint64

func containerWorkspaceTestID(t *testing.T, prefix string) string {
	t.Helper()

	return fmt.Sprintf("%s-%d-%d", sanitizeBranch(prefix), time.Now().UnixNano(), containerWorkspaceTestIDSeq.Add(1))
}

func TestContainerWorkspace_Provision_Success(t *testing.T) {
	// We test the basic behaviour with a minimal environment and a valid ID.
	// This is a very shallow test that will check for no immediate errors
	// and output URL and workspace path are consistent.

	ctx := context.Background()
	w := NewContainer("")
	t.Cleanup(func() {
		if err := w.Destroy(context.Background()); err != nil {
			t.Logf("cleanup Destroy: %v", err)
		}
	})
	opts := Options{
		ID:      containerWorkspaceTestID(t, "test-provision"),
		BaseDir: t.TempDir(),
		Env:     map[string]string{},
	}

	err := w.Provision(ctx, opts)
	if err != nil {
		if isContainerWorkspaceProvisionEnvironmentUnavailable(err) {
			t.Skipf("container workspace provisioning unavailable in this environment: %v", err)
		}
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

func isContainerWorkspaceProvisionEnvironmentUnavailable(err error) bool {
	msg := err.Error()
	unavailableSubstrings := []string{
		"bind: operation not permitted",
		"Cannot connect to the Docker daemon",
		"connect: no such file or directory",
		"permission denied while trying to connect to the Docker daemon",
		"No such image",
	}
	for _, substring := range unavailableSubstrings {
		if strings.Contains(msg, substring) {
			return true
		}
	}
	return false
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
