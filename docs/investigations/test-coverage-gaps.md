# Test Coverage Gaps — go-agent-harness

**Date**: 2026-03-12
**Tool**: `go test ./... -coverprofile=coverage.out` + `go tool cover -func=coverage.out`
**Overall coverage**: 80.6% of statements

---

## Summary

- **934 total tracked functions** across 30 packages
- **36 functions at 0%** (all in `demo-cli`)
- **21 functions at 1–49%** (spread across tools, workspace, provider)
- **1 package with a build failure** (`skills/`) blocking all tests in that package
- **No race conditions** detected in any passing package
- Overall coverage passes the 80% gate, but several high-risk functions in core infrastructure are significantly undertested

---

## Build Failure

### `go-agent-harness/skills` — FAILS TO BUILD

The `skills/` package test suite **does not compile**, blocking all validation tests for the bundled `SKILL.md` files.

**Error:**
```
skills/skills_validation_83_84_85_test.go:31:6: loadAllSkillsNew redeclared in this block
    skills/skills_validation_56_57_73_76_test.go:32:6: other declaration of loadAllSkillsNew
skills/skills_validation_test.go:21:6: skillsDir redeclared in this block
    skills/skills_validation_83_84_85_test.go:20:6: other declaration of skillsDir
```

**Root cause**: Multiple test files in the `skills_validation` package independently declare the helper functions `skillsDir` and `loadAllSkillsNew`. This is a copy-paste error — `skills_validation_83_84_85_test.go` re-declares both functions that already exist in `skills_validation_56_57_73_76_test.go` and `skills_validation_test.go` respectively.

**Impact**: All 40+ skills files that were added in issues #83–#85 have zero test validation because the test package doesn't compile.

---

## Packages with 0% Coverage Functions

All 36 zero-coverage functions are in `demo-cli/`. The rest of the codebase has no untested functions.

### `demo-cli` — 12.8% average coverage (36/45 functions at 0%)

The `demo-cli` package is almost entirely untested. The only tested functions are minor display helpers.

**`demo-cli/client.go` — 0% on all public API functions:**
| Function | Line | Coverage |
|---|---|---|
| `NewClient` | 55 | 0.0% |
| `HealthCheck` | 75 | 0.0% |
| `CreateRun` | 88 | 0.0% |
| `StreamEvents` | 127 | 0.0% |
| `GetPendingInput` | 187 | 0.0% |
| `SubmitInput` | 210 | 0.0% |
| `parseSSEBlock` | 237 | 0.0% |

**`demo-cli/display.go` — 0% on all display functions:**
| Function | Line | Coverage |
|---|---|---|
| `NewDisplay` | 29 | 0.0% |
| `ToggleVerbose` | 38 | 0.0% |
| `FlushAssistantMessage` | 59 | 0.0% |
| `PrintThinkingDelta` | 87 | 0.0% |
| `PrintToolStart` | 91 | 0.0% |
| `PrintToolComplete` | 97 | 0.0% |
| `PrintThinking` | 120 | 0.0% |
| `PrintRunStarted` | 124 | 0.0% |
| `PrintRunCompleted` | 128 | 0.0% |
| `PrintRunFailed` | 132 | 0.0% |
| `PrintWaitingForInput` | 137 | 0.0% |
| `PrintQuestion` | 142 | 0.0% |
| `PrintUsage` | 162 | 0.0% |
| `PrintPrompt` | 170 | 0.0% |
| `promptString` | 180 | 0.0% |
| `PrintBanner` | 187 | 0.0% |
| `PrintModelInfo` | 199 | 0.0% |
| `PrintModelSwitched` | 207 | 0.0% |
| `PrintError` | 257 | 0.0% |
| `resolveAnswer` | 262 | 0.0% |
| `toFloat` | 292 | 0.0% |

**`demo-cli/main.go` — 0% on main program logic:**
| Function | Line | Coverage |
|---|---|---|
| `main` | 62 | 0.0% |
| `update` | 26 | 0.0% |
| `completer` | 54 | 0.0% |
| `loadFileArg` | 239 | 0.0% |
| `envOrDefault` | 279 | 0.0% |
| `formatTokens` | 287 | 0.0% |
| `streamRun` | 314 | 0.0% |
| `handleUserInput` | 373 | 0.0% |

---

## Functions with 1–30% Coverage (Highest Risk)

These are non-trivial production functions with nearly no test coverage.

