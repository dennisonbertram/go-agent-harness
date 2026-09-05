# Harness CLI Live Testing

This runbook covers the current `cmd/harnesscli` entrypoint and how it talks to the live harness server.

## Prerequisites

- A running harness server, usually `go run ./cmd/harnessd`
- `OPENAI_API_KEY` set in the server environment
- A reachable server base URL, usually `http://127.0.0.1:8080`

## What The CLI Does

The CLI creates a run with `POST /v1/runs`, then follows `GET /v1/runs/{id}/events` until the run reaches a terminal state.

## Current Flags

The CLI currently accepts:

- `-base-url`
- `-prompt`
- `-model`
- `-system-prompt`
- `-agent-intent`
- `-task-context`
- `-prompt-profile`
- `-prompt-behavior`
- `-prompt-talent`
- `-prompt-custom`
- `-list-profiles`
- `-tui`

The prompt-extension flags are forwarded into the run request and are the current way to exercise prompt customization from the CLI.

The CLI also supports:

- `harnesscli auth login` (flags: `-server`, `-tenant`, `-name`)
- `harnesscli list` / `harnesscli status <run-id>` / `harnesscli cancel <run-id>`
- `harnesscli steer <run-id> <prompt>`
- `harnesscli continue <run-id> <prompt>` — only valid once the named run has
  reached a terminal state (`run.completed`/`run.failed`/`run.cancelled`)
- `harnesscli input <run-id> "<question>=<answer>" [...]` — answers a run
  blocked on `run.waiting_for_user`
- `harnesscli approve <run-id>` / `harnesscli deny <run-id>` — resolves a run
  blocked on `tool.approval_required` or `plan.approval_required`

## Typical Commands

```bash
go run ./cmd/harnessd
```

```bash
go run ./cmd/harnesscli \
  -base-url http://127.0.0.1:8080 \
  -model gpt-4.1 \
  -prompt "Review the repository documentation for stale claims"
```

```bash
go run ./cmd/harnesscli \
  -base-url http://127.0.0.1:8080 \
  -prompt "Summarize the current API surface" \
  -tui
```

## Expected Behavior

- The CLI should print or render run progress from the event stream.
- Terminal events should stop the session cleanly.
- If a live run fails, inspect the server event stream first, then the run summary and conversation endpoints.

## Blocked runs (waiting for input or approval)

A headless (non-`-tui`) run exits with code 3 when it hits a signal it
cannot answer without a human: `run.waiting_for_user` (the agent called
`AskUserQuestion`) or `tool.approval_required` / `plan.approval_required`
(a gated tool call or plan exit is pending). The server-side run is left
intact — only the CLI process exits. The printed stderr hint names the
command that actually resolves that specific block:

- `run.waiting_for_user` → `harnesscli input <run-id> "<question>=<answer>" [...]`.
  Use `harnesscli status <run-id>` or `GET /v1/runs/{id}/input` first to see
  the pending question text and valid option labels.
- `tool.approval_required` / `plan.approval_required` → `harnesscli approve
  <run-id>` or `harnesscli deny <run-id>`.

`harnesscli continue <run-id> <prompt>` is a different operation — it starts
a new run continuing a **completed** conversation. Calling it on a run that
is still `waiting_for_user` or `waiting_for_approval` returns
`409 run_not_completed`; the CLI now catches that and reprints the correct
command for the run's actual state instead of repeating `continue`.

## Dashboard smoke

1. Start two runs, then enter the TUI and type `/dashboard` (or press `Ctrl+D`).
2. Confirm grouped running/waiting/completed rows refresh without closing the current session.
3. Select a running row and press `p`; confirm its event stream appears. Press `Esc` once to close peek and again to close the dashboard.
4. Use `s` to enter a steering prompt, `x` to cancel a selected run, and `n` to dispatch a new prompt. Confirm each change appears on the next refresh.

## Causal PTY acceptance artifacts

For real slash-command acceptance, use an owned direct 30x100 PTY with one
collector. Do not send the next key until the current action has an immutable
VT-interpreted screen/frame record. The first #1088 batch records `/help`,
`/cost`, `/stats`, `/config`, `/context`, `/doctor`, `/permissions`, transcript
search/Escape, unknown-command feedback, and `/resume` plus `/continue`.

`/continue` targets the completed `/resume` child rather than reusing a source
run: continuation source runs are one-shot. Verify that target through the API
before typing, then independently prove one shared conversation, distinct run
IDs, exactly-one `assistant.message`/`run.completed` event for each child, and
the durable conversation messages. Failed runs retain their private artifacts;
they are not a passing substitute.

## Relevant Code Paths

- CLI entrypoint: `cmd/harnesscli/main.go`
- Auth subcommand dispatch and login flow: `cmd/harnesscli/auth.go`
- Run HTTP API: `internal/server/http.go`
- Run payload and response types: `internal/harness/types.go`
