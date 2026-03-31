# socialagent

A Telegram-facing social agent that delegates work to `harnessd` over HTTP. Users interact with it through a Telegram bot; the agent forwards each message as a run request to the harness, streams the response back to the user, and persists conversation state in Postgres. It intentionally has **no imports of `internal/` packages** — all harness interaction goes through the public `harnessd` REST API.

## Prerequisites

- **Docker** — for the local Postgres instance (Docker Desktop: https://www.docker.com/products/docker-desktop)
- **Go 1.22+** — to build the binaries
- **Telegram bot token** — from [@BotFather](https://core.telegram.org/bots#botfather)
- **OpenAI API key** — used by `harnessd` for model calls

## Quick start

```bash
# 1. One-time setup: start Postgres, create .env, build binaries
apps/socialagent/scripts/setup.sh

# 2. Fill in your credentials
#    Required: TELEGRAM_BOT_TOKEN
#    Required: OPENAI_API_KEY (auto-filled if set in your shell env)
$EDITOR apps/socialagent/.env

# 3. Start both services (harnessd in background, socialagent in foreground)
apps/socialagent/scripts/dev.sh
```

Press `Ctrl-C` to stop. `harnessd` is automatically shut down when `dev.sh` exits.

## Architecture

```
Telegram API
    |
    | HTTPS webhook (POST /webhook/telegram)
    v
socialagent  (:8081)
    |
    | HTTP REST  (POST /api/v1/runs, GET /api/v1/runs/:id/events)
    v
harnessd     (:8080)
    |
    | OpenAI API calls
    v
OpenAI

socialagent also reads/writes:
    Postgres (:5433)  -- conversation history, user sessions
```

## Environment variables

### Required

| Variable                  | Description                                                  |
|---------------------------|--------------------------------------------------------------|
| `TELEGRAM_BOT_TOKEN`      | Bot token issued by @BotFather.                              |
| `TELEGRAM_WEBHOOK_SECRET` | Secret for verifying incoming Telegram webhook requests.     |
| `DATABASE_URL`            | Postgres connection string.                                  |
| `OPENAI_API_KEY`          | OpenAI API key (used by harnessd).                           |

### Optional

| Variable                    | Default                    | Description                                      |
|-----------------------------|----------------------------|--------------------------------------------------|
| `HARNESS_URL`               | `http://localhost:8080`    | Base URL of the harnessd HTTP API.               |
| `LISTEN_ADDR`               | `:8081`                    | TCP address for socialagent's own HTTP server.   |
| `SOCIALAGENT_SYSTEM_PROMPT` | Built-in personality       | System prompt injected into every run.           |

See `.env.example` for the full template with comments.

## Scripts

| Script              | Purpose                                                         |
|---------------------|-----------------------------------------------------------------|
| `scripts/setup.sh`  | One-time setup: start Postgres, create `.env`, build binaries. Idempotent — safe to re-run. |
| `scripts/dev.sh`    | Start both services for local development.                      |
| `scripts/teardown.sh` | Stop services. Use `--clean` to remove container and data.    |

## Worktree usage

When working in a git worktree (e.g. via `scripts/init.sh`), `dev.sh` will automatically find the `.env` and Postgres container created in the main worktree. The Postgres container runs at the machine level, not per-worktree, so you do not need to run `setup.sh` again in each new worktree.

```bash
# In a worktree — no setup needed if main worktree is already set up
.claude/worktrees/my-branch/apps/socialagent/scripts/dev.sh
```

## Teardown

```bash
# Stop services, keep Postgres data and binaries
apps/socialagent/scripts/teardown.sh

# Full clean: remove Postgres container (deletes all data) and .tmp/ directory
apps/socialagent/scripts/teardown.sh --clean
```

## Important notes

- `apps/socialagent/.env` — **never commit this file**; it contains secrets.
- `apps/socialagent/.tmp/` — built binaries and runtime data; gitignored.
- Postgres runs on port **5433** (not 5432) to avoid conflicts with any locally installed Postgres instance.

## Development

Run the config tests:

```bash
go test ./apps/socialagent/config/...
```
