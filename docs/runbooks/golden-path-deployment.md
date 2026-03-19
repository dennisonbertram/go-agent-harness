# Golden Path Deployment

## Supported Profile: `full`

The `full` profile is the canonical deployment baseline for go-agent-harness.
It enables all available tools with sensible defaults and is the reference
configuration for integration testing and production deployments.

Profile source: `internal/profiles/builtins/full.toml`

---

## Prerequisites

| Requirement | Details |
|-------------|---------|
| Go | 1.25+ |
| Provider key | At least one of `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, or `GEMINI_API_KEY` |
| `curl` | Required only for the smoke test |
| `python3` | Required only for the smoke test (JSON parsing) |

---

## Quick Start

### 1. Build

```bash
go build -o harnessd ./cmd/harnessd
```

### 2. Set provider credentials

```bash
export OPENAI_API_KEY="sk-..."
# or
export ANTHROPIC_API_KEY="sk-ant-..."
# or
export GEMINI_API_KEY="..."
```

### 3. Start the server

```bash
./harnessd --profile full
```

The server listens on `:8080` by default.

---

## Configuration

### Profile defaults (`full` profile)

| Field | Default | Description |
|-------|---------|-------------|
| `model` | `gpt-4.1-mini` | Default model for runs that do not specify one |
| `max_steps` | `30` | Maximum tool-use steps per run |
| `max_cost_usd` | `2.00` | Per-run cost ceiling in USD |
| `tools.allow` | `[]` (all) | Empty allow list means all tools are available |

### Environment variable overrides

All `HARNESS_*` environment variables override the profile at startup.
They take the highest precedence in the config layering order.

| Variable | Default | Description |
|----------|---------|-------------|
| `HARNESS_ADDR` | `:8080` | HTTP listen address (e.g. `:9090` or `127.0.0.1:8080`) |
| `HARNESS_MODEL` | profile value | Override the default model |
| `HARNESS_MAX_STEPS` | profile value | Override max steps per run |
| `HARNESS_MAX_COST_PER_RUN_USD` | profile value | Override per-run cost ceiling |
| `HARNESS_AUTH_DISABLED` | `false` | Set to `true` to bypass API key authentication |

### Authentication

When a persistent store is configured, the server requires `Authorization: Bearer <token>`
on all requests except `GET /healthz`.

For local development or smoke testing, disable auth:

```bash
HARNESS_AUTH_DISABLED=true ./harnessd --profile full
```

---

## Smoke Test

Run the smoke test to verify the server is operating correctly end-to-end:

```bash
# Build the binary first
go build -o harnessd ./cmd/harnessd

# Run the smoke test (requires a live provider key)
./scripts/smoke-test.sh
```

The smoke test:

1. Starts `harnessd` on a random ephemeral port (avoids conflicts)
2. Waits for `/healthz` to return 200
3. Verifies `GET /v1/providers` returns at least one provider
4. Verifies `GET /v1/models` returns at least one model
5. Creates a run with the prompt `Reply with exactly: SMOKE_TEST_PASS`
6. Polls until the run reaches `completed` status (120s timeout)
7. Streams `GET /v1/runs/{id}/events` and verifies at least one event arrived
8. Exits 0 on all-pass, non-zero on any failure

### Smoke test environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `HARNESS_BINARY` | `./harnessd` | Path to the binary under test |
| `HARNESS_PROFILE` | `full` | Profile to start the server with |
| `HARNESS_SMOKE_MODEL` | `gpt-4.1-mini` | Model used for the smoke run |
| `HARNESS_SMOKE_TIMEOUT` | `120` | Seconds to wait for run completion |
| `HARNESS_SMOKE_LOG` | `/tmp/harnessd-smoke.log` | Server log path |

---

## Key Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/healthz` | GET | Health check — returns `{"status":"ok"}` |
| `/v1/providers` | GET | List providers and their configured state |
| `/v1/models` | GET | List all models across all providers |
| `/v1/runs` | POST | Create a new run |
| `/v1/runs/{id}` | GET | Get run status and output |
| `/v1/runs/{id}/events` | GET | SSE stream of run events |

---

## Notes

- The smoke test is a **manual validation tool**. It is not part of the regression
  suite (`scripts/test-regression.sh`) because it requires live API credentials.
- To run unit and integration tests without network access, use:
  ```bash
  ./scripts/test-regression.sh
  ```
- The `full` profile does not restrict tools. To restrict available tools, use
  a custom profile or set `tools.allow` in a project-level `config.toml`.
