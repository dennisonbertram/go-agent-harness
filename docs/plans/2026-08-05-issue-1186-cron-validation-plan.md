# Plan: Issue #1186 Harness cron validation errors

## Context

- Governing issue: #1186.
- Problem: invalid cron writes reach the public harnessd facade as generic errors and are rendered as HTTP 500, despite raw cronsd already returning `400 validation_error`.
- User impact: callers cannot correct malformed schedules, execution configuration, or timeouts and may mistake a client error for a scheduler outage.

## Scope

- In scope: retain typed cron validation errors across raw cronsd HTTP client, remote adapter, embedded adapter, and harnessd POST/PATCH rendering; add equivalent embedded/remote tests and prove no persistence on rejected create.
- Out of scope: scheduler execution, durable schema, authentication, GUI/TUI rendering, cron lifecycle, or status semantics beyond correctly classifying an invalid PATCH status.

## Test-first plan

1. Add public-facade tests expecting `400 validation_error` for typed create and update errors, while checking unknown/conflict/dependency errors stay 404/409/500. These fail before a validation sentinel exists.
2. Add client/adapter tests for raw remote `400 validation_error`; they fail because `parseError` flattens it.
3. Make embedded validation paths produce the same typed error and prove invalid create does not enter the store.
4. Run focused normal/race packages, then `TMPDIR=/private/tmp ./scripts/test-regression.sh`.

## Implementation checklist

- [x] Verify #1186 contract and search the owning seams.
- [x] Record this plan and cross-surface impact map before source changes.
- [x] Capture focused expected-red failures.
- [x] Add typed validation error translation without changing other error identities.
- [x] Add/update durable logs and indexes.
- [x] Run targeted, race, full regression, and API-path proof.

## Rollout and rollback

- No schema or configuration migration. Deploy harnessd and cronsd normally; mixed versions merely retain old 500 behavior until both pieces are updated.
- Roll back with one commit revert. Rejected requests do not write a job, so no data repair is needed.
