# Issues #182-#186 Grooming: Workspace Implementations

Date: 2026-03-11

## Summary Table

| Issue | Title | Verdict | Effort | Blocker |
|-------|-------|---------|--------|---------|
| #182 | LocalWorkspace (directory-based) | ✅ Well-specified | Small | #181 |
| #183 | WorktreeWorkspace (git worktree-based) | ✅ Well-specified | Small | #181 |
| #184 | ContainerWorkspace (Docker-based) | ✅ Well-specified (research needed) | Medium | #181 |
| #185 | VMWorkspace (cloud VM-based) | ✅ Well-specified (pluggable design) | Large | #181 |
| #186 | PoolWorkspace (pre-provisioned pool) | ✅ Well-specified | Medium | #181 |

All issues depend on #181 (interface definition). None are already addressed.

## Issue #182 — LocalWorkspace
- Clear design: create dir, return shared harnessd URL, delete dir on Destroy
- No external dependencies
- Simplest possible implementation; baseline for abstraction
- Labels: `well-specified`, `small`, `workspace`

## Issue #183 — WorktreeWorkspace
- Clear: git worktree add/remove per workspace
- Branch name sanitization explicitly required (`[A-Za-z0-9._-]`)
- Path containment enforcement important (no traversal)
- Implementation note: `go-git` library worktree support may be incomplete; shell exec to `git` binary likely needed
- Labels: `well-specified`, `small`, `workspace`

## Issue #184 — ContainerWorkspace
- Clear goal but implementation details need research:
  - Docker Go SDK: `github.com/docker/docker/client` — use context7 to confirm API
  - Dynamic port allocation: use OS-assigned port (listen on :0, get assigned port, then close and use)
  - Healthcheck polling strategy needed
  - Dockerfile for harnessd image needed
- Proceed with implementation after context7 research on Docker SDK
- Labels: `well-specified`, `medium`, `workspace`

## Issue #185 — VMWorkspace
- Pluggable VMProvider interface is the right call
- Start with one provider (DigitalOcean godo or Hetzner hcloud)
- Bootstrap strategy: cloud-init is standard; SSH exec is alternative
- Integration test skip tag needed (no credentials in CI)
- Labels: `well-specified`, `large`, `workspace`

## Issue #186 — PoolWorkspace
- Clear pool model: warm instances, lease/return pattern, background goroutine for replenishment
- Wraps any other Workspace implementation (composition pattern)
- Reset on return is the tricky part — strategy depends on inner workspace type
- Concurrency: pool operations must be safe under -race
- Labels: `well-specified`, `medium`, `workspace`
