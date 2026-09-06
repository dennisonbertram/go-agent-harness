package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// osSandboxAvailable reports whether this host has the OS-level confinement
// mechanism buildSandboxedCommand would actually apply for scope, so tests
// that prove real isolation (rather than the string heuristic) can skip
// gracefully on hosts/CI where it's missing.
func osSandboxAvailable(t *testing.T) bool {
	t.Helper()
	switch runtime.GOOS {
	case "darwin":
		if _, err := os.Stat("/usr/bin/sandbox-exec"); err != nil {
			return false
		}
		return true
	case "linux":
		_, err := exec.LookPath("bwrap")
		return err == nil
	default:
		return false
	}
}

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
		if err := CheckSandboxCommand(SandboxScopeUnrestricted, NetworkPolicyDeny, workspace, cmd); err != nil {
			t.Errorf("unrestricted scope: unexpected error for command %q: %v", cmd, err)
		}
	}
	// Empty scope is also unrestricted.
	for _, cmd := range commands {
		if err := CheckSandboxCommand("", NetworkPolicyDeny, workspace, cmd); err != nil {
			t.Errorf("empty scope: unexpected error for command %q: %v", cmd, err)
		}
	}
}

// TestCheckSandboxCommandLocalScope verifies that SandboxScopeLocal's
// network heuristic is gated by NetworkPolicy (issue #1397): explicit deny
// still blocks curl/wget/nc/netcat/telnet, but the new default (allow, both
// as an explicit value and as the empty zero value) does not.
func TestCheckSandboxCommandLocalScope(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()

	networkCommands := []string{
		"curl https://example.com",
		"wget http://example.com",
		"nc -l 1234",
		"netcat example.com 80",
		"telnet example.com",
	}
	for _, cmd := range networkCommands {
		if err := CheckSandboxCommand(SandboxScopeLocal, NetworkPolicyDeny, workspace, cmd); err == nil {
			t.Errorf("local scope, network=deny: expected error for command %q, got nil", cmd)
		}
	}
	for _, network := range []NetworkPolicy{NetworkPolicyAllow, ""} {
		for _, cmd := range networkCommands {
			if err := CheckSandboxCommand(SandboxScopeLocal, network, workspace, cmd); err != nil {
				t.Errorf("local scope, network=%q: unexpected error for command %q: %v", network, cmd, err)
			}
		}
	}

	// Local scope allows filesystem operations regardless of network policy.
	allowed := []string{
		"ls /tmp",
		"cat /etc/hosts",
		"echo hello",
		"go test ./...",
	}
	for _, cmd := range allowed {
		if err := CheckSandboxCommand(SandboxScopeLocal, NetworkPolicyDeny, workspace, cmd); err != nil {
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
	// "ls /tmp" is deliberately NOT included here: on hosts where TMPDIR is
	// unset, os.TempDir() (and so toolchainWritableDirs(), issue #1399)
	// resolves to exactly "/tmp", which would make this assertion
	// host-dependent; TestCheckWorkspaceScopeCommandToolchainWritableDirs
	// covers that acceptance case directly via os.TempDir() instead of a
	// hardcoded path.
	outsideAbsPaths := []string{
		"cat /etc/passwd",
		"ls /usr/local/bin/x",
		"rm /var/log/messages",
	}
	for _, cmd := range outsideAbsPaths {
		if err := CheckSandboxCommand(SandboxScopeWorkspace, NetworkPolicyAllow, absWorkspace, cmd); err == nil {
			t.Errorf("workspace scope: expected error for command %q with outside absolute path, got nil", cmd)
		}
	}

	// Commands entirely within the workspace should be allowed.
	insideCmd := "ls " + absWorkspace
	if err := CheckSandboxCommand(SandboxScopeWorkspace, NetworkPolicyAllow, absWorkspace, insideCmd); err != nil {
		t.Errorf("workspace scope: unexpected error for in-workspace command %q: %v", insideCmd, err)
	}

	// cd .. style escapes should be blocked.
	cdEscape := []string{
		"cd ..",
		"cd ../../etc",
		"cd ../  ",
	}
	for _, cmd := range cdEscape {
		if err := CheckSandboxCommand(SandboxScopeWorkspace, NetworkPolicyAllow, absWorkspace, cmd); err == nil {
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
		if err := CheckSandboxCommand(SandboxScopeWorkspace, NetworkPolicyAllow, absWorkspace, cmd); err != nil {
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

	// Writing to a path outside the workspace via absolute path should be
	// blocked. /var/tmp (not a sibling directory under os.TempDir()) is
	// used deliberately: issue #1399 opens up os.TempDir() itself for
	// writes under workspace scope, so a sibling of the workspace's t.TempDir()
	// parent would no longer prove an escape.
	outsideFile := filepath.Join("/var/tmp", "harness-sandbox-outside-test.txt")
	cmd := "echo secret > " + outsideFile
	if err := CheckSandboxCommand(SandboxScopeWorkspace, NetworkPolicyAllow, absWorkspace, cmd); err == nil {
		t.Errorf("workspace scope: expected error for write to %q, got nil", outsideFile)
	}

	// Writing inside the workspace is fine.
	insideFile := filepath.Join(absWorkspace, "inside.txt")
	cmd2 := "echo hello > " + insideFile
	if err := CheckSandboxCommand(SandboxScopeWorkspace, NetworkPolicyAllow, absWorkspace, cmd2); err != nil {
		t.Errorf("workspace scope: unexpected error for write to %q: %v", insideFile, err)
	}
}

// TestCheckSandboxCommandUnknownScope checks that an unknown scope returns an error.
func TestCheckSandboxCommandUnknownScope(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if err := CheckSandboxCommand("badscope", NetworkPolicyAllow, workspace, "echo hi"); err == nil {
		t.Error("expected error for unknown sandbox scope, got nil")
	}
}

// TestCheckWorkspaceScopeCommandToolchainWritableDirs verifies (issue #1399)
// that checkWorkspaceScopeCommand no longer flags absolute-path tokens that
// fall under one of toolchainWritableDirs()'s roots — e.g. a GOTMPDIR or
// GOCACHE override pointed at the process temp dir or per-user cache dir —
// while still rejecting genuinely out-of-scope system paths.
func TestCheckWorkspaceScopeCommandToolchainWritableDirs(t *testing.T) {
	workspace := t.TempDir()
	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		t.Fatal(err)
	}

	tempFile := filepath.Join(os.TempDir(), "harness-1399-gotmpdir-probe")
	accepted := []string{
		"ls " + os.TempDir(),
		"echo hi > " + tempFile,
	}
	for _, cmd := range accepted {
		if err := CheckSandboxCommand(SandboxScopeWorkspace, NetworkPolicyAllow, absWorkspace, cmd); err != nil {
			t.Errorf("expected command %q referencing a toolchain-writable dir to be accepted, got error: %v", cmd, err)
		}
	}

	rejected := []string{
		"cat /etc/passwd",
		"ls /usr/local/bin/x",
		"cat ~/.ssh/id_rsa",
	}
	for _, cmd := range rejected {
		if err := CheckSandboxCommand(SandboxScopeWorkspace, NetworkPolicyAllow, absWorkspace, cmd); err == nil {
			t.Errorf("expected command %q to still be rejected as a sandbox violation, got nil", cmd)
		}
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
// under SandboxScopeLocal when the network policy is explicitly deny.
func TestJobManagerSandboxScopeLocal(t *testing.T) {
	t.Parallel()

	// Skip if no workspace needed.
	workspace, _ := os.MkdirTemp("", "sandbox-test")
	defer os.RemoveAll(workspace)

	mgr := NewJobManager(workspace, nil)
	mgr.SetSandboxScope(SandboxScopeLocal)

	ctx := WithNetworkPolicy(context.Background(), NetworkPolicyDeny)

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

// TestJobManagerSandboxScopeLocalAllowsNetworkByDefault verifies the default
// behavior change from issue #1397: with no network policy configured on
// either the JobManager or the context, SandboxScopeLocal no longer rejects
// curl before it runs (the pre-execution heuristic check must not fire).
func TestJobManagerSandboxScopeLocalAllowsNetworkByDefault(t *testing.T) {
	t.Parallel()

	workspace, _ := os.MkdirTemp("", "sandbox-test")
	defer os.RemoveAll(workspace)

	mgr := NewJobManager(workspace, nil)
	mgr.SetSandboxScope(SandboxScopeLocal)

	_, err := mgr.RunForeground(context.Background(), "curl --version", 5, "")
	if err != nil {
		t.Errorf("expected curl not to be rejected under default (allow) network policy, got error: %v", err)
	}
}

func TestJobManagerContextSandboxScopeOverridesDefault(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	absWorkspace, _ := filepath.Abs(workspace)

	mgr := NewJobManager(absWorkspace, nil)
	mgr.SetSandboxScope(SandboxScopeWorkspace)

	ctx := WithSandboxScope(context.Background(), SandboxScopeUnrestricted)

	result, err := mgr.RunForeground(ctx, "cat /etc/hosts", 5, "")
	if err != nil {
		t.Fatalf("expected context sandbox override to allow command, got error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if exitCode, _ := result["exit_code"].(int); exitCode != 0 {
		t.Fatalf("expected exit_code 0, got %v", result["exit_code"])
	}
}

func TestJobManagerContextSandboxScopeBlocksBackgroundCommand(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()

	mgr := NewJobManager(workspace, nil)
	mgr.SetSandboxScope(SandboxScopeUnrestricted)

	ctx := WithSandboxScope(context.Background(), SandboxScopeLocal)
	ctx = WithNetworkPolicy(ctx, NetworkPolicyDeny)

	if _, err := mgr.RunBackgroundWithContext(ctx, "curl https://example.com", 5, ""); err == nil {
		t.Fatal("expected local sandbox override with network=deny to block background network command")
	}
}

// TestSandboxWorkspaceScopeBlocksWriteOutsideWorkspaceAtOSLevel proves that
// workspace-scope confinement is enforced by the OS, not by string matching:
// the destination path is built via shell variable indirection so it never
// appears as a literal absolute-path token in the command, which the
// existing regex/token heuristic in checkWorkspaceScopeCommand would have
// caught. The write must still fail at the OS level.
func TestSandboxWorkspaceScopeBlocksWriteOutsideWorkspaceAtOSLevel(t *testing.T) {
	if !osSandboxAvailable(t) {
		t.Skip("no OS-level sandbox mechanism (seatbelt/bubblewrap) available on this host")
	}

	workspace := t.TempDir()
	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		t.Fatal(err)
	}

	mgr := NewJobManager(absWorkspace, nil)
	mgr.SetSandboxScope(SandboxScopeWorkspace)

	// /var/tmp, not os.TempDir(), is the "outside" location here: issue
	// #1399 deliberately opens up os.TempDir() (and a handful of per-user
	// cache dirs) for writes under workspace scope, so a proof of
	// OS-level confinement needs a destination outside every one of those
	// toolchain-writable roots to still demonstrate a real boundary.
	target := filepath.Join("/var/tmp", fmt.Sprintf("harness-sandbox-proof-%d", time.Now().UnixNano()))
	_ = os.Remove(target)
	defer os.Remove(target)

	dir := filepath.Dir(target)
	base := filepath.Base(target)
	command := fmt.Sprintf(`D=%s; echo pwned > "$D/%s"`, dir, base)

	// The heuristic layer must NOT catch this obfuscated escape — that is
	// what makes this a proof of OS-level enforcement rather than a
	// duplicate of the existing string-matching tests above.
	if err := CheckSandboxCommand(SandboxScopeWorkspace, NetworkPolicyAllow, absWorkspace, command); err != nil {
		t.Fatalf("expected heuristic to miss the obfuscated escape (so the OS layer is what's under test), got error: %v", err)
	}

	result, _ := mgr.RunForeground(context.Background(), command, 5, "")
	if result != nil {
		if exitCode, _ := result["exit_code"].(int); exitCode == 0 {
			t.Errorf("expected non-zero exit code for OS-blocked write outside workspace, got 0; result=%v", result)
		}
	}
	if _, statErr := os.Stat(target); statErr == nil {
		t.Fatalf("sandbox violation: file %q was created outside the workspace despite workspace sandbox scope", target)
	}
}

// TestSandboxLocalScopeBlocksObfuscatedNetworkAtOSLevel proves that, when the
// network policy is deny, local-scope network denial is enforced by the OS,
// not by regex matching against the command string: "curl" is assembled from
// two shell variables so the literal substring "curl" never appears in the
// command, defeating the \bcurl\b pattern in checkLocalScopeCommand. The
// request must still fail because the OS layer denies network operations
// outright.
func TestSandboxLocalScopeBlocksObfuscatedNetworkAtOSLevel(t *testing.T) {
	if !osSandboxAvailable(t) {
		t.Skip("no OS-level sandbox mechanism (seatbelt/bubblewrap) available on this host")
	}

	workspace := t.TempDir()
	mgr := NewJobManager(workspace, nil)
	mgr.SetSandboxScope(SandboxScopeLocal)
	ctx := WithNetworkPolicy(context.Background(), NetworkPolicyDeny)

	command := `A=cur; B=l; "$A$B" -s -m 5 https://example.com -o /dev/null -w '%{http_code}'`

	if err := CheckSandboxCommand(SandboxScopeLocal, NetworkPolicyDeny, workspace, command); err != nil {
		t.Fatalf("expected heuristic to miss the obfuscated network command (so the OS layer is what's under test), got error: %v", err)
	}

	result, err := mgr.RunForeground(ctx, command, 10, "")
	if err != nil {
		// A hard exec failure also demonstrates the network call never
		// succeeded; only a clean success (exit 0) would be a problem.
		return
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	exitCode, _ := result["exit_code"].(int)
	if exitCode == 0 {
		t.Fatalf("expected obfuscated curl to fail under OS-level network denial, got exit_code 0; result=%v", result)
	}
}

func TestResolveSandboxUnavailableDegradesByDefault(t *testing.T) {
	res, err := resolveSandboxUnavailable(SandboxScopeWorkspace, "seatbelt", "binary not found")
	if err != nil {
		t.Fatalf("expected no error in default (non-strict) mode, got: %v", err)
	}
	if res.Applied {
		t.Error("expected Applied=false for unavailable mechanism")
	}
	if res.Mechanism != "unavailable" {
		t.Errorf("expected Mechanism=\"unavailable\", got %q", res.Mechanism)
	}
	if res.Warning == "" {
		t.Error("expected a non-empty warning explaining the degradation")
	}
}

func TestResolveSandboxUnavailableFailsClosedWhenStrict(t *testing.T) {
	t.Setenv(SandboxEnforcementEnv, "1")

	if _, err := resolveSandboxUnavailable(SandboxScopeWorkspace, "seatbelt", "binary not found"); err == nil {
		t.Fatal("expected an error when strict mode is enabled and the mechanism is unavailable")
	}
}

// TestSandboxWorkspaceScopeNetworkPolicyLiveCurl proves end-to-end, with a
// real network request, that workspace-scope network confinement follows
// PermissionConfig.Network (issue #1397): curl to a real host fails under
// network=deny and succeeds under network=allow. Skipped when no OS-level
// sandbox mechanism (seatbelt/bubblewrap) or no curl binary is available.
// The allow-case is skipped rather than failed when the host itself has no
// route to the internet, since that is an environment limitation, not a
// sandbox defect.
func TestSandboxWorkspaceScopeNetworkPolicyLiveCurl(t *testing.T) {
	if !osSandboxAvailable(t) {
		t.Skip("no OS-level sandbox mechanism (seatbelt/bubblewrap) available on this host")
	}
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl not available on this host")
	}

	workspace := t.TempDir()
	mgr := NewJobManager(workspace, nil)
	mgr.SetSandboxScope(SandboxScopeWorkspace)

	const command = `curl -sI -m 10 https://proxy.golang.org`

	t.Run("deny", func(t *testing.T) {
		t.Parallel()
		ctx := WithNetworkPolicy(context.Background(), NetworkPolicyDeny)
		result, _ := mgr.RunForeground(ctx, command, 15, "")
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if exitCode, _ := result["exit_code"].(int); exitCode == 0 {
			t.Errorf("expected curl to fail under network=deny, got exit_code 0; result=%v", result)
		}
	})

	t.Run("allow", func(t *testing.T) {
		t.Parallel()
		ctx := WithNetworkPolicy(context.Background(), NetworkPolicyAllow)
		result, err := mgr.RunForeground(ctx, command, 15, "")
		if err != nil {
			t.Skipf("curl exec failed under network=allow (host likely has no route to the internet): %v", err)
		}
		exitCode, _ := result["exit_code"].(int)
		if exitCode != 0 {
			t.Skipf("curl exited %d under network=allow (host likely has no route to the internet); result=%v", exitCode, result)
		}
	})
}

// TestJobManagerRunForegroundReportsSandboxNetworkInResult is a regression
// test for issue #1397: the bash tool result map must surface which network
// policy was actually applied (result["sandbox_network"]), for both allow
// and deny, so an operator inspecting a run's tool output can see the policy
// without cross-referencing the run's permissions separately. If a future
// change stopped threading SandboxExecResult.NetworkPolicy into the result
// map (bash_manager.go), this test would fail by finding the key absent or
// mismatched, independent of whether the command itself succeeded.
func TestJobManagerRunForegroundReportsSandboxNetworkInResult(t *testing.T) {
	if !osSandboxAvailable(t) {
		t.Skip("no OS-level sandbox mechanism (seatbelt/bubblewrap) available on this host")
	}
	t.Parallel()

	workspace := t.TempDir()
	mgr := NewJobManager(workspace, nil)
	mgr.SetSandboxScope(SandboxScopeWorkspace)

	for _, policy := range []NetworkPolicy{NetworkPolicyAllow, NetworkPolicyDeny} {
		policy := policy
		t.Run(string(policy), func(t *testing.T) {
			t.Parallel()
			ctx := WithNetworkPolicy(context.Background(), policy)
			result, err := mgr.RunForeground(ctx, "echo hi", 5, "")
			if err != nil {
				t.Fatalf("RunForeground: %v", err)
			}
			if got := result["sandbox_network"]; got != string(policy) {
				t.Errorf("result[\"sandbox_network\"] = %v, want %q", got, string(policy))
			}
		})
	}
}

// TestSandboxWorkspaceScopeToolchainCanBuildAndTest is the integration proof
// for issue #1399: under SandboxScopeWorkspace, with the REAL Go toolchain
// and no env var overrides steering GOCACHE/GOTMPDIR/GOMODCACHE into the
// workspace, `go build ./...` and `go test ./...` must succeed against a
// throwaway module created inside the workspace. Before this change this
// failed with "failed to initialize build cache ... operation not
// permitted" (build cache lives under the per-user cache dir) or "creating
// work dir: mkdir /var/folders/...: operation not permitted" (Go's scratch
// dir lives under the process temp dir) — neither is the workspace, so
// neither the darwin seatbelt profile nor the Linux bwrap binds covered
// them. Skipped when no OS-level sandbox mechanism is available.
func TestSandboxWorkspaceScopeToolchainCanBuildAndTest(t *testing.T) {
	if !osSandboxAvailable(t) {
		t.Skip("no OS-level sandbox mechanism (seatbelt/bubblewrap) available on this host")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available on this host")
	}

	workspace := t.TempDir()
	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"go.mod": "module sandboxcachetest\n\ngo 1.21\n",
		"main.go": `package main

func add(a, b int) int { return a + b }

func main() { println(add(2, 3)) }
`,
		"main_test.go": `package main

import "testing"

func TestAdd(t *testing.T) {
	if add(2, 3) != 5 {
		t.Fatal("add(2,3) != 5")
	}
}
`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(absWorkspace, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	mgr := NewJobManager(absWorkspace, nil)
	mgr.SetSandboxScope(SandboxScopeWorkspace)

	result, err := mgr.RunForeground(context.Background(), "go env GOCACHE && go build ./... && go test ./...", 90, "")
	if err != nil {
		t.Fatalf("run foreground: %v", err)
	}
	output, _ := result["output"].(string)
	exitCode, _ := result["exit_code"].(int)
	if exitCode != 0 {
		t.Fatalf("expected go build/test to succeed under workspace sandbox with no env overrides, got exit_code=%d output=%q", exitCode, output)
	}
	if !strings.Contains(output, "ok") {
		t.Errorf("expected go test output to report \"ok\", got: %q", output)
	}
}

// TestSandboxWorkspaceScopeAllowsMktempDir is the second integration proof
// required by issue #1399: `mktemp -d` (which creates a directory under the
// process temp dir, not the workspace) must succeed under
// SandboxScopeWorkspace with no env overrides.
func TestSandboxWorkspaceScopeAllowsMktempDir(t *testing.T) {
	if !osSandboxAvailable(t) {
		t.Skip("no OS-level sandbox mechanism (seatbelt/bubblewrap) available on this host")
	}

	workspace := t.TempDir()
	mgr := NewJobManager(workspace, nil)
	mgr.SetSandboxScope(SandboxScopeWorkspace)

	result, err := mgr.RunForeground(context.Background(), "mktemp -d", 10, "")
	if err != nil {
		t.Fatalf("run foreground: %v", err)
	}
	exitCode, _ := result["exit_code"].(int)
	output, _ := result["output"].(string)
	if exitCode != 0 {
		t.Fatalf("expected \"mktemp -d\" to succeed under workspace sandbox, got exit_code=%d output=%q", exitCode, output)
	}
	if strings.TrimSpace(output) == "" {
		t.Errorf("expected \"mktemp -d\" to print the created directory path, got empty output")
	}
}

// TestJobManagerRunForegroundReportsSandboxWritableDirsInResult is a
// regression test for issue #1399: the bash tool result map must surface
// which extra writable roots were opened up under workspace scope
// (result["sandbox_writable_dirs"]), so an operator/model inspecting a run's
// tool output can see why writes outside the literal workspace succeeded.
func TestJobManagerRunForegroundReportsSandboxWritableDirsInResult(t *testing.T) {
	if !osSandboxAvailable(t) {
		t.Skip("no OS-level sandbox mechanism (seatbelt/bubblewrap) available on this host")
	}
	t.Parallel()

	workspace := t.TempDir()
	mgr := NewJobManager(workspace, nil)
	mgr.SetSandboxScope(SandboxScopeWorkspace)

	result, err := mgr.RunForeground(context.Background(), "echo hi", 5, "")
	if err != nil {
		t.Fatalf("RunForeground: %v", err)
	}
	dirs, ok := result["sandbox_writable_dirs"].([]string)
	if !ok || len(dirs) == 0 {
		t.Fatalf(`expected result["sandbox_writable_dirs"] to be a non-empty []string, got %#v`, result["sandbox_writable_dirs"])
	}
	if !containsDir(t, dirs, os.TempDir()) {
		t.Errorf(`expected result["sandbox_writable_dirs"] to include os.TempDir() (%q), got %v`, os.TempDir(), dirs)
	}
}
