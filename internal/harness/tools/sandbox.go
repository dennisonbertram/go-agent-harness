package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// networkRestrictedPatterns are bash command patterns blocked under SandboxScopeLocal.
// These block common network-exfiltration commands to external hosts.
var networkRestrictedPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bcurl\b`),
	regexp.MustCompile(`(?i)\bwget\b`),
	regexp.MustCompile(`(?i)\bnc\b`),
	regexp.MustCompile(`(?i)\bnetcat\b`),
	regexp.MustCompile(`(?i)\btelnet\b`),
}

// CheckSandboxCommand validates a bash command against the given SandboxScope.
// It returns a non-nil error if the command violates the sandbox constraints.
//
// For SandboxScopeWorkspace, commands that write to paths outside the
// workspace (detected via cd/path heuristics) are rejected.  The bash tool
// always runs with workingDir constrained to the workspace, so this is a
// defence-in-depth check rather than a primary enforcement mechanism.
//
// For SandboxScopeLocal, outbound network commands (curl, wget, nc, etc.)
// are blocked.
//
// For SandboxScopeUnrestricted (or empty), no additional checks are applied.
//
// network gates the SandboxScopeLocal heuristic: it only rejects network
// commands when network is NetworkPolicyDeny. An empty network (or
// NetworkPolicyAllow) is the default and lets network commands proceed —
// the real enforcement for that case is simply the absence of any OS-level
// network restriction (see buildSandboxedCommand).
func CheckSandboxCommand(scope SandboxScope, network NetworkPolicy, workspaceRoot, command string) error {
	switch scope {
	case SandboxScopeWorkspace:
		return checkWorkspaceScopeCommand(workspaceRoot, command)
	case SandboxScopeLocal:
		if network == NetworkPolicyDeny {
			return checkLocalScopeCommand(command)
		}
		return nil
	case SandboxScopeUnrestricted, "":
		return nil
	default:
		return fmt.Errorf("unknown sandbox scope %q", scope)
	}
}

// checkWorkspaceScopeCommand blocks bash commands that appear to target paths
// outside the workspace and outside toolchainWritableDirs() (issue #1399).
// It inspects:
//   - Absolute paths embedded in the command (including "~/..." tokens,
//     expanded against the home directory before the containment check).
//   - "cd .." or "cd ../../" style path escapes.
//   - /etc, /tmp, /var, /usr, /home, /root usage, EXCEPT when the token
//     falls under one of toolchainWritableDirs() — e.g. a GOTMPDIR or
//     GOCACHE override pointed at the process temp dir or a per-user cache
//     dir, which language toolchains legitimately need under workspace
//     scope.
func checkWorkspaceScopeCommand(workspaceRoot, command string) error {
	// Resolve workspace root for comparison, canonicalizing symlinks the
	// same way the extra writable roots below already are (and the same
	// way the OS-level sandbox builders already resolve the workspace
	// root) so a token pointing at the same location through a symlink
	// (e.g. macOS's /var -> /private/var) is not falsely rejected.
	absRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		absRoot = workspaceRoot
	}
	absRoot = filepath.Clean(absRoot)
	if resolved, err := filepath.EvalSymlinks(absRoot); err == nil {
		absRoot = resolved
	}

	roots := append([]string{absRoot}, toolchainWritableDirs()...)

	home, homeErr := os.UserHomeDir()

	// Detect absolute paths in the command that escape the workspace and
	// every toolchain writable root. Simple heuristic: split on whitespace
	// and check each token that looks like an absolute (or home-relative
	// "~/...") path.
	tokens := strings.Fields(command)
	for _, tok := range tokens {
		// Strip leading quotes and common shell metacharacters.
		cleaned := strings.TrimLeft(tok, `"'`)
		cleaned = strings.TrimRight(cleaned, `"';`)
		if homeErr == nil && (cleaned == "~" || strings.HasPrefix(cleaned, "~/")) {
			cleaned = filepath.Join(home, strings.TrimPrefix(cleaned, "~"))
		}
		if !filepath.IsAbs(cleaned) {
			continue
		}
		candidate := filepath.Clean(cleaned)
		if resolved, err := canonicalizePathAllowingMissing(candidate); err == nil {
			candidate = resolved
		}
		if pathUnderAnyRoot(candidate, roots) {
			continue
		}
		return fmt.Errorf("sandbox violation: absolute path %q escapes workspace %q", cleaned, absRoot)
	}

	// Detect "cd .." patterns that escape the workspace.
	cdRe := regexp.MustCompile(`(?i)\bcd\s+(\.\.[\s/]|\.\.+$)`)
	if cdRe.MatchString(command) {
		return fmt.Errorf("sandbox violation: cd outside workspace is not permitted in workspace sandbox scope")
	}

	return nil
}

