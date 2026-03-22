# Subagent Debugging

## Finding a Child Run's ID

When a parent agent spawns a child via `run_agent` or `spawn_agent`, the child
run gets its own run ID. There are two ways to find it.

### From the parent run's output

The `run_agent` and `spawn_agent` tools return a `ChildResult` JSON object. The
child's run ID is not directly embedded in `ChildResult`, but the parent run's
event stream includes it. Subscribe to parent run events and look for tool call
completion events that contain the child run ID in the payload.

### Via GET /v1/subagents

```bash
curl -s http://localhost:8080/v1/subagents | python3 -m json.tool
```

Response:

```json
{
  "subagents": [
    {
      "id": "subagent_f3a2b1c0-...",
      "run_id": "run_abc123",
      "status": "completed",
      "isolation": "inline",
      "cleanup_policy": "preserve",
      "workspace_path": "",
      "workspace_cleaned": false,
      "output": "...",
      "error": "",
      "created_at": "2026-03-20T14:01:00Z",
      "updated_at": "2026-03-20T14:01:45Z"
    }
  ]
}
```

The `run_id` field is the run ID you can use with `GET /v1/runs/{id}` and
`GET /v1/runs/{id}/events`.

### Via GET /v1/subagents/{id}

```bash
curl -s http://localhost:8080/v1/subagents/subagent_f3a2b1c0-... | python3 -m json.tool
```

Returns the same `Subagent` object for a specific subagent by its subagent ID.

---

## Reading a Child Run's Status

```bash
curl -s http://localhost:8080/v1/runs/run_abc123 | python3 -m json.tool
```

Example response:

```json
{
  "id": "run_abc123",
  "prompt": "Search internal/harness for all TODO comments.",
  "model": "gpt-4.1-mini",
  "status": "completed",
  "output": "Found 12 TODO comments across 7 files...",
  "usage_totals": {
    "prompt_tokens": 4200,
    "completion_tokens": 380
  },
  "cost_totals": {
    "total_cost_usd": 0.031
  },
  "created_at": "2026-03-20T14:01:00Z",
  "updated_at": "2026-03-20T14:01:45Z"
}
```

### Run status values

| Status | Meaning |
|--------|---------|
| `queued` | Accepted but not yet started (pool at capacity) |
| `running` | Agent is actively executing steps |
| `waiting_for_user` | Agent called `ask_user_question` and is blocked |
| `waiting_for_approval` | Agent is awaiting tool approval |
| `completed` | Run finished normally (or hit cost limit) |
| `failed` | Run terminated due to an error |
| `cancelled` | Run was cancelled via `POST /v1/runs/{id}/cancel` |

---

## Understanding ChildResult

All child completion paths (`task_complete`, `spawn_agent`, `run_agent`) return
output normalised to the `ChildResult` schema. This lets parent agents parse
child results without branching on which tool was used.

```json
{
  "summary": "Identified 12 TODO comments across 7 files in internal/harness.",
  "status": "completed",
  "findings": [
    {
      "type": "observation",
      "title": "TODO in runner.go line 482",
      "content": "// TODO: add retry on transient provider errors"
    }
  ],
  "output": "",
  "profile": "researcher"
}
```

### ChildResult fields

| Field | Type | Description |
|-------|------|-------------|
| `summary` | string | 1–3 sentence description of what the child accomplished. Present on all completion paths. |
| `status` | string | Terminal state: `"completed"`, `"partial"`, or `"failed"`. |
| `findings` | array | Structured key discoveries from `task_complete`. Omitted when empty or when the completion path is `run_agent`/`spawn_agent`. |
| `output` | string | Raw text output from `run_agent`/`spawn_agent` fallback. Omitted when empty. |
| `profile` | string | Profile name used by `run_agent`. Omitted when not set. |

A `status` of `"partial"` indicates the child completed some work but did not
fully accomplish the task (e.g., hit `max_steps`). The `summary` field should
explain what was and was not completed.

---

## Streaming Child Run Events via SSE

```bash
curl -s -N http://localhost:8080/v1/runs/run_abc123/events
```

Each event arrives as a newline-delimited JSON object:

```
data: {"id":"evt_001","run_id":"run_abc123","type":"run.started","timestamp":"2026-03-20T14:01:00Z","payload":{}}
data: {"id":"evt_002","run_id":"run_abc123","type":"tool.call.started","timestamp":"2026-03-20T14:01:05Z","payload":{"tool":"read"}}
data: {"id":"evt_003","run_id":"run_abc123","type":"tool.call.completed","timestamp":"2026-03-20T14:01:06Z","payload":{"tool":"read","duration_ms":120}}
data: {"id":"evt_004","run_id":"run_abc123","type":"run.completed","timestamp":"2026-03-20T14:01:45Z","payload":{}}
```

`GET /v1/runs/{id}/events` returns all buffered events first (history), then
stays connected to stream new events in real time until the run terminates.

### Key event types for debugging