| Coverage | Package | Function | File:Line |
|---|---|---|---|
| 4.8% | `internal/workspace` | `ContainerWorkspace.Provision` | `container.go:51` |
| 11.1% | `internal/harness/tools/core` | `GitDiffTool` | `git.go:72` |
| 12.5% | `internal/harness/tools/deferred` | `LspReferencesTool` | `lsp.go:62` |
| 13.9% | `internal/harness/tools/core` | `ObservationalMemoryTool` | `observational_memory.go:35` |
| 15.8% | `internal/harness/tools/deferred` | `LspDiagnosticsTool` | `lsp.go:17` |
| 18.8% | `internal/harness/tools/deferred` | `SetDelayedCallbackTool` | `delayed_callback.go:14` |
| 19.0% | `internal/harness/tools/core` | `GitStatusTool` | `git.go:16` |
| 20.0% | `internal/provider/openai` | `computeCostFromUsage` | `client.go:829` |
| 20.0% | `internal/symphd` | `handleDeadLetters` | `http.go:58` |
| 20.0% | `internal/workspace` | `ContainerWorkspace.Destroy` | `container.go:153` |
| 22.0% | `internal/harness/tools/deferred` | `SourcegraphTool` | `sourcegraph.go:18` |
| 25.0% | `internal/workspace` | `VM.init` | `vm.go:72` |
| 29.7% | `cmd/symphd` | `run` | `main.go:30` |

---

## Functions with 30–60% Coverage (Moderate Risk)

| Coverage | Package | Function | File:Line |
|---|---|---|---|
| 31.2% | `internal/harness/tools/core` | `AskUserQuestionTool` | `ask_user_question.go:16` |
| 31.2% | `internal/harness/tools/deferred` | `DynamicMCPTools` | `mcp.go:99` |
| 34.9% | `internal/harness/tools/core` | `collectEntries` | `ls.go:88` |
| 35.0% | `internal/harness/tools/deferred` | `AgenticFetchTool` | `agent.go:67` |
| 42.9% | `internal/harness` | `completionUsage` | `runner.go:1716` |
| 43.8% | `internal/harness` | `Migrate` | `conversation_store_sqlite.go:98` |
| 46.2% | `internal/harness/tools/core` | `parseUnifiedPatch` | `apply_patch.go:311` |
| 50.0% | `internal/harness` | `SubmitInput` | `runner.go:499` |
| 50.0% | `internal/server` | `handleCronDeleteJob` | `http_cron.go:130` |
| 50.0% | `internal/symphd` | `NewDispatcherSimple` | `dispatcher.go:98` |
| 52.5% | `internal/harness` | `parseTurnsHTTP` | `runner.go:2107` |
| 55.6% | `internal/server` | `handleCronUpdateJob` | `http_cron.go:115` |
| 55.6% | `internal/server` | `handleCronPauseJob` | `http_cron.go:139` |
| 55.6% | `internal/server` | `handleCronResumeJob` | `http_cron.go:156` |
| 57.1% | `internal/server` | `handleCronListJobs` | `http_cron.go:68` |
| 57.9% | `internal/server` | `handleRunEvents` | `http.go:641` |
| 58.3% | `internal/harness/tools/core` | `BashTool` | `bash.go:14` |
| 58.3% | `internal/harness/tools/core` | `JobOutputTool` | `job.go:13` |
| 58.3% | `internal/harness/tools/core` | `JobKillTool` | `job.go:52` |
| 58.3% | `internal/server` | `writeSSE` | `http.go:1023` |

---

## Package Coverage Summary (Sorted by Average)

| Avg Coverage | Package | Functions |
|---|---|---|
| 12.8% | `demo-cli` | 45 funcs |
| 34.1% | `cmd/symphd` | 2 funcs |
| 70.0% | `internal/harness/tools/core` | 49 funcs |
| 76.0% | `internal/harness/tools/deferred` | 51 funcs |
| 75.3% | `internal/workspace` | 51 funcs |
| 79.8% | `internal/provider/openai` | 25 funcs |
| 80.0% | `cmd/harnessd` | 33 funcs |
| 82.8% | `cmd/cronctl` | 14 funcs |
| 83.4% | `internal/server` | 65 funcs |
| 84.2% | `internal/harness` | 133 funcs |
| 85.7% | `internal/harness/tools` | 139 funcs |
| 86.7% | `internal/cron` | 50 funcs |
| 86.6% | `internal/observationalmemory` | 68 funcs |
| 86.6% | `internal/systemprompt` | 17 funcs |
| 87.2% | `internal/watcher` | 7 funcs |
| 87.9% | `internal/symphd` | 49 funcs |
| 89.2% | `internal/provider/anthropic` | 14 funcs |
| 89.3% | `internal/rollout` | 4 funcs |
| 90.8% | `cmd/harnesscli` | 10 funcs |
| 92.4% | `internal/harness/tools/recipe` | 8 funcs |
| 95.2% | `internal/provider/catalog` | 20 funcs |
| 95.5% | `internal/skills` | 23 funcs |
| 94.2% | `internal/skills/packs` | 10 funcs |
| 95.8% | `internal/deploy` | 24 funcs |
| 100.0% | `internal/harness/tools/descriptions` | 1 func |

---

## Packages with No Test Files

Only `demo-cli` has source files with no corresponding test coverage in practice (though it does have `main_test.go` and `display_test.go`, those test files only cover a small subset).

