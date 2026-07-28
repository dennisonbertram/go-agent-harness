package tools

// TestCommonPathsAndHelpers exercises the shared path/exec helpers that stayed
// in this package after the duplicate tool catalog was removed.
//
// The rest of this file asserted behaviour of the deleted BuildCatalog builder
// and of the duplicate tool implementations. Per-tool behaviour is covered by
// the tests in tools/core and tools/deferred; the cross-cutting invariants it
// carried (every tool has a non-empty description, conditional registration,
// policy wrapping, SSRF guarding) moved to internal/harness, where they now run
// against the surviving registry.

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCommonPathsAndHelpers(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if err := validateWorkspaceRelativePattern("../bad"); err == nil {
		t.Fatalf("expected pattern escape error")
	}
	if err := validateWorkspaceRelativePattern("*.go"); err != nil {
		t.Fatalf("expected valid pattern: %v", err)
	}
	if err := ValidateWorkspaceRelativePattern("ok/*.txt"); err != nil {
		t.Fatalf("expected exported helper to pass: %v", err)
	}

	abs, err := ResolveWorkspacePath(workspace, "a/b.txt")
	if err != nil {
		t.Fatalf("resolve path: %v", err)
	}
	if got := NormalizeRelPath(workspace, abs); got != "a/b.txt" {
		t.Fatalf("unexpected normalized path %q", got)
	}

	if _, err := BuildLineMatcher("(", true, false); err == nil {
		t.Fatalf("expected regex compile error")
	}
	matcher, err := BuildLineMatcher("Needle", false, false)
	if err != nil {
		t.Fatalf("build matcher: %v", err)
	}
	if !matcher("contains needle") {
		t.Fatalf("expected case-insensitive match")
	}

	if _, _, timedOut, err := RunCommand(context.Background(), 20*time.Millisecond, "bash", "-lc", "sleep 0.2"); err != nil || !timedOut {
		t.Fatalf("expected timeout branch")
	}
	output, exitCode, timedOut, err := RunCommand(context.Background(), 2*time.Second, "bash", "-lc", "echo hi; exit 3")
	if err != nil {
		t.Fatalf("run command non-zero: %v", err)
	}
	if exitCode != 3 || timedOut || !strings.Contains(output, "hi") {
		t.Fatalf("unexpected command branch output=%q code=%d timeout=%v", output, exitCode, timedOut)
	}
	if !IsDangerousCommand("rm -rf /") {
		t.Fatalf("expected dangerous command detection")
	}
}
