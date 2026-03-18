# go-agent-harness

`go-agent-harness` is a Go-based coding-agent platform centered on `harnessd`, an HTTP service that runs deterministic LLM/tool loops and streams lifecycle events over SSE. The repository also includes supporting CLIs and services for cron scheduling, MCP bridging, rollout forensics, coverage gating, and orchestration.

## Repository Surface

Primary binaries in `cmd/`:

- `harnessd`: main HTTP API for runs, conversations, skills, recipes, providers, models, cron forwarding, code search, and MCP server management.
- `harnesscli`: lightweight CLI that starts a run and streams SSE output to the terminal.
- `harness-mcp`: stdio MCP server that proxies the `harnessd` REST API as MCP tools.
- `cronsd`: standalone cron scheduling service with SQLite-backed job storage.
- `cronctl`: CLI for creating, listing, pausing, resuming, and inspecting cron jobs in `cronsd`.
- `forensics`: rollout-analysis CLI for comparing JSONL rollout files.
- `coveragegate`: coverage validation CLI used by the regression script.
- `symphd`: orchestration service for issue refresh/state/dead-letter workflows.

## Harness Capabilities

- Deterministic run loop with bounded steps, optional per-run cost ceilings, and SSE event streaming.
- Modular tool architecture under `internal/harness/tools/`.
- Prompt routing via file-backed prompt profiles, behaviors, talents, and runtime context injection.
- Optional observational memory with SQLite or Postgres-backed storage modes.
- Optional conversation persistence, compaction, export, and retention cleanup.
- Optional deferred tooling for cron, skills, MCP, sourcegraph, model catalogs, recipes, agent delegation, and web operations.

Default built-in tool surface includes:

- `AskUserQuestion`
- `apply_patch`
- `bash`
- `compact_history`
- `context_status`
- `download`
- `edit`
- `fetch`
- `git_diff`
- `git_status`
- `glob`
- `grep`
- `job_kill`
- `job_output`
- `ls`
- `lsp_diagnostics`
- `lsp_references`
- `lsp_restart`
- `observational_memory`
- `read`
- `todos`
- `write`

Additional tools are registered when their supporting dependencies are configured, including `find_tool`, `list_models`, conversation tools, cron tools, skill tools, recipe execution, Sourcegraph, MCP resources, dynamic `mcp_<server>_<tool>` tools, callbacks, agent delegation, and web search/fetch tools.

## Primary HTTP APIs

`harnessd` serves:

- `GET /healthz`
- `POST /v1/runs`
- `GET /v1/runs`
- `POST /v1/runs/replay`
- `GET /v1/runs/{id}`
- `GET /v1/runs/{id}/events`
- `GET|POST /v1/runs/{id}/input`
- `GET /v1/runs/{id}/summary`
- `POST /v1/runs/{id}/continue`
- `POST /v1/runs/{id}/steer`
- `GET /v1/runs/{id}/context`
- `POST /v1/runs/{id}/compact`
- `GET|PUT /v1/runs/{id}/todos`
- `GET /v1/conversations/`
- `GET /v1/conversations/search`
- `DELETE /v1/conversations/{id}`
- `GET /v1/conversations/{id}/messages`
- `GET /v1/conversations/{id}/runs`
- `GET /v1/conversations/{id}/export`
- `POST /v1/conversations/{id}/compact`
- `POST /v1/conversations/cleanup`
- `GET /v1/models`
- `GET /v1/providers`
- `POST /v1/agents`
- `POST /v1/summarize`
- `GET|POST /v1/cron/jobs`
- `GET|PATCH|DELETE /v1/cron/jobs/{id}`
- `GET /v1/cron/jobs/{id}/history`
- `GET /v1/skills`
- `GET /v1/skills/{name}`
- `POST /v1/skills/{name}/verify`
- `GET /v1/recipes`
- `GET /v1/recipes/{name}`
- `GET /v1/recipes/{name}/schema`
- `POST /v1/search/code`
- `GET|POST /v1/mcp/servers`

Key run statuses are `queued`, `running`, `waiting_for_user`, `completed`, and `failed`.

Common streamed event types include:

- `run.started`
- `llm.turn.requested`
- `assistant.message.delta`
- `tool.call.delta`
- `llm.turn.completed`
- `usage.delta`
- `hook.started`
- `hook.completed`
- `hook.failed`
- `tool.call.started`
- `tool.call.completed`
- `run.waiting_for_user`
- `run.resumed`
- `prompt.resolved`
- `prompt.warning`
- `memory.observe.started`
- `memory.observe.completed`
- `memory.observe.failed`
- `memory.reflection.completed`
- `assistant.message`
- `run.completed`
- `run.failed`

The full event contract lives in `docs/design/event-catalog.md`.

## Quick Start

Set the minimum environment, then run `harnessd` in `tmux`:

```bash
export OPENAI_API_KEY=your_key_here
export HARNESS_WORKSPACE=/absolute/path/to/workspace

tmux new-session -d -s harnessd \
  'cd /absolute/path/to/go-agent-harness && \
   HARNESS_AUTH_DISABLED=true \
   go run ./cmd/harnessd'
```

Start a run with `curl`:

```bash
curl -sS -X POST http://localhost:8080/v1/runs \
  -H 'content-type: application/json' \
  -d '{"prompt":"List the files in this repo and run go test ./..."}'
```

