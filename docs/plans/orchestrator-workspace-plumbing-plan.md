# Plan: Orchestrator → Workspace → Harness Plumbing

## Context

- **Problem:** `symphd.Dispatcher` currently holds a single static `workspace.Workspace` and a static `HarnessURL` in `DispatchConfig`. Every issue dispatched shares the same workspace object. The Dispatcher calls `d.workspace.Provision(ctx, opts)` but then ignores `d.workspace.HarnessURL()` entirely — it uses the statically configured `DispatchConfig.HarnessURL` for all HTTP calls to harnessd. This means only one workspace type can be used per process, and the URL is never dynamically derived from the provisioned workspace.
- **User impact:** Cannot run concurrent agent dispatches each in their own isolated container, VM, or worktree with their own harnessd instance.
- **Constraints:** Must not break existing tests. Dispatcher interface must remain testable with mocks. Pool ownership must be determined. HarnessURL is already on the Workspace interface — this is the key integration point.

---

## Current State

```
GitHub Issues
     |
     v
 GitHubTracker (poll/claim/start/complete/fail)
     |
     v
 Orchestrator.Start() polling loop
     |
     v
 Dispatcher.Dispatch(ctx, issue)
     |
     +-- tracker.Start(issue.Number)        [Claimed → Running]
     |
     +-- d.workspace.Provision(ctx, opts)   [single shared Workspace object]
     |   workspacePath := d.workspace.WorkspacePath()
     |
     +-- d.client.StartRun(ctx, prompt, workspacePath)
     |   ^ uses d.config.HarnessURL (STATIC, ignores workspace.HarnessURL())
     |
     +-- poll d.client.RunStatus(ctx, runID)
     |   ^ same static URL
     |
     +-- tracker.Complete() or tracker.Fail()
     |
     v
  RunResult → d.results channel
```

**Critical bug in `dispatcher.go`:**
The current `HTTPHarnessClient` is constructed once with a static base URL from `DispatchConfig.HarnessURL`. The workspace's `HarnessURL()` is never consulted after provisioning.

---

## Target State

```
GitHub Issues
     |
     v
 GitHubTracker (poll/claim/start/complete/fail)
     |
     v
 Orchestrator.Start()  ←── owns workspace.Pool (optional, for pool mode)
     |
     v
 Dispatcher.Dispatch(ctx, issue)
     |
     +-- tracker.Start(issue.Number)
     |
     +-- ws := d.factory.New()              [WorkspaceFactory creates fresh Workspace per issue]
     |
     +-- ws.Provision(ctx, opts)            [provision: mkdir / worktree / docker / VM]
     |   workspacePath := ws.WorkspacePath()
     |   harnessURL    := ws.HarnessURL()   ← DYNAMIC, from provisioned workspace
     |
     +-- client := NewHTTPHarnessClient(harnessURL)  [per-issue client, points at ws]
     |
     +-- runID := client.StartRun(ctx, prompt, workspacePath)
     |
     +-- poll client.RunStatus(ctx, runID)
     |
     +-- tracker.Complete() or tracker.Fail()
     |
     +-- ws.Destroy(ctx)                    [teardown]
     |
     v
  RunResult → d.results channel
```

---

## Key Insight: The "Factory" Pattern vs. Shared Workspace

The fundamental problem is that `Dispatcher` holds a single `workspace.Workspace` pointer and calls `.Provision()` on it each time an issue is dispatched. But a `Workspace` is a stateful object — once provisioned it holds a path and URL. Sharing it across concurrent dispatches is a data race and logical error.

The fix: replace `workspace.Workspace` in `Dispatcher` with a `WorkspaceFactory` function (or interface) that creates a **fresh, unprovisioned Workspace per dispatch**. The factory can be backed by:

1. A `workspace.Registry` lookup by name (for `"local"`, `"worktree"`, `"container"`, `"vm"`)
2. A `workspace.Pool` (for pool-backed leasing, where `NewPoolWorkspace` acts as the factory)

This is a minimal, backward-compatible change: the mock in `dispatcher_test.go` can continue implementing `workspace.Workspace` — tests just need to be updated to use a factory.

---

## What Changes Where

### 1. `internal/symphd/dispatcher.go`

**New types:**
```go
// WorkspaceFactory creates a new, unprovisioned Workspace for each dispatch.
type WorkspaceFactory func() workspace.Workspace

// HarnessClientFactory creates a HarnessClient for a given harness URL.
type HarnessClientFactory func(harnessURL string) HarnessClient
```

**Struct changes:**
```go
type Dispatcher struct {
    config           DispatchConfig
    workspaceFactory WorkspaceFactory    // replaces single workspace field
    tracker          Tracker
    clientFactory    HarnessClientFactory // creates per-workspace clients
    ...
}
```

