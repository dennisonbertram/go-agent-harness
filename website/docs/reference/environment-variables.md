---
title: "Environment Variable Reference"
sidebar_label: "Environment Variables"
sidebar_position: 4
---

import { Callout } from '@site/src/components/ui';

Environment variables are the primary way to configure `harnessd` and its companion programs without editing files. They form layer 5 of the [6-layer configuration cascade](/docs/reference/cli-flags) — higher priority than any TOML file and lower priority than future cloud/team constraints (not yet implemented). This page lists every recognized variable, its default, and its effect.

**When to use env vars instead of TOML:** use env vars for secrets (API keys), CI/CD pipelines, per-run overrides, and anything you do not want committed to a config file. Use TOML for settings you want shared and version-controlled.

<Callout type="warning">
Invalid env var values (wrong type, for example `HARNESS_MAX_STEPS=notanumber`) are silently ignored — the previous layer's value is kept with no warning or error message. Double-check spelling and type when a setting does not take effect.
</Callout>

---

## Core and provider keys

These variables cover the fundamental harness runtime: address, model, cost limits, workspace root, system prompt, and all provider API keys.

### Runtime behavior

| Variable | TOML equivalent | Default | Description |
|---|---|---|---|
| `HARNESS_MODEL` | `model` | `"gpt-4.1-mini"` | Default LLM model identifier for every run. Any model ID or alias from the catalog is valid. |
| `HARNESS_ADDR` | `addr` | `"127.0.0.1:8080"` | TCP listen address for the HTTP server (socket form, e.g. `":9090"`, not a URL). A non-loopback bind is refused at startup unless authentication is configured or `HARNESS_AUTH_DISABLED=true` is set deliberately. |
| `HARNESS_MAX_STEPS` | `max_steps` | `0` (unlimited) | Max tool-calling steps per run. `0` means unlimited — there is no default step cap. |
| `HARNESS_MAX_COST_PER_RUN_USD` | `cost.max_per_run_usd` | `0.0` (unlimited) | Per-run cost ceiling in USD. `0` means no limit. |
| `HARNESS_WORKSPACE` | — | `"."` | Workspace root directory. Determines where project TOML config and database files are found. |
| `HARNESS_SYSTEM_PROMPT` | — | Built-in coding assistant prompt | Override the default system prompt text for all runs. |
| `HARNESS_DEFAULT_AGENT_INTENT` | — | `""` | Default prompt overlay when no per-run intent is given. Empty means base prompt only. Set `autonomous` for headless/benchmark runs; `code_review`/`frontend_design` are task overlays. |
| `HARNESS_PROMPTS_DIR` | — | Auto-detected (`prompts/catalog.yaml` walk from cwd) | Directory for the file-based system prompt engine. |
| `HARNESS_TOOL_APPROVAL_MODE` | — | `"full_auto"` | Tool approval mode. Valid values: `"full_auto"` or `"permissions"`. |
| `HARNESS_ASK_USER_TIMEOUT_SECONDS` | — | `300` | How long (in seconds) `harnessd` waits at an `ask_user` checkpoint before timing out. |
| `HARNESS_SSE_KEEPALIVE_SECONDS` | — | `15` | Interval between SSE keepalive pings on event streams. |
| `HARNESS_AUTH_DISABLED` | — | — | Set to `"true"` to disable Bearer token authentication entirely. Implied when `HARNESS_RUN_DB` is not set (no key store exists). |

<Callout type="info">
`HARNESS_MAX_STEPS=0` (the default) means "unlimited" — there is no daemon-level fallback that raises it to a non-zero cap. Set an explicit positive value to cap tool-calling steps per run.
</Callout>

### Fake provider (key-free testing)

| Variable | Default | Description |
|---|---|---|
| `HARNESS_PROVIDER` | — | Set to `"fake"` to use the deterministic fake provider. No API key needed. Requires `HARNESS_FAKE_TURNS`. |
| `HARNESS_FAKE_TURNS` | — | Path to a JSON turns file used when `HARNESS_PROVIDER=fake`. |

