# socialagent

A Telegram-facing social agent that delegates work to `harnessd` over HTTP.

This application lives inside the `go-agent-harness` module but intentionally
has **no imports of `internal/` packages**. All harness interaction happens
through the public `harnessd` REST API (`/api/v1/runs`, etc.).

## What it does

- Accepts incoming Telegram messages via the Bot API.
- Forwards each message as a run request to `harnessd`.
- Streams the response back to the Telegram user.
- Exposes a small HTTP health/status endpoint for ops monitoring.

## How to run

```bash
export TELEGRAM_BOT_TOKEN=<your BotFather token>
export DATABASE_URL=postgres://user:pass@localhost/socialagent
export HARNESS_URL=http://localhost:8080   # optional, this is the default
export LISTEN_ADDR=:8081                   # optional, this is the default

go run ./apps/socialagent/
```

## Required environment variables

| Variable             | Description                                      |
|----------------------|--------------------------------------------------|
| `TELEGRAM_BOT_TOKEN` | Bot token issued by @BotFather. **Required.**    |
| `DATABASE_URL`       | Database connection string. **Required.**        |

## Optional environment variables

| Variable                    | Default                    | Description                          |
|-----------------------------|----------------------------|--------------------------------------|
| `HARNESS_URL`               | `http://localhost:8080`    | Base URL of the harnessd HTTP API.   |
| `LISTEN_ADDR`               | `:8081`                    | TCP address for the agent's own HTTP server. |
| `SOCIALAGENT_SYSTEM_PROMPT` | Built-in personality       | System prompt injected into every run. |

## Development

Run the config tests:

```bash
go test ./apps/socialagent/config/...
```
