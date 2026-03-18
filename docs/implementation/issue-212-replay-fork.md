# Issue #212: Forensics — Run Replay and Fork from Any Step

**Date**: 2026-03-18
**Issue**: [#212 Forensics: Run replay and fork from any step (phased)](https://github.com/dennisonbertram/go-agent-harness/issues/212)
**Status**: Complete (all 4 phases implemented and passing regression)

---

## Summary

Issue #212 implements a forensic replay/fork system that lets operators:
- **Simulate** a historical run offline from a JSONL rollout file (no live LLM calls)
- **Fork** a run from any step: reconstruct conversation history up to step N, then hand off to a live LLM

All four phases were fully implemented and all tests pass with `[regression] PASS (total=85.1%, min=80.0%, zero-functions=0)`.

---

## Implementation Overview

### Phase 1: Rollout Loader + Canonicalization (`internal/forensics/rollout/`)

- `loader.go`: Reads JSONL rollout files into `[]RolloutEvent`
- `canonicalizer.go`: Normalizes event payloads for deterministic comparison

### Phase 2: Offline Replay Simulation (`internal/forensics/replay/replayer.go`)

- `replay.Replay(events []RolloutEvent) ReplayResult`: Steps through recorded events, replays tool outputs without executing them live. Returns step count, match count, and mismatches.

### Phase 3: Fork from Step (`internal/forensics/replay/forker.go`)

- `replay.Fork(events []RolloutEvent, forkStep int, opts *ForkOptions) (ForkResult, error)`: Reconstructs conversation history (messages slice) up to the specified step. Returns a `ForkResult` with the reconstructed `[]harness.Message` ready to seed a new live run.

### Phase 4: Replay/Fork API (`internal/server/http_replay.go`)

The HTTP endpoint `POST /v1/runs/replay` with two modes:

**Simulate mode** (`mode=simulate`):
```json
POST /v1/runs/replay
{
  "rollout_path": "/path/to/run.jsonl",
  "mode": "simulate"
}
```
Response:
```json
{
  "mode": "simulate",
  "events_replayed": 47,
  "step_count": 8,
  "matched": 42,
  "mismatches": 5
}
```

**Fork mode** (`mode=fork`):
```json
POST /v1/runs/replay
{
  "rollout_path": "/path/to/run.jsonl",
  "mode": "fork",
  "fork_step": 3
}
```
Response:
```json
{
  "mode": "fork",
  "run_id": "run_abc123",
  "from_step": 3,
  "original_step_count": 8,
  "original_outcome": "completed",
  "messages_restored": 7
}
```

The endpoint is registered in `internal/server/http.go` under scope `store.ScopeRunsWrite`.

---

## Files Changed

### Core Implementation (pre-existing, confirmed working)
- `internal/forensics/rollout/loader.go` — JSONL rollout file loader
- `internal/forensics/rollout/canonicalizer.go` — Event normalization
- `internal/forensics/replay/replayer.go` — Offline simulation engine
- `internal/forensics/replay/forker.go` — Step-based conversation fork
- `internal/server/http_replay.go` — Phase 4 HTTP endpoint
- `internal/server/http_replay_test.go` — 9 comprehensive tests for Phase 4

### Workspace Provisioning (co-landed with this commit)
- `internal/harness/events.go` — Added `EventWorkspaceProvisioned`, `EventWorkspaceDestroyed`, `EventWorkspaceProvisionFailed`
- `internal/harness/types.go` — Added `WorkspaceType` field to `RunRequest`, `WorkspaceProvisionOptions` struct, `WorkspaceBaseOptions` to `RunnerConfig`
- `internal/harness/runner.go` — Added `provisionRunWorkspace()`, workspace lifecycle event emission, cleanup on failure

### Test Fixes (regressions fixed in this commit)
- `cmd/harnessd/main_test.go` — Fixed syntax corruption from commit `aa26055` (bad git patch application): restored `runMatrixTest` body, extracted `TestRunWithSignalsInvalidModelCatalogContinues` as standalone, fixed `TestRunWithSignalsMCPParseFailureContinues`, removed orphaned select block
- `internal/harness/runner_store_durability_test.go` — Fixed `TestRunnerStore_EventsPersistedAsTheyStream` race: added retry loop with `time.Sleep(10ms)` to handle subscriber-vs-store ordering
- `internal/harness/events_test.go` — Updated `AllEventTypes()` count for new workspace lifecycle events
- `internal/harness/workspace_selection_test.go` — New test file for `WorkspaceType` validation and provisioning
- `cmd/harnesscli/tui/model_gateway_test.go` — Added `TestModelConfigAccessors_DefaultZeroValues` to cover three zero-coverage accessor functions (`ModelConfigGatewayCursor`, `ModelConfigKeyInputMode`, `ModelConfigKeyInput`)

---

## Test Coverage

```
[regression] PASS (total=85.1%, min=80.0%, zero-functions=0)
```

All Phase 4 HTTP endpoint tests pass:
- `TestHandleRunReplay_Simulate`
- `TestHandleRunReplay_Fork`
- `TestHandleRunReplay_MethodNotAllowed`
- `TestHandleRunReplay_MissingRolloutPath`
- `TestHandleRunReplay_InvalidMode`
- `TestHandleRunReplay_RolloutNotFound`
- `TestHandleRunReplay_ForkStepExceedsMax`
- `TestHandleRunReplay_InvalidJSON`
- `TestExtractLastUserPrompt`

---

## Key Design Decisions

1. **Offline simulation is purely in-memory** — `replay.Replay()` does not hit the LLM or any external service. It validates that recorded tool outputs still match what the tool would return.

2. **Fork seeding via `extractLastUserPrompt`** — The forked run's `Prompt` is set to the last user message in the reconstructed history. This ensures `StartRun` receives a non-empty prompt while the full conversation context is preserved in the message history.

3. **Auth propagation** — `handleReplayFork` reads `TenantID` and `InitiatorAPIKeyPrefix` from the HTTP request context (set by the auth middleware), so forked runs are always scoped to the authenticated tenant.

4. **Error classification** — `writeRolloutError` distinguishes between file-not-found (`404 rollout_not_found`) and other I/O errors (`500 replay_error`) for clear client-facing diagnostics.

5. **Workspace lifecycle events** — Three new event types (`run.workspace.provisioned`, `run.workspace.destroyed`, `run.workspace.provision_failed`) provide observability into per-run workspace creation and teardown, useful for diagnosing replay failures caused by missing working directories.
