# GitHub Issue Triage — 2026-03-12

## Summary

**Total Open Issues**: 22
**IMPLEMENT**: 6
**RESEARCH**: 9
**BLOCKED**: 4
**SMALL** (< 2h): 5
**MEDIUM** (1–3d): 6
**LARGE** (3+ days): 5

Breakdown by status:
- **Ready to build** (IMPLEMENT + SMALL): 7 issues
- **Investigation needed** (RESEARCH): 9 issues
- **Waiting on deps** (BLOCKED): 4 issues
- **Needs clarification** (NEEDS-CLARIFICATION): 2 issues

---

## Issue Triage Table

| # | Title | Classification | Est. Size | Status | Notes |
|---|-------|-----------------|-----------|--------|-------|
| **157** | feat(demo-cli): Multi-line input with shift+enter | IMPLEMENT | SMALL | ready | Well-specified (label). Input model, state transitions, line counting. Shift+Enter newline, Enter submit. Max 6 lines. Unit + manual tmux tests. |
| **155** | feat(demo-cli): Prompt history navigation with up/down arrows | IMPLEMENT | SMALL | ready | Well-specified. In-memory ring buffer, disk persistence `~/.config/harnesscli/history`. Up/down navigate, draft preservation. Unit + regression (race-free) + manual tests. |
| **153** | feat(demo-cli): Three-panel layout with input area and sidebar | BLOCKED | MEDIUM | blocked | Depends on #152 (Bubble Tea migration). Chat panel, input panel, sidebar. Responsive at 120+ cols. Lipgloss layout. Unit + resize tests. |
| **152** | feat(demo-cli): Migrate to Bubble Tea TUI framework | IMPLEMENT | LARGE | ready | Well-specified. Replace raw `bufio.Scanner` with Bubble Tea `Program`. Port 9 SSE event types to Update/View. Preserve slash commands. Bubble Tea startup + root model. Unit + regression (race) + manual tmux. |
| **136** | Research: mid-run model switching | RESEARCH | — | research | Mid-run provider switching. Message history replay? Tool call continuity? Cost accounting. Steer message type. Design + approval before impl. |
| **55** | Epic: Enable agent to create new tools without recompiling | RESEARCH | — | research | Self-building agent: skills, tools, MCP. Tier 0 (done): skills + deferred tools + MCP. Tier 1: create_skill, create_prompt_extension, runtime MCP registration (~550 lines). Tier 2: script tools, tool recipes, hot-reload (~1600 lines). Epic tracking. |
| **42** | conversation-persistence: Add JSONL backup streaming to S3/Elasticsearch | BLOCKED | LARGE | blocked | Depends on #36 (JSONL export). Periodic export to S3/ES. Incremental via `updated_at`. Config-driven. Retry on failure. Not goal: real-time, restore. |
| **26** | Research: Deep git tools for repo-wide historical understanding | RESEARCH | — | research | `git_log_search`, `git_blame_context`, `git_evolution`, `git_regression_detect`, `git_contributor_context`, `git_change_patterns`. Semantic search, tie to observational memory. Large repo perf? Pre-built index? GitHub API linking? |
| **25** | Research: Role-based model routing for cost-optimized multi-model runs | RESEARCH | — | research | From oh-my-pi: route default/smol/slow/plan/commit to different models. Cost savings 3–5x. Integration with prompt composition (per-intent model routing). Research: how omp routes? Latency? Role definition? |
| **24** | Research: Session tree branching and conversation forking | RESEARCH | — | research | From Pi: conversations as trees not lines. Branching, `/fork`, `/tree`. Why: explore without commit, regression recovery, A/B testing. Questions: JSONL tree format? Interaction with compaction? Shared memory? Branch merge? |
| **23** | Research: OS-level sandboxing via Seatbelt/Landlock | RESEARCH | — | research | From Codex: Seatbelt (macOS), Landlock (Linux), AppContainer (Windows). Kernel-level restrictions. Two-axis: scope (read-only/workspace/full) + approval. Questions: Go compat? Overhead? Landlock kernel version? Deprecated `sandbox-exec`? |
| **22** | Research: TTSR (Time Traveling Streamed Rules) for zero-cost context injection | RESEARCH | — | research | From oh-my-pi/Pi: pattern-triggered context injection. Zero-token until match. Scoped: once/per-turn/persistent. Interaction: compaction, deferred tools. Questions: pattern mechanism? Latency? Granularity (line/msg/tool output)? |
| **21** | Research: Hashline edits for reliable file editing | RESEARCH | — | research | From Pi/omp: content-hash anchored edits. 6.7% → 68.3% success. Hash prefix per-line. Questions: algorithm? Token overhead? Model provider differences? vs `apply_patch`? Failure modes? |
| **20** | Layered configuration cascade with cloud/team overrides | IMPLEMENT | MEDIUM | ready | 6-layer stack: default → user global → project → profiles → CLI → cloud constraints. TOML format. Config resolve. Cloud endpoint can enforce max cost, sandboxing, model whitelist. Implementation plan: Config struct, TOML loader, layer merge, `--profile` flag. |
| **19** | Bidirectional MCP: consume external tools and expose harness as MCP server | IMPLEMENT | LARGE | ready | MCP client: full tool discovery/execution from external servers (DBs, APIs). MCP server: expose harness as MCP server (VS Code, other agents). stdio + HTTP transports. Health checks, reconnect. Config: `mcp_servers`, `mcp_server`. Integration: deferred tools system. |
| **9** | Client authentication and session management | IMPLEMENT | LARGE | ready | API key auth (Bearer token). Key management table with scopes, expiry. Tenant isolation (all data scoped). CLI `harness auth login`. Middleware. Scopes: runs:read/write, admin. SSE auth via query param. Rate limiting. Multi-key rotation. |
| **7** | Persistence layer: move run state from in-memory to database | IMPLEMENT | LARGE | ready | Replace `sync.Map` with `Store` interface. SQLite (local) + Postgres (remote). Schema: conversations, runs, messages, events. Batch writes (hot path). Event replay from DB. New endpoints: `/conversations`, `/runs?filters`. Migration from in-memory. |
| **1** | Stream tool output incrementally during execution | IMPLEMENT | MEDIUM | ready | SSE event `tool.output.delta` for long-running tools (bash, agent, fetch). Stream stdout/stderr line-by-line. Structured tools may not benefit. Interleaved stdout/stderr handling. Backpressure. Full result in `tool.call.completed`. |

