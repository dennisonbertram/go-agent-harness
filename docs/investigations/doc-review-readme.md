# README.md Accuracy Review

**Date**: 2026-03-18
**Files compared**: `README.md` vs actual source in `internal/server/http.go`, `internal/harness/events.go`, `internal/harness/types.go`, `internal/harness/tools_default.go`, `internal/harness/tools/catalog.go`, `internal/config/config.go`, `cmd/harnessd/main.go`, `cmd/harnesscli/main.go`

---

## 1. Routes / Endpoints

### Accurate

The following routes listed in the README match `buildMux()` in `internal/server/http.go` and the sub-route dispatchers:

- `GET /healthz`
- `POST /v1/runs`, `GET /v1/runs`
- `GET /v1/runs/{id}`, `GET /v1/runs/{id}/events`
- `GET|POST /v1/runs/{id}/input`
- `GET /v1/runs/{id}/summary`
- `POST /v1/runs/{id}/continue`
- `POST /v1/runs/{id}/steer`
- `GET /v1/runs/{id}/context`
- `POST /v1/runs/{id}/compact`
- `GET|PUT /v1/runs/{id}/todos`
- `POST /v1/runs/replay`
- `GET /v1/conversations/`
- `GET /v1/conversations/search`
- `POST /v1/conversations/cleanup`
- `DELETE /v1/conversations/{id}`
- `GET /v1/conversations/{id}/messages`
- `GET /v1/conversations/{id}/runs`
- `GET /v1/conversations/{id}/export`
- `POST /v1/conversations/{id}/compact`
- `POST /v1/agents`
- `GET /v1/subagents`, `POST /v1/subagents`
- `GET /v1/subagents/{id}`, `DELETE /v1/subagents/{id}`
- `GET /v1/cron/jobs`, `POST /v1/cron/jobs`
- `GET /v1/cron/jobs/{id}`, `PATCH /v1/cron/jobs/{id}`, `DELETE /v1/cron/jobs/{id}`
- `POST /v1/cron/jobs/{id}/pause`, `POST /v1/cron/jobs/{id}/resume`
- `GET /v1/skills/`, `GET /v1/skills/{name}`
- `POST /v1/skills/{name}/verify`
- `GET /v1/recipes/`, `GET /v1/recipes/{name}`, `GET /v1/recipes/{name}/schema`
- `GET /v1/models`
- `GET /v1/providers`
- `GET /v1/search/code`
- `GET /v1/mcp/servers`

### Inaccurate / Missing

| Issue | README says | Code actually shows |
|-------|-----------|-------------------|
| **`GET /v1/conversations/{id}` does not exist** | README lists it (line 68) | `handleConversations` has no handler for `len(parts) == 1 && r.Method == GET` with a non-keyword ID. Only `DELETE` is handled for single-part paths. The route falls through to `http.NotFound`. |
| **`PUT /v1/providers/{name}/key` not listed** | Not mentioned | `handleProviderByName` handles `PUT /v1/providers/{name}/key` (line 242 of http.go). This endpoint was added in commit 279ed1c. |
| **`GET /v1/summarize` should be `POST /v1/summarize`** | Listed as `GET` (line 45) | `handleSummarize` rejects non-POST requests (line 282). |

### Summary

- 2 inaccurate entries (missing `PUT /v1/providers/{name}/key`, wrong method for `/v1/summarize`)
- 1 phantom route (`GET /v1/conversations/{id}` is listed but not implemented)

---

## 2. Tool Surface

### Accurate

The README's description is intentionally high-level and avoids listing individual tool names. The general categories it describes (core file/shell helpers, process helpers, clarification/memory, conversation helpers, optional integrations) are accurate.

### Details from Code

**Core tools** (always visible, from `tools_default.go`):
- `read`, `write`, `edit`, `bash`, `job_output`, `job_kill`, `apply_patch`
- `ask_user_question`, `observational_memory`, `file_inspect`, `context_status`, `compact_history`
- `skill` (when skills enabled and at least one skill registered)
- `list_conversations`, `search_conversations` (when ConversationStore provided)
- `todos` (always enabled via `EnableTodos: true`)

**Deferred tools** (hidden until activated via `find_tool`):
- `create_prompt_extension`, `sourcegraph`, `list_mcp_resources`, `read_mcp_resource`
- `list_models`, `agent`, `agentic_fetch`, `web_search`, `web_fetch`
- `cron_create`, `cron_list`, `cron_get`, `cron_delete`, `cron_pause`, `cron_resume`
- `set_delayed_callback`, `cancel_delayed_callback`, `list_delayed_callbacks`
- `verify_skill`, `manage_skill_packs`, `run_recipe`, `connect_mcp`, `create_skill`
- Script tools (loaded from `ScriptToolsDir`)
- Dynamic MCP tools (per connected MCP server)

### Missing from README

