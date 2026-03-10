package deploy

import (
	"context"
	"strings"
	"testing"
)

// compile-time assertion that adapters implement Platform.
var _ Platform = (*RailwayAdapter)(nil)
var _ Platform = (*FlyAdapter)(nil)

// TestDeployPlatformInterface verifies all adapters satisfy the Platform interface
// and return consistent platform names.
func TestDeployPlatformInterface(t *testing.T) {
	platforms := []Platform{
		NewRailwayAdapter(nil),
		NewFlyAdapter(nil),
	}
	for _, p := range platforms {
		name := p.Name()
		if name == "" {
			t.Errorf("platform %T returned empty name", p)
		}
	}
}

// TestMockDeployWorkflow exercises the end-to-end flow: detect → deploy → status → logs.
func TestMockDeployWorkflow(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "fly.toml")

	// Step 1: detect
	detected, err := DetectPlatform(dir)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if detected != "flyio" {
		t.Fatalf("expected 'flyio', got %q", detected)
	}

	// Step 2: deploy (dry-run to avoid needing CLI)
	platform := NewFlyAdapter(func(ctx context.Context, dir, cmd string, args ...string) (string, error) {
		return "v99 deployed to https://test.fly.dev Hostname = test.fly.dev", nil
	})

	result, err := platform.Deploy(context.Background(), dir, DeployOpts{Environment: "production", DryRun: true})
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	if result.Platform != "flyio" {
		t.Errorf("deploy result platform: got %q, want 'flyio'", result.Platform)
	}

	// Step 3: status
	status, err := platform.Status(context.Background(), dir)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status == nil {
		t.Fatal("status returned nil")
	}

	// Step 4: logs
	reader, err := platform.Logs(context.Background(), dir, false)
	if err != nil {
		t.Fatalf("logs: %v", err)
	}
	var buf strings.Builder
	tmp := make([]byte, 256)
	for {
		n, readErr := reader.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
		}
		if readErr != nil {
			break
		}
	}
	// Logs might be empty in the mock, just ensure no panic.
	_ = buf.String()
}

// TestDefaultExecNotNilForRailway verifies that nil exec falls back to DefaultExec.
func TestDefaultExecNotNilForRailway(t *testing.T) {
	r := NewRailwayAdapter(nil)
	if r.exec == nil {
		t.Error("expected exec to be non-nil (defaulted to DefaultExec)")
	}
}

// TestDefaultExecNotNilForFly verifies that nil exec falls back to DefaultExec.
func TestDefaultExecNotNilForFly(t *testing.T) {
	f := NewFlyAdapter(nil)
	if f.exec == nil {
		t.Error("expected exec to be non-nil (defaulted to DefaultExec)")
	}
}
