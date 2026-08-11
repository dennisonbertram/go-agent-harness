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
