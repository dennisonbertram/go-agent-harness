# SSE Event Catalog

All events returned by `AllEventTypes()` in `internal/harness/events.go` (79
event types as of this writing). Events use dot-notation naming:
`category.action` or `category.subcategory.action`.

## Terminal Events

Three events signal stream termination: `run.completed`, `run.failed`, and
`run.cancelled` (`IsTerminalEvent`, `internal/harness/events.go:477-479`).
Clients **must** close the connection after receiving any of these three.

## Event Categories

### Run Lifecycle (10 events)

| Event | Terminal | Description |
|-------|----------|-------------|
| `run.started` | No | Run begins execution |
| `run.completed` | **Yes** | Run completed successfully |
| `run.failed` | **Yes** | Run failed with error |
| `run.waiting_for_user` | No | Waiting for user input (`AskUserQuestion` tool call) |
| `run.resumed` | No | Run resumed after user answers |
| `run.cost_limit_reached` | No | Cumulative run cost reached/exceeded `max_cost_usd`; the run is then terminated with `run.completed` (not `run.failed`) |
| `run.step.started` | No | A run step begins |
| `run.step.completed` | No | A run step finishes |
| `run.cancelled` | **Yes** | Run cancelled via `CancelRun` or `POST /v1/runs/{id}/cancel`; status becomes `cancelled`, any in-flight provider/tool call is interrupted via context cancellation |
| `run.queued` | No | Run accepted but held because the runner's bounded worker pool is at capacity; transitions to `running` (emitting `run.started`) once a slot opens. Only emitted when `WorkerPoolSize > 0` |

#### `run.started`
```json
{ "prompt": "string" }
```

#### `run.completed`
```json
{
  "output": "string",
  "usage_totals": { "prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0 },
  "cost_totals": { "total_usd": 0.0 }
}
```

#### `run.failed`
```json
{
  "error": "string",
  "usage_totals": { "prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0 },
  "cost_totals": { "total_usd": 0.0 }
}
```

#### `run.waiting_for_user`
```json
{
  "call_id": "string",
  "tool": "AskUserQuestion",
  "questions": [],
  "deadline_at": "RFC3339 timestamp"
}
```

`tool` is the tool's registered name (`htools.AskUserQuestionToolName`,
`internal/harness/tools/ask_user_question.go:12`), not a snake_case alias.

#### `run.resumed`
```json
{
  "call_id": "string",
  "tool": "string",
  "answered_at": "RFC3339 timestamp"
}
```

### LLM Turn (5 events)

| Event | Description |
|-------|-------------|
| `llm.turn.requested` | LLM call initiated for step N |
| `llm.turn.completed` | LLM response received |
| `assistant.message.delta` | Streaming content chunk |
| `assistant.thinking.delta` | Streaming reasoning/thinking chunk |
| `reasoning.complete` | Emitted after a turn when `CaptureReasoning` is enabled and the provider returned reasoning text. Payload: `text`, `tokens`, `step` |

#### `llm.turn.requested`
```json
{ "step": 1 }
```

#### `llm.turn.completed`
```json
{ "step": 1, "tool_calls": 2 }
```

#### `assistant.message.delta`
```json
{ "step": 1, "content": "string" }
```

### Tool Execution (9 events)

| Event | Description |
|-------|-------------|
| `tool.call.started` | Tool execution begins |
| `tool.call.completed` | Tool execution finished (success or error) |
| `job.completed` | A background job finished; carries its result onto the originating conversation's live run (`internal/harness/job_bridge.go:14-16`) |
| `tool.call.delta` | Streaming tool call argument chunk |
| `tool.activated` | A deferred tool was activated via `find_tool` |
| `tool.output.delta` | Incremental output chunk from a running tool |
| `tool.approval_required` | A tool call requires operator approval; run transitions to `waiting_for_approval`. Resume with `POST /v1/runs/{id}/approve` or `/deny`. Payload: `call_id`, `tool`, `arguments`, `deadline_at` |
| `tool.approval_granted` | Operator approved a pending tool call; it executes immediately after. Payload: `call_id`, `tool` |
| `tool.approval_denied` | Operator denied a pending tool call; a `permission_denied` error is returned to the LLM and the run continues. Payload: `call_id`, `tool` |

#### `tool.call.started`
```json
{ "call_id": "string", "tool": "string", "arguments": "string (JSON)" }
```

#### `tool.call.completed`
```json
{ "call_id": "string", "tool": "string", "output": "string", "error": "string (optional)" }
```

#### `tool.call.delta`
```json
{ "step": 1, "index": 0, "call_id": "string", "tool": "string", "arguments": "string" }
```

