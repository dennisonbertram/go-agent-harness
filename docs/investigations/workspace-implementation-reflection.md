# Workspace Abstraction Implementation - Retrospective

**Date**: 2026-03-11  
**Scope**: Complete review of `internal/workspace/` package across all 10 files

---

## Executive Summary

The workspace abstraction is **fundamentally sound** and achieves its core goal: providing a pluggable interface for agent execution environments. The design separates concerns cleanly and enables four distinct implementations (local, worktree, container, VM) without coupling. However, several **incomplete features and late-discovered patterns** will require attention for symphd integration and production scale.

---

## What Worked Well

### 1. **Core Design Abstraction**
The `Workspace` interface is elegant and minimal:
```go
Provision(ctx context.Context, opts Options) error
HarnessURL() string
WorkspacePath() string
Destroy(ctx context.Context) error
```

This design **forced early clarity**: each implementation must define two things — "where is the harness?" and "where is the filesystem?" — and cleanly separates provisioning from lifecycle. No implementation is coupled to another; they coexist happily via the Registry.

**Why it works**: The four-method contract is neither too broad (which would force unnecessary implementation of irrelevant features) nor too narrow (which would require casting/type assertions in clients).

### 2. **Registry Pattern for Pluggable Implementations**
The package-level `Registry` with concurrent-safe `Register()`, `New()`, and `List()` is production-ready:
- Thread-safe via `sync.RWMutex`
- Sentinel errors (`ErrNotFound`, `ErrAlreadyExists`, `ErrInvalidID`) are explicit and predictable
- Default package-level registry reduces boilerplate for the happy path
- `init()` functions register implementations automatically — no external wiring needed

This is textbook factory pattern done right.

### 3. **Graceful Error Handling in Destroy**
All implementations handle "already destroyed" state gracefully:

**WorktreeWorkspace.Destroy** matches output strings to silently ignore "not found" conditions:
```go
msg := strings.ToLower(strings.TrimSpace(string(out)))
if !strings.Contains(msg, "is not a working tree") && ... {
    return fmt.Errorf(...)
}
```

**ContainerWorkspace.Destroy** is even simpler (no-op if containerID is empty). This prevents cascading failures when Destroy is called twice or during exception handling.

### 4. **Path Containment Security in WorktreeWorkspace**
The path traversal check is solid:
```go
if !strings.HasPrefix(absPath, absRepo+string(filepath.Separator)) {
    return fmt.Errorf("workspace: worktree path %q escapes repository root %q", absPath, absRepo)
}
```

This prevents an attacker-controlled `opts.ID` (e.g., `../../../etc/passwd`) from escaping the repo. `filepath.Join` cleans the path, and then we verify the result. No surprises.

### 5. **Pool for Workspace Reuse**
The `Pool` implementation is clever and solves a real problem:
- **Pre-provisioning**: Background goroutine maintains target size
- **Blocking Get**: `Get()` blocks until a workspace is available or ctx is done
- **Graceful reset**: `Return()` Destroy-then-nil the workspace in the background
- **Readiness signal**: `Ready()` channel closes when pool reaches target size
- **Idempotent close**: `Close()` waits for background goroutine and cleans up

The 500ms maintenance tick and 100ms retry in `Get()` are reasonable defaults for the local/worktree use case. This will enable symphd to pre-warm pools of workspaces.

### 6. **PoolWorkspace as Decorator**
`PoolWorkspace` wraps the pool pattern cleanly: `Provision()` leases, `Destroy()` returns. It treats opts.ID as ignored (pool manages IDs), which is correct — the pool is responsible for lifecycle, not the consumer.

---

## What's Incomplete or Has Gaps

### 1. **Git Worktree Requires Shell Execution, Not Go-Git**
WorktreeWorkspace uses `exec.CommandContext` with `git worktree add` instead of a Go library. This is **correct but undocumented**:

