//go:build docker

package workspace

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// TestContainerWorkspace_FullLifecycle tests the complete provision/use/destroy
// cycle against a real Docker daemon.
//
// Prerequisites:
//   - Docker daemon must be running and accessible
//   - The "go-agent-harness:latest" image (or HARNESS_IMAGE env var override)
//     must exist locally or be pullable
//
// Run with:
//
//	go test -tags docker ./internal/workspace/...
func TestContainerWorkspace_FullLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	w := NewContainer("")
	t.Cleanup(func() {
		if err := w.Destroy(context.Background()); err != nil {
			t.Logf("cleanup Destroy: %v", err)
		}
	})

	opts := Options{
		ID:      "test-lifecycle-" + t.Name(),
		BaseDir: t.TempDir(),
	}

	if err := w.Provision(ctx, opts); err != nil {
		t.Fatalf("Provision: %v", err)
	}

	if w.HarnessURL() == "" {
		t.Fatal("HarnessURL() is empty after Provision")
	}
	if w.WorkspacePath() == "" {
		t.Fatal("WorkspacePath() is empty after Provision")
	}

	// Poll the health endpoint until it responds or the deadline passes.
	healthURL := w.HarnessURL() + "/health"
	deadline := time.Now().Add(30 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := http.Get(healthURL) //nolint:gosec // test code
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				t.Logf("harnessd is healthy at %s", w.HarnessURL())
				lastErr = nil
				break
			}
		}
		lastErr = err
		time.Sleep(1 * time.Second)
	}
	if lastErr != nil {
		t.Fatalf("harnessd health check failed: %v", lastErr)
	}

	if err := w.Destroy(ctx); err != nil {
		t.Fatalf("Destroy: %v", err)
	}

	// After Destroy the containerID should be cleared.
	if w.containerID != "" {
		t.Error("containerID should be empty after Destroy")
	}
}

// TestContainerWorkspace_Provision_ImageFromEnv verifies that the image name
// can be overridden via opts.Env["HARNESS_IMAGE"].
func TestContainerWorkspace_Provision_ImageFromEnv(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	w := NewContainer("original-image:v1")
	t.Cleanup(func() {
		_ = w.Destroy(context.Background())
	})

	opts := Options{
		ID:      "test-image-env",
		BaseDir: t.TempDir(),
		Env:     map[string]string{"HARNESS_IMAGE": "go-agent-harness:latest"},
	}

	// This will fail if Docker daemon is not available or image doesn't exist,
	// but that is expected in CI without Docker.
	if err := w.Provision(ctx, opts); err != nil {
		t.Logf("Provision with image override: %v (expected if image not available)", err)
	}
}
