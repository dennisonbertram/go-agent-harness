# Clarification-Needed Issues — 2026-03-18

This document covers 11 open issues that are blocked or need clarification before implementation can begin.
For each issue: what we know, what is unclear, and 2–3 sharp clarifying questions to ask the owner.

---

## #313 — TUI: show model availability based on provider configuration

**Labels:** enhancement, medium, needs-clarification, tui

### What we know

- The backend `GET /v1/providers` endpoint already returns `configured: bool` per provider (populated via `ProviderRegistry.IsConfigured()` which checks whether the provider's `api_key_env` var is set in the environment).
- The TUI already fetches this data via `fetchProvidersCmd` and stores it in `m.apiKeyProviders` when `ProvidersLoadedMsg` arrives.
- However, the `modelswitcher` component has **no `Available`/`Configured` field on `ModelEntry`** — provider availability is never passed into the model picker, so all models render identically regardless of whether their provider is configured.
- `ModelEntry` fields: `ID`, `DisplayName`, `Provider`, `ProviderLabel`, `ReasoningMode`, `IsCurrent`. No availability field.
- The `WithModels()` method populates the switcher from `ModelsFetchedMsg` (server model list) but does not cross-reference provider availability.
- The `ProvidersLoadedMsg` and `ModelsFetchedMsg` arrive separately; they need to be joined before the switcher can render availability.

**Gap:** Provider availability data is in the TUI state (`m.apiKeyProviders`) but is not threaded into the `modelswitcher` component.

### What's unclear / decisions needed

1. **Which endpoint drives model+availability?** The `GET /v1/models` endpoint returns the model list; `GET /v1/providers` returns provider availability. Should the backend grow a combined endpoint (e.g. `GET /v1/models?include_availability=true`), or should the TUI join two separate fetches client-side?
2. **Should unavailable models be selectable?** The issue says "muted/greyed out" but does not say whether the user can still select them (e.g. to pre-configure before adding an API key), or whether selection is blocked with an error message.
3. **Hardcoded `DefaultModels` list:** `modelswitcher.DefaultModels` is a static hardcoded list with 15 entries. This list becomes stale as providers are added. Should this list be eliminated in favor of always fetching from the server, or kept as a fallback when the server is unreachable?

### Clarifying questions

1. Should unavailable models be selectable (with a warning) or non-selectable (blocked with a prompt to add an API key)?
2. Should availability be driven by joining two separate API calls in the TUI, or should the backend provide a single endpoint that returns models with their availability state already merged?
3. Should the hardcoded `DefaultModels` list be removed in favor of server-only data, or retained as offline fallback?

---

## #314 — Feature: add Codex MCP server integration as a future optional capability

**Labels:** deferred, large, needs-clarification

### What we know

- The harness already has a `codex` provider path backed by `codex app-server` (OpenAI Responses API for Codex models like `gpt-5.1-codex-mini`).
- `connect_mcp` is a live deferred tool that can register any MCP server at runtime — so `codex mcp-server` could already be connected ad hoc without new code.
- The issue asks for a **first-class integration** of `codex mcp-server` as distinct from the provider path, but has no design beyond three architectural options: global config, per-run option, or delegated tool surface.
- The issue is labeled `deferred` — intentionally out of scope for current Codex provider work. No acceptance criteria beyond "write a design doc."

### What's unclear / decisions needed

1. **What does `codex mcp-server` expose that `codex app-server` does not?** The motivation for a separate MCP path is unclear: if `codex app-server` handles inference and `connect_mcp` already handles runtime MCP registration, what specific capability gap does a first-class `codex mcp-server` integration fill?
2. **Architecture decision:** Global config vs. per-run option vs. delegated tool surface — none of these has been evaluated or selected.
3. **Is this actually blocked on Codex login/auth (issue #315)?** The two issues are related but have no explicit dependency arrow.

### Clarifying questions

1. What specific capability does `codex mcp-server` expose that cannot be covered by `codex app-server` inference plus the existing `connect_mcp` tool?
2. Should this be global server config, a per-run option in `RunRequest`, or a deferred tool the agent invokes? Pick one before any design work begins.
3. Is this issue dependent on #315 (provider auth management) landing first, since Codex login state would need to be surfaced?

---

## #315 — TUI: add provider authentication management for Codex login and API keys

**Labels:** deferred, large, needs-clarification, tui

### What we know

- The TUI already has a provider key management panel (`configpanel` component, `setProviderKeyCmd` in `api.go`, `PUT /v1/providers/{provider}/key` endpoint).
- The issue asks for a **more complete UX**: initiating Codex login flow (not just API key entry), surfacing auth state clearly, and documenting security constraints before implementation.
- The server can start without a default API key; providers have different auth modes but `ProviderEntry` only has `APIKeyEnv` — no `AuthMode` field distinguishing `api_key_env` vs `codex_login`.
- This is intentionally deferred and out of scope for the current sprint.

### What's unclear / decisions needed

1. **Codex login UX:** Codex uses OAuth/browser-based login, not an API key. The current `PUT /v1/providers/{id}/key` endpoint pattern does not fit the Codex auth flow. What is the expected flow — launch a browser, poll for token, store in harnessd? Or instruct the user to run `codex auth` in a terminal?
2. **Security constraint scope:** The issue requires "security constraints for storing or forwarding credentials are documented before implementation." What is the intended credential storage model — environment variable injection, encrypted config file, harness keychain?
3. **Relationship to #313:** If provider auth state is surfaced better in the TUI (this issue), does that drive or replace the model availability rendering in #313?

### Clarifying questions

1. For the Codex login flow specifically: should harnessd orchestrate the OAuth redirect/callback, or should the TUI instruct the user to run `codex auth` externally and then poll for the resulting token?
2. What is the approved credential storage model — env vars injected at harnessd start, encrypted config file, or OS keychain?
3. Does this issue need to land before #313 (model availability), or are they independent?

---

## #324 — feat(runs): make workspace backends selectable per run

**Labels:** enhancement, infrastructure, large, well-specified, workspace

### What we know

- The workspace abstraction (`internal/workspace/`) is complete with `local`, `worktree`, `container`, `vm`, and `pool` backends.
- Subagents (`internal/subagents/manager.go`) and symphd (`internal/symphd/orchestrator.go`) already use workspace backends.
- `RunRequest` does not expose a `workspace` block — normal runs via `POST /v1/runs` always execute in the server's local process context.
- This issue is well-specified with a proposed `RunRequest.workspace` block design, cleanup semantics, and a test plan.
- The issue is labeled `well-specified` not `needs-clarification` but is in the list to review.

### What's unclear / decisions needed

1. **Config gate:** The issue says "dependency-gate container/VM/pool modes when the server is not configured to support them." What is the exact config mechanism? A new `[workspace]` section in `harnessd.toml`? Or should the server detect available backends at startup via `BuildWorkspaceFactory`?
2. **Cleanup policy granularity:** The issue says "cleanup semantics are explicit for normal runs, including failure and cancellation paths." The current `WorkspaceFactory` implementation in symphd uses a simple deferred `Close()`. For runs that include large container/VM workspaces, should cleanup be: immediate on terminal status, delayed (TTL after completion), or user-controlled via a `DELETE /v1/runs/{id}/workspace` endpoint?
3. **Pool workspace ownership:** When `workspace.type=pool`, a workspace is borrowed from the pool. What happens if the pool is exhausted when a run requests it — queue the run, fail fast, or fall back to a local workspace?

### Clarifying questions

1. How should the server gate unsupported workspace types — a config file section, a startup-time probe, or just a runtime error when the client requests an unsupported type?
2. For `container` and `vm` workspace types, what is the cleanup policy — immediate on run completion, TTL-based, or explicit client-controlled deletion?
3. When `type=pool` and the pool is exhausted, should the run queue (block), fail with a structured error, or silently fall back to `local`?

---

## #237 — feat(agents): built-in profile system with self-improving efficiency review loop

**Labels:** blocked, enhancement, large, needs-clarification

### What we know

- No `internal/profiles/` package exists. No `run_agent`, `create_profile`, or `list_profiles` tools exist anywhere in the codebase.
- Both blockers (#234 per-run tool filtering and #236 config propagation) are now **closed** — they have been implemented and merged. The formal blockers are gone.
- The issue presents four unresolved storage options (hardcoded/embedded TOML/DB/YAML) without deciding among them.
- The self-improving efficiency review loop is a large separate concern bundled into the same issue.
- The grooming note from the prior triage session recommends splitting into three sub-issues: (a) profile registry + `run_agent` tool, (b) HTTP CRUD API, (c) efficiency review loop.
- Six open design questions remain in the issue body (versioning, score visibility, reviewer profile, auto-apply vs. suggest-only, cross-project sharing, profile discovery).

### What's unclear / decisions needed

1. **Storage backend decision:** The issue proposes TOML files in `~/.harness/profiles/` with embedded built-ins. This is the only viable path given the issue text, but has not been formally decided. An alternative is a DB-backed registry similar to `ConversationStore`.
2. **Scope boundary:** Should the efficiency review loop be part of this issue or a separate one? The grooming note strongly recommends splitting it out, but the original issue treats it as core.
3. **Profile vs. existing `AllowedTools` in `RunRequest`:** The `RunRequest` already has `AllowedTools` and `SystemPrompt`. A profile is essentially a named preset for these fields. Should `run_agent` accept a `profile` name that gets resolved to these fields, or should it accept inline configuration that bypasses the profile system?

### Clarifying questions

1. Should profiles be stored as TOML files on disk (as specified) or in the SQLite store alongside conversations and run records?
2. Should the efficiency review loop be split into a separate issue, or is it required for the initial `run_agent` tool to be considered complete?
3. Can `run_agent` be the same tool as the existing subagent spawning in `http_agents.go` (just with profile resolution added), or must it be a new tool with a different code path?

---

## #235 — feat(recursion): recursive agent spawning with DB-backed suspension, result pointers, and JSONL grep

**Labels:** blocked, enhancement, large, needs-clarification

### What we know

- `ContextKeyForkedSkill` in `internal/harness/tools/core/skill.go` is a binary on/off flag at line 123 that explicitly blocks all nesting ("nested skill forking is not supported"). This must become a depth counter.
- No `run_results` table exists in the store. No `parent_run_id` or `depth` column in the `runs` table.
- `RunForkedSkill()` interface exists in the `ForkedAgentRunner` interface but parent suspension on child run is not implemented.
- Both declared blockers (#234 and #236) are now **closed**.
- The prior grooming note recommends 4 phases: (1) depth counter + suspension, (2) result pointer storage + JSONL grep, (3) backpressure management, (4) oversight hooks. The issue has 31 acceptance criteria.
- The `task_complete` tool, `spawn_agent` tool, path materialization, and watcher goroutine are all described but none exist.

### What's unclear / decisions needed

1. **Phase prioritization:** The issue is too large for a single PR. Which phase should be implemented first? The grooming note says Phase 1 (depth counter + suspension) is the right starting point, but the issue owner has not confirmed this split.
2. **`task_complete` as mandatory return path:** The issue proposes that `task_complete` is the only valid return path for subagents. This is a behavior change for all existing subagent runs (skills using `ForkedAgentRunner`). Should this be enforced only for runs spawned via the new `spawn_agent` tool, or retrofitted to all `depth > 0` runs including existing skill forks?
3. **Path materialization vs. depth counter:** The issue adds a `path TEXT` column for subtree queries. Is this required for Phase 1, or is it an optimization that can be deferred to a later phase once the basic depth counter and suspension work?

### Clarifying questions

1. Should this be split into 4 separate GitHub issues (one per phase) before implementation starts, or implemented as a single large PR?
2. Should the `task_complete` mandatory return path apply to all `depth > 0` runs (including current skill forks), or only to runs spawned via the new `spawn_agent` tool?
3. Is the `path` materialization column required in Phase 1, or can it be deferred until subtree cancellation and budget rollup are needed?

---

## #153 — feat(demo-cli): Three-panel layout with input area and sidebar

**Labels:** blocked, demo-cli, enhancement, medium, tui

### What we know

- This issue is explicitly blocked on #152 (Bubble Tea migration of `demo-cli/`).
- `demo-cli/main.go` still uses a raw `bufio.Scanner` loop with `go-prompt` — no Bubble Tea, no lipgloss.
- The issue is otherwise well-specified: three panels (chat, input, sidebar), responsive hide at <120 cols, independent scroll, cost+model in sidebar.
- The grooming comment notes two remaining clarifications: exact terminal width breakpoint and chat scroll anchoring.

### What's unclear / decisions needed

1. **Blocker status:** Is #152 (Bubble Tea migration) actively being worked or is it also stalled? If #152 is not being prioritized, #153 cannot start.
2. **Terminal width breakpoint:** The issue says "<120 cols" hides the sidebar but the grooming note asks if this should be a different value (80? 100?). This needs to be pinned before layout implementation.
3. **Scroll anchoring:** When a new message arrives during streaming, should the chat panel auto-scroll to the bottom (lock-to-bottom), or stay at the user's current scroll position until they manually scroll down?

### Clarifying questions

1. Is #152 (Bubble Tea migration) being actively worked, and what is the expected merge date? #153 cannot start until it lands.
2. What is the exact terminal width breakpoint for hiding the sidebar — 80, 100, or 120 columns?
3. When a streaming message arrives: does the chat panel auto-scroll to bottom (lock), or preserve the user's current scroll position?

---

## #152 — feat(demo-cli): Migrate to Bubble Tea TUI framework

**Labels:** demo-cli, enhancement, large, needs-clarification, tui

### What we know

- `demo-cli/main.go` is a raw `bufio.Scanner`/`go-prompt` REPL — confirmed still present.
- `cmd/harnesscli/` is a fully-built Bubble Tea TUI (`cmd/harnesscli/tui/`) with BubbleTea v2, lipgloss, glamour, 60+ TUI issues closed. This was built _for the main CLI_, not as a demo-cli replacement.
- The `demo-cli/` is a separate, simpler program in the repo root. It is not the same as `cmd/harnesscli/`.
- Issue comments ask: should `cmd/harnesscli` be deprecated once demo-cli is stable, or maintained in parallel?
- The second comment asks 4 more questions: backward-compat flags/env vars, existing tests, Bubble Tea version, full replacement vs. wrapper.

### What's unclear / decisions needed

1. **`demo-cli/` vs. `cmd/harnesscli/` relationship:** `cmd/harnesscli/` is already a full Bubble Tea TUI. Is the intent for `demo-cli/` to become a second TUI, or should `demo-cli/` be deprecated/removed and `cmd/harnesscli/` treated as the canonical TUI? This is the most important clarification — it changes whether #152 is a new build or a migration.
2. **Breaking change tolerance:** Should the migrated demo-cli maintain the same CLI flags (`-server`, `-model`, etc.) and env vars as the current demo-cli, or is a clean break acceptable?
3. **Bubble Tea version:** `cmd/harnesscli/tui/` uses BubbleTea v2. The issue says "v2 as mentioned." This should be confirmed and pinned.

### Clarifying questions

1. Given that `cmd/harnesscli/` is already a full Bubble Tea TUI, should `demo-cli/` be migrated to Bubble Tea as a separate program, or should `demo-cli/` be retired in favor of `cmd/harnesscli/`?
2. Is backward-compatibility with current `demo-cli` flags and env vars required, or is a clean break acceptable for this migration?
3. If `demo-cli/` is being kept as a separate lighter program, should it share components from `cmd/harnesscli/tui/` (e.g. the modelswitcher, bridge), or be a fully independent implementation?

---

## #55 — Epic: Enable agent to create new tools without recompiling

**Labels:** epic, infrastructure, needs-clarification, self-building

### What we know

- The grooming comment on the issue confirms: **Tier 0–2 is largely implemented.** Specifically: skills system, deferred tool activation (`find_tool`), MCP integration, `create_skill`, `connect_mcp`, script tool loader (`internal/harness/tools/script/loader.go`), tool recipe system (`internal/harness/tools/recipe/`), and skill verification.
- Two Tier 2 items remain: hot-reload file watcher (`internal/watcher/` package exists but its integration with tool registration is unconfirmed) and automated skill verification loop.
- Tier 3 (go-plugin gRPC subprocess, WebAssembly) has not been implemented. The issue's own "What NOT to Build" section argues against it, but the grooming note asks if it's still in scope.
- The acceptance criteria in the issue body are stale — they still list completed work as unchecked.

### What's unclear / decisions needed

1. **Tier 3 scope decision:** Is the go-plugin/WASM dynamic compilation path in scope or out? The issue's own text argues against it, but the label `epic` and the grooming note both leave it open.
2. **Hot-reload watcher:** `internal/watcher/` exists. Is it integrated with the tool catalog (does it currently auto-register new scripts/skills dropped into `~/.harness/skills/`) or is that the remaining work?
3. **Epic closure criteria:** What would close this epic — completion of all Tier 2 remaining items, or a deliberate decision to defer Tier 3?

### Clarifying questions

1. Is Tier 3 (go-plugin/WASM runtime plugins) definitively out of scope? If yes, the epic can close when the hot-reload watcher and automated verification loop land.
2. Is `internal/watcher/` already integrated to auto-register skills/scripts at runtime, or is that the remaining implementation gap?
3. Should this epic be closed now (updated acceptance criteria, remaining Tier 2 items tracked as separate issues), or kept open as an umbrella?

---

## #42 — conversation-persistence: Add JSONL backup streaming to S3/Elasticsearch

**Labels:** blocked, enhancement, large, needs-clarification

### What we know

- Issue #36 (JSONL export) is **closed** — `GET /v1/conversations/{id}/export` is live and returns JSONL.
- The blocker is resolved. However, the grooming comment says "do not start until #36 is closed" — it is now closed, so the formal blocker is gone.
- No S3 or Elasticsearch integration exists in the codebase.
- The grooming comment lists 4 unresolved clarifications: backup trigger frequency (real-time vs. periodic), granularity (per-message vs. per-run), whether Elasticsearch is mandatory or optional, and retry policy/alerting.
- The issue's design says "configurable" via `HARNESS_BACKUP_DESTINATION` env var — format `s3://bucket/prefix/` or `es://host:9200/index`.

### What's unclear / decisions needed

1. **Backup trigger:** Real-time streaming on every message write vs. periodic batch export (e.g., every 5 minutes) vs. on run completion only. These have very different implementation complexity and cost characteristics.
2. **Elasticsearch requirement:** Is ES a hard requirement for v1, or is S3-only the MVP with ES as a stretch goal?
3. **Operational readiness:** This feature requires AWS SDK or ES client as a new Go dependency. What is the policy for adding external backup dependencies — are AWS credentials and bucket names expected to be pre-configured, or should the feature work with a mock/local MinIO for development?

### Clarifying questions

1. Now that #36 is closed, is this issue unblocked and ready to prioritize, or still deferred?
2. For the backup trigger: periodic batch (every N minutes) or event-driven (on run completion)? This is the single most important design decision.
3. Is Elasticsearch required for v1, or is S3-only acceptable as the initial release with ES added later?

---

## #26 — Research: Deep git tools for repo-wide historical understanding

**Labels:** large, needs-clarification, research

### What we know

- None of the six proposed tools (`git_log_search`, `git_blame_context`, `git_evolution`, `git_regression_detect`, `git_contributor_context`, `git_change_patterns`) exist anywhere in the codebase.
- The existing git tools are shallow: `git_diff` and `git_status` only, both implemented as thin wrappers.
- The issue is labeled `research` — the ask is a design doc, prototypes of two tools, performance benchmarks, and a memory integration design.
- The grooming note from the prior triage session identified 6 missing clarifications, chief among them: the search mechanism for `git_log_search` (regex vs. embedding vs. heuristic), performance target repo size, and whether `git_evolution` triggers automatically or on-demand.
- Strategic context: the issue positions this as the "biggest differentiation opportunity" vs. Codex/OpenCode/Crush.

### What's unclear / decisions needed

1. **Search mechanism for `git_log_search`:** Semantic embedding search (requires a local embedding model or API call) vs. regex/keyword search (fast but shallow) vs. a hybrid. This is a fundamental architecture decision that determines whether the feature requires Ollama/OpenAI embeddings.
2. **Performance target:** Should the tools target typical project repos (1k–10k commits, reasonable latency) or scale to Linux-kernel-size repos (1M commits)? Pre-built index vs. on-demand query has very different tradeoffs.
3. **GitHub API dependency:** `git_blame_context` linking commits to PRs/issues requires the GitHub API. Does this require a `GITHUB_TOKEN` to be configured, making the tool unavailable in non-GitHub repos?

### Clarifying questions

1. Should `git_log_search` use semantic embedding search (requires an embedding model) or keyword/regex search over commit messages? Pick one for the prototype.
2. What is the target repo size for performance? Typical project (10k commits) or large monorepo (100k+ commits)?
3. Does `git_blame_context` require GitHub API integration (and thus a `GITHUB_TOKEN`), or should it work purely from local git metadata for non-GitHub repos?

---

*Generated: 2026-03-18*
