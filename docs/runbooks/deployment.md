> **Binding beyond this machine.** `harnessd` listens on `127.0.0.1:8080` by
> default. Authentication is implicitly disabled when no API key store is
> configured, so an unauthenticated daemon on a routable address is an open
> agent-execution service: anyone who can reach the port can start runs in the
> workspace with the daemon's provider credentials and read the results.
>
> The daemon therefore **refuses to start** when `HARNESS_ADDR` names a
> non-loopback address and no authentication is configured. To listen publicly,
> either configure an API key store, or set `HARNESS_AUTH_DISABLED=true` to accept
> an open daemon deliberately. `/mcp` is authenticated exactly like `/v1`.

# Deployment Runbook

## Goal

Deploy MVP safely with practical controls; do not over-engineer for enterprise scale.

## Pre-Deployment Checklist

- [ ] Feature plan completed and checked off.
- [ ] Tests pass in CI/local.
- [ ] Security-sensitive changes reviewed.
- [ ] Required environment variables documented.
- [ ] Rollback steps prepared.

## Deployment Steps

1. Merge tested branch into `main`.
2. Trigger deployment pipeline.
3. Run smoke tests on deployed environment.
4. Validate critical user paths.
5. Log deployment result in engineering log.

## Post-Deployment

- [ ] Monitor error rates and key metrics.
- [ ] Record anomalies in observational log.
- [ ] Create GitHub issues for any discovered defects.

## Shutdown semantics

On SIGINT/SIGTERM (or SIGHUP for config reload), `harnessd` stops accepting
new callback/cron work, then cancels every still-in-flight run through the
same path `POST /v1/runs/{id}/cancel` uses: this interrupts an in-progress
provider call or tool execution (a `bash` tool child is killed by process
group within milliseconds) and gives each run up to a bounded few seconds to
reach `RunStatusCancelled` before the daemon moves on. The number of runs
cancelled is logged. Only after that does the HTTP server drain (10s budget)
and the run store close, so a cancelled run's terminal status is durable
across a restart — a redeploy or `launchd`/`harnesscli service` restart does
not leave orphaned tool subprocesses or runs stuck `running` forever (issue
#1373). Store-close ordering while a client still holds an SSE subscription
open, and per-request handler-context cancellation, are tracked separately
(issue #1356) and are not covered by this bound.
