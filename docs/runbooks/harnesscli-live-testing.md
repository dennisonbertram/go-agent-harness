# Harness CLI Live Testing Runbook

## Purpose

Run a real end-to-end harness test in `tmux` using:

- `cmd/harnessd` (server)
- `cmd/harnesscli` (sample client)

This documents exactly what was implemented, where configuration variables live, how to run it, and known issues seen in live tests.

## What Was Implemented

- New sample CLI client: `cmd/harnesscli`
  - Creates a run with `POST /v1/runs`
  - Streams events from `GET /v1/runs/{id}/events`
  - Exits on `run.completed` or `run.failed`
- Test coverage for CLI behavior and regressions:
  - payload contract
  - SSE parsing
  - success path
  - API error paths
  - invalid SSE data path

## Variables and Where They Are Defined

### Server environment variables (`cmd/harnessd`)

Defined in `cmd/harnessd/main.go`:

- `OPENAI_API_KEY` (required)
- `OPENAI_BASE_URL` (optional, default OpenAI API URL)
- `HARNESS_ADDR` (optional, default `:8080`)
- `HARNESS_MODEL` (optional, default `gpt-4.1-mini`)
- `HARNESS_WORKSPACE` (optional, default `.`)
- `HARNESS_SYSTEM_PROMPT` (optional, has default assistant prompt)
- `HARNESS_MAX_STEPS` (optional, default `8`)
- `HARNESS_PRICING_CATALOG_PATH` (optional, enables token->USD cost reporting)

### CLI runtime inputs (`cmd/harnesscli`)

Defined as flags in `cmd/harnesscli/main.go`:

- `-base-url` (default `http://localhost:8080`)
- `-prompt` (required)
- `-model` (optional)
- `-system-prompt` (optional)

## How To Run (tmux)

### 1) Start harness server in tmux

```bash
tmux new-session -d -s harnessd-live \
  'cd /Users/dennisonbertram/Develop/go-agent-harness && \
   HARNESS_ADDR=127.0.0.1:18081 \
   HARNESS_WORKSPACE=/Users/dennisonbertram/Develop/go-agent-harness \
   HARNESS_MODEL=gpt-5-nano \
   go run ./cmd/harnessd'
```

### 2) Verify server is up

```bash
curl -fsS http://127.0.0.1:18081/healthz
```

### 3) Run CLI in tmux

```bash
tmux new-session -d -s harnesscli-live \
  'cd /Users/dennisonbertram/Develop/go-agent-harness && \
   go run ./cmd/harnesscli \
     -base-url=http://127.0.0.1:18081 \
     -model=gpt-5-nano \
     -prompt="Create demo/tmux-live-smoke.html with heading Tmux Live Test and a short paragraph, then verify it exists with ls."'
```

### 4) Inspect tmux output

```bash
tmux capture-pane -pt harnesscli-live | tail -n 120
tmux capture-pane -pt harnessd-live | tail -n 120
```

### 5) Verify generated file

```bash
sed -n '1,120p' /Users/dennisonbertram/Develop/go-agent-harness/demo/tmux-live-smoke.html
```

### 6) Cleanup sessions

```bash
tmux kill-session -t harnesscli-live
tmux kill-session -t harnessd-live
```

## Expected Success Signals

- CLI exits with code `0`.
- CLI output includes:
  - `run_id=<...>`
  - streamed events
  - `usage.delta` events while the run executes
  - `terminal_event=run.completed`
- Generated file exists under `demo/`.

## Known Issues Observed in Live Runs

- The model may first call a tool with imperfect arguments (example seen: `apply_patch` called without required `find` argument).
- This does not break the harness loop by itself:
  - tool error is emitted in `tool.call.completed`
  - model can recover in later steps (example seen: followed by successful `write` and `ls`)
- For deterministic smoke tests, prompts should strongly bias toward `write` + `ls` rather than `apply_patch`.

## Troubleshooting

- Missing key:
  - Ensure `OPENAI_API_KEY` is present in shell environment before starting `harnessd`.
- Connection refused:
  - Check `HARNESS_ADDR` and health endpoint.
- No tmux session output:
  - `tmux list-sessions`
  - `tmux capture-pane -pt <session-name> | tail -n 200`
- Regression sanity check before live run:

```bash
go test ./...
./scripts/test-regression.sh
```