| Event Type | Meaning |
|------------|---------|
| `run.started` | Agent began executing |
| `run.completed` | Normal completion |
| `run.failed` | Failure; check `error` field on the run object |
| `run.cost_limit_reached` | Hit `max_cost_usd` ceiling (run still completes, not fails) |
| `run.step.started` / `run.step.completed` | Individual LLM turn boundaries |
| `tool.call.started` / `tool.call.completed` | Tool invocation boundaries |
| `tool.activated` | A deferred tool was promoted via `find_tool` |
| `error.context` | Emitted immediately before `run.failed` with error details |
| `profile.efficiency_suggestion` | Efficiency score < 0.6; inspect `efficiency_score` and `unused_tools` in payload |
| `workspace.provision_failed` | Workspace setup failed (worktree/container/VM runs only) |

---

## Interpreting Efficiency Reports for a Run

After a run completes, you can retrieve an aggregate efficiency report for the
profile it used via the `get_efficiency_report` tool or `GET /v1/profiles/{name}`
followed by the tool call. The per-run efficiency score uses:

```
efficiency = 1.0 / (1.0 + steps * 0.1 + cost_usd * 10.0)
```

When a run's score falls below 0.6, the runner emits a
`profile.efficiency_suggestion` event on the run's event stream. This event's
payload includes:

```json
{
  "profile": "researcher",
  "efficiency_score": 0.42,
  "unused_tools": ["web_search", "web_fetch"],
  "run_id": "run_abc123"
}
```

`unused_tools` lists tools in the profile's `allow` list that were never called.
These are candidates for removal from the profile to tighten its scope.

Aggregate reports (across all runs for a profile) are available via the
`get_efficiency_report` tool. Reports require at least 3 completed runs
(`has_history: true`) before suggestions are generated.

---

## Common Failure Modes

### Profile not found

**Symptom**: `run_agent` returns an error; no run is created.

**Cause**: The `profile` field specifies a name that does not exist in any of
the three resolution tiers (project / user / built-in).

**Fix**: Verify the profile name with `GET /v1/profiles`. Check for typos.
Confirm that the `.harness/profiles/` directory (project or user) contains the
expected TOML file. Use `recommend_profile` if you are unsure which profile
to use.

### Tool not allowed

**Symptom**: The run reaches `failed` status. The `run.failed` event or the
`error.context` event mentions that a tool is not permitted.

**Cause**: The profile's `tools.allow` list does not include the tool the agent
tried to call.

**Fix**: Inspect the profile's allowed tools: `GET /v1/profiles/{name}`. Either
add the required tool to the profile's `allow` list via `PUT /v1/profiles/{name}`
or choose a profile that already includes it.

```bash
curl -s http://localhost:8080/v1/profiles/researcher \
  | python3 -c "import sys,json; p=json.load(sys.stdin); print(p['allowed_tools'])"
```

### max_steps exceeded

**Symptom**: The event stream includes a `run.step.completed` event near the
configured step limit, followed by `run.completed`. The `ChildResult.status` is
`"partial"` rather than `"completed"`.

**Cause**: The agent consumed all allowed steps without calling `task_complete`.

**How to identify**: Look for a `run.completed` event whose payload includes
`"reason": "max_steps_reached"` and a `"max_steps"` field.

**Fix**: Either increase `max_steps` in the profile, or provide a more focused
task description so the agent completes sooner.

### Cost limit reached

**Symptom**: The event stream includes `run.cost_limit_reached`, followed
immediately by `run.completed` (not `run.failed`). The run terminates early.

**Cause**: The cumulative cost of LLM calls exceeded the `max_cost_usd` ceiling.

**How to identify**: Check for `run.cost_limit_reached` in the event stream:

```bash
curl -s -N http://localhost:8080/v1/runs/run_abc123/events \
  | grep cost_limit
```

**Fix**: Increase `max_cost_usd` in the profile or reduce the task scope.
Check `cost_totals.total_cost_usd` on the run object to see how much was spent.

### Workspace provision failed

**Symptom**: Run reaches `failed` status almost immediately. The event stream
contains `workspace.provision_failed`.

**Cause**: The worktree or container workspace could not be set up. Common
causes: `git worktree` cannot create a branch (conflict), Docker daemon is not
running, or `base_ref` does not exist.

**Fix**: Check the `error` field on the run object and the
`workspace.provision_failed` event payload. Verify that the `base_ref` exists
in the repository and that the isolation backend is available.

---

## Inspecting Which Tools Are Allowed for a Profile

```bash
curl -s http://localhost:8080/v1/profiles/researcher | python3 -m json.tool
```

The `allowed_tools` array lists all tools the agent may call. An empty array
means all tools are allowed (no restriction). The `allowed_tool_count` field
gives the list length for quick inspection.

To compare what a run actually used against what was allowed, check the
`tool_calls` array on the `RunSummary` returned by `GET /v1/runs/{id}`, then
cross-reference with `allowed_tools` from the profile.
