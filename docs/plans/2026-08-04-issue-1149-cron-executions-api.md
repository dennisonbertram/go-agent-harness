# Issue #1149: Cron execution-history HTTP API

## Intent and success

The harness's cron tools already read execution history through `CronClient`,
but the public harness API has no equivalent route. Add the canonical,
authenticated `GET /v1/cron/jobs/{id}/executions?limit=&offset=` endpoint so
operators and product clients can observe a cron job's actual dispatch/run
history. A request must never disclose another tenant's job or executions.

Success is a real HTTP request that returns the adapter's execution records and
pagination arguments for an owned job; unauthenticated or insufficiently
scoped requests are rejected by existing middleware, foreign/missing jobs are
indistinguishable 404s, and adapter failures remain 500s. Existing tool and
remote/embedded adapters remain the source of execution data.

## Scope

In scope: server routing/authorization, bounded query parsing, a JSON
`{"executions": [...]}` response, server HTTP regression coverage, and
documentation/log entries. Out of scope: new persistence/schema work, a new
cron daemon endpoint, TUI/native rendering, changing the existing deferred
`cron_history` tool, or changing job scheduling semantics.

## Test-first sequence

1. Add an HTTP acceptance test for the new path that expects 200 and records
   `limit`/`offset`; run it red because the current route returns 404.
2. Add table/real HTTP coverage for pagination, scope, tenant isolation,
   missing jobs, and adapter error handling.
3. Implement routing and the owned-job guard before `ListExecutions`, then run
   the targeted test and race package.
4. Run `./scripts/test-regression.sh`; only then offer the branch for review.

## Rollout and rollback

The route is additive and uses the existing `CronClient` contract, so no
migration or flag is required. Deploy after the server binary. Rollback is
safe by reverting the handler: no durable state is written by reads. Operators
can verify a response via the job ID returned by cron create/list and correlate
the returned `run_id` with existing run endpoints.