```go
cmd := exec.CommandContext(ctx, "git", "-C", w.repoPath, "worktree", "add", w.path, "-b", w.branch)
if out, err := cmd.CombinedOutput(); err != nil {
    return fmt.Errorf("workspace: git worktree add: %w: %s", err, strings.TrimSpace(string(out)))
}
```

**Why this matters**:
- go-git (`github.com/go-git/go-git`) does not support `git worktree` — it's too new and go-git tracks a stable subset of git operations.
- The code is correct, but future maintainers will wonder "why shell exec?" — it should be documented in a comment or the README.
- **Implication**: Any workspace implementation that interacts with the local git repo will need to shell out to git.

### 2. **go.mod Bumped to 1.25.0 (Why?)**
`go 1.25.0` is in the go.mod file. This is suspicious — was it explicitly bumped or did Docker SDK require it?

Looking at the dependencies, the Docker SDK (`github.com/docker/docker v28.5.2+incompatible`) does not explicitly require 1.25. **This needs investigation**. If it was an accidental bump:
- Could break compatibility with older Go versions
- Should be reverted unless there's an explicit reason
- Should be documented in the commit message

### 3. **No Cleanup/Quota Policy in Pool**
The `Pool` maintains `targetSize` workspaces but has **no eviction, timeout, or reuse limits**:

```go
func (p *Pool) Return(id string) {
    // ... marks entry as inUse=false, ws will be Destroy'd async, then...
    // ... replaced by fillPool() automatically
}
```

