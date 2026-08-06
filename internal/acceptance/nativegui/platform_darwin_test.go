//go:build darwin && cgo

package nativegui

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// This test uses only the test process's AX identity and an impossible window
// PID. None of these APIs request TCC permission or target a user application.
func TestDarwinCorePlatformFailsSafelyWithoutRenderedApp(t *testing.T) {
	platform := DarwinCorePlatform{}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if err := platform.SubmitPrompt(ctx, os.Getpid(), "test prompt"); err == nil {
		t.Fatal("test process unexpectedly exposed a rendered GoCode composer")
	}

	snapshot, err := platform.AccessibilitySnapshot(context.Background(), os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(snapshot), "{") {
		t.Fatalf("AX snapshot is not JSON: %q", snapshot)
	}

	if err := platform.CaptureScreenshot(context.Background(), 1<<30, t.TempDir()+"/screen.png"); err == nil {
		t.Fatal("impossible PID unexpectedly produced a screenshot")
	}
}

func TestDarwinAXHelpersOperateOnlyOnSuppliedReference(t *testing.T) {
	tree := axNode{Role: "AXApplication", Children: []axNode{{Role: "AXButton", Title: "Send message"}}}
	if got := findAXNode(&tree, func(node *axNode) bool { return node.Title == "Send message" }); got == nil {
		t.Fatal("findAXNode did not return the supplied child")
	}

	root := axApplication(os.Getpid())
	if root == nil {
		t.Fatal("could not create AX identity for test process")
	}
	_ = axSetBool(root, "AXFrontmost", false)
	_ = axPerform(root, "AXRaise")
	releaseAXTree(&axNode{ref: root})
}