The `.tmp/` directory contains scratch Go files (`om_live_*.go`) with no test files — these appear to be development experiments, not production code.

---

## Race Conditions and Test Failures

**No race conditions detected** in any passing package (`go test ./... -race` passed cleanly for all packages except the known build failure).

**One build failure**: `go-agent-harness/skills` — prevents skill validation tests from running. This is a compile-time error (duplicate function declarations), not a runtime issue.

**Linker warnings** (non-fatal): macOS linker warnings about malformed `LC_DYSYMTAB` appear in `cmd/symphd`, `internal/symphd`, and `internal/workspace` — these are macOS toolchain issues, not code bugs.

---

## Top 10 Highest-Risk Coverage Gaps

These are ranked by business logic complexity and risk of silent failure.

### 1. `internal/workspace/container.go:Provision` — 4.8%
**Risk: CRITICAL.** `ContainerWorkspace.Provision` is the entry point for spinning up Docker-based agent workspaces. At 4.8% coverage, the Docker client creation, container lifecycle management, port binding, and readiness polling are almost entirely untested. A regression here silently breaks all container-backed agent runs.

### 2. `skills/` package build failure — 0% (fails to compile)
**Risk: HIGH.** The `skills_validation` test suite doesn't compile due to duplicate helper function declarations across test files. All skill file validation for issues #83–#85 is non-functional. Any malformed skill file added in those issues would go undetected.

### 3. `internal/harness/runner.go:SubmitInput` — 50%
**Risk: HIGH.** `SubmitInput` is the mechanism by which mid-run user steering input reaches the active runner. It involves mutex operations and channel sends. At 50% coverage, error paths and concurrent-access scenarios are untested.

### 4. `internal/provider/openai/client.go:computeCostFromUsage` — 20%
**Risk: HIGH.** Cost computation from provider usage data is core to billing accuracy. This function handles cache-read token pricing, billable token calculation, and pricing resolver lookups. At 20%, most of the token-accounting logic is untested.

### 5. `internal/harness/tools/core/ObservationalMemoryTool` — 13.9%
**Risk: HIGH.** The observational memory tool is invoked in every conversation turn. At 13.9%, the vast majority of its argument handling, scope resolution, and storage interactions are untested. Regressions in memory storage or retrieval would be invisible.

### 6. `internal/harness/tools/core/GitStatusTool` + `GitDiffTool` — 19% / 11.1%
**Risk: HIGH.** Two of the most frequently used agent tools for code understanding. `GitDiffTool` at 11.1% means the diff parsing, filtering, and output formatting logic is almost entirely unexercised by tests.

### 7. `internal/harness/runner.go:parseTurnsHTTP` — 52.5%
**Risk: HIGH.** Parses multi-turn conversation history from HTTP payloads. Malformed turn data, missing fields, or unexpected JSON shapes could cause silent data loss or panics in production runs.

### 8. `internal/server/http.go:handleRunEvents` — 57.9%
**Risk: HIGH.** The SSE event stream endpoint is the primary real-time interface for all clients. At 57.9%, edge cases like client disconnection, slow consumers, and SSE formatting errors are not covered.

### 9. `cmd/symphd/main.go:run` — 29.7%
**Risk: MEDIUM-HIGH.** The `symphd` daemon startup function at 29.7% means most of the service wiring, signal handling, and configuration loading is untested. `cmd/symphd` overall is at 34.1% — the lowest of all runnable binaries.

### 10. `internal/harness/conversation_store_sqlite.go:Migrate` — 43.8%
**Risk: MEDIUM-HIGH.** Schema migration is a high-blast-radius operation. At 43.8%, error handling for migration failures, partial migrations, and schema version checks are not reliably tested. A regression here can corrupt the conversation store.

---

## Recommendations

1. **Fix the `skills/` build failure immediately.** Delete the duplicate `skillsDir` and `loadAllSkillsNew` declarations from `skills_validation_83_84_85_test.go` and rename them to unique helpers or remove them entirely if they are redundant.

2. **Add integration tests for `ContainerWorkspace.Provision`** using a Docker mock or a testcontainers-style helper to bring the 4.8% up to the 80% minimum.

3. **Add unit tests for `computeCostFromUsage`** covering cache-read token split, unpriced models, and provider-unreported status. These are pure functions that need no external dependencies.

4. **Add `demo-cli` test coverage** for `client.go` (SSE parsing, HTTP client construction) and key `main.go` functions (`streamRun`, `handleUserInput`). The package is at 12.8% average with 36/45 functions untested.

5. **Add regression tests for `SubmitInput`** covering concurrent access patterns with the race detector enabled, ensuring mid-run input steering doesn't lose or duplicate messages.

6. **Add tests for cron HTTP handlers** (`handleCronDeleteJob`, `handleCronPauseJob`, `handleCronResumeJob`) — all currently at 50–56% — covering error paths for missing IDs and backend failures.
