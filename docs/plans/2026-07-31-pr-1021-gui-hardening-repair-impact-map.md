# PR #1021 GUI Hardening Repair Impact Map

## Task

- Task / issue: Repair PR #1021 for epic #991 children #992–#999 after independent production review.
- Plan link: `2026-07-30-001-feat-macapp-gui-hardening-plan.md`
- Owner: PR #1021 repair branch `codex/pr-1021-repair`
- Status: Source repair and Swift/live-harnessd gates complete; hosted Go race remains blocked on existing issue #1039 / PR #1041, while installed-app smokes and the separate Settings investigation remain pending.

## Current Ownership, Callers, and Data Flow

- Entry points: transcript streaming and history keys in `ChatView`; run controls and conversation SSE in `RunSession`; project collection/lifecycle actions in `ProjectSession`; Sessions, Activity, Models, and conversation chrome controls.
- Owning packages/types/functions and source of truth: `RunSession` owns active run/control request state; `ProjectSession` owns project collections and conversation selection; `Transcript` owns rendered event reduction; harnessd remains authoritative for runs, pending input, conversations, and rewind results.
- Callers, consumers, events, and downstream data: per-run and conversation-wide SSE both call `RunSession.apply`; SwiftUI tasks and Retry controls call project refresh methods; lifecycle controls call the shared `ProjectSession` boundaries.
- Similar abstractions searched: `cancelState`, `answerInFlight`, `seenEventIDs`, `reconcilePersistedMessages`, `syncCurrentConversation`, collection load states, prompt-history cursor state, and destructive-confirmation presentation.
- Search commands/evidence: `rg -n "answerInFlight|runControlTask|pendingInput|refreshCatalog|refreshConversations|refreshActivity|refreshRewindPoints|openConversation|scrollIfPinned|isRecallingHistory|rewind\\(" macapp`; three-tree merge inspection against `origin/main`.
- Duplication/ownership conclusion: request identity belongs with the owning async model, not individual views. Views consume observable state and supply accessibility presentation only.

## Config, API, CLI, and Tools

- User-facing config added or changed: None; repository and macOS settings formats are unchanged.
- Defaults / fallbacks: Existing model, profile, provider, and run fallbacks are unchanged.
- Environment variables, config files, or saved settings touched: None; live-test environment variables are used only by verification.
- Endpoints, request fields, response fields, or server wiring affected: None. Existing runs, pending-input, conversation, catalog, and rewind endpoints are consumed without wire changes.
- CLI commands, tools, wire formats, or integrations affected: None; TUI and web code are unchanged.
- Error states / validation changes: stale async completions are ignored; controls become single-flight; steering restores an unmodified draft after failure; stale collection data stays visible with a refresh failure.

## Persistence and Compatibility

- Schemas, migrations, caches, generated data, or ownership changes: None.
- Backward/forward compatibility and versioning: Client-only state ownership; compatible with the current harnessd API.
- Partial rollout and mixed-version behavior: A repaired app remains compatible with current harnessd. An older app retains the reviewed races but does not corrupt persisted data.

## Lifecycle, Security, and Reliability

- Concurrency, cancellation, retries, cleanup, and resource ownership: Per-request generations or owned tasks protect answer, pending-input, run-control, autoscroll, and collection loads. Cancellation and generation invalidation occur on reset, selection changes, and newer requests. Pending-answer view identity follows the server call id; transcript pin/autoscroll identity follows the selected conversation; reserved startup refreshes validate generation before setting loading; a slow or stale run-scoped todo result cannot block or abort independent task/run refreshes; rewind refusals retain and synchronously claim their originating conversation before force I/O is scheduled; local force cancel releases the run identity; submitted-run accounting is bound to the server-returned run id and sealed cost status is owned by that run so unrelated or late duplicate-stream events cannot overwrite it.
- Authentication, authorization, permissions, trust, privacy, and secrets: None; no credentials or authorization boundaries change.
- Failure modes, recovery, idempotency, and data repair: Duplicate control POSTs and stale state writes are prevented. Existing Retry controls remain the recovery path. No data repair is required.