**Constructor changes:**
```go
func NewDispatcher(cfg DispatchConfig, wsFactory WorkspaceFactory, tracker Tracker, clientFactory HarnessClientFactory) *Dispatcher

// Convenience helper with default HTTP client factory:
func NewDispatcherSimple(cfg DispatchConfig, wsFactory WorkspaceFactory, tracker Tracker) *Dispatcher {
    return NewDispatcher(cfg, wsFactory, tracker, func(url string) HarnessClient {
        return NewHTTPHarnessClient(url)
    })
}
```

**`runIssue` core change:**
```go
func (d *Dispatcher) runIssue(ctx context.Context, issue *TrackedIssue) RunResult {
    ws := d.workspaceFactory()   // fresh per-dispatch

    opts := workspace.Options{
        ID:      fmt.Sprintf("issue-%d", issue.Number),
        BaseDir: d.config.BaseDir,
    }
    if err := ws.Provision(ctx, opts); err != nil {
        _ = d.tracker.Fail(issue.Number, fmt.Sprintf("workspace provision failed: %v", err))
        return RunResult{IssueNumber: issue.Number, Error: err}
    }
    defer ws.Destroy(context.Background())

    harnessURL := ws.HarnessURL()   // DYNAMIC — no longer static
    client := d.clientFactory(harnessURL)

    // Wait for harnessd to be ready (important for container/VM workspaces)
    if err := waitForHarnessReady(ctx, harnessURL, 60*time.Second); err != nil {
        _ = d.tracker.Fail(issue.Number, fmt.Sprintf("harness not ready: %v", err))
        return RunResult{IssueNumber: issue.Number, Error: err}
    }

    // ... rest of existing poll loop unchanged ...
}
```

**New helper:**
```go
func waitForHarnessReady(ctx context.Context, url string, timeout time.Duration) error {
    deadline := time.Now().Add(timeout)
    for {
        if time.Now().After(deadline) {
            return fmt.Errorf("harness at %s did not become ready within %s", url, timeout)
        }
        resp, err := http.Get(url + "/healthz")
        if err == nil && resp.StatusCode == 200 {
            return nil
        }
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(2 * time.Second):
        }
    }
}
```

### 2. `internal/symphd/config.go`

Add fields:
```go
type Config struct {
    // ... existing fields ...

    // PoolSize: number of pre-provisioned workspaces in pool mode. Default: 3.
    PoolSize int `yaml:"pool_size"`

    // PoolWorkspaceType: inner workspace type when workspace_type is "pool". Default: "container".
    PoolWorkspaceType string `yaml:"pool_workspace_type"`

    // RepoURL: optional git repo URL passed to workspace.Options.RepoURL.
    RepoURL string `yaml:"repo_url"`
}
```

Document `HarnessURL`: only used for `"local"` and `"worktree"` types. Ignored for `"container"` and `"vm"` — URL comes from the provisioned workspace.

### 3. `internal/symphd/orchestrator.go`

```go
type Orchestrator struct {
    // ... existing fields ...
    pool *workspace.Pool  // non-nil only when workspace_type is "pool"
}

func NewOrchestrator(cfg *Config) *Orchestrator {
    // ... existing setup ...
    wsFactory, pool := buildWorkspaceFactory(cfg)
    o.pool = pool
    if wsFactory != nil {
        o.dispatcher = NewDispatcherSimple(dispatchCfg, wsFactory, o.tracker)
    }
    return o
}

func (o *Orchestrator) Shutdown(ctx context.Context) error {
    // ... existing dispatcher shutdown ...
    if o.pool != nil {
        o.pool.Close()
    }
    return nil
}
```

**New `buildWorkspaceFactory` helper:**
```go
func buildWorkspaceFactory(cfg *Config) (WorkspaceFactory, *workspace.Pool) {
    switch cfg.WorkspaceType {
    case "local":
        return func() workspace.Workspace {
            return workspace.NewLocal(cfg.HarnessURL, cfg.BaseDir)
        }, nil
    case "container":
        return func() workspace.Workspace { return workspace.NewContainer("") }, nil
    case "vm":
        return func() workspace.Workspace {
            return workspace.NewVM(workspace.NewHetznerProvider(os.Getenv("HETZNER_API_KEY")))
        }, nil
    case "pool":
        inner := buildRawFactory(cfg.PoolWorkspaceType, cfg)
        if inner == nil { return nil, nil }
        pool := workspace.NewPool(inner, workspace.Options{BaseDir: cfg.BaseDir}, cfg.PoolSize)
        return func() workspace.Workspace { return workspace.NewPoolWorkspace(pool) }, pool
    default:
        return nil, nil
    }
}
```

---

## Lifecycle Sequence (per issue)

