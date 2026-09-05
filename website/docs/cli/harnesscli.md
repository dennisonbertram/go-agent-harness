---
title: "harnesscli Reference"
sidebar_label: "harnesscli"
sidebar_position: 2
---

import { Callout, Tabs, TabsList, TabsTrigger, TabsContent } from '@site/src/components/ui';

`harnesscli` is the terminal client for the go-code harness. It speaks to a running `harnessd` server over HTTP and lets you do everything from sending a one-shot prompt and watching events stream in real time, to listing past runs, continuing multi-turn conversations, replaying recorded rollouts, and running the autoresearch improvement loop — all without leaving your shell.

The key design split: `harnessd` owns the model, the tools, and all run state; `harnesscli` is a thin client that formats output for humans and scripts.

---

## Installation

`harnesscli` is built alongside `harnessd` by the standard install script:

```bash
bash scripts/install.sh          # installs to ~/.local/bin by default
bash scripts/install.sh --system  # installs to /usr/local/bin
```

Or build directly:

```bash
go build -o harnesscli ./cmd/harnesscli
```

For isolated development worktrees, `scripts/init.sh` builds `harnesscli` into `.tmp/bootstrap/bin/harnesscli` and exports `HARNESS_CLI_BINARY` in the generated `dev.env` file.

---

## Default streaming run mode

When you invoke `harnesscli` with no recognized subcommand — typically just `--prompt` and other flags — it runs in **streaming run mode**:

1. `POST /v1/runs` to create the run.
2. Print `run_id=<id>` to stdout.
3. Stream SSE events from `GET /v1/runs/{id}/events` until a terminal event arrives.
4. Print `terminal_event=<event_type>` and exit.

Terminal events are `run.completed`, `run.failed`, and `run.cancelled`. Every non-terminal event is printed as a single line: `<event_type> <full_event_json>`.

### Exit codes

The process exit code reports the run's outcome, so scripts and CI can branch on `$?`:

