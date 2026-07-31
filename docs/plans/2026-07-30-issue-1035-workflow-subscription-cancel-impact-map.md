# Cross-Surface Impact Map: Issue #1035 Workflow Subscription Cancellation Test

## Task

- Task / issue: remove a race-gate flake caused by invalid buffered-channel
  expectations, #1035.
- Plan: `2026-07-30-issue-1035-workflow-subscription-cancel-plan.md`.
- Owner: Codex.
- Status: implemented; full verification and merge pending.

## Ownership and Data Flow

- Entry: `TestEngineDefinitionSubscribeAndFailure`.
- Runtime owner: `Engine.Subscribe` registers a buffered channel; its cancel
  closure removes and closes it under `Engine.mu`.
- Producer: `Engine.emit` uses the same mutex before sending.
- Search:
  `rg -n 'Subscribe|subscribers|close\\(.*ch' internal/workflows`.
- Conclusion: production prevents send-after-close; the test owns the faulty
  assumption about draining buffered events.

## Cross-Surface Review

- Config/env/defaults: none; test-only.
- API/CLI/wire/tools: none; no public contract.
- Persistence/schema/cache: none.
- Concurrency/lifecycle: only the test's observation of closed buffered-channel
  semantics changes; runtime synchronization is unchanged.
- Security/auth/privacy: none.
- TUI/web/macOS/providers/models: none.
- Deployment/operations: improves reliability of the required race gate.
- Compatibility/mixed versions: none.
- Documentation: plan, impact map, long-term log, engineering log, plans index.

## Regression and Verification

- Deterministic fixture: enqueue a named event before cancellation.
- Old assertion: fails on the first buffered receive.
- New assertion: accepts buffered values and requires closure before a bounded
  deadline.
- False-positive control: a non-closing channel would hit the deadline.
- Commands:
  `go test -race ./internal/workflows -run TestEngineDefinitionSubscribeAndFailure -count=100`;
  `go test ./internal/workflows`;
  `go test -race ./internal/workflows`;
  `./scripts/test-regression.sh`.

## Rollout and Rollback

- No deployment, migration, state repair, or runtime rollout.
- Roll back if the revised test cannot detect a deliberately unclosed
  subscription.

## Warning Check

- Every runtime/product surface was searched and is unaffected because the
  patch is confined to the existing regression and process evidence.