| Tool | Notes |
|------|-------|
| `file_inspect` | Core tool, not mentioned in README |
| `reset_context` | Has its own source file `tools/reset_context.go` and event `context.reset`, not mentioned |
| `find_tool` | The meta-tool for activating deferred tools is not mentioned by name |
| `connect_mcp` | Deferred tool for live MCP server connection |
| `create_skill`, `create_prompt_extension` | Deferred authoring tools |
| `manage_skill_packs` | Skill pack management |
| `deploy` | Has source file in `tools/deferred/deploy.go` |
| `download` | Has source file in `tools/download.go` |
| Script tools | User-defined script tools from `ScriptToolsDir` |
| `list_conversations`, `search_conversations` | Core tools when conversation store is configured |

The README says "check `tools_default.go` and `tools/catalog.go`" for the full picture, which is correct, but `catalog.go` is the **old** build path. The current canonical path is `tools_default.go` for the tiered (core/deferred) registry.

---

## 3. Environment Variables

### Accurate

All variables listed in the README are read by the code:

- Server/Provider: `HARNESS_ADDR`, `OPENAI_API_KEY`, `OPENAI_BASE_URL`, `HARNESS_MODEL`, `HARNESS_SYSTEM_PROMPT`, `HARNESS_DEFAULT_AGENT_INTENT`, `HARNESS_MAX_STEPS`, `HARNESS_MAX_COST_PER_RUN_USD`, `HARNESS_TOOL_APPROVAL_MODE`, `HARNESS_ASK_USER_TIMEOUT_SECONDS`, `HARNESS_MODEL_CATALOG_PATH`, `HARNESS_PRICING_CATALOG_PATH`
- Workspace/Content: `HARNESS_WORKSPACE`, `HARNESS_PROMPTS_DIR`, `HARNESS_RECIPES_DIR`, `HARNESS_GLOBAL_DIR`, `HARNESS_ROLLOUT_DIR`, `HARNESS_SUBAGENT_BASE_REF`, `HARNESS_SUBAGENT_WORKTREE_ROOT`
- Integrations: `HARNESS_SKILLS_ENABLED`, `HARNESS_WATCH_ENABLED`, `HARNESS_WATCH_INTERVAL_SECONDS`, `HARNESS_CRON_URL`, `HARNESS_ENABLE_CALLBACKS`, `HARNESS_SOURCEGRAPH_ENDPOINT`, `HARNESS_SOURCEGRAPH_TOKEN`, `HARNESS_MCP_SERVERS`, `HARNESS_ROLE_MODEL_PRIMARY`, `HARNESS_ROLE_MODEL_SUMMARIZER`
- Memory: `HARNESS_CONVERSATION_RETENTION_DAYS`, `HARNESS_CONVERSATION_DB`
- Conclusion Watcher: `HARNESS_CONCLUSION_WATCHER_ENABLED`, `HARNESS_CONCLUSION_WATCHER_INTERVENTION_MODE`, `HARNESS_CONCLUSION_WATCHER_EVALUATOR_ENABLED`, `HARNESS_CONCLUSION_WATCHER_EVALUATOR_MODEL`

### Missing from README

| Variable | Where used | Purpose |
|----------|-----------|---------|
| `HARNESS_MEMORY_MODE` | `main.go:204` | Memory mode (`auto`, `off`, `local_coordinator`) |
| `HARNESS_MEMORY_DB_DRIVER` | `main.go:205` | Memory store driver (`sqlite`, `postgres`) |
| `HARNESS_MEMORY_DB_DSN` | `main.go:206` | Postgres DSN for memory store |
| `HARNESS_MEMORY_SQLITE_PATH` | `main.go:207` | SQLite file path for memory store |
| `HARNESS_MEMORY_DEFAULT_ENABLED` | `main.go:208` | Whether memory is enabled by default |
| `HARNESS_MEMORY_OBSERVE_MIN_TOKENS` | `main.go:209` | Minimum tokens to trigger observation |
| `HARNESS_MEMORY_SNIPPET_MAX_TOKENS` | `main.go:210` | Maximum tokens for memory snippets |
| `HARNESS_MEMORY_REFLECT_THRESHOLD_TOKENS` | `main.go:211` | Token threshold for memory reflection |
| `HARNESS_MEMORY_LLM_MODE` | `main.go:212` | LLM mode for memory (`openai`, `inherit`) |
| `HARNESS_MEMORY_LLM_MODEL` | `main.go:213` | Model for memory LLM calls |
| `HARNESS_MEMORY_LLM_BASE_URL` | `main.go:214` | Base URL for memory LLM |
| `HARNESS_MEMORY_LLM_API_KEY` | `main.go:215` | API key for memory LLM |
| `HARNESS_AUTH_DISABLED` | `auth.go` (inferred from `authDisabledFromEnv()`) | Disable Bearer auth |

The README uses the wildcard notation `HARNESS_MEMORY_*` which technically covers these, but is vague. The specific variables and their meanings are not documented.

### Env vars read only by `internal/config/config.go` (layer 5)

These are applied via the TOML config cascade but also readable as env vars:

