# README.md Documentation Review — 2026-03-21

## 1. Routes / Endpoints

### Accurate
- `GET /healthz` — confirmed at `internal/server/http.go:146`
- `GET /v1/models` — confirmed at `http.go:167`
- `GET /v1/providers` — confirmed at `http.go:177`
- `GET /v1/mcp/servers` — confirmed at `http.go:198`
- `GET /v1/search/code` — confirmed at `http.go:195`
- `GET /v1/summarize` — confirmed at `http.go:181`
- `POST /v1/runs` — confirmed at `http.go:160`
- `GET /v1/runs` — confirmed at `http.go:160` (same handler, dispatches on method)
- `GET /v1/runs/{id}` — confirmed at `http.go:161`
- `GET /v1/runs/{id}/events` — confirmed at `http.go:632`
- `GET|POST /v1/runs/{id}/input` — confirmed at `http.go:637`
- `GET /v1/runs/{id}/summary` — confirmed at `http.go:654`
- `POST /v1/runs/{id}/continue` — confirmed at `http.go:664`
- `POST /v1/runs/{id}/steer` — confirmed at `http.go:674`
- `GET /v1/runs/{id}/context` — confirmed at `http.go:684`
- `POST /v1/runs/{id}/compact` — confirmed at `http.go:694`
- `GET|PUT /v1/runs/{id}/todos` — confirmed at `http.go:704-716`
- `POST /v1/runs/replay` — confirmed at `http.go:607-612`
- All conversation endpoints — confirmed at `http.go:1126+`
- `POST /v1/agents` — confirmed at `http.go:169`
- `GET/POST /v1/subagents`, `GET/DELETE /v1/subagents/{id}` — confirmed at `http.go:172-173`
- Cron endpoints — confirmed at `http.go:184-185`
- Skills endpoints — confirmed at `http.go:188-189`
- Recipes endpoints — confirmed at `http.go:192-193`

### Missing from README
- **`POST /v1/runs/{id}/cancel`** — exists at `http.go:719` but not listed in README
- **`POST /v1/runs/{id}/approve`** — exists at `http.go:725` but not listed in README
- **`POST /v1/runs/{id}/deny`** — exists at `http.go:735` but not listed in README
- **`GET /v1/providers/{name}`** — exists at `http.go:178` (with admin middleware) but not listed
- **`GET /v1/profiles`** — exists at `http.go:202` but not listed
- **`GET/POST/PUT/DELETE /v1/profiles/{name}`** — exists at `http.go:203` + `http_profiles.go:26-60` but not listed

### Mismatch
- None found — all listed routes exist and have the correct methods.

---

## 2. Environment Variables

### Accurate (confirmed in `cmd/harnessd/main.go`)
- `HARNESS_ADDR` — `config/config.go:475`
- `OPENAI_API_KEY` — used in main.go provider setup
- `OPENAI_BASE_URL` — `main.go:226`
- `HARNESS_MODEL` — `config/config.go:472`
- `HARNESS_SYSTEM_PROMPT` — `main.go:207`
- `HARNESS_DEFAULT_AGENT_INTENT` — `main.go:208`
- `HARNESS_MAX_STEPS` — `config/config.go:478`
- `HARNESS_MAX_COST_PER_RUN_USD` — `config/config.go:484`
- `HARNESS_TOOL_APPROVAL_MODE` — `main.go:211`
- `HARNESS_ASK_USER_TIMEOUT_SECONDS` — `main.go:210`
- `HARNESS_MODEL_CATALOG_PATH` — `main.go:229`
- `HARNESS_PRICING_CATALOG_PATH` — `main.go:228`
- `HARNESS_WORKSPACE` — `main.go:168`
- `HARNESS_PROMPTS_DIR` — `main.go:209`
- `HARNESS_RECIPES_DIR` — `main.go:246`
- `HARNESS_GLOBAL_DIR` — `main.go:378`
- `HARNESS_ROLLOUT_DIR` — `main.go:256`
- `HARNESS_SUBAGENT_BASE_REF` — `main.go:247`
- `HARNESS_SUBAGENT_WORKTREE_ROOT` — `main.go:248`
- `HARNESS_SKILLS_ENABLED` — `main.go:243`
- `HARNESS_WATCH_ENABLED` — `main.go:244`
- `HARNESS_WATCH_INTERVAL_SECONDS` — `main.go:245`
- `HARNESS_CRON_URL` — `main.go:252`
- `HARNESS_ENABLE_CALLBACKS` — `main.go:253`
- `HARNESS_SOURCEGRAPH_ENDPOINT` — `main.go:254`
- `HARNESS_SOURCEGRAPH_TOKEN` — `main.go:255`
- `HARNESS_MCP_SERVERS` — `internal/mcp/config.go:13`
- `HARNESS_ROLE_MODEL_PRIMARY` — `main.go:579`
- `HARNESS_ROLE_MODEL_SUMMARIZER` — `main.go:580`
- `HARNESS_MEMORY_MODE` — `main.go:212`
- `HARNESS_MEMORY_LLM_MODE` — `main.go:224`
- `HARNESS_MEMORY_LLM_MODEL` — `main.go:225`
- `HARNESS_MEMORY_LLM_API_KEY` — `main.go:227`
- `HARNESS_MEMORY_LLM_BASE_URL` — `main.go:226`
- `HARNESS_CONVERSATION_RETENTION_DAYS` — `main.go:497`
- `HARNESS_CONVERSATION_DB` — `main.go:501`
- `HARNESS_CONCLUSION_WATCHER_ENABLED` — `config/config.go:489`
- `HARNESS_CONCLUSION_WATCHER_INTERVENTION_MODE` — `config/config.go:494`
- `HARNESS_CONCLUSION_WATCHER_EVALUATOR_ENABLED` — `config/config.go:497`
- `HARNESS_CONCLUSION_WATCHER_EVALUATOR_MODEL` — `config/config.go:502`

