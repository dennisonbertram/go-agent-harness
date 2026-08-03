# Issue #994 — acknowledged native run controls

Issue: #994. Status: in implementation.

## Intent and success definition

Make macOS cancel, approve, deny, plan approval, steering, and structured answers acknowledge the harness before client state claims success. A failed answer must remain visible and editable; a control retry must be possible and duplicate taps must not send duplicate requests.

## Scope and boundaries

In scope: `RunSession`, `ChatView` control states, an answer-completeness helper, focused Swift tests, and required engineering/system/observation logs. Out of scope: Harness endpoints, TUI/web clients, callback/cron behavior, transcript redesign, and unrelated #1021 GUI work.

## Test-first plan

1. Add a stub-client test suite for all control endpoint success/failure/retry/single-flight paths and run it red against main.
2. Implement the smallest `RunSession` async ownership/state changes and answer validation to turn the suite green.
3. Add UI reachability tests for disabled duplicate controls and nonempty trimmed answers.
4. Run focused Swift tests, `swift build`, formatting and strict lint, then the repository regression command. Run a live daemon smoke only if the local macOS daemon prerequisites are available; otherwise report the exact blocker.

## Rollout and rollback

The change is client-local and wire-compatible: no migration or server rollout order is required. Roll back the PR to restore prior UI behavior; no persisted state needs repair. Failure messages stay local and never contain secrets.

## Checklist

- [x] Read #994, bootstrap/runbook documents, and current ownership.
- [x] Record impact map and test-first plan.
- [ ] Record expected red evidence.
- [ ] Implement and pass focused tests.
- [ ] Run format/build/full verification and document results.
- [ ] Push one reviewable PR with `Closes #994`.