The fake provider is the recommended path for CI smoke tests. See the [benchmark smoke runbook](/docs/operations/training-and-benchmarks) for full details.

### Provider API keys

Each provider in the model catalog declares an `api_key_env` field. The table below lists every currently registered provider and the variable `harnessd` reads to authenticate with it.

| Provider | Key variable | Base URL |
|---|---|---|
| `openai` | `OPENAI_API_KEY` | `https://api.openai.com/v1` |
| `anthropic` | `ANTHROPIC_API_KEY` | `https://api.anthropic.com/v1` |
| `deepseek` | `DEEPSEEK_API_KEY` | `https://api.deepseek.com/v1` |
| `groq` | `GROQ_API_KEY` | `https://api.groq.com/openai/v1` |
| `cerebras` | `CEREBRAS_API_KEY` | `https://api.cerebras.ai/v1` |
| `xai` | `XAI_API_KEY` | `https://api.x.ai/v1` |
| `kimi` | `MOONSHOT_API_KEY` | `https://api.moonshot.ai/v1` |
| `kimi-subscription` | (none — vendor-CLI session import) | `https://api.kimi.com/coding/v1` |
| `qwen` | `DASHSCOPE_API_KEY` | `https://dashscope-intl.aliyuncs.com/compatible-mode/v1` |
| `together` | `TOGETHER_API_KEY` | `https://api.together.xyz/v1` |
| `codex-subscription` | (none — vendor-CLI session import) | `https://chatgpt.com/backend-api/codex` |
| `openrouter` | `OPENROUTER_API_KEY` | `https://openrouter.ai/api/v1` |
| `gemini` | `GOOGLE_API_KEY` | `https://generativelanguage.googleapis.com/v1beta/openai` |
| `ollama` | (none — local server) | `http://localhost:11434/v1` |
| `lmstudio` | (none — local server) | `http://localhost:1234/v1` |

