# Cross-Surface Impact Map: Issue #1236 plural workflow Subscribe

## Task

- Task / issue: #1236 close the `workflows.Engine.Subscribe` history/live gap.
- Plan link: `2026-08-07-issue-1236-workflows-subscribe-plan.md`.
- Owner: `internal/workflows.Engine`.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry points: `(*workflows.Engine).Subscribe` and workflow-run SSE callers.
- Source of truth: `internal/workflows/engine.go` owns `subs`, `eventSeqs`,
  persistence append, and live fan-out; `Store.GetEvents` owns history.
- Data flow: caller receives history, then the live channel; event sequence
  is per workflow run.
- Similar abstraction searched: singular `internal/workflow.Engine.Subscribe`
  already implements a watermark plus initializing pending buffer.
- Evidence: `rg -n "Subscribe|GetEvents|initializing|watermark" internal/workflow internal/workflows internal/server/http_workflows.go`.
- Conclusion: adapt the singular handoff locally; do not duplicate its unrelated
  terminal-channel lifecycle because plural engine semantics differ.

## Config, API, CLI, and Tools

- User-facing config/defaults/environment: None.
- Endpoints/wire/CLI/tools: no event fields, SSE identifiers, or commands
  change. The pre-existing HTTP terminal-history hang is #1237, not this slice.
- Error states: `GetEvents` errors retain their returned error contract.

## Persistence and Compatibility

- Schemas/migrations/caches: None; Store interface and event payload stay
  unchanged.
- Compatibility: old callers still receive history and a channel; the split
  becomes lossless and exactly once.
- Mixed rollout: safe because the change is internal to one in-process engine.

## Lifecycle, Security, and Reliability

- Concurrency: register and capture a sequence watermark under `e.mu`, fetch
  history unlocked, then atomically fold/reset the initialization buffer.
- Cancellation/cleanup: failed setup removes its entry; cancel remains the
  channel close owner for plural workflows.
- Security/privacy: None; no authorization or secret data changes.
- Failure/idempotency: events are partitioned by watermark so no history/live
  duplicate; pending prevents burst loss before the caller can drain.

## Product and Integration Surfaces

- Server/runtime: server consumes the same history/live contract; no handler
  change here.
- TUI/web/macOS: indirect workflow observers gain reliable events; no UI code.
- Provider/model/tool routing: None.
- Automation/cron/callback: indirect observer reliability only; no scheduler
  or tool semantics change.

## Deployment and Operations

- Order/flags: no migration or flag required.
- Observability: existing workflow event records remain the diagnostic source.
- Rollback: revert this narrow engine/test/documentation change if required.
- Runbooks: testing evidence is retained in the PR; no operator procedure changes.

## Regression Tests

- First red: controlled store snapshots then blocks; emit in the old
  snapshot/register window must appear exactly once across history/live.
- Acceptance: pending burst above channel capacity; exact-once sequence map.
- Failure/lifecycle: `GetEvents` error has no subscriber residue; cancel closes
  and deregisters once.
- Integration: server terminal-history return is deliberately #1237.
- Commands: focused normal/race/stress for `./internal/workflows`, then
  `TMPDIR=/private/tmp GOCACHE=/private/tmp/gocode-1236-cache ./scripts/test-regression.sh`.

## Documentation and Handoff

- Specs/public docs: no public behavior claim changes.
- Logs/indexes: add plan/impact entries now; record cause/fix/red-green evidence
  in the engineering log after implementation.
- Training/release notes: None.
