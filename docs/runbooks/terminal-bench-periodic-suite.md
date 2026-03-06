# Terminal Bench Periodic Suite

## Purpose

Use this suite to run a small, stable Terminal Bench smoke benchmark against the current `go-agent-harness` checkout.

It is designed for periodic quality checks, not leaderboard chasing:

- The agent under test is this repository's harness, not a stock Terminal Bench integration.
- The benchmark bridge copies the current repo into each task container, builds `harnessd` and `harnesscli`, starts the server in tmux inside the task container, and drives the task through the real HTTP API.
- The task set is intentionally small and deterministic so failures are actionable.

## Included Tasks

- `go-retry-schedule-fix`: simple Go bugfix that must end with `go test ./...` passing.
- `staging-deploy-docs`: config + documentation edit that verifies stable file mutations.
- `incident-summary-shell`: shell-script repair that verifies file generation and output formatting.

## Prerequisites

- `OPENAI_API_KEY` must be set.
- Docker must be available.
- Either `tb` must be installed, or `uv` must be available so the runner can use `uv tool run terminal-bench`.

Optional environment overrides:

- `OPENAI_BASE_URL`
- `HARNESS_BENCH_MODEL` (defaults to `gpt-5-nano`)
- `HARNESS_BENCH_MAX_STEPS` (defaults to `12`)
- `HARNESS_BENCH_MEMORY_MODE` (defaults to `off`)
- `TERMINAL_BENCH_OUTPUT_DIR`

## Local Run

```bash
./scripts/run-terminal-bench.sh
```

Runner behavior:

- Uses dataset path `benchmarks/terminal_bench/tasks`.
- Imports `benchmarks.terminal_bench.agent:GoAgentHarnessAgent`.
- Writes benchmark output under `.tmp/terminal-bench/<timestamp>/` by default.

## CI Schedule

Workflow: `.github/workflows/terminal-bench-periodic.yml`

- Triggers nightly via cron and on manual `workflow_dispatch`.
- Uploads `.tmp/terminal-bench/` as a workflow artifact.

## Failure Triage

1. Inspect the uploaded Terminal Bench result directory for per-task logs.
2. Check `/tmp/harnessd.log` inside the failing task log stream when startup fails.
3. Re-run the exact suite locally with the same `HARNESS_BENCH_MODEL`.
4. If the harness regressed on a task, add a repo-native regression test before fixing the benchmark failure.