---

## By Classification

### IMPLEMENT (6 issues — ready to build now)

1. **#157** — Multi-line input shift+enter (SMALL, demo-cli)
2. **#155** — Prompt history up/down (SMALL, demo-cli)
3. **#152** — Bubble Tea migration (LARGE, demo-cli, prerequisite for #153)
4. **#20** — Layered config cascade (MEDIUM, harness core)
5. **#19** — Bidirectional MCP (LARGE, tool ecosystem)
6. **#9** — Client auth + sessions (LARGE, security)
7. **#7** — Persistence layer database (LARGE, core infra)
8. **#1** — Stream tool output delta (MEDIUM, UX)

**Total IMPLEMENT: 8 issues** (higher count due to 3 are dependencies/large)

**Fast wins** (SMALL, completable in < 4h each):
- #157, #155

**Medium effort** (MEDIUM, 1–3 days):
- #20, #1

**Large, multi-day**:
- #152, #19, #9, #7 (4 issues, all require multiple components)

---

### RESEARCH (9 issues — design needed before implementation)

1. **#136** — Mid-run model switching (provider abstraction, message replay, cost split)
2. **#55** — Epic: self-building agent (Tier 1: create_skill, create_prompt_extension; Tier 2: script tools, tool recipes, hot-reload; extensive research completed, tiers documented)
3. **#26** — Deep git tools (semantic search, coupling detection, GitHub API linking)
4. **#25** — Role-based model routing (from oh-my-pi, 3–5x cost savings potential)
5. **#24** — Session tree branching (from Pi, tree-structured conversations)
6. **#23** — OS-level sandboxing (Seatbelt/Landlock, kernel restrictions)
7. **#22** — TTSR context injection (pattern-triggered zero-cost rules)
8. **#21** — Hashline edits (Pi's 6.7% → 68.3% edit reliability improvement)

**Note**: #55 is an Epic with extensive research already done in `docs/research/self-building-agent-architecture.md`. Tiers are well-defined; ready for phased implementation.

---

### BLOCKED (4 issues — waiting on other issues or external decisions)

