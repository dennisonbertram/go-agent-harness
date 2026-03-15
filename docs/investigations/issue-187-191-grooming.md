# Issue Grooming: #187, #188, #189, #190, #191 (symphd Initiative)

**Date**: 2026-03-11  
**Context**: Workspace abstraction (#181-#186) is now COMPLETE in main. These issues are ready for implementation.

---

## Executive Summary

All five symphd issues are **well-scoped, unblocked, and ready to implement sequentially**. The workspace layer provides everything needed: `Workspace` interface, `Pool` for reuse, and four implementations (local, worktree, container, VM). No changes to workspace code are required before starting symphd.

**Dependency chain**: `#187 → #188 → #189 → #190` (parallel). `#191` is independent but should be fleshed out before #189 starts.

---

## Individual Issue Analysis

### Issue #187: symphd daemon scaffold and CLI

| Aspect | Assessment |
|--------|-----------|
| **Already Addressed?** | No — symphd binary doesn't exist yet. |
| **Clarity** | Good — framework, config loading, graceful shutdown are standard patterns. |
| **Acceptance Criteria** | Exist but minimal. Flesh out: "Graceful shutdown" should specify (SIGTERM → wait for running agents → close pool → exit). "HTTP API" should specify default port (8888?), request/response formats. |
| **Scope** | Atomic — this is the harness for the other 4 issues. |
| **Dependencies** | Blocks #188, #189, #190, #191 (all require the daemon framework). |
| **Effort** | **Small** (2-3 hours). Standard Go HTTP server + context lifecycle + YAML config loader. |
| **Blockers** | None. Workspace is ready. |

**Verdict**: Ready now. Flesh out acceptance criteria (SIGTERM handling, default ports, error responses).

---

### Issue #188: symphd issue tracker client (GitHub Issues polling)

| Aspect | Assessment |
|--------|-----------|
| **Already Addressed?** | No — tracker client doesn't exist. |
| **Clarity** | Excellent — "GitHub Issues API + label filter + claim state machine" is precise. |
| **Acceptance Criteria** | Good. State machine is clear: `Unclaimed → Claimed → Running → Done/Failed`. Clarify: How is state persisted? (In-memory, or in GitHub issue comments/labels?) Are multiple symphd daemons supported (distributed state)? |
| **Scope** | Atomic — tracker polling is orthogonal to dispatch/retry. |
| **Dependencies** | Requires #187 (daemon framework) to call this. |
| **Effort** | **Medium** (4-5 hours). GitHub API client, state transitions, concurrent polling, error handling. |
| **Blockers** | None. GitHub API is public. Consider: Will this use `github.com/google/go-github` or raw HTTP? Document. |

**Verdict**: Ready. Clarify state persistence model (in-memory vs. durable) and multi-daemon assumptions.

---

### Issue #189: symphd dispatcher (workspace provision + harness dispatch)

| Aspect | Assessment |
|--------|-----------|
| **Already Addressed?** | No — dispatcher loop doesn't exist. |
| **Clarity** | Good — "For each claimed issue: provision workspace → POST to harnessd → monitor SSE → handle completion" is clear. |
| **Acceptance Criteria** | Partially defined. Missing details: How many concurrent agents? (Max N from WORKFLOW.md?) How is SSE monitored? (Parse JSON events?) What does "completion" mean — run succeeded, or max_turns exceeded? |
| **Scope** | Atomic — orchestration logic is separate from retry logic. |
| **Dependencies** | Requires #187 (daemon), #188 (tracker), #191 (WORKFLOW.md config). |
| **Effort** | **Large** (6-8 hours). SSE client, workspace pooling coordination, timeout/stall detection, completion state machine. |
| **Blockers** | None. Workspace pool is ready. **Note**: workspace-implementation-reflection.md flagged incomplete container health checks — fix that before symphd relies on it. |

**Verdict**: Ready. Clarify concurrency model, SSE event parsing, and completion/stall detection semantics. **Action**: Verify/fix container health check in workspace layer before dispatch starts.

---

### Issue #190: symphd retry logic and exponential backoff

| Aspect | Assessment |
|--------|-----------|
| **Already Addressed?** | No — retry state machine doesn't exist. |
| **Clarity** | Excellent — exponential backoff formula is precise: `min(10000 * 2^(attempt-1), max_backoff_ms)`. Two modes (continuation vs. failure) are clear. |
| **Acceptance Criteria** | Good. Missing: What are the defaults for `max_backoff_ms`, `max_attempts`, `max_per_issue`? Should retries be tracked in issue state/comments? |
| **Scope** | Atomic — retry policy is independent of dispatch. |
| **Dependencies** | Requires #189 (dispatcher calls retry logic). Can be implemented in parallel if clear about retry trigger interface. |
| **Effort** | **Small** (2-3 hours). Timer/backoff calculation, attempt counter, dead letter queue. |
| **Blockers** | None. |

**Verdict**: Ready. Define default retry limits and persistence model (where is retry count stored?).

---

### Issue #191: symphd WORKFLOW.md configuration format

| Aspect | Assessment |
|--------|-----------|
| **Already Addressed?** | No — WORKFLOW.md loader doesn't exist. |
| **Clarity** | Good — YAML front matter + Markdown body + Liquid templates is clear. |
| **Acceptance Criteria** | Partial. Missing: Example WORKFLOW.md file. What variables are available to templates? (issue_id, workspace_path, etc.?) How is "dynamic reload" implemented — inotify, HTTP endpoint, or poll? What does "fail on unknown variables" mean exactly (Liquid strict mode)? |
| **Scope** | Atomic — config format is separate from dispatch. |
| **Dependencies** | Required by #189 (dispatcher reads WORKFLOW.md for max_concurrent_agents, max_turns). Can be implemented in parallel. |
| **Effort** | **Medium** (3-4 hours). YAML parsing (use stdlib `gopkg.in/yaml.v3`), Liquid template engine (library?), file watcher for reload. |
| **Blockers** | **Technology decision**: Which Liquid library? `github.com/Shopify/liquid` is the Go standard. Verify it supports strict mode. |

**Verdict**: Ready. Provide example WORKFLOW.md + template variable reference. Verify Liquid library support.

---

## Dependency Graph

```
#187 (Daemon Scaffold)
  ├─→ #188 (Tracker Client)
  │    └─→ #189 (Dispatcher)
  │         ├─→ #190 (Retry Logic) [can be parallel if interface is clear]
  │         └─→ #191 (WORKFLOW.md) [can be parallel; #189 consumes it]
  ├─→ #190 (Retry Logic)
  └─→ #191 (WORKFLOW.md)

**Critical path**: #187 → #188 → #189. #190 and #191 can start after #187.
**Suggested merge order**: #187 → #191 → #188 → #189 → #190
```

---

## Workspace Layer Integration Points

The workspace implementation (#181-#186) provides everything needed:

| symphd Requirement | Workspace Layer | Status |
|-------------------|-----------------|--------|
| Interface abstraction | `Workspace` interface | ✅ |
| Multi-implementation | Local, Worktree, Container, VM | ✅ |
| Pre-warm pool | `Pool` with background maintain loop | ✅ |
| Registry/discovery | `workspace.Registry` with `Register()`, `New()` | ✅ |
| Get/Return semantics | `Pool.Get(ctx)` + `Pool.Return(id)` | ✅ |

**Gaps identified in workspace-implementation-reflection.md** that symphd should address:

1. **Container health check incomplete**: Pool waits for `State.Running`, but harnessd may not be serving yet. **Action**: Verify container implementation polls HTTP endpoint. If not, defer to #187 or #189 to add retry loop.
2. **No observability/logging**: symphd will need to add structured logging (slog) for provision/lease/return/destroy events.
3. **No TTL/quota enforcement**: symphd will need to add deadline tracking on leased workspaces.
4. **Pool.Return() Destroy is async and unmonitored**: symphd should log/retry failures.
5. **LocalWorkspace doesn't clone RepoURL**: symphd may need to add post-Provision setup hook for cloning repo + branch.

---

## Acceptance Criteria Refinements

### #187: daemon scaffold and CLI

**Add to issue body**:
- Graceful shutdown: SIGTERM → close tracker poller → wait for in-flight dispatcher → close pool → exit (with timeout)
- HTTP API port: Default 8888, configurable via `SYMPHD_ADDR` env var
- Config file: `config.yaml` or via `SYMPHD_CONFIG_FILE` env var
- Response format: `{"status": "ok", "state": {...}}` (JSON)
- Logging: Use slog with JSON output to stdout; log level configurable via `SYMPHD_LOG_LEVEL`

### #188: issue tracker client

**Add to issue body**:
- State persistence: In-memory only (restart loses state; re-polling reconstructs it)
- Supported trackers: GitHub Issues (via `github.com/google/go-github`); Linear as future extension
- Multiple symphd daemons: Assumed single orchestrator (no distributed consensus)
- Claim logic: Issue must have `symphd` label to be eligible; transition tracked in memory, not persisted to issue

### #189: dispatcher

**Add to issue body**:
- Concurrency: `max_concurrent_agents` from WORKFLOW.md (default 5)
- SSE monitoring: Parse harnessd SSE output; track `step/*` and `complete` events
- Completion criteria: Run succeeded if `complete` event with no error; timeout if no event for 1h (from WORKFLOW.md)
- Stall detection: If no SSE event for 5 minutes, assume stuck; mark for retry
- Workspace lifecycle: Provision → dispatch POST → monitor SSE → Destroy (always, success or failure)

### #190: retry logic

**Add to issue body**:
- Defaults: `max_backoff_ms = 300000` (5 min), `max_attempts = 5`, `max_per_issue = 10`
- Continuation retries: Used when agent exits but could continue (e.g., max_turns hit); 1s fixed delay
- Failure retries: Used when agent crashes or times out; exponential backoff
- Dead letter: After max_attempts, log error + close issue (or comment on GitHub?)

### #191: WORKFLOW.md

**Add to issue body**:
- Template variables: `{{ issue_id }}`, `{{ workspace_path }}`, `{{ workspace_harness_url }}`, `{{ github_token }}`
- Liquid library: Use `github.com/Shopify/liquid` v3
- Strict mode: Fail render if template contains `{{ undefined_var }}`; don't silently use zero/nil
- Dynamic reload: Poll file every 5s for changes; update in-memory config on change (no daemon restart)
- Example front matter:
  ```yaml
  max_concurrent_agents: 5
  max_turns: 20
  turn_timeout_ms: 3600000  # 1 hour
  stall_timeout_ms: 300000  # 5 minutes
  workspace_type: "local"  # or "worktree", "container", "vm"
  ```

---

## Pre-Implementation Checklist

- [ ] #191: Pick Liquid library + verify strict mode support
- [ ] #189: Verify or fix container workspace health check (poll HTTP, not just `State.Running`)
- [ ] #188: Clarify if using `google/go-github` or raw HTTP
- [ ] #187: Decide daemon port (suggest 8888)
- [ ] All: Add structured logging (slog) to all components

---

## Effort Breakdown

| Issue | Effort | Notes |
|-------|--------|-------|
| #187 | Small (2-3h) | HTTP server + config + lifecycle |
| #188 | Medium (4-5h) | API client + state machine + polling |
| #189 | Large (6-8h) | Dispatcher loop + SSE + workspace coordination + stall detection |
| #190 | Small (2-3h) | Backoff calculation + retry queue |
| #191 | Medium (3-4h) | YAML + Liquid + file watcher |

**Total**: ~17-23 hours (2-3 days if sequential, with parallelization of #190-#191).

---

## Recommendations

1. **Start with #187**: Gets everyone aligned on daemon structure, config format, HTTP API shape.
2. **Follow with #191**: Define WORKFLOW.md + template variables before #189 needs to read it.
3. **Parallel track**: #188 (tracker client) and #190 (retry logic) can start after #187.
4. **Then #189**: Once tracker + retry are ready, dispatcher is straightforward.
5. **Testing**: Add integration test that: tracks a fake issue, dispatches to harnessd, monitors SSE, retries on timeout.
6. **Workspace health**: Before merging #189, verify container workspace health check is solid (poll HTTP endpoint).

---

## Final Verdict

| Issue | Status | Verdict |
|-------|--------|---------|
| **#187** | Ready | Implement now. Flesh out SIGTERM, port, response format. |
| **#188** | Ready | Implement after #187. Clarify state persistence model. |
| **#189** | Ready | Implement after #188. Verify container health check first. |
| **#190** | Ready | Implement after #189 or in parallel. Define retry defaults. |
| **#191** | Ready | Implement in parallel with #188. Define template variables. |

**Blockers**: None. Workspace is done.