Stream events:

```bash
curl -N http://localhost:8080/v1/runs/<run_id>/events
```

Inspect the server session:

```bash
tmux capture-pane -pt harnessd | tail -n 120
```

Clean up:

```bash
tmux kill-session -t harnessd
```

## Harness CLI

Run the CLI client against a local server:

```bash
go run ./cmd/harnesscli \
  -base-url=http://localhost:8080 \
  -model=gpt-5-nano \
  -agent-intent=code_review \
  -task-context='Review retry logic and report regressions' \
  -prompt-behavior=precise \
  -prompt-talent=review \
  -prompt-custom='Keep final response concise.' \
  -prompt='Create demo/sample.html with a heading and paragraph, then verify it exists'
```

Output includes `run_id=<id>`, streamed event lines, and `terminal_event=run.completed|run.failed`.

For a full tmux-based live-smoke workflow, see `docs/runbooks/harnesscli-live-testing.md`.

## Configuration

`harnessd` resolves configuration in layers:

1. `~/.harness/config.toml`
2. `<workspace>/.harness/config.toml`
3. optional profile from `~/.harness/profiles/<name>.toml` via `go run ./cmd/harnessd --profile <name>`
4. `HARNESS_*` and provider environment overrides

Core runtime and prompt configuration:

- `OPENAI_API_KEY` (required)
- `OPENAI_BASE_URL`
- `HARNESS_ADDR`
- `HARNESS_WORKSPACE`
- `HARNESS_MODEL`
- `HARNESS_MAX_STEPS`
- `HARNESS_MAX_COST_PER_RUN_USD`
- `HARNESS_SYSTEM_PROMPT`
- `HARNESS_DEFAULT_AGENT_INTENT`
- `HARNESS_PROMPTS_DIR`
- `HARNESS_ASK_USER_TIMEOUT_SECONDS`
- `HARNESS_TOOL_APPROVAL_MODE`
- `HARNESS_AUTH_DISABLED`

Memory configuration:

- `HARNESS_MEMORY_MODE`
- `HARNESS_MEMORY_DB_DRIVER`
- `HARNESS_MEMORY_DB_DSN`
- `HARNESS_MEMORY_SQLITE_PATH`
- `HARNESS_MEMORY_DEFAULT_ENABLED`
- `HARNESS_MEMORY_OBSERVE_MIN_TOKENS`
- `HARNESS_MEMORY_SNIPPET_MAX_TOKENS`
- `HARNESS_MEMORY_REFLECT_THRESHOLD_TOKENS`
- `HARNESS_MEMORY_LLM_MODE`
- `HARNESS_MEMORY_LLM_MODEL`
- `HARNESS_MEMORY_LLM_BASE_URL`
- `HARNESS_MEMORY_LLM_API_KEY`

Catalog, integrations, and automation:

- `HARNESS_PRICING_CATALOG_PATH`
- `HARNESS_MODEL_CATALOG_PATH`
- `HARNESS_SKILLS_ENABLED`
- `HARNESS_WATCH_ENABLED`
- `HARNESS_WATCH_INTERVAL_SECONDS`
- `HARNESS_RECIPES_DIR`
- `HARNESS_CRON_URL`
- `HARNESS_ENABLE_CALLBACKS`
- `HARNESS_SOURCEGRAPH_ENDPOINT`
- `HARNESS_SOURCEGRAPH_TOKEN`
- `HARNESS_ROLLOUT_DIR`
- `HARNESS_GLOBAL_DIR`

Conversation persistence:

- `HARNESS_CONVERSATION_DB`
- `HARNESS_CONVERSATION_RETENTION_DAYS`

When a persistent run store is configured, API endpoints require Bearer auth. SSE also accepts `?token=` for clients that cannot set headers on event-stream requests. In the default local setup without a run store, auth is effectively off.

## Related Services

`cronsd` exposes:

- `GET /healthz`
- `GET|POST /v1/jobs`
- `GET|PATCH|DELETE /v1/jobs/{id}`
- `GET /v1/jobs/{id}/history`

`symphd` exposes:

- `GET /api/v1/state`
- `GET /api/v1/issues`
- `POST /api/v1/refresh`
- `GET /api/v1/dead-letters`

## Development

Run the standard regression gate:

```bash
./scripts/test-regression.sh
```

For the private Terminal Bench suite:

```bash
./scripts/run-terminal-bench.sh
```

The benchmark bridge copies the current repo into each task container, builds `harnessd` and `harnesscli`, and drives tasks through the live harness API.

## Documentation Map

- `docs/INDEX.md`: master index of all documentation folders.
- `docs/context/`: critical project context for fast onboarding.
- `docs/design/`: product and technical design documents.
- `docs/explorations/`: spikes and experiment notes.
- `docs/implementation/`: completed implementation writeups and issue notes.
- `docs/investigations/`: deep-dive investigations and reviews.
- `docs/logs/`: engineering, observational, system, and intent logs.
- `docs/operations/`: recurring operating procedures and completion templates.
- `docs/plans/`: plans and checklists created before implementation work.
- `docs/research/`: source-backed research artifacts.
- `docs/runbooks/`: operational procedures for testing, deployment, and maintenance.
- `docs/testing/`: benchmark notes and tool-usability findings.