### Missing from README (env vars used in code but not documented)
- **`HARNESS_MEMORY_DB_DRIVER`** — `main.go:213`
- **`HARNESS_MEMORY_DB_DSN`** — `main.go:214`
- **`HARNESS_MEMORY_SQLITE_PATH`** — `main.go:215`
- **`HARNESS_MEMORY_DEFAULT_ENABLED`** — `main.go:216`
- **`HARNESS_MEMORY_OBSERVE_MIN_TOKENS`** — `main.go:217`
- **`HARNESS_MEMORY_SNIPPET_MAX_TOKENS`** — `main.go:218`
- **`HARNESS_MEMORY_REFLECT_THRESHOLD_TOKENS`** — `main.go:219`
- **`HARNESS_RUN_DB`** — `main.go:479`

---

## 3. Tools

### Accurate
The README describes tool categories rather than individual tool names, which is a reasonable approach. The described categories align with the actual catalog:

- Core file/shell helpers: `read`, `write`, `edit`, `apply_patch`, `bash` — confirmed in `tools/catalog.go:33-37`
- Process helpers: `job_output`, `job_kill`, `compact_history`, `context_status` — confirmed in `tools/catalog.go:38-48`
- Clarification/memory: `ask_user_question`, `observational_memory` — confirmed in `tools/catalog.go:31-32`
- Optional integrations: MCP, skills, recipes, sourcegraph, cron, subagent, fetch/search — confirmed in `tools/catalog.go:51-113`

### Not mentioned in README (tools that exist but are not called out)
- `glob`, `grep`, `ls`, `git_status`, `git_diff`, `fetch`, `download` — all TierCore tools in the catalog
- `find_tool` — the TierCore discovery tool for deferred tools
- `reset_context` — TierCore context reset tool
- `todos` — conditionally included TierCore tool
- Deferred-tier tools: `git_log_search`, `git_file_history`, `git_blame_context`, `git_diff_range`, `git_contributor_context`, `deploy`, `spawn_agent`, `task_complete`, `run_agent`, `connect_mcp`, `create_skill`, `create_prompt_extension`, `skill_packs`, `lsp_diagnostics`, `lsp_references`, `lsp_restart`
- Profile tools (deferred): `list_profiles`, `get_profile`, `create_profile`, `update_profile`, `delete_profile`, `validate_profile`, `recommend_profile`, `get_efficiency_report`

The README's approach of describing categories is acceptable, but the `find_tool` and `reset_context` core tools are worth explicitly mentioning as they are important user-facing capabilities.

### Reference files
- `internal/harness/tools/catalog.go` (the old/original catalog builder)
- `internal/harness/tools/core/` (core tier tools, refactored)
- `internal/harness/tools/deferred/` (deferred tier tools, refactored)

---

## 4. Build / Run Instructions

### Accurate
- `go run ./cmd/harnessd` — correct entry point
- `go run ./cmd/harnesscli -base-url http://127.0.0.1:8080 -prompt "..."` — correct usage
- Go module: `go-agent-harness`, Go version `1.25.0` per `go.mod`
- Default address is `:8080` per `internal/config/config.go:175`

### Notes
- No Makefile was found; the README does not claim one exists (correct)
- Test script at `./scripts/test-regression.sh` is not mentioned in the README (acceptable; it is an internal detail)

---

## 5. Event Types / SSE Events

