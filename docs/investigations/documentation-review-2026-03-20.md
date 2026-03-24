# Documentation Review — 2026-03-20

Automated scheduled review comparing project documentation against the actual codebase.

## Overall Assessment

Documentation covers roughly **50-60%** of actual functionality. Several major subsystems are entirely undocumented. The README is the most visible gap, followed by the event catalog and missing reference docs.

---

## Critical Issues

### 1. Event Catalog Severely Incomplete
**File:** `docs/design/event-catalog.md`
- **Documented:** ~22 event types
- **Actual:** 76 `EventType` constants in `internal/harness/events.go`
- **Missing categories:** approval events, cost forensics, hook events, callback events, skill constraints, auto-compaction, context window, LLM request envelope, error context, audit trail, causal graph, workspace lifecycle, spawn/task events, steering events, and more
- **Impact:** API consumers cannot understand the full event stream

### 2. README Missing Major Sections
**File:** `README.md`
- **3 undocumented endpoints:** `POST /v1/runs/{id}/cancel`, `/approve`, `/deny`
- **13+ undocumented env vars:** All `HARNESS_MEMORY_*` variables, `HARNESS_RUN_DB`, `HARNESS_CONVERSATION_RETENTION_DAYS`
- **Missing features:** Approval/denial workflow, profile system, worker pool mode, forensics system, memory system, script tools, subagent delegation
- **6-layer config cascade** not explained (defaults → global TOML → project TOML → profile → env vars → cloud constraints)

### 3. No API Endpoint Reference
- No comprehensive API documentation file exists
- `internal/server/http.go` defines 30+ routes across runs, conversations, models, agents, subagents, providers, cron, skills, recipes, search, MCP
- Developers must read source code to understand the API

### 4. No Configuration Reference
- `internal/config/config.go` defines: CostConfig, MemoryConfig, AutoCompactConfig, ConclusionWatcherConfig, ForensicsConfig (15+ feature flags)
- No user-facing documentation of available TOML sections, enum values, or defaults

---

## High-Priority Issues

### 5. Tool Catalog Documentation Stale
- **README** doesn't mention 20+ deferred tools
- **`docs/design/tool-roadmap.md`** missing newer tools (spawn_agent, task_complete, git deep history)
- **`docs/investigations/tool-catalog-review.md`** (2026-03-09) has unimplemented recommendations:
  - `job_kill` recommended for demotion → still Core
  - LSP tools, `agentic_fetch`, `web_fetch` recommended for CUT → still present
- **Wiring bugs:** `sourcegraph` gated by config never set; `list_models` requires `ModelCatalog` never populated in cmd/harnessd

### 6. Forensics System Undocumented (14 flags)
Config fields exist but no docs: `trace_tool_decisions`, `detect_anti_patterns`, `trace_hook_mutations`, `capture_request_envelope`, `snapshot_memory_snippet`, `error_chain_enabled`, `error_context_depth`, `capture_reasoning`, `cost_anomaly_detection_enabled`, `cost_anomaly_step_multiplier`, `audit_trail_enabled`, `context_window_snapshot_enabled`, `context_window_warning_threshold`, `causal_graph_enabled`

### 7. Auto-Compaction Undocumented (5 fields)
`auto_compact.enabled`, `mode` (strip/summarize/hybrid), `threshold` (default 0.80), `keep_last` (default 8), `model_context_window` (default 128000)

---

## Medium-Priority Issues

### 8. Tool Tier/Tag System Not Documented
- Each tool has search tags for `find_tool` discovery, but tag values aren't documented publicly
- Core vs Deferred tier assignments aren't listed anywhere user-facing

### 9. Deployment Runbook Minimal
- `docs/runbooks/deployment.md` lacks pre/post-deploy checks from CLAUDE.md
- No mention of Railway or specific deployment infrastructure

### 10. CLI Flag Descriptions Imprecise
- `-prompt-behavior` and `-prompt-talent` are repeatable/comma-separated but docs don't clarify

---

## Verified Accurate (No Issues Found)

| Document | Status |
|----------|--------|
| `docs/design/system-prompt-architecture.md` | Accurate — folder structure, RunRequest fields match code |
| `docs/design/observational-memory-architecture.md` | Accurate — packages and tool references valid |
| `docs/runbooks/mcp.md` | Comprehensive and accurate |
| `docs/runbooks/testing.md` | Accurate — test commands and coverage gates correct |
| `docs/runbooks/worktree-flow.md` | Accurate — script references valid |
| `docs/runbooks/harnesscli-live-testing.md` | Accurate — all CLI flags listed |
| `docs/runbooks/documentation-maintenance.md` | Accurate — INDEX files exist |
| `docs/context/critical-context.md` | Accurate |
| Provider statement (OpenAI primary, Anthropic exists) | Correct |
| Build/run instructions in README | Correct |

---

## Recommendations (Priority Order)

1. **Update `docs/design/event-catalog.md`** — enumerate all 76 events from `AllEventTypes()` with payload descriptions
2. **Create `docs/context/api-reference.md`** — document all HTTP endpoints, methods, request/response formats
3. **Create `docs/context/configuration.md`** — document all config options, TOML sections, env vars with defaults
4. **Expand README.md** — add missing endpoints (cancel/approve/deny), env vars (memory/forensics), and feature sections (approval workflow, profiles, memory, forensics, script tools)
5. **Update `docs/design/tool-roadmap.md`** — add newer tools, resolve stale CUT recommendations
6. **Document forensics and auto-compaction** — create dedicated section or runbook
7. **Add sample TOML config** — show all available sections with commented defaults
8. **Fix tool wiring bugs** — sourcegraph env var support, list_models catalog population

---

## Environment Variable Coverage

| Category | Documented | Undocumented | Coverage |
|----------|-----------|-------------|----------|
| Core server | 6 | 0 | 100% |
| Provider (OpenAI) | 2 | 0 | 100% |
| Workspace/Paths | 6 | 0 | 100% |
| Skills/Watch | 3 | 0 | 100% |
| Memory system | 0 | 12 | 0% |
| Forensics | 0 | 14 | 0% |
| Auto-compaction | 0 | 5 | 0% |
| Run database | 0 | 1 | 0% |
| Conversation | 1 | 1 | 50% |
| **Total** | **18** | **33** | **~35%** |
