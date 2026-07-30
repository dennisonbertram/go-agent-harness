# Cross-Surface Impact Map: Issue #1031 Terminal Provenance

## Task

- Task / issue: recover provisional transport failure during durable
  reconciliation, #1031.
- Plan link: `2026-07-30-issue-1031-terminal-provenance-plan.md`.
- Owner: Codex.
- Status: implemented; verification and merge pending.

## Current Ownership, Callers, and Data Flow

- Entry points: `RunSession.submit`, per-run/conversation SSE, and durable Chat
  reconciliation.
- Owners/source of truth: server terminal events are authoritative;
  `Transcript.markFailed/markCancelled` are local presentation fallbacks;
  `RunSession` owns async event provenance.
- Consumers: transcript status/error UI and callback/cron conversation replay.
- Similar abstractions searched: event dedupe, run start/reset/load/rebind,
  transcript load/reconcile, local terminal events.
- Search evidence:
  `rg -n 'markFailed|localTerminalEvent|runFailed|reconcilePersistedMessages|seenEventIDs' macapp/Sources macapp/Tests`.
- Ownership conclusion: store provenance in `RunSession`; do not infer it from
  the reduced transcript and do not create a second transcript.

## Config, API, CLI, and Tools

- Config/defaults/env/saved settings: none.
- Endpoints/request/response/server wiring: none.
- CLI/tools/wire/integrations: none.
- Error states: a local transport failure becomes provisional and may recover;
  authoritative server failure/cancellation remains terminal.

## Persistence and Compatibility

- Schemas/migrations/caches/generated data: none.
- Compatibility: native-client-only interpretation of unchanged events and
  messages.
- Mixed versions: old clients may retain false failure; new clients recover.

## Lifecycle, Security, and Reliability

- Concurrency/lifecycle: clear provenance when a new run begins or conversation
  changes; update it only after deduping an authoritative terminal event.
- Auth/security/privacy/secrets: none; no new data.
- Recovery/idempotency: repeated durable reconciliation yields the same state;
  no durable data repair.

## Product and Integration Surfaces

- Server/runtime: unchanged.
- TUI/web/macOS: macOS only; TUI/API unaffected.
- Provider/model/tool routing: none; downstream client reduction only.
- External automation: cron/callback consume the repaired conversation path.
- UX/accessibility/motion: status correctness only; no layout/input change.

## Deployment and Operations

- Order/flags/migrations: native patch, no flag or migration.
- Diagnostics: deterministic transport-failure test plus engineering log.
- Rollback: revert if authoritative failure/cancellation stops persisting.
- Runbooks: none; operator contract unchanged.

## Regression Tests

- First red: transport exception -> local failed -> durable completed messages
  currently remains failed.
- Acceptance: recovery to completed with connection error cleared.
- Controls: authoritative failed/cancelled and completed replay dedupe.
- Real path: shared native callback/cron acceptance after #1031, #1032, #1027.
- Commands:
  `swift test --package-path macapp --filter transportFailureReconciliationRecoversToCompleted`;
  `swift test --package-path macapp`;
  `./scripts/test-regression.sh`.

## Documentation and Handoff

- Before code: plan and impact map.
- After code: engineering/long-term logs and plans index/status.
- Release/training: none.

## Warning Check

- All applicable surfaces are mapped; none entries include rationale.