### Accurate
The README groups events by family, which generally aligns. Confirming against `internal/harness/events.go`:

- Lifecycle events: `run.started`, `run.completed`, `run.failed`, `run.cancelled`, `run.cost_limit_reached`, `run.step.started`, `run.step.completed` — all confirmed

### Mismatches in README
- **`run.input.required`** — listed in README but does NOT exist in `events.go`. The actual event is `run.waiting_for_user` (which IS listed). This is a phantom event name.
- **`run.continued`** — listed in README but does NOT exist in `events.go`. The actual event is `conversation.continued`, not `run.continued`.
- **`assistant.message.completed`** — listed in README but does NOT exist. The actual event is `assistant.message` (no `.completed` suffix).
- **`run.waiting_for_user`** — listed and correct.

### Missing from README (events that exist but are not mentioned)
- `run.queued` — added for bounded worker pool mode
- `run.resumed` — exists at `events.go:23`
- `llm.turn.requested`, `llm.turn.completed` — LLM turn lifecycle events
- `assistant.thinking.delta` — listed in README under streaming (correct)
- `tool.call.delta` — tool streaming event, not mentioned
- `tool.approval_required`, `tool.approval_granted`, `tool.approval_denied` — approval workflow events
- `conversation.continued` — exists but README lists incorrect name `run.continued`
- `prompt.resolved`, `prompt.warning` — prompt resolution events, not mentioned
- `usage.delta` — accounting event, not mentioned
- `cost.anomaly` — cost forensics event, not mentioned
- `error.context` — error chain event, not mentioned
- `audit.action` — audit trail event, not mentioned
- `tool.hook.mutation` — hook mutation tracing, not mentioned
- `causal.graph.snapshot` — causal graph event, not mentioned
- `rule.injected` — dynamic rule event, not mentioned
- `recorder.drop_detected` — recorder gap marker event, not mentioned
- `workspace.provisioned`, `workspace.destroyed`, `workspace.provision_failed` — workspace lifecycle events, not mentioned
- `profile.efficiency_suggestion` — profile system event, not mentioned
- `spawn_agent.started`, `spawn_agent.completed`, `task.completed`, `step_budget.pressure` — recursive agent events, not mentioned
- `skill.fork.started`, `skill.fork.completed`, `skill.fork.failed` — skill fork events, not mentioned

The README states "Some events are feature-gated" and points to `events.go` as canonical. The listed families are broadly correct but contain 3 specific phantom event names.

---

## 6. Provider Information

### Accurate
- OpenAI is the primary provider — confirmed (default `newProvider` in `main.go:88-90` uses `openai.NewClient`)
- Anthropic provider exists — confirmed via import at `main.go:26` and `anthropic.NewClient` usage at `main.go:306-307`
- The README correctly states "Anthropic provider support exists in the provider catalog"

---

## 7. CLI Flags

### Accurate (confirmed in `cmd/harnesscli/main.go:124-136`)
- `-base-url` — line 124
- `-model` — line 126
- `-system-prompt` — line 127
- `-agent-intent` — line 128
- `-task-context` — line 129
- `-prompt-profile` — line 130
- `-prompt-custom` — line 131
- `-prompt-behavior` — line 135 (repeatable flag)
- `-prompt-talent` — line 136 (repeatable flag)
- `-tui` — line 144 (checked via os.Args substring match)

### Mismatch
- None. All listed flags are confirmed. `-prompt` is registered at `main.go:125`.

---

## Summary of Findings

### Critical Mismatches (should be fixed)
1. **3 phantom event names** in README: `run.input.required`, `run.continued`, `assistant.message.completed` — none exist in the codebase
2. **6 missing endpoints** from README: `/cancel`, `/approve`, `/deny`, `/providers/{name}`, `/profiles`, `/profiles/{name}`

### Moderate Gaps (good to fix)
3. **8 undocumented env vars**: `HARNESS_MEMORY_DB_DRIVER`, `HARNESS_MEMORY_DB_DSN`, `HARNESS_MEMORY_SQLITE_PATH`, `HARNESS_MEMORY_DEFAULT_ENABLED`, `HARNESS_MEMORY_OBSERVE_MIN_TOKENS`, `HARNESS_MEMORY_SNIPPET_MAX_TOKENS`, `HARNESS_MEMORY_REFLECT_THRESHOLD_TOKENS`, `HARNESS_RUN_DB`
4. **20+ undocumented event types** (many are opt-in forensic events, so the README's family-based approach is defensible, but the 3 phantom names are not)

### Minor Items
5. Profile tools and endpoints are fully implemented but not reflected in README
6. Tool approval workflow (approve/deny/cancel) is a significant feature omitted from the README
