# Symphony Spec Analysis

Source: https://github.com/openai/symphony/blob/main/SPEC.md
Date: 2026-03-11

## What Symphony Is

Symphony is a long-running daemon from OpenAI that automates coding tasks by continuously polling an issue tracker (Linear), creating isolated workspaces, and dispatching AI coding agents to complete work. It's a workflow orchestration system designed for repeatability, isolation, version control, and observability of concurrent agent runs.

## Key Architectural Concepts

### 1. Core Components
- **Workflow Loader**: Parses WORKFLOW.md files with YAML front matter (config) + Markdown body (prompt templates)
- **Orchestrator**: Single in-memory authority managing scheduling, dispatch, retries, and reconciliation
- **Workspace Manager**: Per-issue directories with sanitized identifiers, reused across runs with lifecycle hooks
- **Issue Tracker Client**: Supports Linear via GraphQL; fetches candidates, refreshes state, handles terminal cleanup
- **Agent Runner**: Spawns subprocess with JSON-line RPC protocol over stdio

### 2. Agent/Orchestration Model

**State Machine**: Issues flow through claim states independent of tracker states:
```
Unclaimed → Claimed → Running → RetryQueued → Released
```

**Multi-turn Continuation**: Workers run multiple turns on the same thread until exhausting max_turns, exiting active state, or encountering errors.

**Polling & Dispatch**: Fixed cadence polling with candidate sorting (priority ASC, creation date ASC, ID), dispatch eligibility checks, and stall detection.

**Retry Strategy**:
- Continuation retries: 1-second fixed delay
- Failure retries: Exponential backoff — `min(10000 * 2^(attempt-1), max_retry_backoff_ms)`

### 3. Protocol Design

**JSON-RPC Line Protocol**: Agent subprocess communicates via newline-delimited JSON over stdio.

Startup handshake:
```
initialize → response → initialized (notification) → thread/start → turn/start
```

**Continuation Turns**: Reuse same `thread_id` on live process; send only continuation guidance, not full prompt.

**Completion Signals**: `turn/completed` (success), `turn/failed`/`turn/cancelled`/timeout (failure).

**Optional HTTP API**:
- `GET /api/v1/state`
- `GET /api/v1/<issue_identifier>`
- `POST /api/v1/refresh`

### 4. Configuration

**Three-tier hierarchy**: Runtime overrides > WORKFLOW.md front matter > environment variables > defaults.

**Dynamic reload**: Polling intervals, concurrency limits, hooks must be re-applicable without restart.

**Key defaults**: `max_concurrent_agents` (10), `max_turns` (20), `turn_timeout_ms` (1h), `stall_timeout_ms` (5m).

## Novel Patterns Worth Noting

1. **Tracker as source of truth**: Eliminates need for persistent database; restart recovery via re-polling.
2. **Workspace persistence**: Workspaces survive successful runs to preserve build artifacts and avoid re-setup.
3. **Workspace lifecycle hooks**: Pre/post-run customization without orchestrator changes.
4. **Stall detection via reconciliation**: Proactive timeout management independent of agent implementation.
5. **Turn continuation on same thread**: Multiple turns reuse the same thread to reduce overhead and context reset.
6. **No prescribed trust model**: Implementations choose their approval/sandbox posture and document it explicitly.
7. **Strict template rendering**: Unknown variables fail rendering (Liquid-compatible semantics).
8. **Sanitized directory names**: Only `[A-Za-z0-9._-]` allowed; workspace path containment enforced.

## Comparison to go-agent-harness

| Dimension | Symphony | go-agent-harness |
|-----------|----------|------------------|
| **Scope** | Issue-tracker-driven automation daemon | Event-driven HTTP backend for LLM tool-calling loops |
| **State Management** | In-memory orchestrator; tracker is source of truth | Memory/conversation-scoped (SQLite-backed) |
| **Dispatch** | Tracker polling + eligibility checks; multi-issue concurrency | Direct HTTP API; per-conversation |
| **Agent Loop** | Multi-turn on same thread until completion | Deterministic step loop (LLM → tools → repeat) |
| **Workspace** | Per-issue persistent directories with lifecycle hooks | Per-conversation; tools operate on provided paths |
| **Protocol** | JSON-line RPC over subprocess stdio | REST with SSE streaming |
| **Retry Strategy** | Exponential backoff with stall detection | Max steps / error-driven termination |
| **Tool Visibility** | Implicit (built into agent binary) | Explicit tool registry with tiers (Core vs Deferred) |
| **Configuration** | Workflow-centric (YAML front matter + templates) | Provider/model/server config + prompt composition |
| **Approval Model** | Implementation-defined | Tool-level approval in harness tools layer |

**Key Differences**:
- Symphony is **pull-based** (orchestrator polls tracker); go-agent-harness is **push-based** (HTTP endpoints)
- Symphony achieves **multi-issue concurrency** at the orchestrator level; go-agent-harness is **per-conversation**
- Symphony's **workspace persistence** enables artifact reuse; go-agent-harness tools operate on **provided paths**
- Symphony's **turn continuation on same thread** avoids prompt re-entry; go-agent-harness uses **deterministic step loop**

## Ideas Worth Borrowing

- **Workflow-as-YAML**: WORKFLOW.md pattern (front matter config + Markdown prompt templates) could simplify harness run configuration
- **Workspace lifecycle hooks**: Pre/post-run hooks would enable artifact preservation between runs
- **Stall detection**: Reconciliation loop for detecting stuck runs is missing from harness
- **Tracker-driven dispatch**: If harness gains a job queue, Symphony's claim state machine is a solid model
- **Per-state concurrency limits**: Fine-grained throughput control per job category