### Todos (1 event)

| Event | Description |
|-------|-------------|
| `todos.updated` | Emitted after a run's todo list changes |

### Assistant Completion (1 event)

| Event | Description |
|-------|-------------|
| `assistant.message` | Final assistant message (no tool calls) |

#### `assistant.message`
```json
{ "content": "string" }
```

### Conversation (1 event)

| Event | Description |
|-------|-------------|
| `conversation.continued` | Prior conversation history loaded |

#### `conversation.continued`
```json
{ "conversation_id": "string", "prior_message_count": 5 }
```

### Prompt Resolution (2 events)

| Event | Description |
|-------|-------------|
| `prompt.resolved` | System prompt resolved via the prompt engine |
| `prompt.warning` | Warning from prompt resolution |

#### `prompt.resolved`
```json
{
  "intent": "string",
  "model_profile": "string",
  "model_fallback": "string",
  "applied_behaviors": [],
  "applied_talents": [],
  "applied_skills": [],
  "has_warnings": false
}
```

`applied_skills` lists skills resolved from `prompt_extensions.skills[]` and
injected into the composed system prompt (`internal/systemprompt/engine.go:46-60`,
`internal/harness/runner.go:1909-1919`) — skills are applied, not ignored.

#### `prompt.warning`
```json
{ "code": "string", "message": "string" }
```

### Provider (1 event)

| Event | Description |
|-------|-------------|
| `provider.resolved` | The provider/model chosen for the run was resolved |

### Memory (4 events)

| Event | Description |
|-------|-------------|
| `memory.observe.started` | Memory observation begins |
| `memory.observe.completed` | Memory observation succeeded |
| `memory.observe.failed` | Memory observation failed |
| `memory.reflection.completed` | Memory reflection triggered |

#### `memory.observe.started`
```json
{ "step": 1 }
```

#### `memory.observe.completed`
```json
{ "step": 1, "observed": true, "reflected": false, "observation": 5 }
```

#### `memory.observe.failed`
```json
{ "step": 1, "error": "string" }
```

#### `memory.reflection.completed`
```json
{ "step": 1 }
```

### Accounting (1 event)

| Event | Description |
|-------|-------------|
| `usage.delta` | Token usage and cost for a turn |

#### `usage.delta`
```json
{
  "step": 1,
  "usage_status": "provider_reported",
  "cost_status": "available",
  "turn_usage": { "prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0 },
  "turn_cost_usd": 0.001,
  "cumulative_usage": { "prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0 },
  "cumulative_cost_usd": 0.003,
  "pricing_version": "string"
}
```

### Hooks — message-level (3 events)

| Event | Description |
|-------|-------------|
| `hook.started` | Pre/post-message hook execution begins |
| `hook.failed` | Pre/post-message hook execution failed |
| `hook.completed` | Pre/post-message hook execution completed |

#### `hook.started`
```json
{ "stage": "pre_message|post_message", "hook": "string", "step": 1 }
```

#### `hook.failed`
```json
{
  "stage": "pre_message|post_message",
  "hook": "string",
  "step": 1,
  "error": "string",
  "mode": "fail_closed|fail_open",
  "ignored": false
}
```

#### `hook.completed`
```json
{
  "stage": "pre_message|post_message",
  "hook": "string",
  "step": 1,
  "action": "continue|block",
  "mutated": false,
  "reason": "string (optional)"
}
```

### Callbacks (3 events)

| Event | Description |
|-------|-------------|
| `callback.scheduled` | A callback was scheduled |
| `callback.fired` | A scheduled callback fired |
| `callback.canceled` | A scheduled callback was canceled |

`internal/harness/events.go` also declares `callback.dispatching`,
`callback.retry_wait`, `callback.started`, and `callback.failed`, but nothing
in the codebase emits them and `AllEventTypes()` does not return them — treat
those four as reserved, not currently observable on the stream.

### Skill Constraints (2 events)

| Event | Description |
|-------|-------------|
| `skill.constraint.activated` | A skill constraint was activated |
| `skill.constraint.deactivated` | A skill constraint was deactivated |

### Tool Call Blocked (1 event)

| Event | Description |
|-------|-------------|
| `tool.call.blocked` | A tool call was blocked (permission rule, hook, etc.) |

### Meta Message (1 event)

| Event | Description |
|-------|-------------|
| `meta.message.injected` | A meta-level message was injected into the transcript |

### Skill Fork (3 events)

| Event | Description |
|-------|-------------|
| `skill.fork.started` | A skill fork begins |
| `skill.fork.completed` | A skill fork completed |
| `skill.fork.failed` | A skill fork failed |

