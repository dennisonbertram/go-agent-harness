//go:build integration

package workspace

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestHetznerProvider_FullLifecycle(t *testing.T) {
	apiKey := os.Getenv("HETZNER_API_KEY")
	if apiKey == "" {
		t.Skip("HETZNER_API_KEY not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	provider := NewHetznerProvider(apiKey)
	vm, err := provider.Create(ctx, VMCreateOpts{
		Name:       "test-workspace-lifecycle",
		ImageName:  "ubuntu-24.04",
		ServerType: "cx22",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	t.Logf("Created VM %s with IP %s (status: %s)", vm.ID, vm.PublicIP, vm.Status)

	if vm.ID == "" {
		t.Error("expected non-empty VM ID")
	}
	if vm.PublicIP == "" {
		t.Error("expected non-empty VM PublicIP")
	}
	if vm.Status == "" {
		t.Error("expected non-empty VM Status")
	}

	defer func() {
		if err := provider.Delete(ctx, vm.ID); err != nil {
			t.Errorf("Delete failed: %v", err)
		}
		t.Logf("Deleted VM %s", vm.ID)
	}()
}

func TestVMWorkspace_FullLifecycle(t *testing.T) {
	apiKey := os.Getenv("HETZNER_API_KEY")
	if apiKey == "" {
		t.Skip("HETZNER_API_KEY not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	provider := NewHetznerProvider(apiKey)
	w := NewVM(provider)

	if err := w.Provision(ctx, Options{ID: "integration-test-185"}); err != nil {
		t.Fatalf("Provision failed: %v", err)
	}
	t.Logf("Provisioned workspace: harnessURL=%s workspacePath=%s", w.HarnessURL(), w.WorkspacePath())

	if w.HarnessURL() == "" {
		t.Error("expected non-empty HarnessURL after Provision")
	}
	if w.WorkspacePath() != "/workspace" {
		t.Errorf("expected WorkspacePath=/workspace, got %q", w.WorkspacePath())
	}

	if err := w.Destroy(ctx); err != nil {
		t.Fatalf("Destroy failed: %v", err)
	}
	t.Log("Workspace destroyed")
}
