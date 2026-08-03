# Issue #1005 durable delayed callbacks

## Intent and acceptance

Persist a callback before `Set` acknowledges it; rebuild pending local timers after
a harness restart; and preserve scope, cancellation, and terminal history. Retry,
idempotency, and dispatch-run linkage policies remain explicitly owned by #1006.

## Test-first plan

1. Add SQLite round-trip/migration/scope tests and manager restart, overdue,
   shutdown, and terminal-skip tests; capture their failure on `2709fa1a`.
2. Add a callback `Store` abstraction and SQLite implementation with atomic
   create/update/list and an explicit migration.
3. Make callback manager persist before arming/acknowledgement, recover only
   pending records after its real starter is available, and keep shutdown local.
4. Wire `harnessd` to `.harness/callbacks.db`, fail startup on open/migration
   errors, and close it after callback shutdown.
5. Run focused normal/race tests, command wiring tests, then full regression.

## Rollout/rollback

The additive local database starts empty on legacy installations. Rollback may
disable callback recovery but must retain `.harness/callbacks.db`; no destructive
migration or cron data interaction is permitted.
