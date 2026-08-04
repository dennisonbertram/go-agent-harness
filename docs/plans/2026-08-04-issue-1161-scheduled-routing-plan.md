# Issue #1161 Scheduled Routing Preservation Plan

## Intent and success

- Command intent: repair only Issue #1161 in one reviewable PR rebased onto
  `origin/main` `c991a725`, preserving the safe model/provider/fallback policy
  from an origin run through embedded cron, remote cronsd, and delayed callback
  continuation admission.
- User intent: a scheduled continuation that was accepted under an allowed
  provider fallback must execute in the same scoped conversation instead of
  failing during provider resolution after the timer fires.
- Success: durable scheduled payloads carry only model, provider name,
  `allow_fallback`, and ordered fallback provider names; retries and restarts
  replay the same values; tenant/auth/scope and secret handling are unchanged.

## Scope

In scope:

- Extend existing run tool metadata with the safe routing selection.
- Store routing in existing harness cron execution config and delayed callback
  rows, with empty/false legacy defaults.
- Extend `cron.RunStartRequest`, embedded harnessd mapping, remote cronsd JSON,
  authenticated server mapping, and cron request fingerprint.
- Add permanent red-first coverage for embedded cron, callback durability,
  typed dispatch, remote payload/server mapping, and idempotency conflicts.

Out of scope:

- Issue #1162, provider catalog redesign, credentials or arbitrary provider
  configuration, transcript reducer/UI changes, scheduler retry policy,
  callback lease ownership, auth/scope changes, and deployment.

## Test-first sequence

1. Add the permanent embedded-cron regression: an origin-style routing policy
   must reach the starter with prompt and scope unchanged. Run it before source
   changes and retain the exact failing output.
2. Add typed `RunStartRequest`/dispatch assertions, then implement the smallest
   additive cron metadata and execution-config propagation.
3. Add remote JSON and authenticated server mapping tests, including a same-key
   routing-policy change that must return an idempotency conflict; extend the
   fingerprint only after observing red.
4. Add callback tool, SQLite round-trip/migration, restart, and harnessd starter
   mapping tests; then propagate the same four safe routing values.
5. Run focused normal/race suites, complete affected packages, and
   `./scripts/test-regression.sh`. Update logs with exact green evidence.

## Compatibility, rollout, and rollback

- All JSON and SQLite columns are additive. Missing legacy fields decode to
  empty model/provider/fallback list and false `allow_fallback`, preserving the
  historical runner-default behavior.
- Fallback provider slices are copied at boundaries so caller mutation cannot
  change a durable or in-flight continuation.
- No secret, endpoint, token, API key, or provider client configuration is
  persisted or logged.
- Monitor scheduled `run.failed` provider-resolution errors and continuation
  completion. Roll back the single PR if routing changes alter tenant/auth,
  retry ownership, or legacy scheduled-run behavior; additive empty columns
  remain backward readable.

## Completion gates

- [x] Red evidence captured before production edits.
- [x] Embedded cron, remote cron, and callback routing regressions pass.
- [x] Idempotency fingerprint distinguishes routing policy.
- [x] Legacy rows/payloads and tenant/auth/scope invariants pass.
- Final handoff evidence must include the complete affected normal/race suites,
  `./scripts/test-regression.sh`, and exact head/base/merge-base SHAs.
- Delivery remains one commit and one pushed PR with `Closes #1161` only; the
  PR must remain unmerged.