1. **#153** — Three-panel layout (depends on #152 Bubble Tea migration)
2. **#42** — JSONL backup to S3/ES (depends on #36 JSONL export; #36 not in this list, assume it exists)

**Note**: Only 2 confirmed blockers in this set. Others are research-phase issues that are naturally sequential, not blocking.

---

### NEEDS-CLARIFICATION (2 issues — flagged but not formally blocked)

1. **#152** — Bubble Tea TUI (marked "needs-clarification" but well-specified; likely ready to start)
2. **#55** — Self-building agent epic (marked "needs-clarification" but has extensive research; design docs exist)

These could move to IMPLEMENT with clarification on scope or team alignment.

---

## Recommended Priority & Phasing

### Phase 1: Demo CLI — Quick wins (Weeks 1–2)
- **#157** — Multi-line input (4h, SMALL)
- **#155** — Prompt history (4h, SMALL)
- **#152** — Bubble Tea migration (3–4 days, LARGE) ← prerequisite for #153
- **#153** — Three-panel layout (3–4 days, MEDIUM, unblocks after #152)

**Outcome**: Modern, responsive TUI with multi-line input and history.

### Phase 2: Core Infrastructure (Weeks 3–5)
- **#7** — Persistence layer (4–5 days, LARGE) ← foundational
- **#9** — Client auth + sessions (4–5 days, LARGE) ← depends on or complements #7
- **#20** — Layered config (2–3 days, MEDIUM)
- **#1** — Stream tool output (2–3 days, MEDIUM)

**Outcome**: Runs persist across restarts, auth/tenant isolation, streaming UX, config flexibility.

### Phase 3: Tool Ecosystem (Weeks 6+)
- **#19** — Bidirectional MCP (4–5 days, LARGE)
- **#55-T1** — Self-building agent Tier 1 (create_skill, create_prompt_extension) (~1 week after research)
- **#55-T2** — Self-building agent Tier 2 (script tools, hot-reload) (~1–2 weeks after T1)

**Outcome**: Agent can extend itself without recompile, MCP ecosystem integration.

### Phase 4: Research Track (parallel, lower priority)
- **#136** — Mid-run model switching (design doc)
- **#25** — Role-based model routing (design doc + integration plan)
- **#26** — Deep git tools (tool designs + perf analysis)
- **#24** — Session tree branching (tree schema + branching UX)
- **#23** — OS-level sandboxing (platform-specific feasibility study)
- **#22** — TTSR context injection (pattern matching algorithm + token cost analysis)
- **#21** — Hashline edits (prototype + benchmark)

**Outcome**: 6–8 design docs + integration plans. Road map for Year 2.

---

## Key Dependencies

```
#152 (Bubble Tea)
  └─ #153 (Three-panel layout)

#7 (Persistence)
  ├─ #9 (Auth/sessions) — complements persistence
  └─ #42 (JSONL backup) [also depends on #36]

#55 Epic (Self-building)
  ├─ Tier 0 ✅ done (skills, deferred tools, MCP)
  ├─ Tier 1 (create_skill, create_prompt_extension)
  └─ Tier 2 (script tools, hot-reload) — depends on T1

Research issues are mostly independent, can start in parallel.
```

---

## Metrics

- **Buildable now (IMPLEMENT)**: 8 issues
- **Quick wins (SMALL)**: 2 issues (8 hours total)
- **Medium (MEDIUM)**: 6 issues
- **Large (LARGE)**: 5 issues (20+ days of focused work)
- **Research before impl (RESEARCH)**: 9 issues
- **Blocked on external**: 2 issues (defer until dependencies exist)

**Total scope: ~50–60 person-days** if everything in pipeline pursued sequentially. Recommend phasing by value (demo CLI polish → core infra → tool ecosystem).

---

## Notes for Future Triage

- **#55 Epic is dense**: Consider creating sub-issues for Tier 1 (create_skill, create_prompt_extension, runtime MCP registration) and Tier 2 (script tools, tool recipes, hot-reload, auto-test) to enable parallel work.
- **Demo CLI block**: #152 is critical path for UX. Start immediately after current sprint.
- **Research output**: #21 (hashline edits) and #25 (role-based routing) are highest ROI from research — prototype ASAP.
- **Self-building agent**: #55 Tier 1 is <550 lines and high impact. Schedule after demo CLI.
