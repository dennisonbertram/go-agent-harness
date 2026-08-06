# Plan: Issue #1214 source-workflow invalid-protocol handshake

## Context

- Governing GitHub issue: #1214
- Problem: `TestSourceManagerRunWorkflowFailsOnInvalidProtocolAfterResult` used a
  raw child which emitted its terminal result without first consuming the host
  `start` request. On Linux the parent can hit EPIPE before it reaches the
  intended post-terminal protocol violation.
- User impact: the ordinary workflow regression can fail nondeterministically
  while testing the wrong failure path.
- Constraints: test-only change; production source-workflow, protocol wire
  format, lifecycle ordering, and the #1209 native scenarios remain unchanged.

## Scope

- In scope: make the test child use `workflowsdk.Main` to consume the real
  start frame, then deliberately emit its raw late log after the SDK emits its
  terminal result; delivery records and verification.
- Out of scope: `internal/workflow/source.go`, workflow SDK behavior, API,
  TUI, native macOS, persistence, or any #1209 file.

## Test Plan (TDD)

- Baseline red classification: the unhandshaken raw child can exit before the
  parent initial write on Linux, producing EPIPE rather than the asserted
  `message after terminal result` protocol error.
- Minimal green: SDK startup consumes `start`; `sdk.Main` emits the valid
  terminal result; the following raw log preserves the intentionally invalid
  ordering.
- Regression: focused normal and race repetitions, then
  `TMPDIR=/private/tmp ./scripts/test-regression.sh`.

## Checklist

- [x] Verify issue scope and current source/test ownership.
- [x] Write this plan and cross-surface impact map before the change.
- [x] Replace only the invalid-protocol fixture child source.
- [x] Update durable logs and indexes.
- [x] Run focused normal/race and the full regression gate.
- [x] Commit and push a single `Closes #1214` PR; do not merge.

## Delivery Status

Implementation and local verification are complete in PR #1219. Independent
review and hosted CI remain merge gates; this plan does not claim the issue or
Epic #1000 complete.

## Risk and Rollback

- Risk: the fixture could accidentally validate only SDK behavior. Mitigation:
  retain the explicit raw post-result `log`, which is the contract under test.
- Rollback: revert this isolated test/documentation commit; no runtime state,
  deployment, or data migration exists.
