package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckSandboxCommandUnrestricted(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	// All commands should pass in unrestricted mode.
	commands := []string{
		"ls /tmp",
		"curl https://example.com",
		"wget http://example.com",
		"cd /etc && cat passwd",
	}
	for _, cmd := range commands {
		if err := CheckSandboxCommand(SandboxScopeUnrestricted, workspace, cmd); err != nil {
			t.Errorf("unrestricted scope: unexpected error for command %q: %v", cmd, err)
		}
	}
	// Empty scope is also unrestricted.
	for _, cmd := range commands {
		if err := CheckSandboxCommand("", workspace, cmd); err != nil {
			t.Errorf("empty scope: unexpected error for command %q: %v", cmd, err)
		}
	}
}

func TestCheckSandboxCommandLocalScope(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()

	blocked := []string{
		"curl https://example.com",
		"wget http://example.com",
		"nc -l 1234",
		"netcat example.com 80",
		"telnet example.com",
	}
	for _, cmd := range blocked {
		if err := CheckSandboxCommand(SandboxScopeLocal, workspace, cmd); err == nil {
			t.Errorf("local scope: expected error for command %q, got nil", cmd)
		}
	}

	// Local scope allows filesystem operations.
	allowed := []string{
		"ls /tmp",
		"cat /etc/hosts",
		"echo hello",
		"go test ./...",
	}
	for _, cmd := range allowed {
		if err := CheckSandboxCommand(SandboxScopeLocal, workspace, cmd); err != nil {
			t.Errorf("local scope: unexpected error for command %q: %v", cmd, err)
		}
	}
}

// TestCheckSandboxCommandWorkspaceScope verifies workspace-scope enforcement.
func TestCheckSandboxCommandWorkspaceScope(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	absWorkspace, _ := filepath.Abs(workspace)

	// Commands with absolute paths outside the workspace should be blocked.
	outsideAbsPaths := []string{
		"cat /etc/passwd",
		"ls /tmp",
		"rm /var/log/messages",
	}
	for _, cmd := range outsideAbsPaths {
		if err := CheckSandboxCommand(SandboxScopeWorkspace, absWorkspace, cmd); err == nil {
			t.Errorf("workspace scope: expected error for command %q with outside absolute path, got nil", cmd)
		}
	}

	// Commands entirely within the workspace should be allowed.
	insideCmd := "ls " + absWorkspace
	if err := CheckSandboxCommand(SandboxScopeWorkspace, absWorkspace, insideCmd); err != nil {
		t.Errorf("workspace scope: unexpected error for in-workspace command %q: %v", insideCmd, err)
	}

	// cd .. style escapes should be blocked.
	cdEscape := []string{
		"cd ..",
		"cd ../../etc",
		"cd ../  ",
	}
	for _, cmd := range cdEscape {
		if err := CheckSandboxCommand(SandboxScopeWorkspace, absWorkspace, cmd); err == nil {
			t.Errorf("workspace scope: expected error for cd-escape command %q, got nil", cmd)
		}
	}

	// Commands without absolute paths or cd escapes should be allowed.
	safeCommands := []string{
		"echo hello",
		"go test ./...",
		"ls",
		"cat notes.txt",
	}
	for _, cmd := range safeCommands {
		if err := CheckSandboxCommand(SandboxScopeWorkspace, absWorkspace, cmd); err != nil {
			t.Errorf("workspace scope: unexpected error for safe command %q: %v", cmd, err)
		}
	}
}

// TestSandboxWorkspaceScopeEnforcesFilePaths checks the case required by the issue:
// workspace scope blocks ../outside paths.
func TestSandboxWorkspaceScopeEnforcesFilePaths(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	absWorkspace, _ := filepath.Abs(workspace)

	// Writing to a path outside the workspace via absolute path should be blocked.
	outsideFile := filepath.Join(filepath.Dir(absWorkspace), "outside.txt")
	cmd := "echo secret > " + outsideFile
	if err := CheckSandboxCommand(SandboxScopeWorkspace, absWorkspace, cmd); err == nil {
		t.Errorf("workspace scope: expected error for write to %q, got nil", outsideFile)
	}

	// Writing inside the workspace is fine.
	insideFile := filepath.Join(absWorkspace, "inside.txt")
	cmd2 := "echo hello > " + insideFile
	if err := CheckSandboxCommand(SandboxScopeWorkspace, absWorkspace, cmd2); err != nil {
		t.Errorf("workspace scope: unexpected error for write to %q: %v", insideFile, err)
	}
}

// TestCheckSandboxCommandUnknownScope checks that an unknown scope returns an error.
func TestCheckSandboxCommandUnknownScope(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if err := CheckSandboxCommand("badscope", workspace, "echo hi"); err == nil {
		t.Error("expected error for unknown sandbox scope, got nil")
	}
}

// TestJobManagerSandboxScopeWorkspace verifies that commands blocked by the
// workspace sandbox scope are rejected by JobManager.runForeground.
func TestJobManagerSandboxScopeWorkspace(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	absWorkspace, _ := filepath.Abs(workspace)

	mgr := NewJobManager(absWorkspace, nil)
	mgr.SetSandboxScope(SandboxScopeWorkspace)

	ctx := context.Background()

	// A command that references /etc/passwd (outside workspace) should be rejected.
	_, err := mgr.RunForeground(ctx, "cat /etc/passwd", 5, "")
	if err == nil {
		t.Error("expected sandbox error for 'cat /etc/passwd', got nil")
	}

	// A safe command should pass.
	result, err := mgr.RunForeground(ctx, "echo hello", 5, "")
	if err != nil {
		t.Errorf("unexpected error for safe command: %v", err)
	}
	if result == nil {
		t.Error("expected non-nil result for safe command")
	}
}

// TestJobManagerSandboxScopeLocal verifies that network commands are blocked
// under SandboxScopeLocal.
func TestJobManagerSandboxScopeLocal(t *testing.T) {
	t.Parallel()

	// Skip if no workspace needed.
	workspace, _ := os.MkdirTemp("", "sandbox-test")
	defer os.RemoveAll(workspace)

	mgr := NewJobManager(workspace, nil)
	mgr.SetSandboxScope(SandboxScopeLocal)

	ctx := context.Background()

	// curl should be blocked.
	_, err := mgr.RunForeground(ctx, "curl https://example.com", 5, "")
	if err == nil {
		t.Error("expected sandbox error for curl, got nil")
	}

	// echo should be allowed.
	result, err := mgr.RunForeground(ctx, "echo hi", 5, "")
	if err != nil {
		t.Errorf("unexpected error for echo: %v", err)
	}
	if result == nil {
		t.Error("expected non-nil result for echo")
	}
}
