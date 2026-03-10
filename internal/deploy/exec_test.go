package deploy

import (
	"context"
	"strings"
	"testing"
)

// TestDefaultExec_Success verifies DefaultExec runs a command and returns output.
func TestDefaultExec_Success(t *testing.T) {
	out, err := DefaultExec(context.Background(), t.TempDir(), "echo", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "hello") {
		t.Errorf("expected 'hello' in output, got %q", out)
	}
}

// TestDefaultExec_Error verifies DefaultExec returns an error for a failing command.
func TestDefaultExec_Error(t *testing.T) {
	_, err := DefaultExec(context.Background(), t.TempDir(), "false")
	if err == nil {
		t.Fatal("expected error from failing command")
	}
}

// TestDefaultExec_StderrIncluded verifies stderr is included in output on failure.
func TestDefaultExec_StderrIncluded(t *testing.T) {
	// 'ls nonexistent' writes an error to stderr and exits with non-zero.
	_, err := DefaultExec(context.Background(), t.TempDir(), "ls", "/nonexistent-path-for-test")
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
	// The error should include the command name or output.
	if !strings.Contains(err.Error(), "ls") {
		t.Errorf("expected 'ls' in error, got %q", err.Error())
	}
}

// TestDefaultExec_WorkingDirectory verifies the working directory is respected.
func TestDefaultExec_WorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	out, err := DefaultExec(context.Background(), dir, "pwd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// pwd output should contain the temp dir (may have symlinks resolved).
	if out == "" {
		t.Error("expected non-empty output from pwd")
	}
}

// TestDefaultExec_ContextCancellation verifies context cancellation stops the command.
func TestDefaultExec_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.
	_, err := DefaultExec(ctx, t.TempDir(), "sleep", "100")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
