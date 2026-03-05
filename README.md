# go-agent-harness

OpenAI-powered Go coding harness POC implemented as an event-driven service.

The service runs a deterministic tool-calling loop and emits run lifecycle events via SSE so a GUI or TUI can consume the stream directly.

## What Is Included

- HTTP service (`cmd/harnessd`)
- Sample CLI test client (`cmd/harnesscli`)
- In-memory run manager with event history + live subscribers
- SSE event stream per run
- OpenAI Chat Completions provider adapter
- Hook pipeline support in runner (pre-message and post-message hook stages)
- Modular tool architecture under `internal/harness/tools/` (catalog + one file per tool family)
- Coding toolset:
  - `AskUserQuestion`
  - `read`
  - `write`
  - `edit`
  - `bash`
  - `job_output`
  - `job_kill`
  - `ls`
  - `glob`
  - `grep`
  - `apply_patch`
  - `fetch`
  - `download`
  - `git_status`
  - `git_diff`
  - `todos`
  - `lsp_diagnostics`
  - `lsp_references`
  - `lsp_restart`
  - optional integrations (enabled when dependencies are configured): `sourcegraph`, `list_mcp_resources`, `read_mcp_resource`, dynamic `mcp_<server>_<tool>`, `agent`, `agentic_fetch`, `web_search`, `web_fetch`

## API

- `POST /v1/runs`
  - Body: `{ "prompt": "...", "model": "...", "system_prompt": "..." }`
  - Returns: `202 Accepted` with run id
- `GET /v1/runs/{runID}`
  - Returns current run state (`queued|running|waiting_for_user|completed|failed`)
- `GET /v1/runs/{runID}/events`
  - Server-Sent Events stream with run lifecycle events
- `GET /v1/runs/{runID}/input`
  - Returns the pending `AskUserQuestion` payload while the run is waiting for input
- `POST /v1/runs/{runID}/input`
  - Body: `{ \"answers\": { \"<question>\": \"<label or comma-separated labels>\" } }`
  - Submits answers and resumes the run
- `GET /healthz`

Event types currently emitted:

- `run.started`
- `llm.turn.requested`
- `llm.turn.completed`
- `hook.started`
- `hook.completed`
- `hook.failed`
- `tool.call.started`
- `tool.call.completed`
- `run.waiting_for_user`
- `run.resumed`
- `assistant.message`
- `run.completed`
- `run.failed`

## Quick Start

1. Set required environment variables:

```bash
export OPENAI_API_KEY=your_key_here
export HARNESS_WORKSPACE=/absolute/path/to/workspace
```

2. Run the server (preferred via `tmux` for long-running process management):

```bash
tmux new-session -d -s harnessd 'cd /absolute/path/to/go-agent-harness && go run ./cmd/harnessd'
```

Or run directly for a short local check:

```bash
go run ./cmd/harnessd
```

3. Start a run:

```bash
curl -sS -X POST localhost:8080/v1/runs \
  -H 'content-type: application/json' \
  -d '{"prompt":"List the files in this repo and run go test ./..."}'
```

4. Stream events:

```bash
curl -N localhost:8080/v1/runs/<run_id>/events
```

## Sample CLI Test Client

Run the lightweight CLI client to create a run and stream all events:

```bash
go run ./cmd/harnesscli \
  -base-url=http://localhost:8080 \
  -model=gpt-5-nano \
  -prompt='Create demo/sample.html with a heading and paragraph, then verify it exists'
```

Output includes:

- `run_id=<id>`
- streamed event lines (`run.started`, `tool.call.*`, `assistant.message`, ...)
- `terminal_event=run.completed|run.failed`

Detailed tmux live-test procedure, variables, and troubleshooting:

- `docs/runbooks/harnesscli-live-testing.md`

## Configuration

- `OPENAI_API_KEY` (required)
- `OPENAI_BASE_URL` (optional, default `https://api.openai.com`)
- `HARNESS_ADDR` (optional, default `:8080`)
- `HARNESS_MODEL` (optional, default `gpt-4.1-mini`)
- `HARNESS_WORKSPACE` (optional, default `.`)
- `HARNESS_SYSTEM_PROMPT` (optional)
- `HARNESS_MAX_STEPS` (optional, default `8`)
- `HARNESS_ASK_USER_TIMEOUT_SECONDS` (optional, default `300`)
- `HARNESS_TOOL_APPROVAL_MODE` (optional, `full_auto` or `permissions`, default `full_auto`)

## Development

```bash
go test ./...
./scripts/test-regression.sh
```

## Documentation Map

- `docs/INDEX.md`: Master index of all documentation folders.
- `docs/research/`: Research notes and source-backed findings.
- `docs/design/`: Product and technical design notes.
- `docs/explorations/`: Spikes and experiments.
- `docs/plans/`: Feature plans with checklists (required before implementation).
- `docs/logs/`: Engineering, observational, and system logs.
- `docs/context/`: Critical context for fast onboarding.
- `docs/runbooks/`: Operational procedures (testing, deployment, issue triage, worktree flow).
- `docs/operations/`: Nightly tasks and agent completion formats.
