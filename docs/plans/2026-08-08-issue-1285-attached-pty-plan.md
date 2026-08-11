# Plan: Attached 100x30 PTY Acceptance Runner

## Context

- Governing GitHub issue: #1285.
- Problem: the existing `ptyrunner` starts its own fake harnessd, which cannot truthfully prove a scheduled child was rendered against the daemon that owns its API/SSE conversation.
- User impact: #1279 needs a reusable, real TUI attachment without treating cross-daemon artifacts as same-conversation evidence.
- Constraints: build on merged #1283; do not implement cron/callback scenarios or change production harness/TUI behavior; preserve exact 100x30 geometry.

## Scope

- In scope: typed lifecycle attachment validation, a no-daemon-start attached PTY runner, immutable frame/identity artifacts, and negative tests.
- Out of scope: scenario semantics, fake-turn provisioning, scheduled-job API calls, native GUI proof, remote cronsd, and #1010 closure.

## Documentation Contract

- Feature status: in implementation; acceptance-only internal API.
- Public docs affected: none, because product behavior and wire contracts do not change.
- Spec docs to update before code: this plan and its impact map.
- Implementation notes to add after code: durable logs and indexes.

## Test Plan (TDD)

- New failing tests: reject missing/malformed lifecycle attachment before PTY launch; prove attachment config has no daemon launch input; run a real 100x30 attached PTY and validate sealed frame and serialized identity artifacts.
- Existing tests to update: ptyrunner package only.
- Regression tests required: focused normal/race, full regression, and a real attached-PTY smoke recorded in the PR.

## Cross-Surface Impact Map

See `2026-08-08-issue-1285-attached-pty-impact-map.md`.

## Implementation Checklist

- [x] Verify structured issue #1285 and merged #1283 dependency.
- [x] Record ownership/search evidence and complete impact map.
- [x] Add expected-red attachment tests (undefined attachment API).
- [x] Implement the smallest typed attachment seam.
- [x] Run focused normal/race tests and real two-message PTY smoke.
- [x] Address independent-review P1s: active action tokens reject stale seals,
  and safe-slug/non-root containment rejects unsafe artifact names and roots.
- [x] Correct the scenario deadline to exclude disposable binary-build setup
  after the full coverpkg suite exposed a false pre-lifecycle timeout.
- [ ] Update logs/indexes and run `./scripts/test-regression.sh`.
- [ ] Request independent review, commit, and open one PR with `Closes #1285`.

## Risks and Mitigations

- Risk: an attached path accidentally regains daemon ownership. Mitigation: attachment config deliberately has no daemon executable or launch setting; reject invalid identity before the CLI process starts.
- Risk: historical terminal bytes receive credit for later actions. Mitigation: reuse the existing single-reader collector and per-action barriers.
- Risk: full-suite build instrumentation consumes the lifecycle behavior
  deadline. Mitigation: start the deadline only when the owned lifecycle is
  about to start, while retaining individual startup/run probe bounds.
