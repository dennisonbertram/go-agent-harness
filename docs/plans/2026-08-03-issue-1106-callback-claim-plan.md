# Plan: Issue #1106 durable callback claim ownership

## Context

- Governing GitHub issue: #1106.
- Problem: competing persistent callback managers can lose a valid lease after a transient SQLite lock, allowing a later expired-lease reclaim to start the same reserved callback run twice.
- User impact: a one-shot callback can advance one conversation more than once, violating its visible automation intent.
- Constraints: preserve the public callback tool/API shape and the reserved run ID; do not claim exactly-once external effects across process crash.

## Scope

- In scope: SQLite pooled-connection setup, token-verified claim/reclaim reads,
  lifetime claim retry with a capped delay, heartbeat behavior through the last
  confirmed lease deadline, token-fenced live-owner release with same-manager
  rearm, a persisted mixed-version ownership state, and a process-lifetime
  workspace recovery fence plus expected-token CAS for expired-dispatch
  recovery.
- Out of scope: cron behavior, callback tool schemas, distributed coordination, and external provider exactly-once guarantees after process crash.

## Documentation Contract

- Feature status: implementation and callback-focused validation are complete.
  The #1112 fixture repair is now merged in the rebase baseline; the exact
  current-tree regression passed. PR #1107 is ready to be committed, pushed,
  and sent for fresh independent review.
- Public docs affected: None; durable semantics are internal and existing user-visible events remain unchanged.
- Spec docs to update before code: this plan and its impact map.
- Implementation notes to add after code: engineering, observational, and system logs plus indexes.

## Test Plan (TDD)

- New failing tests to add first: two competing managers dispatch one callback once despite a transient heartbeat busy error; a store claim/reclaim cannot report ownership without the caller token.
- Existing tests to update: durable callback store and manager retry/lease tests.
- Regression tests required: repeated busy errors only cancel at the last confirmed deadline and permit later safe takeover; pooled SQLite connections retain WAL/busy configuration.
- Review repair: a blocking renewal reaches its deadline while the original
  admission is still active, then proves the deadline guard cancels it before
  replacement admission; literal `?` database paths round-trip exactly.
- Structural handoff repair: a normally armed contender is present before
  expiry and cannot claim `dispatching` merely because a clock has passed. The
  old owner first returns from its canceled `StartCallback`, then atomically
  clears its exact token into `retry_wait`; only that released row can be
  claimed. An expired row is converted to retry work only by `Recover` at the
  confirmed harness-startup/process-loss boundary. The documented post-crash
  external side-effect boundary is unchanged. Final status remains blocked on
  full local and hosted gates plus fresh reviews.
- Liveness/recovery repair: after a deadline-cancelled admission durably
  releases to `retry_wait`, its same manager re-arms that retry; ordinary
  one-daemon deployments therefore cannot strand it. `Recover` holds a
  filesystem sidecar `flock` for the manager lifetime, which the kernel drops
  on process death. A second daemon sharing the workspace fails closed even
  after a callback's clock lease expires. Legacy `NULL` lease timestamps are
  treated as abandoned only under that authority and become retry work.
- Final review repair: a startup snapshot with a future crash-orphan lease
  re-enters the authorized recovery transition when its timer reaches that
  lease; it cannot poll `dispatching` forever. Deadline release observes the
  persisted retry budget and exponential backoff, terminalizing instead of
  rearming when the bound is exhausted. Durable recovery fails closed for
  stores without workspace authority and for in-memory/opaque SQLite
  locations. A killed child-process test proves the kernel, not graceful
  shutdown, releases the workspace fence.
- Final liveness repair: every filesystem-backed durable manager joins the
  workspace process-loss fence before Set or dispatch, not only during
  startup recovery. The subsequent atomic compatibility repair supersedes the
  token-prefix draft with a private persisted dispatch state. Claim failures
  after several bounded windows rearm under capped exponential local backoff
  without consuming the durable admission-attempt limit. Deadline release
  persists the owned safe `callback admission unavailable` retry reason.
- Atomic compatibility repair: current claims persist the private
  `dispatching_fenced` state. The older binary's literal
  `state='dispatching'` reclaim predicate cannot acquire that state. If the
  older binary wins a pending/retry row first, current recovery recognizes the
  old `dispatching` state and leaves the live admission untouched. Current
  crash recovery mutates only `dispatching_fenced` with the exact token it
  observed, under the process-loss lock. Public API/lifecycle state is still
  `dispatching`. A finite local claim-attempt cap is removed; cancellation
  stops rearming, while exponential backoff duration remains capped.

## Cross-Surface Impact Map

- Required artifact: `2026-08-03-issue-1106-callback-claim-impact-map.md`.

## Implementation Checklist

- [x] Define acceptance criteria in tests.
- [x] Link the existing issue before implementation.
- [x] Record ownership/search evidence and the impact map.
- [x] Add deterministic failing regressions.
- [x] Implement the minimal ownership repair.
- [x] Run focused, race, and full regression gates.
- [x] Update logs/indexes.
- [x] Run final focused/stress/full gates and update logs/indexes.
- [ ] Commit and push the rebased existing PR #1107 (`Closes #1106`), then
  obtain fresh independent review. Do not merge from this slice.

## Current Verification Blocker

- The previous cron race timeout is resolved in the merged #1113 baseline
  (tracked by #1112). On the rebased #1106 tree, callback normal stress x3 and
  race stress x3 passed; the exact-current `./scripts/test-regression.sh`
  passed normal tests, repository race tests, and `coveragegate` at 85.5% with
  zero uncovered functions. No cron source is changed in this worktree.

## Risks and Mitigations

- Risk: backing off a transient heartbeat error past the lease deadline would allow overlapping admission. Mitigation: retain the last confirmed deadline, cancel at expiry, wait for the local admission to return, then token-fenced release to retry work before any ordinary contender may claim.
- Risk: a crash cannot acknowledge a live-owner release. Mitigation: `Recover`
  holds a filesystem `flock` for its manager lifetime; kernel release on
  process loss plus bootstrap-captured exact-token provenance is required
  before it converts an expired or NULL current-version fenced dispatch into
  retry work. Old public `dispatching` rows fail closed. External side effects
  remain at-least-once across a crash.
- Risk: a released deadline retry could remain unarmed in an ordinary single daemon. Mitigation: the releasing owner calls `syncDurableState(..., true)` and a deterministic test proves its second reserved-ID admission without a replacement manager.
- Risk: SQLite driver configuration is connection-local. Mitigation: configure every physical connection before query use and test a pooled second connection.
- Risk: a rolling upgrade can place a fenced manager beside an older daemon
  that never acquired the sidecar lock. Mitigation: current ownership uses a
  persisted state that the older daemon's hard-coded reclaim predicate cannot
  match; old ownership retains `dispatching`, which current code never
  automatically reclaims. A downgrade while `dispatching_fenced` is active is
  intentionally fail-closed until a current binary completes or recovers it.
- Risk: transient claim errors can outlast the original one-shot retry.
  Mitigation: capped exponential rearm is cancellation-aware and does not
  consume the bounded admission attempt counter before ownership is acquired.
- Rollback: drain current callback admissions before reverting. There is no
  schema migration, but an old binary deliberately ignores an active
  `dispatching_fenced` row rather than risking overlap.