| Code | Meaning |
|---|---|
| `0` | `run.completed` — the run finished successfully |
| `1` | Client-side error: bad flags, missing prompt, connection/HTTP/stream failure |
| `2` | `run.failed` — a turn failed server-side |
| `3` | Blocked — the run needs input it will never get headlessly (`run.waiting_for_user`, `tool.approval_required`, or `plan.approval_required` observed while stdin is non-interactive) |
| `6` | `run.cancelled` — interrupted; **not** resumable via `harnesscli continue` (that command requires the source run's status to be `completed` and returns HTTP 409 otherwise). Start a new run to continue the work. |
| `130` | SIGINT/SIGTERM while streaming |

The `run_id=` / `terminal_event=` stdout lines are unchanged by this mapping. See [Exit Codes](/docs/reference/exit-codes) for the full contract — blocked-signal details, goal-status reservations, and per-command coverage.

### Flags

| Flag | Default | Description |
|---|---|---|
| `-base-url` | `http://localhost:8080` | Harness API base URL |
| `-prompt` | (required) | Prompt to send |
| `-model` | `""` | Model override for this run |
| `-system-prompt` | `""` | System prompt override |
| `-agent-intent` | `""` | Startup intent for prompt routing (e.g. `code_review`) |
| `-task-context` | `""` | Task context injected into the startup prompt |
| `-prompt-profile` | `""` | Prompt profile override for model routing |
| `-prompt-custom` | `""` | Custom prompt extension text |
| `-workspace` | cwd | Workspace directory for this run (sent as `workspace_path`; see the callout below) |
| `-plan-mode` | `false` | Start the run in enforced read-only plan mode (`plan_mode` in the request); see [Enforced Plan Mode](/docs/concepts/configuration) |
| `-resume` | `""` | Resume an existing conversation by ID in the TUI; implies `-tui` |
| `-tui` | `false` | Launch the interactive BubbleTea TUI (requires a real terminal) |
| `-list-profiles` | `false` | List available profiles and exit |
| `-prompt-behavior` | (empty) | Behavior extension IDs — repeatable or comma-separated |
| `-prompt-talent` | (empty) | Talent extension IDs — repeatable or comma-separated |

Go's `flag` package accepts both `-flag` and `--flag` forms; examples in this page use a single dash to match the source.

### Example: run and watch events

```bash
# Start harnessd in fake (key-free) mode first:
HARNESS_PROVIDER=fake \
HARNESS_FAKE_TURNS=turns.json \
HARNESS_AUTH_DISABLED=true \
go run ./cmd/harnessd &

# Stream a run:
harnesscli -prompt "What is 2+2?"
```

Output:

```
run_id=<uuid>
run.started {"id":"evt_...","run_id":"...","type":"run.started","timestamp":"2026-...Z","payload":{"prompt":"What is 2+2?"}}
run.step.started {"id":"evt_...","run_id":"...","type":"run.step.started","timestamp":"2026-...Z","payload":{"step":1}}
run.step.completed {"id":"evt_...","run_id":"...","type":"run.step.completed","timestamp":"2026-...Z","payload":{"step":1}}
run.completed {"id":"evt_...","run_id":"...","type":"run.completed","timestamp":"2026-...Z","payload":{"output":"..."}}
terminal_event=run.completed
```

<Callout type="info">
`-workspace` defaults to the current working directory via `os.Getwd()` and is sent as `workspace_path` in the run creation request. The server honors `workspace_path` when it is an absolute path to an existing directory: tools for the run are rooted there instead of the server's own working directory. `workspace_type` and profile-level runner configuration control workspace *provisioning* (local directory, git worktree, container, VM); `workspace_path` only selects which existing directory a non-provisioned (local-process) run is rooted in.
</Callout>

<Callout type="warning">
`-tui` requires a real terminal. If stdout is a pipe, harnesscli exits with: `--tui requires a terminal; pipe output or use without --tui for streaming mode`.
</Callout>

### Two HTTP clients

The CLI maintains two separate `http.Client` instances:

- **`requestHTTPClient`** — 60-second timeout, used for all non-streaming HTTP calls (creating runs, fetching status, etc.).
- **`streamHTTPClient`** — no timeout, idle-connection reaping disabled, keep-alives enabled. Used exclusively for the SSE event stream so that long tool-call pauses do not cause the connection to be dropped mid-run.

---

## Run-management subcommands

The non-streaming subcommands below (`list`, `cancel`, `status`, `replay`, `search`) exit `0` on success and `1` on error — the terminal-event exit-code mapping does not apply to them since they never observe an event stream. Streaming `continue` follows the same [exit-code contract](/docs/reference/exit-codes) as the one-shot mode.

### list / runs

List recent runs. Both `list` and `runs` invoke the same handler.

```bash
harnesscli list
harnesscli list -status running
harnesscli list -conversation-id <cid>
```

**API:** `GET /v1/runs`

| Flag | Default | Description |
|---|---|---|
| `-base-url` | `http://localhost:8080` | Harness API base URL |
| `-status` | `""` | Filter: `queued`, `running`, `completed`, or `failed` |
| `-conversation-id` | `""` | Filter by conversation ID |

Output is a 4-column table (ID, STATUS, MODEL, PROMPT). The prompt column is truncated at 40 characters. When no runs match, the command prints `No runs found`.

---

### cancel

Cancel a running or queued run.

```bash
harnesscli cancel <run-id>
```

**API:** `POST /v1/runs/{id}/cancel`

On success prints: `Run <id> cancelling`.

---

### status / show

Show the full details of a single run. Both `status` and `show` invoke the same handler.

```bash
harnesscli status <run-id>
harnesscli show <run-id>
```

**API:** `GET /v1/runs/{id}`

Output includes: ID, Status, Model, Created, Updated, Prompt (truncated at 80 chars), Error, Output, and workflow recap fields when present.

---

### continue

Send a follow-up prompt to an existing run, creating a new run in the same conversation.

<Callout type="warning">
`continue` only works when the source run's status is `completed`. It returns HTTP 409 `run_not_completed` for any other status (`waiting_for_user`, `waiting_for_approval`, `running`, `queued`, `failed`, or `cancelled`) — a cancelled run cannot be resumed at all, and a run blocked on a question or approval must be unblocked first (see `harnesscli input` below, or `POST /v1/runs/{id}/approve` / `/deny`).
</Callout>

```bash
# Stream the continuation (default):
harnesscli continue <run-id> Now explain it to a 5-year-old

# Create without streaming:
harnesscli continue -no-stream <run-id> Now explain it to a 5-year-old
```

**API:** `POST /v1/runs/{id}/continue` with body `{"prompt": "..."}`

The continuation prompt is everything after the run ID, joined with spaces.

| Flag | Default | Description |
|---|---|---|
| `-base-url` | `http://localhost:8080` | Harness API base URL |
| `-no-stream` | `false` | Print only `run_id=<id>` and exit without streaming events |

When `-no-stream` is false (the default), the new run's events are streamed and `terminal_event=<type>` is printed on completion; the same [exit-code contract](/docs/reference/exit-codes) as the one-shot mode applies. When `-no-stream` is true, only `run_id=<id>` is printed and the exit code stays `0`/`1` (no terminal event is observed).

---

### input

Answer a run that is blocked on `run.waiting_for_user` (the run invoked the `AskUserQuestion` tool).

```bash
harnesscli input <run-id> "question-key=the answer"
harnesscli input <run-id> "q1=yes" "q2=no"
```

**API:** `POST /v1/runs/{id}/input` with body `{"answers": {"<question-key>": "<answer>"}}`

Each positional argument after the run ID is split on the first `=`; the part before `=` is the question key (as returned by `GET /v1/runs/{id}/input`) and the part after is the answer. The run resumes automatically once all pending questions are answered.

---

### replay

Replay a recorded rollout. The rollout can be provided as a run ID (the server locates the JSONL file) or as a direct rollout file path.

```bash
# Simulate replay (default):
harnesscli replay <run-id>

# Fork at step 5 and hand off to a live runner:
harnesscli replay -mode fork -fork-step 5 <run-id>

# Detect drift between a recording and a fresh run:
harnesscli replay -detect-drift <run-id>
```

**API:** `POST /v1/runs/replay`

| Flag | Default | Description |
|---|---|---|
| `-base-url` | `http://localhost:8080` | Harness API base URL |
| `-mode` | `simulate` | Replay mode: `simulate` or `fork` |
| `-fork-step` | `0` | Step to fork from (only used when `-mode=fork`) |
| `-detect-drift` | `false` | Run drift detection during simulate replay |

The `fork_step` field is included in the request payload only when `-mode=fork`. Output is pretty-printed JSON to stdout.

See the [Rollout & Replay](/docs/operations/rollout-replay-forensics) reference for details on the underlying replay semantics.

---

### search

Search runs by substring. This is a **client-side** operation.

```bash
harnesscli search "fix the login bug"
harnesscli search -status completed authentication
```

<Callout type="warning">
`search` fetches all runs from `GET /v1/runs` and filters them in-process. There is no dedicated server-side search endpoint. On large run histories this can be slow.
</Callout>

| Flag | Default | Description |
|---|---|---|
| `-base-url` | `http://localhost:8080` | Harness API base URL |
| `-status` | `""` | Pre-filter by status before searching |

The query is all positional args joined with spaces. Matching is case-insensitive substring across: ID, conversation\_id, tenant\_id, model, prompt, output, status, error, and workflow recap fields.

---

### steer

Inject a steering message into an active run without stopping it.

```bash
harnesscli steer <run-id> "focus on the auth module instead"
```

**API:** `POST /v1/runs/{id}/steer` with body `{"prompt": "..."}`

The server queues the message; the harness delivers it to the agent as a user message at the next step boundary, and the run keeps going. Empty or whitespace-only prompts are rejected client-side before any request is sent.

---

### viz

Print the URL for the `/viz` static visualization UI served by `harnessd`, optionally opening it in the default browser.

```bash
harnesscli viz
harnesscli viz --open
```

---

### acp

Serve the Agent Client Protocol (newline-delimited JSON-RPC 2.0) over stdin/stdout so ACP-compatible editors (Zed, JetBrains via ACP) can drive go-code as a subprocess. This is the same protocol the standalone `harness-acp` binary exposes; `harnesscli acp` is an equivalent entrypoint reached through the main CLI. See the [ACP runbook](https://github.com/dennisonbertram/go-code/blob/main/docs/runbooks/acp.md) for the manual Zed verification checklist.

```bash
harnesscli acp
harnesscli acp -server http://my-harness:9090
```

stdout is a pure protocol channel — all diagnostics go to stderr.

---

### plugin

Manage installable plugin bundles (`plugin.json` bundles under `~/.go-harness/plugins`).

```bash
harnesscli plugin install <path-or-url>
harnesscli plugin list
harnesscli plugin uninstall <name>
harnesscli plugin update <name>
harnesscli plugin trust <name>
harnesscli plugin untrust <name>
harnesscli plugin marketplace <subcommand>
```

Trusted bundles alone reach profiles, MCP validation, and hooks; enabled visibility is independent from executable trust. See `docs/design/plugins.md` for the bundle schema.

---

### mcp

Manage saved credentials for remote MCP servers configured for this CLI.

```bash
harnesscli mcp login <server-name>
harnesscli mcp status <server-name>
harnesscli mcp logout <server-name>
```

---

### hooks

Manage trust for config-driven lifecycle hook files (shell/HTTP hooks, epic #737).

```bash
harnesscli hooks trust <hook-file>
harnesscli hooks revoke <hook-file>
harnesscli hooks list
```

Hook loading itself is read-only and startup-computed; use `GET /v1/hooks` to see what a running `harnessd` actually loaded. See `docs/design/plugins.md` → "Config-driven hooks" for the hook-file schema.

---

### service

Install, manage, and check the status of `harnessd` as an OS-level background service (launchd on macOS, systemd on Linux).

```bash
harnesscli service install --binary /path/to/harnessd --addr 127.0.0.1:8080
harnesscli service start
harnesscli service stop
harnesscli service status
harnesscli service uninstall
```

| Flag (install) | Default | Description |
|---|---|---|
| `--binary` | look up `harnessd` on `PATH` | Path to the `harnessd` binary |
| `--addr` | resolve like `harnessd` — `HARNESS_ADDR` env or `127.0.0.1:8080` | Listen address for harnessd |
| `--log-dir` | `~/.harness/logs` | Directory for service logs |
| `--dry-run` | `false` | Print the rendered unit file and target path without writing anything |

---

## auth login and config files

### auth login

Generate a local API key and save it for use with the harness server.

```bash
harnesscli auth login
harnesscli auth login -server http://my-harness:9090 -tenant myteam -name laptop
```

| Flag | Default | Description |
|---|---|---|
| `-server` | `http://localhost:8080` | Harness server URL (stored in config, not contacted) |
| `-tenant` | `default` | Tenant ID for the generated key |
| `-name` | `cli` | Human-readable label for the key |

<Callout type="warning">
`auth login` does **not** contact the server. The API key is generated locally using `store.GenerateAPIKey`. The `-server` flag only controls what URL is written into the saved config file so subsequent requests know where to connect.
</Callout>

On success, `auth login`:
1. Writes `~/.harness/config.json` (directory mode `0700`, file mode `0600`).
2. Prints the file path, the raw key, and a ready-to-use `Authorization: Bearer <key>` example.

The generated key carries three scopes: `store.ScopeRunsRead`, `store.ScopeRunsWrite`, and `store.ScopeAdmin`.

### auth kimi

Manage Kimi Code subscription auth (epic #848). Reuses a `kimi-code`-authenticated vendor session through a harness-owned credential copy at `~/.harness/subscription-auth/kimi.json`; it never writes under `~/.kimi-code/`.

```bash
kimi-code login          # vendor CLI login, done once outside harnesscli
harnesscli auth kimi login
harnesscli auth kimi status
harnesscli auth kimi logout
```

`logout` removes only `~/.harness/subscription-auth/kimi.json`.

### auth codex

Manage Codex subscription auth (epic #847). Reuses a ChatGPT-authenticated vendor Codex session through a harness-owned credential copy at `~/.harness/subscription-auth/codex.json`; it never writes under `~/.codex/` and only reads from it.

```bash
codex login               # vendor CLI login, done once outside harnesscli
harnesscli auth codex login
harnesscli auth codex status
harnesscli auth codex logout
```

`logout` removes only `~/.harness/subscription-auth/codex.json`. The `openai` provider (`OPENAI_API_KEY`) remains the primary, unaffected path.

### Config file locations

`harnesscli` uses two separate config files for different purposes:

<Tabs defaultValue="auth">
  <TabsList>
    <TabsTrigger value="auth">Auth config</TabsTrigger>
    <TabsTrigger value="persistent">Persistent CLI config</TabsTrigger>
  </TabsList>
  <TabsContent value="auth">

**`~/.harness/config.json`** — Written by `auth login`.

```json
{
  "server": "http://localhost:8080",
  "api_key": "<raw-token>"
}
```

  </TabsContent>
  <TabsContent value="persistent">

**`~/.config/harnesscli/config.json`** — Written by the `config` package (`cmd/harnesscli/config`).

```go
type Config struct {
    StarredModels  []string          `json:"starred_models,omitempty"`
    Gateway        string            `json:"gateway,omitempty"`
    APIKeys        map[string]string `json:"api_keys,omitempty"`
    HistoryEntries []string          `json:"history_entries,omitempty"`
}
```

`Gateway` controls provider routing: `""` means direct, `"openrouter"` routes through OpenRouter.

  </TabsContent>
</Tabs>

---

## improve (autoresearch loop)

`harnesscli improve` wraps the `scripts/autoresearch-loop.sh` shell script, exposing the autoresearch self-improvement loop as a first-class CLI command.

<Callout type="warning">
`improve` must be run from the repository root. It looks for `scripts/autoresearch-loop.sh` relative to the current working directory and exits with an error if the file is not found: `scripts/autoresearch-loop.sh not found; run from the go-code repository`.
</Callout>

### Common usage

```bash
# Run one autoresearch iteration on a specific code seam:
harnesscli improve -target "internal/harness.Runner.SubmitInput"

# Run three iterations with a 30-second pause between each:
harnesscli improve -iterations 3 -pause 30 \
  -target "internal/harness.Runner.SubmitInput"

# Preview the plan without executing:
harnesscli improve -dry-run -target "internal/workflow"

# Run the score suite (tests + race + regression) and exit:
harnesscli improve -score-only
```

### Flags

| Flag | Default | Description |
|---|---|---|
| `-target` | (repeatable) | Target seam to inspect; may be repeated |
| `-dry-run` | `false` | Print the planned command without running it |
| `-score-only` | `false` | Run the score suite and exit |
| `-iterations` | `"1"` | Number of autoresearch loop iterations |
| `-pause` | `"0"` | Seconds to pause between iterations |
| `-report-dir` | `.tmp/autoresearch` | Directory for autoresearch reports |
| `-base-url` | `http://localhost:8080` | Harness API base URL |
| `-profile` | `"full"` | Run profile sent to `harnessd` |
| `-prompt-profile` | `"autoresearch"` | Prompt routing profile |
| `-model` | `""` | Optional model override |
| `-max-steps` | `"50"` | Step budget passed to autoresearch runs |
| `-test-cmd` | `./scripts/test-regression.sh` | Default validation command for unknown targets |

The `-test-cmd` value is passed to the loop script via the `HARNESS_AUTORESEARCH_DEFAULT_TEST_CMD` environment variable.

### -score-only in detail

`-score-only` runs these three commands in order and exits on the first failure:

```bash
go test ./...
go test ./... -race
./scripts/test-regression.sh
```

This is a fast sanity check that the tree is green before kicking off a longer autoresearch loop.

---

## HTTP endpoints used by the CLI

For reference, here are all the server routes that `harnesscli` calls:

| Method | Path | Used by |
|---|---|---|
| `POST` | `/v1/runs` | default run mode |
| `GET` | `/v1/runs/{id}/events` | streaming (run + continue) |
| `GET` | `/v1/runs` | list, search |
| `POST` | `/v1/runs/{id}/cancel` | cancel |
| `GET` | `/v1/runs/{id}` | status / show |
| `POST` | `/v1/runs/{id}/continue` | continue |
| `POST` | `/v1/runs/replay` | replay |
| `GET` | `/v1/profiles` | -list-profiles |
| `GET` | `/v1/runs/{id}/input` | `input` (reads pending questions) |
| `POST` | `/v1/runs/{id}/input` | `input` (posts answers) |
| `POST` | `/v1/runs/{id}/steer` | steer |
| `POST` | `/v1/runs/{id}/approve` | approve (TUI) |
| `POST` | `/v1/runs/{id}/deny` | deny (TUI) |

---

## Quick reference

<Tabs defaultValue="run">
  <TabsList>
    <TabsTrigger value="run">Run a prompt</TabsTrigger>
    <TabsTrigger value="manage">Manage runs</TabsTrigger>
    <TabsTrigger value="replay">Replay</TabsTrigger>
    <TabsTrigger value="auth">Auth</TabsTrigger>
  </TabsList>
  <TabsContent value="run">

```bash
# Key-free smoke run (fake provider):
HARNESS_PROVIDER=fake \
HARNESS_FAKE_TURNS=turns.json \
HARNESS_AUTH_DISABLED=true \
go run ./cmd/harnessd &

harnesscli -prompt "Summarize the diff"
```

  </TabsContent>
  <TabsContent value="manage">

```bash
# List all running:
harnesscli list -status running

# Check one run:
harnesscli status <run-id>

# Cancel:
harnesscli cancel <run-id>

# Continue a conversation:
harnesscli continue <run-id> "Now add tests"

# Search by keyword:
harnesscli search "authentication"
```

  </TabsContent>
  <TabsContent value="replay">

```bash
# Simulate replay:
harnesscli replay <run-id>

# Fork at step 3:
harnesscli replay -mode fork -fork-step 3 <run-id>
```

  </TabsContent>
  <TabsContent value="auth">

```bash
# Generate and save a local API key:
harnesscli auth login

# Point at a non-default server:
harnesscli auth login -server http://myserver:9090 -tenant myteam
```

  </TabsContent>
</Tabs>

---

## Next steps

- **Server setup** — See [harnessd Reference](/docs/server/harnessd) to learn how to configure and start the server `harnesscli` connects to.
- **Exit codes** — See [Exit Codes](/docs/reference/exit-codes) for the headless exit-code contract used when scripting runs from shell or CI.
- **Event reference** — See [Events](/docs/concepts/events) for the full list of SSE event types and their payloads.
- **Rollout & replay** — See [Rollout & Replay](/docs/operations/rollout-replay-forensics) for the underlying replay, fork, and drift-detection semantics.
- **Configuration** — See [Environment Variables](/docs/reference/environment-variables) for `HARNESS_PROVIDER`, `HARNESS_FAKE_TURNS`, and the full `HARNESS_*` env var reference.