// pathUnderAnyRoot reports whether candidate lies within any of roots,
// reusing pathWithinRoot's component-wise (not string-prefix) comparison.
func pathUnderAnyRoot(candidate string, roots []string) bool {
	for _, root := range roots {
		if pathWithinRoot(candidate, root) {
			return true
		}
	}
	return false
}

// checkLocalScopeCommand blocks outbound network commands.
func checkLocalScopeCommand(command string) error {
	for _, pattern := range networkRestrictedPatterns {
		if pattern.MatchString(command) {
			return fmt.Errorf("sandbox violation: network command is not permitted in local sandbox scope")
		}
	}
	return nil
}

// SandboxEnforcementEnv controls what happens when the OS-level confinement
// mechanism required for a scope (seatbelt on macOS, bubblewrap on Linux) is
// unavailable on the host. When set to a truthy value ("1", "true", "yes",
// "on"), unavailability is fatal: the command is rejected rather than run
// unconfined (fail closed). Left unset, the JobManager degrades to the
// string-heuristic checks above and reports the degradation via
// SandboxExecResult.Warning so it is observable rather than silent.
const SandboxEnforcementEnv = "HARNESS_SANDBOX_STRICT"

func sandboxStrictModeEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(SandboxEnforcementEnv))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// SandboxExecResult describes whether OS-level process confinement was
// actually applied to a spawned bash command, and by what mechanism.
type SandboxExecResult struct {
	// Applied is true when the command was actually wrapped in an OS-level
	// confinement mechanism (not just the string heuristics above).
	Applied bool
	// Mechanism identifies what was used: "seatbelt", "bubblewrap", "none"
	// (unrestricted scope, confinement intentionally not applicable), or
	// "unavailable" (confinement was required but could not be applied).
	Mechanism string
	// Warning is non-empty when confinement degraded to heuristic-only
	// enforcement and should be surfaced to the caller.
	Warning string
	// NetworkPolicy is the network policy actually applied when building the
	// command (issue #1397): "allow" or "deny". It is populated even for
	// scopes/mechanisms that do not enforce it at the OS level, so bash tool
	// output always reflects the effective policy rather than leaving the
	// caller to infer it.
	NetworkPolicy NetworkPolicy
	// WritableDirs lists the extra per-user temp/cache roots (beyond the
	// workspace itself) that were opened up for writes under
	// SandboxScopeWorkspace (issue #1399), e.g. os.TempDir(), the Go build
	// and module caches. Empty for scopes where it is not meaningful
	// ("local"/"unrestricted" already permit unrestricted filesystem
	// writes).
	WritableDirs []string
}

// resolveSandboxUnavailable is called by the platform-specific
// buildSandboxedCommand implementations when the OS-level confinement
// mechanism for the given scope cannot be applied (binary missing,
// unsupported platform, etc). In strict mode it fails closed by returning an
// error; otherwise it returns a degraded SandboxExecResult carrying an
// explicit, observable warning so callers do not silently believe they are
// isolated when they are not.
func resolveSandboxUnavailable(scope SandboxScope, mechanism, reason string) (SandboxExecResult, error) {
	if sandboxStrictModeEnabled() {
		return SandboxExecResult{}, fmt.Errorf("sandbox: refusing to run %q-scope command: OS-level confinement (%s) unavailable: %s (set %s=0 to allow degraded execution)", scope, mechanism, reason, SandboxEnforcementEnv)
	}
	warning := fmt.Sprintf("sandbox: OS-level confinement (%s) unavailable for %q scope: %s — falling back to heuristic checks only, isolation is NOT guaranteed", mechanism, scope, reason)
	return SandboxExecResult{Applied: false, Mechanism: "unavailable", Warning: warning}, nil
}