### Tool Hooks — tool-level (3 events)

| Event | Description |
|-------|-------------|
| `tool_hook.started` | Pre/post individual-tool hook execution begins |
| `tool_hook.failed` | Pre/post individual-tool hook execution failed |
| `tool_hook.completed` | Pre/post individual-tool hook execution completed |

### Steering (1 event)

| Event | Description |
|-------|-------------|
| `steering.received` | A user steering message was injected into the transcript before an LLM call |

### Context Management (1 event)

| Event | Description |
|-------|-------------|
| `compact_history.completed` | Conversation history was compacted |

### Error Chain (1 event)

| Event | Description |
|-------|-------------|
| `error.context` | Emitted immediately before `run.failed` when `ErrorChainEnabled` is set; carries an error classification, a context snapshot of the last N tool calls/messages, and an optional cause chain |

### Auto-Compaction (2 events)

| Event | Description |
|-------|-------------|
| `auto_compact.started` | Proactive auto-compaction begins |
| `auto_compact.completed` | Proactive auto-compaction finishes |

### Forensics — tool decision tracing (3 events, opt-in via `TraceToolDecisions`/`DetectAntiPatterns`/`TraceHookMutations`)

| Event | Description |
|-------|-------------|
| `tool.decision` | Emitted once per step when `TraceToolDecisions` is enabled. Payload: `step`, `call_sequence`, `available_tools`, `selected_tools` |
| `tool.antipattern` | Emitted when `DetectAntiPatterns` is enabled and the same (tool, args) pair has been seen 3+ times in one run. Payload: `type`, `tool`, `call_count`, `step` |
| `tool.hook.mutation` | Emitted when `TraceHookMutations` is enabled and a pre-tool-use hook modified or blocked a tool call. Payload: `tool_call_id`, `hook`, `action`, `args_before`, `args_after` |

### LLM Request Envelope (2 events, opt-in via `CaptureRequestEnvelope`)

| Event | Description |
|-------|-------------|
| `llm.request.snapshot` | Emitted before each provider call. Payload: `step`, `prompt_hash` (SHA-256), `tool_names`, `memory_snippet` |
| `llm.response.meta` | Emitted after each provider call. Payload: `step`, `latency_ms`, `model_version` |

### Cost Forensics (1 event, opt-in via `CostAnomalyDetectionEnabled`)

| Event | Description |
|-------|-------------|
| `cost.anomaly` | Emitted when a step's cost exceeds `CostAnomalyStepMultiplier` × the rolling average cost of prior steps. Payload: `step`, `anomaly_type`, `step_cost_usd`, `avg_cost_usd`, `threshold_multiplier` |

### Audit Trail (1 event, opt-in via `AuditTrailEnabled`)

| Event | Description |
|-------|-------------|
| `audit.action` | Emitted for each state-modifying tool call; written to the append-only `audit.jsonl` file alongside `rollout.jsonl`. Payload: `tool`, `call_id`, `arguments` |

### Context Window Forensics (2 events, opt-in via `ContextWindowSnapshotEnabled`)

| Event | Description |
|-------|-------------|
| `context.window.snapshot` | Per-step snapshot of context window usage: token counts, usage ratio, headroom, and a breakdown by component. Non-provider-sourced counts are labeled `"estimated": true` |
| `context.window.warning` | Emitted when usage exceeds `ContextWindowWarningThreshold` (only when snapshots are also enabled) |

### Causal Graph (1 event, opt-in via `CausalGraphEnabled`)

| Event | Description |
|-------|-------------|
| `causal.graph.snapshot` | Emitted at run end; carries the causal dependency graph (Tier 1 context dependencies + Tier 2 data-flow heuristic edges) |

### Context Reset (1 event)

| Event | Description |
|-------|-------------|
| `context.reset` | Emitted when an agent calls `reset_context` to clear its transcript and start a new context segment. Payload: `reset_index`, `at_step`, `persist` |

### Empty-Response Retry (1 event)

| Event | Description |
|-------|-------------|
| `llm.empty_response.retry` | The LLM returned no text and no tool calls (e.g. some thinking-mode responses); the harness injects a retry prompt instead of treating it as completion. Payload: `step`, `retry`, `max_retries` |

### Dynamic Rule Injection (1 event)

| Event | Description |
|-------|-------------|
| `rule.injected` | A `DynamicRule` fired and its content was injected into the system prompt for the current step. Payload: `rule_id`, `step`, `trigger_tool` |

### Recorder Observability (1 event)

| Event | Description |
|-------|-------------|
| `recorder.drop_detected` | A non-terminal event was dropped because the recorder channel was full; an explicit gap marker in the JSONL file. Payload: `dropped_event_id`, `dropped_event_type`, `dropped_seq` |

