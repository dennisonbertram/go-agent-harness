# Remote cronsd harness execution

Standalone `cronsd` can dispatch `execution_type="harness"` jobs to an
authenticated `harnessd`. Shell jobs remain local and are never used as a
fallback for a harness job.

## Configuration

Set these variables on the `cronsd` process:

```bash
export CRONSD_INGRESS_API_KEY=cron_ingress_...
export CRONSD_INGRESS_TENANT_ID=tenant-production
export CRONSD_HARNESS_URL=http://127.0.0.1:8080
export CRONSD_HARNESS_API_KEY=harness_sk_...
export CRONSD_HARNESS_CONNECT_TIMEOUT=5s
export CRONSD_HARNESS_REQUEST_TIMEOUT=15s
```

`CRONSD_INGRESS_API_KEY` and `CRONSD_INGRESS_TENANT_ID` are always required.
They are separate from the privileged outbound `CRONSD_HARNESS_API_KEY`.
Startup rejects configurations that reuse the same secret for both directions.
The ingress credential authenticates the management caller and binds this
cronsd instance to exactly one authoritative tenant. A missing request
`tenant_id` is stamped from that identity; a conflicting tenant is rejected.
On startup, legacy tenantless shell jobs are claimed by this configured tenant,
while tenantless harness jobs or rows owned by another tenant stop startup.
The claim is atomic in SQLite: if two differently configured daemons race, only
one tenant can start or expose that row. An alternative cron store that cannot
provide durable claims keeps tenantless rows invisible.

The URL and API key are required when an active harness job exists or when a
harness job is created. The API key must be accepted by `harnessd` and carry
the `runs:write` scope. Timeout values are positive Go durations; finite
defaults are used when omitted.

`CRONSD_HARNESS_API_KEY` is a secret. Keep it in the process environment or a
secret manager and do not place it in job configuration, logs, or issue
reports.

Set the matching client credential wherever cronsd is managed:

```bash
# harnessd remote-cron client
export HARNESS_CRON_URL=http://127.0.0.1:9090
export HARNESS_CRON_API_KEY="$CRONSD_INGRESS_API_KEY"

# cronctl
export CRONSD_URL=http://127.0.0.1:9090
export CRONSD_API_KEY="$CRONSD_INGRESS_API_KEY"
```

`GET /healthz` is deliberately unauthenticated and reports liveness only.
`GET /readyz` and every `/v1/jobs` route require the ingress bearer. `cronctl
health` and the harnessd remote client use authenticated readiness, so a live
but unusable daemon is not reported ready.

## Readiness and dispatch

At startup, `cronsd` first requires valid ingress identity, then loads jobs and
fails startup if any row could escape that tenant boundary. It also fails if an active
harness job lacks a valid remote URL or credential. Shell-only deployments can
start without the outbound `CRONSD_HARNESS_*` variables but still require
ingress authentication. The create path performs the same validation,
so a missing remote cannot leave an active but unschedulable harness job in
the database.

The remote request is sent to `POST /v1/cron/runs` with tenant, agent,
conversation, prompt, job ID, execution ID, and a deterministic correlation
key. `harnessd` authenticates the bearer token, enforces the authenticated
tenant, durably reserves the tenant/key/fingerprint-to-run binding, starts the
reserved run, and returns `202` with `run_id`. Concurrent, sequential, and
restart-spanning replays return that same run ID; a key reused for a different
request is rejected. The remote client does not follow redirects, so the bearer
credential and POST remain on the explicitly configured harnessd boundary.
The server retains process-local state only while a start is in flight; the
durable binding answers all later sequential or restart-spanning deliveries.
The reserved run record is committed before dispatch and before the binding is
marked accepted. If harnessd shuts down after that commit but before dispatch,
the next process validates the still-queued row against the replayed request,
dispatches that same run ID, and only then marks the binding accepted.
If dispatch succeeds but the accepted-binding write fails transiently, a retry
in the same process reuses the active run and retries only the mark. Resume is
reserved for queued durable rows absent from current runner state, and carries
the queued row's persisted model into prompt resolution and execution.
Accepted bindings are inspected too: if shutdown drained an accepted queued
item before a worker started it, the replacement process resumes that row.

Deploy and migrate `harnessd` before enabling remote harness jobs in `cronsd`.
On startup, a configured `HARNESS_RUN_DB` applies both the base run schema and
the API-key schema, including for a brand-new database; create the
`runs:write` key only after that bootstrap succeeds. The additive run-store
idempotency table is required for restart-safe replay; an unavailable durable
binding fails the start visibly rather than falling back to process-local
dedupe or shell.

During upgrade, run-store migration backfills conversation tenant/agent owners
from existing runs before accepting new claims. If one historical conversation
contains more than one normalized owner, or an existing owner row disagrees
with its runs, migration fails and harnessd must remain unready. The migration
does not select a winner or rewrite the conflicting run/owner rows; inspect and
resolve the historical data before restarting.

Remote errors are recorded as failed or timed-out executions. They include a
safe error code, HTTP status when applicable, and retryability classification;
response bodies, prompts, and credentials are not copied into errors. Logs
contain only endpoint class, job ID, execution ID, status, latency, and
retryability. The scheduling request uses the earliest of the job's
`timeout_seconds`, any inherited parent deadline, and the configured remote
request timeout. Omitted create timeouts default to 30 seconds; explicit
nonpositive create or PATCH values are rejected before mutation or dispatch.
Deadline or cancellation while reading a successful response body is
classified the same as a transport failure before headers.

## Local canary

1. Start a local `harnessd` with a deterministic provider and its API enabled.
2. Start `cronsd` with a distinct ingress key and tenant, the harness URL, a
   `runs:write` outbound API key, a temporary database, and finite timeouts.
3. Require the ingress bearer on `GET /readyz`, then create a harness job
   through authenticated `POST /v1/jobs` with a one-minute schedule and
   an execution config such as `{"prompt":"remote canary"}`.
4. Verify an unauthenticated management request is `401`, a conflicting body
   tenant is `403`, and the accepted job is stamped with the configured tenant.
5. Wait for the scheduled fire, then query authenticated
   `GET /v1/jobs/<id>/history`. A successful history row proves that cronsd
   received an accepted remote-start response; it does **not** prove that the
   harness run later completed successfully.
6. In this transitional #1003 canary, extract the accepted run ID only from
   the execution `output_summary` (`started run <run_id>`) and use it only for
   authenticated `GET /v1/runs/<run_id>` scope inspection. Do not parse prose
   into `Execution.RunID`: that structured durable linkage remains #1004 and
   is intentionally empty here.
7. Verify the execution is not shell and the accepted run's tenant, agent,
   and conversation match the job. A terminal run outcome requires separate
   run/event evidence. Capture sanitized daemon logs and the exact commands in
   the PR evidence.

## Rollback

Disable remote harness dispatch by pausing/deleting harness jobs and removing
the remote configuration. Shell jobs continue independently. Do not redirect
failed harness prompts to the shell executor. Re-enable only after the
`/v1/cron/runs` endpoint, API key scope, and canary are healthy.