- `HARNESS_CONCLUSION_WATCHER_ENABLED` / `_INTERVENTION_MODE` / `_EVALUATOR_ENABLED` / `_EVALUATOR_MODEL` -- all listed and accurate.

---

## 4. Build / Run Instructions

### Accurate

- `go run ./cmd/harnessd` and `go run ./cmd/harnesscli` are correct entry points.
- `OPENAI_API_KEY` is required (the server exits with an error if unset).

### Notes

| Item | README says | Actual |
|------|-----------|--------|
| Go version | Not mentioned | `go.mod` requires `go 1.25.0` -- should be documented as it is unusually high |
| Default port | Implied `8080` in CLI example | Correct: `config.Defaults()` sets `Addr: ":8080"` |
| `--profile` flag on `harnessd` | Not mentioned | `harnessd` accepts `--profile <name>` to load a named TOML profile |

---

## 5. Event Names and Run Request Fields

### Event Names

#### Accurate

All event families listed in the README ("Lifecycle", "Model streaming", "Tooling", "Context and compaction", "Hooks and steering", "Memory and skills") are broadly correct.

#### Inaccurate / Missing

| Issue | README says | Code actually shows |
|-------|-----------|-------------------|
| **`run.cancelled` does not exist** | Listed in Lifecycle events | Not defined in `events.go`. No constant named `EventRunCancelled` exists. |
| **`run.input.required` does not exist** | Listed in Lifecycle events | Not defined. The actual event is `run.waiting_for_user` (`EventRunWaitingForUser`). |
| **`run.continued` does not exist** | Listed in Lifecycle events | Not defined. The related event is `conversation.continued` (`EventConversationContinued`). |
| **Missing events not mentioned in README** | -- | `run.resumed`, `llm.turn.requested`, `llm.turn.completed`, `tool.call.delta`, `assistant.message`, `conversation.continued`, `prompt.resolved`, `prompt.warning`, `usage.delta`, `cost.anomaly`, `skill.fork.*`, `error.context`, `audit.action`, `tool.hook.mutation`, `causal.graph.snapshot`, `rule.injected`, `recorder.drop_detected` |

The README says 54 event types exist (matching the AllEventTypes() slice). The actual count is 57 event types in `AllEventTypes()`.

### Run Request Fields

#### Accurate

All fields listed in the README match `RunRequest` in `types.go`:
- `prompt`, `system_prompt`, `agent_intent`, `task_context`, `prompt_profile`, `prompt_extensions`
- `model`, `provider_name`, `allow_fallback`, `reasoning_effort`
- `max_steps`, `max_cost_usd`
- `allowed_tools`, `mcp_servers`, `dynamic_rules`
- `role_models.primary`, `role_models.summarizer`
- `tenant_id`, `agent_id`
- `permissions.sandbox`, `permissions.approval`
- `profile`

#### Missing from README

| Field | Type | Purpose |
|-------|------|---------|
| `conversation_id` | `string` | Links run to an existing conversation for continuation |

This is a significant omission since `conversation_id` is a first-class field used for multi-turn conversations.

---

## 6. Provider Information

### Accurate

- OpenAI is the primary provider: `harnessd/main.go` requires `OPENAI_API_KEY` and creates an OpenAI provider as the default.
- Anthropic provider support exists: the `anthropic` package is imported and wired into the `ProviderRegistry.SetClientFactory` (line 294-299 of main.go).

### Additional provider support not mentioned

- **Gemini** is also supported via the OpenAI-compatible provider with special handling: `NoParallelTools: providerName == "gemini"` and `ModelIDPrefix: "models/"` for Gemini (lines 310-316 of main.go). The README does not mention Gemini at all.

---

## Summary of Findings

### Inaccuracies to fix

1. **`GET /v1/summarize` should be `POST /v1/summarize`** (wrong HTTP method)
2. **`GET /v1/conversations/{id}` is listed but not implemented** (phantom route)
3. **Three event names are fictitious**: `run.cancelled`, `run.input.required`, `run.continued` do not exist in the codebase
4. **`PUT /v1/providers/{name}/key` is missing** from the routes section

### Missing items to add

1. `conversation_id` field in run request documentation
2. `--profile` flag on `harnessd`
3. Gemini provider support
4. Go 1.25.0 requirement
5. Specific `HARNESS_MEMORY_*` environment variables (12 vars currently hidden behind a wildcard)
6. `HARNESS_AUTH_DISABLED` environment variable
7. Notable tools not mentioned: `find_tool`, `file_inspect`, `reset_context`, `deploy`, `connect_mcp`, `create_skill`, `download`
8. The tiered tool architecture (core vs deferred) is the actual current design; the README references `tools/catalog.go` as canonical but the real path is `tools_default.go` with the core/deferred split

### What the README gets right

- The overall architecture description is accurate
- Source-of-truth references are mostly correct (events.go, types.go, config package)
- Quick-start instructions work
- The run request shape is comprehensive (one field missing)
- Provider information is directionally correct
- Config environment variables are mostly complete