### Workspace Lifecycle (3 events)

| Event | Description |
|-------|-------------|
| `workspace.provisioned` | A per-run workspace was provisioned. Only emitted when `RunRequest.WorkspaceType` is non-empty. Payload: `workspace_type`, `workspace_path` |
| `workspace.destroyed` | A per-run workspace was torn down after completion, failure, or cancellation. Payload: `workspace_type`, `workspace_path` |
| `workspace.provision_failed` | Workspace provisioning failed; the run transitions to `run.failed` immediately after. Payload: `workspace_type`, `error` |

### Profile Efficiency (1 event)

| Event | Description |
|-------|-------------|
| `profile.efficiency_suggestion` | Emitted after a subagent run completes on a named profile with an efficiency score below 0.6. Suggest-only. Payload: `profile_name`, `run_id`, `efficiency_score`, `unused_tools`, `remove_tools` |

### Recursive Agent Spawning (4 events)

| Event | Description |
|-------|-------------|
| `spawn_agent.started` | `spawn_agent` begins executing a child agent. Payload: `task`, `depth`, `max_steps` |
| `spawn_agent.completed` | The child agent finished. Payload: `task`, `depth`, `status` (`completed`\|`partial`\|`failed`), `summary` |
| `task.completed` | A subagent called `task_complete`. Payload: `status`, `summary`, `depth`, `findings_count` |
| `step_budget.pressure` | A subagent's step budget is running low; a warning was injected. Payload: `steps_remaining`, `depth` |

### Max Turns (1 event)

| Event | Description |
|-------|-------------|
| `max_turns.exhausted` | An agent exhausted its `MaxTurns` budget; the run terminates with `run.failed` (reason `max_turns_exhausted`). Payload: `run_id`, `step`, `turn_count`, `max_turns` |

## Emitted but not in `AllEventTypes()`

`internal/harness/plan_mode.go:75,104,107` emits `plan.approval_required`,
`plan.approval_granted`, and `plan.approval_denied` on the plan-mode exit
checkpoint, but `AllEventTypes()` (`internal/harness/events.go:392`) does not
list them. A client enumerating known events from `AllEventTypes()` alone will
not recognize these three even though the runner sends them on real plan-mode
runs. This is a gap in `AllEventTypes()`, not in this document; tracked for a
follow-up fix, not corrected here (docs-only scope).

Payload shapes actually emitted:
- `plan.approval_required`: `{ "tool": "plan_exit", "plan": "string", "options": [] }` (`options` only present when the plan has a parsed "## Approaches" section)
- `plan.approval_granted`: `{ "plan": "string", "option": "string (optional)", "option_label": "string (optional)" }`
- `plan.approval_denied`: `{ "plan": "string" }`

## SSE Wire Format

Each event is sent as an SSE block:

```
id: <runID>:<seq>
retry: 3000
event: <event-type>
data: <JSON-encoded Event object>

```

Event IDs use the format `{runID}:{seq}` where `seq` is a 0-based contiguous index into the run's event history. For example: `run_1:0`, `run_1:1`, `run_1:2`, etc. Each run's sequence starts at 0.

The `Event` JSON envelope:
```json
{
  "id": "run_1:0",
  "run_id": "run_1",
  "type": "event.type",
  "timestamp": "RFC3339",
  "payload": { ... }
}
```

## Reconnection

SSE clients can reconnect by sending the `Last-Event-ID` header with the exact
ID of the last event they received. Event IDs are opaque resume tokens to
clients.

For a run-scoped stream, the server skips all events through the matching
run-local sequence. For example, reconnecting to run `run_1` with
`Last-Event-ID: run_1:3` begins replay at `run_1:4`.

For `GET /v1/conversations/{id}/events`, the server resolves the complete event
ID against conversation-wide append order. It can therefore replay completed
and live runs in order without confusing `run_1:3` with `run_2:3`. A client
must retain the complete ID rather than parsing or comparing the sequence
suffix.

Conversation replay is bounded. When another page remains, the response
includes `X-Harness-Conversation-Replay: more` and closes after the current
page; the SSE client reconnects from the last event it received. When a
non-empty cursor is no longer present in retained scoped history, the response
includes `X-Harness-Conversation-Resync: required` and replays the retained
history so the client can rebuild state.

The `retry: 3000` field tells clients to retry after 3 seconds on disconnect.

## Constants

All event types are defined as `EventType` constants in `internal/harness/events.go`. Use `IsTerminalEvent()` to check for stream-ending events. Use `AllEventTypes()` to enumerate all known types.