That is 15 providers total (`catalog/models.json`). Only one key is required at startup: whichever matches the default model. Additional keys are loaded on demand when a run requests a different provider. `kimi-subscription` and `codex-subscription` authenticate via `harnesscli auth kimi login` / `auth codex login` (imported vendor-CLI credentials) instead of an API key env var — see [harnesscli Reference](/docs/cli/harnesscli#auth-kimi).

`OPENAI_API_KEY` also serves as the legacy bootstrap path: if no catalog-matched provider is configured, `harnessd` falls back to a bare OpenAI client if this variable is set.

### OpenAI base URL override

| Variable | Default | Description |
|---|---|---|
| `OPENAI_BASE_URL` | — | Override the base URL for the default OpenAI-compatible provider. Useful for proxies or OpenAI-compatible local servers. |

### OpenRouter headers

| Variable | Default | Description |
|---|---|---|
| `HARNESS_OPENROUTER_REFERER` | `"https://github.com/dennisonbertram/go-agent-harness"` | Value sent as the HTTP `Referer` header on OpenRouter requests. |
| `HARNESS_OPENROUTER_TITLE` | `"go-agent-harness"` | Value sent as the `X-Title` header on OpenRouter requests. |

### Retry, process lifecycle, and misc

| Variable | Default | Description |
|---|---|---|
| `HARNESS_RETRY_MAX_ATTEMPTS` | `3` | Maximum provider-call retry attempts (`internal/provider/retry.go:41-44`). Useful against providers with aggressive rate limiting, e.g. free tiers. |
| `HARNESS_RETRY_MAX_TOTAL_SEC` | `60` | Maximum total time (seconds) spent retrying a single provider call (`internal/provider/retry.go:46-49`). |
| `HARNESS_LISTEN_FD` | — | File descriptor number to adopt as the HTTP listener instead of binding a new socket. Used for zero-downtime restart handoff (`cmd/harnessd/main.go:1116`). |
| `HARNESS_EXIT_WITH_PARENT` | `false` | When `true`, `harnessd` watches its parent process and exits when the parent exits (`cmd/harnessd/parent_watchdog.go:76-85`). |
| `HARNESS_MODEL_STORE_PATH` | `modelstore.DefaultPath()` | Override the file path for the model-settings store (`cmd/harnessd/model_settings.go:104-107`). |
| `HARNESS_PROVIDER_CATALOG_DIR` | Auto-detected | Override the directory containing per-provider catalog JSON files (`cmd/harnessd/bootstrap_helpers.go:733`). |
| `HARNESS_COLOR_PROFILE` | `"auto"` | TUI color profile override: `truecolor`, `256`, `ansi`, or `none` (`cmd/harnesscli/main.go:526-528`). |
| `HARNESS_CRON_API_KEY` | — | API key `harnessd`'s embedded cron dispatch uses when calling back into itself (`cmd/harnessd/main.go:550`). |
| `HARNESS_SOURCE_ROOT` | Auto-detected | Override the repository root the Go workflow engine resolves source files against (`internal/workflow/source.go:862`). |
| `HARNESS_BENCHMARK_CMD` | `./scripts/test-regression.sh` | Override the regression command run by the training/scoring loop (`internal/training/regression.go:145`). |

---

## Memory, cron, and persistence

### Observational memory

Observational memory watches the conversation transcript and periodically extracts durable observations — facts, preferences, and constraints — that persist across runs.

| Variable | TOML equivalent | Default | Description |
|---|---|---|---|
| `HARNESS_MEMORY_MODE` | `memory.mode` | `"auto"` | Memory mode. Values: `"auto"` (resolves to `"local_coordinator"`), `"off"`, `"local_coordinator"`. |
| `HARNESS_MEMORY_DB_DRIVER` | `memory.db_driver` | `"sqlite"` | Persistence backend. Values: `"sqlite"` or `"postgres"`. |
| `HARNESS_MEMORY_DB_DSN` | `memory.db_dsn` | — | Postgres connection string. Only used when `HARNESS_MEMORY_DB_DRIVER=postgres`. |
| `HARNESS_MEMORY_SQLITE_PATH` | `memory.sqlite_path` | `".harness/state.db"` | SQLite file path relative to the workspace root. Also used by working memory and workflow stores. |
| `HARNESS_MEMORY_DEFAULT_ENABLED` | `memory.default_enabled` | `false` | Whether new conversation scopes start with memory enabled. |
| `HARNESS_MEMORY_OBSERVE_MIN_TOKENS` | `memory.observe_min_tokens` | `1200` | Minimum accumulated token count before the observer runs. |
| `HARNESS_MEMORY_SNIPPET_MAX_TOKENS` | `memory.snippet_max_tokens` | `900` | Maximum token budget for the memory snippet injected into each turn. |
| `HARNESS_MEMORY_REFLECT_THRESHOLD_TOKENS` | `memory.reflect_threshold_tokens` | `4000` | Accumulated observation token count that triggers reflection compaction. |
| `HARNESS_MEMORY_LLM_MODE` | `memory.llm_mode` | Auto-detected | LLM backend for the observer/reflector. Values: `"inherit"`, `"openai"`, `"provider"`. Auto-detection: use `"provider"` if `HARNESS_MEMORY_LLM_PROVIDER` is set; else `"openai"` if `OPENAI_API_KEY` is set; else `"inherit"`. |
| `HARNESS_MEMORY_LLM_PROVIDER` | `memory.llm_provider` | — | Named provider (e.g. `"openrouter"`, `"anthropic"`) when `HARNESS_MEMORY_LLM_MODE=provider`. |
| `HARNESS_MEMORY_LLM_MODEL` | `memory.llm_model` | `"gpt-5-nano"` (openai mode) | Model used for observer and reflector LLM calls. |
| `HARNESS_MEMORY_LLM_BASE_URL` | `memory.llm_base_url` | Falls back to `OPENAI_BASE_URL` | Base URL override for the memory LLM. |
| `HARNESS_MEMORY_LLM_API_KEY` | — | Falls back to `OPENAI_API_KEY` | API key for the memory LLM. Not persisted in TOML (treated as a secret). |

<Callout type="warning">
`HARNESS_MEMORY_DB_DRIVER=postgres` selects the Postgres store struct, but every method returns `"postgres store is not implemented in v1"`. Postgres memory persistence is not functional in the current release; select this value only if you are prepared to handle the resulting error.
</Callout>

<Callout type="info">
`HARNESS_MEMORY_LLM_API_KEY` is read directly in `cmd/harnessd/main.go` and is not part of the `applyEnvLayer` TOML-override function. It has no TOML equivalent by design — secrets should not be written to config files.
</Callout>

### Cron scheduler

The cron scheduler runs recurring agent tasks. By default, `harnessd` starts an embedded scheduler backed by a SQLite file at `<HARNESS_WORKSPACE>/.harness/cron.db`. Set `HARNESS_CRON_URL` to delegate to an external `cronsd` instance instead.

| Variable | TOML equivalent | Default | Description |
|---|---|---|---|
| `HARNESS_CRON_URL` | — | — | HTTP base URL of an external `cronsd` service. When empty, the embedded scheduler is used. |
| `HARNESS_CRON_JITTER_ENABLED` | `cron.jitter_enabled` | `true` | Enable random jitter on cron job fire times to prevent thundering-herd effects. |
| `HARNESS_CRON_JITTER_MIN_SEC` | `cron.jitter_min_sec` | `60` | Minimum jitter delay in seconds. |
| `HARNESS_CRON_JITTER_MAX_SEC` | `cron.jitter_max_sec` | `300` | Maximum jitter delay in seconds. |

<Callout type="info">
`cron.avoid_minute_marks` (default `[0, 30]`) and `cron.log_jittered_times` (default `true`) are TOML-only settings. No env vars exist for them.
</Callout>

### Persistence (run records, conversations, relay)

These variables enable optional SQLite persistence for run records, conversation history, and relay worker state. All three are disabled by default (empty string). When a variable is set, `harnessd` opens or creates the SQLite file at that path and enables the corresponding feature.

| Variable | Default | Description |
|---|---|---|
| `HARNESS_RUN_DB` | — (disabled) | SQLite path for run, event, and message records. Enables `GET /v1/runs` and Bearer token auth. |
| `HARNESS_CONVERSATION_DB` | — (disabled) | SQLite path for conversation records. |
| `HARNESS_CONVERSATION_RETENTION_DAYS` | `30` | Days to retain conversation records. Requires `HARNESS_CONVERSATION_DB`. |
| `HARNESS_RELAY_DB` | — (disabled) | SQLite path for relay worker persistence. Required for `GET /v1/relay/workers`. |

<Callout type="warning">
Authentication (`Authorization: Bearer` token checking) is implicitly disabled when `HARNESS_RUN_DB` is not set, because there is no key store to validate against. This is true even without setting `HARNESS_AUTH_DISABLED=true`.
</Callout>

### S3 backup

S3 backup is silently disabled unless **all four required variables** are present.

| Variable | Required | Default | Description |
|---|---|---|---|
| `AWS_ACCESS_KEY_ID` | Yes | — | AWS access key ID. |
| `AWS_SECRET_ACCESS_KEY` | Yes | — | AWS secret access key. |
| `AWS_REGION` | Yes | — | AWS region (e.g. `"us-east-1"`). |
| `S3_BUCKET` | Yes | — | Target S3 bucket name. |
| `S3_KEY_PREFIX` | No | `""` | Optional path prefix prepended to every S3 object key. |

---

## Webhooks, MCP, workspaces, and forensics

### Webhook secrets

Setting a webhook secret enables the corresponding `POST /v1/webhooks/<source>` route. Each route authenticates via HMAC signature, not Bearer tokens.

| Variable | Default | Effect |
|---|---|---|
| `GITHUB_WEBHOOK_SECRET` | — | Enables `POST /v1/webhooks/github`. Validates HMAC-SHA256 `X-Hub-Signature-256` headers. |
| `SLACK_SIGNING_SECRET` | — | Enables `POST /v1/webhooks/slack`. Validates Slack's `v0=<hex>` HMAC-SHA256 signature. |
| `LINEAR_WEBHOOK_SECRET` | — | Enables `POST /v1/webhooks/linear`. Validates HMAC-SHA256 signature. |

When a variable is absent, the corresponding endpoint returns `401`.

### MCP servers

| Variable | Default | Description |
|---|---|---|
| `HARNESS_MCP_SERVERS` | — | JSON array of MCP server configs merged additively with TOML-configured servers. |

`HARNESS_MCP_SERVERS` format — each element is a JSON object:

```json
[
  {"name": "my-tool", "transport": "stdio", "command": "/usr/local/bin/server", "args": ["--flag"]},
  {"name": "remote", "transport": "http", "url": "http://localhost:3001/mcp"}
]
```

`transport` is inferred if omitted: a `command`-only entry is `stdio`, a `url`-only entry is `http`. When both sources define a server with the same name, the TOML entry takes precedence.

### Workspaces and subagents

| Variable | Default | Description |
|---|---|---|
| `HARNESS_SUBAGENT_BASE_REF` | `"HEAD"` | Base git ref used when provisioning per-run git worktrees for subagents. |
| `HARNESS_SUBAGENT_WORKTREE_ROOT` | — | Override where per-run worktrees are materialized. When empty, defaults to a `<repo>-subagents` sibling directory. |
| `HARNESS_GO_WORKFLOW_CACHE_DIR` | `<workspace>/.harness/workflow-cache` | Cache directory for compiled Go workflow scripts. |
| `HARNESS_URL` | — | Override the harness HTTP URL at workspace provision time (used by the `local` and `worktree` workspace backends). |
| `HARNESS_IMAGE` | `"go-agent-harness:latest"` | Docker image override at provision time for the `container` workspace backend. |
| `HARNESS_DOWNLOAD_URL` | — | URL to download the `harnessd` binary in the VM bootstrap cloud-init script. When empty, the script expects the binary at `/usr/local/bin/harnessd`. |
| `HETZNER_API_KEY` | — | Hetzner Cloud API key. Required when using the `vm` workspace backend. |

### Model catalog and pricing

| Variable | Default | Description |
|---|---|---|
| `HARNESS_MODEL_CATALOG_PATH` | Auto-detected (`catalog/models.json` walk near workspace) | Path to the JSON model catalog file. |
| `HARNESS_PRICING_CATALOG_PATH` | — | Path to a separate JSON pricing file. When empty, pricing is read from inline blocks in the model catalog. |

### Content directories

| Variable | Default | Description |
|---|---|---|
| `HARNESS_GLOBAL_DIR` | `~/.go-harness` | Global directory for skills, workflows, and script tools. |
| `HARNESS_SKILLS_DIR` | `$HARNESS_GLOBAL_DIR/skills` | Absolute override for the global skill root used consistently by loading, authoring, verification, hot reload, and Go workflow skill-bundle discovery. Relative values fail daemon startup. |
| `HARNESS_RECIPES_DIR` | — | Directory for recipe definitions. |
| `HARNESS_WORKFLOWS_DIR` | — | Directory for YAML workflow definitions. |
| `HARNESS_NETWORKS_DIR` | — | Directory for network (agent graph) definitions. |
| `HARNESS_ROLLOUT_DIR` | — | Root directory for JSONL rollout recording files. Overrides `forensics.rollout_dir` in TOML. |

### Hot-reload and skills

| Variable | Default | Description |
|---|---|---|
| `HARNESS_SKILLS_ENABLED` | `true` | Enable the skills system. |
| `HARNESS_WATCH_ENABLED` | `true` | Enable poll-based hot-reload for skills and workflows. |
| `HARNESS_WATCH_INTERVAL_SECONDS` | `5` | Poll interval in seconds for the hot-reload watcher. |

### Role models

| Variable | Default | Description |
|---|---|---|
| `HARNESS_ROLE_MODEL_PRIMARY` | — | Override the model used for primary role-model tasks. |
| `HARNESS_ROLE_MODEL_SUMMARIZER` | — | Override the model used for message summarization. |

### Callbacks and Sourcegraph

| Variable | Default | Description |
|---|---|---|
| `HARNESS_ENABLE_CALLBACKS` | `true` | Enable the delayed callback manager. |
| `HARNESS_SOURCEGRAPH_ENDPOINT` | — | Sourcegraph code search endpoint URL. |
| `HARNESS_SOURCEGRAPH_TOKEN` | — | Sourcegraph API token. |

### Conclusion watcher

The conclusion watcher detects when the agent jumps to a conclusion prematurely.

| Variable | TOML equivalent | Default | Description |
|---|---|---|---|
| `HARNESS_CONCLUSION_WATCHER_ENABLED` | `conclusion_watcher.enabled` | `false` | Enable the conclusion-jumping detector. |
| `HARNESS_CONCLUSION_WATCHER_INTERVENTION_MODE` | `conclusion_watcher.intervention_mode` | `"inject_validation_prompt"` | How to intervene. Values: `"inject_validation_prompt"`, `"pause_for_user"`, `"request_critique"`. |
| `HARNESS_CONCLUSION_WATCHER_EVALUATOR_ENABLED` | `conclusion_watcher.evaluator_enabled` | `false` | Enable an LLM evaluator alongside phrase-match detection. |
| `HARNESS_CONCLUSION_WATCHER_EVALUATOR_MODEL` | `conclusion_watcher.evaluator_model` | `"gpt-4o-mini"` | Model used by the LLM evaluator. |

<Callout type="info">
`conclusion_watcher.evaluator_api_key` exists in TOML config but has no dedicated env var override. It falls back to `OPENAI_API_KEY` in the harnessd startup code.
</Callout>

---

## Subprojects

### `cronsd` — standalone cron daemon

`cronsd` is a self-contained cron daemon that can run independently of `harnessd`. See [Cron Scheduling](/docs/integrations/cron-scheduling) for details.

| Variable | Default | Description |
|---|---|---|
| `CRONSD_ADDR` | `":9090"` | TCP listen address for `cronsd`. |
| `CRONSD_DB_PATH` | `~/.go-harness/cronsd.db` | SQLite database file path. |
| `CRONSD_MAX_CONCURRENT` | `5` | Maximum number of job executions that may run simultaneously. |
| `CRONSD_INGRESS_API_KEY` | — | API key required on inbound requests to `cronsd` itself. |
| `CRONSD_INGRESS_TENANT_ID` | — | Tenant ID associated with `CRONSD_INGRESS_API_KEY`. |
| `CRONSD_HARNESS_URL` | — | Base URL of the `harnessd` instance `cronsd` dispatches jobs to. |
| `CRONSD_HARNESS_API_KEY` | — | API key `cronsd` uses when calling `harnessd`. Must differ from `CRONSD_INGRESS_API_KEY`. |
| `CRONSD_HARNESS_CONNECT_TIMEOUT` | `5s` | Connect timeout for `cronsd` → `harnessd` requests. |
| `CRONSD_HARNESS_REQUEST_TIMEOUT` | `15s` | Overall request timeout for `cronsd` → `harnessd` requests. |

`cronctl` (the CLI client for `cronsd`) reads one variable:

| Variable | Default | Description |
|---|---|---|
| `CRONSD_URL` | `"http://localhost:9090"` | Base URL of the `cronsd` service. |

### `symphd` — GitHub issue orchestrator

`symphd` polls a GitHub repository for issues matching a label, provisions workspaces, and dispatches agent runs. Config is primarily YAML-file-driven, but one env var provides a fallback for the GitHub token:

| Variable | YAML key | Default | Description |
|---|---|---|---|
| `GITHUB_TOKEN` | `github_token` | — | GitHub API token. Used when `github_token` is not set in the `symphd` YAML config file. |

`symphd` also propagates `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, and `HARNESS_MODEL` from its own environment into each provisioned workspace (container env vars, never written to disk).

### `trainerd` — rollout analyzer

`trainerd` scores and analyzes JSONL rollout traces captured by `harnessd`. It reads one env var:

| Variable | Default | Description |
|---|---|---|
| `HARNESS_ROLLOUT_DIR` | `~/.trainerd/rollouts` | Default rollout directory. The `--db-path` flag controls `trainerd`'s own SQLite store; `ANTHROPIC_API_KEY` is required for LLM-backed analysis. |
| `ANTHROPIC_API_KEY` | — | Required for `trainerd analyze` and `trainerd loop` commands that call Claude for deeper rollout analysis. |

### `socialagent` — example Telegram app

`apps/socialagent` is a complete example application that drives `harnessd` over HTTP. It has its own set of env vars:

| Variable | Required | Default | Description |
|---|---|---|---|
| `TELEGRAM_BOT_TOKEN` | Yes | — | Telegram Bot API token from @BotFather. |
| `TELEGRAM_WEBHOOK_SECRET` | Yes | — | Secret token for `X-Telegram-Bot-Api-Secret-Token` header validation. |
| `DATABASE_URL` | Yes | — | Postgres connection string. |
| `OPENAI_API_KEY` | Yes | — | Passed through to `harnessd`; not used by `socialagent` directly. |
| `HARNESS_URL` | No | `"http://localhost:8080"` | Base URL of the running `harnessd` instance. |
| `LISTEN_ADDR` | No | `":8081"` | TCP address for `socialagent`'s own HTTP server. |
| `MCP_SERVER_URL` | No | `"http://localhost:8082/mcp"` | URL of `socialagent`'s embedded MCP server, passed to each `harnessd` run. |
| `SAFETY_SCREENER_URL` | No | — (disabled) | Llama Guard-compatible safety screening endpoint. Empty disables screening. |
| `SOCIALAGENT_SYSTEM_PROMPT` | No | Built-in personality | System prompt override. |

---

## TOML-only settings (no env override)

Some settings can only be configured via TOML; no corresponding env var exists.

| TOML key | Section | Description |
|---|---|---|
| `cron.avoid_minute_marks` | `[cron]` | List of minute values (e.g. `[0, 30]`) that jitter avoids landing on. |
| `cron.log_jittered_times` | `[cron]` | Whether to log the jittered fire time for each job. |
| `auto_compact.*` | `[auto_compact]` | Context auto-compact settings (mode, threshold, keep_last, model_context_window). |
| `forensics.*` | `[forensics]` | Trace, audit, and forensics flags. |
| `mcp_servers.*` | `[mcp_servers]` | Named MCP server entries (merged with `HARNESS_MCP_SERVERS`; TOML wins on name conflicts). |
| `conclusion_watcher.evaluator_api_key` | `[conclusion_watcher]` | API key for the evaluator LLM; falls back to `OPENAI_API_KEY`. |

---

## Quick-reference: where settings apply

<Callout type="info">
The following variables are read directly in `cmd/harnessd/main.go` rather than through the `applyEnvLayer` TOML-override function: `HARNESS_WORKSPACE`, `HARNESS_SYSTEM_PROMPT`, `HARNESS_DEFAULT_AGENT_INTENT`, `HARNESS_PROMPTS_DIR`, `HARNESS_ASK_USER_TIMEOUT_SECONDS`, `HARNESS_TOOL_APPROVAL_MODE`, `HARNESS_SKILLS_ENABLED`, `HARNESS_WATCH_ENABLED`, `HARNESS_WATCH_INTERVAL_SECONDS`, `HARNESS_MEMORY_LLM_API_KEY`, `HARNESS_MEMORY_LLM_BASE_URL`, and others in the runtime bootstrap section. They behave like env var overrides but are not part of the serialized `Config` struct.
</Callout>

---

## Next steps

- For the full TOML schema and config cascade details, see the [Configuration guide](/docs/concepts/configuration).
- For CLI flags on `harnessd`, `harnesscli`, `cronsd`, and `cronctl`, see [CLI Flags](/docs/reference/cli-flags).
- For provider model IDs and aliases, see [Providers and Models](/docs/reference/providers-and-models-reference).