## Product and Integration Surfaces

- Server/runtime: API behavior unchanged; merged #1008/#1028 replay and terminal reconciliation must remain authoritative.
- TUI/web/macOS/other clients: macOS only. TUI/web are regression-only surfaces.
- Provider/model/tool catalog and routing: Catalog display refresh ownership changes; provider/model routing and saved settings do not.
- External systems and automation: #1007 external cron/callback run-control binding is explicitly not implemented here. Observable external-run busy state remains a #995 guard input when delivered.
- UX states, keyboard/focus/accessibility/motion: Jump to Latest, Reduce Motion, disabled lifecycle reasons, single-flight control feedback, stale-data notices, and duplicate-history key bookkeeping are affected.

## Deployment and Operations

- Deployment/migration order and feature flags: Ship with the next macOS app build after current-main integration; no flag or server ordering.
- Logs, metrics, traces, alerts, and support diagnostics: Existing visible status/error surfaces remain; no new telemetry.
- Rollback triggers and recovery steps: Revert the repair commits while retaining the `origin/main` merge if controls deadlock, transcript following regresses, or current conversation reconciliation duplicates/drops rows.
- Runbooks and operator docs: Existing macOS verification contract and issue #1020 track live installed-app proof.

## Regression Tests

- Characterization and first expected red test: stale answer A clearing newer B; overlapping autoscroll completion; older collection/open response overwriting newer state; older pending-input result replacing a newer prompt; duplicate control request; identical prompt recall without `onChange`.
- New acceptance tests required: generations and reset invalidation; latest-request wins per collection; conversation target validation; single-flight control state; steering draft restoration; Jump to Latest and Reduce Motion reachability; rewind busy guard.
- Edge, negative, failure, lifecycle, and security tests: stale run, new manual draft after steering, response reordering, reset mid-request, repeated keys with equal strings, cancellation/retry, active external transcript state.
- Integration/e2e/real-path proof: Swift package and live-harnessd automated suites now; installed app/manual interactions remain pending issue #1020 and the separate Settings investigation.
- Cross-surface regressions to guard: #1008 persisted/live replay dedupe, #1028 terminal reconciliation (including sealed usage/cost accounting for completed, failed, and cancelled runs across a durable-row rebuild), #995 lifecycle guards, #994 pending-input retention.
- Exact targeted and full commands: focused repair integration passed 93 tests / 12 suites; per-run terminal accounting, pending-answer identity, partial activity refresh, conversation-bound rewind, force-confirmation ordering, local cancel cleanup, slow-todo independence, sealed cost-status authority, submitted-run reservation, reserved-refresh loading ownership, and conversation-scoped transcript pin regressions were observed red then green; the latest focused run passes 5 tests / 2 suites; the exact `RunSessionLiveTests` failure reproduced locally and passed after the repair; `swift build --package-path macapp` passed; `swift test --package-path macapp` passed 316 tests / 55 suites; strict recursive Swift format lint passed; and `go test ./internal/server ./internal/harness ./internal/store` passed. The prior exact hosted head passed build-test, format, live-harnessd, test-fast, and test-race; the new review-repair head must rerun those gates. A prior hosted attempt and repeated targeted diagnostics exposed the current-main worktree-cleanup race already owned by #1039 / green PR #1041; PR #1021 does not duplicate that Go test fix, and the safe stacking order remains #1041 first.

## Documentation and Handoff

- Specs/public docs before code: Child issue repair comments and this impact map; original plan repair addendum.
- Implementation notes/logs/indexes after code: engineering and long-term logs, plan index, residual-findings index, and master docs index.
- Training/onboarding/release notes: None; this repairs already-promised behavior without adding a new public API.

## Warning Check

- Every required surface is covered above. `None` entries include a searched rationale.