```
1. Orchestrator polls GitHubTracker
   - find Unclaimed → Claim(n) [Unclaimed → Claimed]
   - find Claimed → Dispatcher.Dispatch(ctx, issue)

2. Dispatcher.Dispatch
   - tracker.Start(n) [Claimed → Running]
   - acquire semaphore (blocks at MaxConcurrent)
   - goroutine: runIssue(ctx, issue)

3. runIssue goroutine
   a. ws := workspaceFactory()
   b. ws.Provision(ctx, opts)
      local:     mkdir /tmp/symphd/issue-42
      container: docker run -p <port>:8080 go-agent-harness:latest
      vm:        POST Hetzner → poll until Running
      pool:      pool.Get(ctx) → blocks until slot available
   c. harnessURL := ws.HarnessURL()    // e.g. "http://localhost:34521"
   d. client := clientFactory(harnessURL)
   e. waitForHarnessReady(ctx, harnessURL, 60s)
   f. runID := client.StartRun(ctx, prompt)
   g. poll loop: client.RunStatus(ctx, runID)
      → "completed" → tracker.Complete(n)
      → "failed"    → tracker.Fail(n, reason)
      → stall       → tracker.Fail(n, "stalled")
   h. defer ws.Destroy(ctx)
      local:     os.RemoveAll(...)
      container: docker stop + rm
      vm:        DELETE /servers/<id>
      pool:      pool.Return(id) → re-provision async

4. RunResult → results channel
   (Orchestrator drains; future: call RetryFailed on error)
```

---

## Pool Ownership

**The Orchestrator owns the Pool**, not the Dispatcher.

- Pool is a long-lived resource with a background goroutine
- Orchestrator.Shutdown() calls pool.Close() after dispatcher shutdown
- WorkspaceFactory closure captures Pool reference (Dispatcher has indirect access)
- Pattern: `pool.Get()` in PoolWorkspace.Provision(), `pool.Return()` in PoolWorkspace.Destroy()

---

## Poll vs. SSE vs. Webhook

**Recommendation: Keep polling.**

- SSE would require long-lived connections per run + reconnection logic + SSE frame parsing
- Polling at 5s intervals is appropriate for multi-minute agent runs
- `waiting_for_user` status: treat as non-terminal (same as `running`), let stall timer handle it
- SSE available as future optimization via `GET /v1/runs/{id}/events`

---

## Configuration Examples

```yaml
# Local (external harnessd, shared)
workspace_type: local
harness_url: http://localhost:8080
base_dir: /tmp/symphd-workspaces

# Container per issue (harnessd inside container)
workspace_type: container
base_dir: /tmp/symphd-workspaces
max_concurrent_agents: 5

# VM per issue (Hetzner)
workspace_type: vm
base_dir: /workspace

# Pool of pre-warmed containers
workspace_type: pool
pool_size: 5
pool_workspace_type: container
max_concurrent_agents: 5
```

---

## Risk Areas

| Risk | Impact | Mitigation |
|------|--------|-----------|
| Shared Workspace data race (current bug) | High | Factory pattern fixes this |
| Container running ≠ harnessd ready | High | `waitForHarnessReady()` HTTP poll |
| Destroy failure silently swallowed | Medium | Log error; container `--rm` as fallback |
| Worktree type: single harnessd = no per-run isolation | Medium | Document limitation; require per-run harnessd for real isolation |
| Pool entry Destroy slow/failed | Low | Already handled async; maintainLoop reprovisioned on next tick |

---

## Implementation Order

1. **Step 1**: Change `Dispatcher` to `WorkspaceFactory` + `HarnessClientFactory` — update struct, constructor, `runIssue`, tests
2. **Step 2**: Add `waitForHarnessReady` helper to `dispatcher.go`
3. **Step 3**: Add `pool_size`, `pool_workspace_type`, `repo_url` to `Config`
4. **Step 4**: Wire `buildWorkspaceFactory` in `orchestrator.go`; add pool field; close in Shutdown
5. **Step 5** (optional): Integration test behind `//go:build integration`

---

## New Tests Required (TDD — write first)

1. `TestDispatcher_UsesWorkspaceHarnessURL` — dynamic URL from workspace, not static config
2. `TestDispatcher_DestroysWorkspaceOnCompletion` — Destroy called on success
3. `TestDispatcher_DestroysWorkspaceOnFailure` — Destroy called when StartRun fails
4. `TestDispatcher_DestroysWorkspaceOnContextCancel` — Destroy called on cancellation
5. `TestDispatcher_ConcurrentUsesDistinctWorkspaces` — N dispatches → N distinct workspace instances
6. `TestBuildWorkspaceFactory_Local` — returns local factory for workspace_type local
7. `TestBuildWorkspaceFactory_Pool` — creates Pool and PoolWorkspace factory
8. `TestOrchestrator_ShutdownClosesPool` — pool.Close() called on shutdown

**Existing tests to update:**
All `NewDispatcher(cfg, ws, tr, cl)` calls → `NewDispatcher(cfg, func() workspace.Workspace { return ws }, tr, func(url string) HarnessClient { return cl })`
