//go:build !darwin || !cgo

package nativegui

import (
	"context"
	"strings"
	"testing"
)

// TestDarwinCorePlatformStubReportsUnsupportedPlatform pins the documented
// behavior of the non-darwin (or cgo-disabled) build: every DarwinCorePlatform
// method must fail rather than silently no-op, so a caller on this platform
// gets an explicit "unavailable" error instead of a false pass.
func TestDarwinCorePlatformStubReportsUnsupportedPlatform(t *testing.T) {
	platform := DarwinCorePlatform{}
	ctx := context.Background()

	if err := platform.SubmitPrompt(ctx, 1, "test prompt"); err == nil || !strings.Contains(err.Error(), "unavailable on this platform") {
		t.Fatalf("SubmitPrompt error = %v, want unavailable-on-this-platform error", err)
	}

	snapshot, err := platform.AccessibilitySnapshot(ctx, 1)
	if err == nil || !strings.Contains(err.Error(), "unavailable on this platform") {
		t.Fatalf("AccessibilitySnapshot error = %v, want unavailable-on-this-platform error", err)
	}
	if snapshot != nil {
		t.Fatalf("AccessibilitySnapshot snapshot = %v, want nil alongside the error", snapshot)
	}

	if err := platform.CaptureScreenshot(ctx, 1, "/tmp/does-not-matter.png"); err == nil || !strings.Contains(err.Error(), "unavailable on this platform") {
		t.Fatalf("CaptureScreenshot error = %v, want unavailable-on-this-platform error", err)
	}
}
