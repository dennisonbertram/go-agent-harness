# Harder Terminal Bench suite - 2026-03-06

## Purpose

This note captures the current state of the harder private Terminal Bench smoke suite for the harness. The suite is intentionally a step above the original smoke tasks so it gives a more meaningful signal about harness quality, while still being small enough for periodic use.

## Task changes

### `go-retry-schedule-fix`

The task was made harder by requiring one additional behavioral constraint:

- first retry delay must start at `base`
- later retries must keep increasing by one base interval
- each computed delay must be capped at `30 * time.Second`
- non-positive inputs must still return `nil`

Current files:
- `benchmarks/terminal_bench/tasks/go-retry-schedule-fix/task.yaml`
- `benchmarks/terminal_bench/tasks/go-retry-schedule-fix/retry_test.go`
- `benchmarks/terminal_bench/tasks/go-retry-schedule-fix/tests/test_task.py`

### `staging-deploy-docs`

The task was made harder by requiring one additional structured config field and one additional documentation detail:

- add the `staging` target at `https://staging.internal:8443`
- add `healthcheck_path: "/readyz"` for `staging`
- document `make deploy-staging`
- document the smoke-check command `curl -fsS https://staging.internal:8443/readyz`
- keep `dev` and `prod` unchanged

Current files:
- `benchmarks/terminal_bench/tasks/staging-deploy-docs/task.yaml`
- `benchmarks/terminal_bench/tasks/staging-deploy-docs/tests/test_task.py`

### `incident-summary-shell`

The task was made harder by requiring one additional summary output:

- keep the title line `# Incident Summary`
- emit one markdown bullet per service in alphabetical order
- preserve singular/plural incident wording
- add a blank line after the bullets
- add a final line `Total incidents: <n>`

Current files:
- `benchmarks/terminal_bench/tasks/incident-summary-shell/task.yaml`
- `benchmarks/terminal_bench/tasks/incident-summary-shell/tests/test_task.py`

## Known benchmark result

A completed harder-suite run succeeded at:

- output bundle: `.tmp/terminal-bench/20260305-220021/2026-03-05__22-00-25`
- result file: `.tmp/terminal-bench/20260305-220021/2026-03-05__22-00-25/results.json`

Observed score on that completed run:

- `staging-deploy-docs`: passed
- `go-retry-schedule-fix`: failed
- `incident-summary-shell`: failed
- overall accuracy: `1/3`

Interpretation:
- the harder suite did expose weaker harness performance than the easier smoke suite
- the config/docs task remained solid
- the harder code and shell tasks were still weak enough to be useful benchmark signal

## Runner instability discovered during follow-up reruns

Repeated harder-suite reruns were not reliable. Two separate runner-level problems showed up.

### 1. Docker Compose Buildx session failures

Some reruns failed before any task execution with errors of the form:

- `failed to read dockerfile: no active session ... context deadline exceeded`

This came from `docker compose build` delegating to a Buildx container-based builder.

### 2. Local image-build instability and slowness

Attempts to work around the Buildx failure by changing build mode introduced another issue:

- forcing older/non-BuildKit compose behavior could stall builds
- prebuilding task images directly with Docker also showed very slow or inconsistent base-image resolution on this machine

Net effect:
- the harder suite is valid
- one completed harder run exists and is useful
- repeated local reruns are still not reliable enough for unattended periodic execution

## Runner changes attempted so far

The following files were changed while trying to stabilize reruns:

- `scripts/run-terminal-bench.sh`
- `benchmarks/terminal_bench/tasks/*/docker-compose.yaml`

Those changes explored:
- forcing Compose away from flaky bake/buildx paths
- changing concurrency
- prebuilding task images instead of building during each Terminal Bench trial

These attempts improved some failure modes but did not fully stabilize repeated runs on this local Docker setup.

## Issues tracking the underlying robustness work

High-priority issues were created for the main harness/tool problems discovered during Terminal Bench work:

- #12 `High priority: remove premature harnesscli timeout for streamed runs`
- #13 `High priority: make apply_patch compatible with unified diff payloads`
- #14 `High priority: harden structured file writes for JSON and machine-readable files`

## Recommended next steps

1. Separate benchmark validity from runner reliability.
   - The harder tasks are now good enough to keep.
   - The remaining problem is repeated execution reliability, not whether the harder tasks are meaningful.

2. Stop relying on per-trial Docker image builds.
   - The most likely durable fix is to settle on stable prebuilt task images and make Terminal Bench consume those images directly.

3. Treat Docker runner stability as its own work item.
   - The instability is local-environment-sensitive and should not be mixed with harness scoring work.

4. Once reruns are stable, rerun the harder suite several times.
   - That will tell us whether the harness is consistently around `1/3`, or whether the current harder-suite score is itself noisy.