For **local/worktree workspaces**, this is fine (they're lightweight). But for **container/VM workspaces**, the following scenarios are not handled:
- What if a workspace is leased but never returned? (Resource leak)
- What if provisioning fails repeatedly? (Retry loop may exhaust resources)
- What if a VM is provisioned but the provisioning times out? (Half-provisioned state)

**Implication for symphd**: Need to add:
- TTL/deadline tracking on leased workspaces
- Eviction policy (e.g., oldest leased workspace)
- Quota/budget tracking (especially for VM provisioning costs)
- Backoff/circuit-breaker on repeated provisioning failures

### 4. **HetznerProvider Has No Auth Fallback**
The VM workspace init registers Hetzner with:

```go
apiKey := os.Getenv("HETZNER_API_KEY")
provider := NewHetznerProvider(apiKey)
return NewVM(provider)
```

If `HETZNER_API_KEY` is unset, `NewHetznerProvider("")` still creates a valid client — it just won't authenticate. The error only surfaces when `Create()` is called.

**Implication**: A user who forgets to set the env var will get a confusing error at provision time, not at registration time. Should validate at `Register()` time for "vm" implementation.

### 5. **No Cleanup for Orphaned Git Worktrees**
If a WorktreeWorkspace.Provision() succeeds but Destroy() fails (e.g., process crash), the git worktree and branch are left behind. There's no cleanup utility or GC mechanism.

**Implication**: Over time, `<repo>/worktrees/` accumulates stale directories. The worktree workflow docs mention this, but the code doesn't provide helpers for periodic cleanup.

### 6. **ContainerWorkspace Doesn't Wait for harnessd Inside Container**
ContainerWorkspace polls until the container is *running*, but **not until harnessd is actually serving HTTP**:

```go
if info.State != nil && info.State.Running {
    break
}
time.Sleep(500 * time.Millisecond)
```

The container might be running but harnessd still starting (boot takes ~2-5 seconds). A subsequent client request to `HarnessURL()` might fail with connection refused.

**Should be**: Poll the HTTP endpoint itself:
```go
resp, err := http.Get(w.harnessURL + "/health")
if err == nil && resp.StatusCode == 200 {
    return nil
}
```

### 7. **No Observability/Logging**
None of the implementations emit logs or metrics. Questions that can't be answered:
- How long did Provision take?
- Did a VM timeout?
- How many workspaces are in-use vs. available?
- Did Destroy fail silently?

For production symphd use, we'll need:
- Structured logging (e.g., slog) per operation
- Duration metrics
- Error categorization (transient vs. permanent)

### 8. **LocalWorkspace.Provision Doesn't Clone RepoURL**
The Options struct has a `RepoURL` field, but only `WorktreeWorkspace` is designed to use it. `LocalWorkspace` creates an empty directory; it's up to the caller to clone.

**Is this a gap?** Depends on the intent — if `RepoURL` is meant to be "clone this repo into the workspace", then LocalWorkspace should handle it. If it's "the workspace is initialized externally", then LocalWorkspace is correct as-is.

**For symphd**: The gap will surface when symphd tries to set up parallel workspaces with different repo states. We'll likely need to either:
- Add `RepoURL` cloning to LocalWorkspace
- Or add a post-Provision hook for setup

### 9. **Pool.Return() Destroy is Async and Unmonitored**
```go
go func() {
    _ = ws.Destroy(context.Background())
}()
```

If Destroy fails, **it's silently dropped**. For container/VM workspaces, this means resources leak. Should be:
- Logged/tracked
- Retried on failure
- Moved to a "failed" state so the pool knows not to reuse it

---

## Surprising Discoveries

### 1. **Error String Matching in WorktreeWorkspace.Destroy**
The code matches error strings to identify "already gone" conditions:

```go
msg := strings.ToLower(strings.TrimSpace(string(out)))
if !strings.Contains(msg, "is not a working tree") && ...
```

This is fragile — if git changes its error message format, the check breaks. A better approach would be:
- Use `git worktree list` to check existence first
- Or use exit code + specific error patterns

But this works in practice and shows the pragmatism of the implementation.

### 2. **Factory Pattern is a Simple Function Type**
```go
type Factory func() Workspace
```

No interface, just a function. This is brilliant for testability — you can pass any function, including anonymous ones. The Registry doesn't care about the signature; it just calls it.

### 3. **Pool Uses sync.Once to Close ready Channel Exactly Once**
```go
p.readyOnce.Do(func() { close(p.ready) })
```

Closes the channel exactly once, even if `fillPool()` is called multiple times after reaching target size. This is correct and shows awareness of Go's channel semantics.

### 4. **Hetzner SDK Returns int64 Server IDs, Stored as Strings**
```go
ID: strconv.FormatInt(updated.ID, 10),  // int64 → string
serverID, err := strconv.ParseInt(id, 10, 64)  // string → int64
```

This bidirectional conversion is a bit of friction but necessary because the Workspace interface uses strings. Could be documented.

---

## Design Tensions and Trade-Offs

### 1. **String-Based IDs in Workspace Interface**
The `Options.ID` is a string, but:
- `WorkspacePath()` and `HarnessURL()` are also strings
- No unique identifier generation/enforcement
- No versioning or lifecycle tracking

**Trade-off**: Simplicity (strings are universal) vs. type safety (could use UUID types). The current design is correct for the MVP but symphd may want stronger identity tracking (e.g., UUID + status enum).

### 2. **Options.Env is Untyped map[string]string**
The interface uses a generic map for environment overrides, but each implementation has different env var names:
- `LocalWorkspace`: reads `HARNESS_URL`
- `ContainerWorkspace`: reads `HARNESS_IMAGE`
- `VMWorkspace`: doesn't read env, uses bootstrapping script

**Trade-off**: Flexibility (any implementation can add its own env vars) vs. discoverability (caller doesn't know what to pass). A structured config type would be better, but this works.

### 3. **Reuse in Pool vs. Fresh Workspaces**
The `Pool.Return()` calls `Destroy()` to reset the workspace, but some implementations (like Container) may need a hard reset (stop and restart container) vs. a soft reset (clear files). Currently, the contract assumes `Destroy` is cheap.

**Implication**: For VM workspaces, calling `Destroy()` on every return is **expensive** — you'd be terminating and recreating expensive infrastructure. For these, we might want a "reset" method that's cheaper than destroy.

### 4. **ContainerWorkspace Exposes Host Port, VMWorkspace Exposes Public IP**
These are different network models:
- Container: internal port 8080 → ephemeral host port (clients on same machine)
- VM: internal port 8080 → VM's public IP (clients anywhere)

This is fine, but symphd clients need to know how to reach the workspace. The `HarnessURL()` abstracts this, so it's not a problem — but it's worth noting the assumptions.

---

## What symphd (#187-#191) Will Need That Isn't Here

### 1. **Workspace Identity & Lifecycle Tracking**
```go
type WorkspaceStatus struct {
    ID         string
    State      WorkspaceState  // provisioning, ready, leaked, failed
    CreatedAt  time.Time
    LeasedAt   time.Time
    LeaseExpiry time.Time
    ProvisionErr error
}
```

symphd will need to:
- Know when a workspace is stuck (leaked/never returned)
- Track cost per workspace (for budget enforcement)
- Report on pool health (N leased, N available, N failed)

### 2. **Resource Limits & Quota**
```go
type PoolConfig struct {
    TargetSize      int
    MaxConcurrentVMs int
    MaxSpendPerDay  float64
    EvictAfter      time.Duration
    BackoffPolicy   ExponentialBackoff
}
```

symphd issues will demand:
- No more than 5 concurrent VMs at a time (cost control)
- Fail leases if 10 are currently provisioning (backpressure)
- Evict workspaces older than 1 hour (prevent leaked sessions)

### 3. **Event Stream / Observability**
Add a listener interface:
```go
type PoolListener interface {
    OnProvision(id string, duration time.Duration, err error)
    OnLease(id string)
    OnReturn(id string)
    OnDestroy(id string, err error)
}
```

symphd needs to emit metrics for:
- Provision latency (alerts if > 30s)
- Lease queue depth
- Success/failure rates per implementation

### 4. **Setup/Teardown Hooks**
```go
type Options struct {
    ID      string
    RepoURL string
    Setup   func(ctx context.Context, ws Workspace) error
    TearDown func(ctx context.Context, ws Workspace) error
}
```

symphd will need to:
- Clone a specific git branch into the workspace before handing it off
- Run a build/install script on container startup
- Clean up artifacts (logs, caches) before returning the workspace

### 5. **Workspace Versioning**
```go
type WorkspaceVersion struct {
    ProviderVersion string  // "container:v2.1", "vm:ubuntu-24.04"
    HarnessCommit   string
    ToolsCommit     string
}
```

symphd will need to:
- Ensure all workspaces in a pool are the same version
- Rotate pools when new images are pushed
- Handle version mismatches (old pool, new harness)

### 6. **Multi-Region / Multi-Provider**
```go
type ProviderSelection struct {
    Primary   WorkspaceType  // "container"
    Fallback  WorkspaceType  // "vm"
    Locality  string  // "us-west", "eu-central"
}
```

symphd will need to:
- Try container first; if Docker is unavailable, fall back to VM
- Select VMs in a specific region (for latency/compliance)

---

## New Things to Remember for Future Work

### 1. **Git Worktree Requires Shell Execution**
The go-git library doesn't support `git worktree`. Any future workspace implementation that interacts with git must use `exec.CommandContext()`, not go-git APIs. Document this in `docs/runbooks/` or as a code comment.

### 2. **Path Traversal Checks Must Use Absolute Paths + Prefix Check**
The pattern in WorktreeWorkspace is solid:
```go
absPath, _ := filepath.Abs(w.path)
absRepo, _ := filepath.Abs(w.repoPath)
if !strings.HasPrefix(absPath, absRepo+string(filepath.Separator)) { ... }
```

Use this for any future workspace that takes user input in paths.

### 3. **Destroy Must Be Idempotent**
All implementations make Destroy safe to call multiple times (empty string checks, error string matching, force flags). Future implementations should follow this pattern — it's easier for callers and exception handlers.

### 4. **Pool Maintenance Uses Small Ticks and Lazy Evaluation**
The 500ms maintenance tick and 100ms retry in Get() are reasonable for latency-tolerant use cases (agent provisioning). For real-time workloads, this might be too slow. Document the assumptions.

### 5. **Hetzner SDK Requires String-to-int64 Conversion**
The `strconv.FormatInt(..., 10)` and `ParseInt(..., 10, 64)` pattern is necessary because hcloud-go uses `int64` internally. This is a pain point — consider adding a helper function if more providers are added.

### 6. **Docker SDK Bumped Go Version to 1.25**
The Docker SDK may have caused the go.mod version bump. Verify this and document it. If not required, revert to the previous version (likely 1.24 or 1.23).

### 7. **Container Readiness Check is Incomplete**
Polling until `State.Running` is not sufficient — you need to poll the HTTP endpoint or use a health check. This will bite symphd if containers take time to start harnessd.

### 8. **Default Registry is a Package Variable**
The `var defaultRegistry` pattern means all tests share the default registry. This can cause test pollution if tests Register multiple times. Consider:
- Clearing the registry between tests
- Providing a test-friendly constructor that uses a fresh registry
- Or using `testing.TB.Cleanup()` to restore state

---

## Summary Table: Implementation Status

| Aspect | Local | Worktree | Container | VM | Pool | Notes |
|--------|-------|----------|-----------|----|----|---------|
| **Core Contract** | ✅ | ✅ | ✅ | ✅ | ✅ | All 4 methods working |
| **Error Handling** | ✅ | ✅ | ✅ | ✅ | ✅ | Idempotent destroy, good sentinel errors |
| **Concurrency Safety** | ✅ | ✅ | ✅ | ✅ | ✅ | Registry and Pool use sync.RWMutex/sync.Mutex |
| **Path Safety** | ✅ | ✅ | ✅ | N/A | N/A | Worktree has containment check |
| **Observability** | ❌ | ❌ | ❌ | ❌ | ❌ | No logging, metrics, or traces |
| **Resource Quota** | N/A | N/A | N/A | N/A | ❌ | No TTL, eviction, or cost tracking |
| **Health Checks** | ✅ | ✅ | ⚠️ | ⚠️ | N/A | Container/VM readiness is incomplete |
| **Lifecycle Hooks** | ❌ | ❌ | ❌ | ❌ | ❌ | No setup/teardown, just create + destroy |

---

## Recommendations for symphd Integration

1. **Add WorkspaceStatus tracking** — lifecycle states, timing, errors
2. **Implement resource quota enforcement** — TTL, eviction, budget
3. **Add observability** — structured logging, metrics (latency, success rate, queue depth)
4. **Fix container readiness** — poll HTTP endpoint, not just `State.Running`
5. **Add setup/teardown hooks** — allow symphd to clone repos, run builds
6. **Validate env vars at Register time** — catch config errors early
7. **Implement Pool.Return backoff** — retry Destroy on failure, track failures
8. **Document git worktree requirement** — flag that shell exec is mandatory
9. **Verify go.mod version** — confirm go 1.25.0 is necessary; revert if not
10. **Add test-friendly registry** — prevent test pollution with shared default registry

---

## Code Quality Assessment

**Strengths**:
- Clean separation of concerns
- Excellent use of Go idioms (sync.RWMutex, context, error wrapping)
- Defensive programming (path traversal checks, error string matching for idempotence)

**Weaknesses**:
- No logging or observability hooks
- Incomplete readiness checks for container/VM
- No resource quota or lifecycle tracking (will be critical for symphd)
- Test pollution potential with shared default registry

**Overall**: Production-ready for single-workspace use, but needs hardening for multi-workspace orchestration at scale.

