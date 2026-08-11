# Impact Map: #1007 external scheduled-run controls

## Task

- Task / issue: #1007, child of #1000.
- Plan link: `2026-08-03-issue-1007-external-run-controls-plan.md`.
- Owner: macOS client.
- Status: rebased implementation; final combined verification pending.

## Current Ownership, Callers, and Data Flow

- Entry points: `RunSession.load/rebind` start `conversationEvents`; `submit`
  starts a per-run stream; ChatView binds `isBusy`, pending approval/question,
  and `approve/deny/answer/steer/cancel` to RunSession.
- Source of truth: `RunSession.currentRunID`; `Transcript` owns visual
  lifecycle reduction but is not a per-run control authority.
- Consumers: `InlineRunStatus`, `ApprovalBar`, `AskUserView`, and composer
  steering all derive state/actions from RunSession.
- Search evidence: `rg -n --glob '*.swift' 'currentRunID|apply.*Event|run\\.started|approval|steer|cancel|SubmitInput' macapp`; direct inspection of
  `RunSession.swift`, `ChatView.swift`, `HarnessClient.swift`, and existing
  `RunSessionConversationStreamTests.swift`.
- Conclusion: RunSession is the sole correct binding boundary; adding a second
  active-run owner in Transcript or ChatView would race the two SSE paths.

## Config, API, CLI, and Tools

- User-facing config/defaults/environment: none.
- API/wire: existing conversation SSE and run-control endpoints only; no server
  fields/routes change. CLI/tools: none.
- Errors: action failures retain existing connection-error behavior.

## Persistence and Compatibility

- Schemas/migrations/cache: none. The in-memory active-run selection resets on
  conversation switch/reset.
- Compatibility: old harness events remain valid; self-submitted run response
  still establishes the initial identity. Replayed event IDs remain deduped.
- Mixed rollout: newer client safely ignores stale previous-conversation events;
  no server/client version dependency.

## Lifecycle, Concurrency, Security, and Privacy

- Lifecycle: control ownership is selected before accounting. A first external
  active event can bind an empty session and resume visible activity; a
  timestamp-less local start remains provisional; terminal tombstones block
  late replay resurrection. A selected terminal renders before a fallback run
  is resumed, while a foreign terminal cannot change the selected lifecycle.
- Concurrency: main-actor RunSession serializes the two SSE feeds. A stream
  delivery must be scoped to the currently selected conversation before it can
  mutate transcript, questions, or controls.
- Security/privacy: targeting remains an authenticated existing endpoint;
  client never infers cross-conversation authority from a foreign stream.

## Product Clients, Catalogs, Deployment, Compatibility

- Product clients: native macOS UI affected; API/TUI/web are unchanged.
- Provider/model/tool catalogs: none.
- Deployment/observability: no service/config change; status copy exposes a
  scheduled run as active rather than silently losing control.
- Rollback: revert the RunSession identity reducer; no durable state is left.

## Tests and Documentation

- Tests: URLProtocol-backed tests cover single-flight routing for all five
  action endpoints, foreign conversation rejection, selected and foreign
  terminal ordering, replay tombstones, first active evidence, and timestamp
  ordering. Final full macapp/repository gates remain pending.
- Docs: plan/impact plus plans index, active plan, engineering, observational,
  system, and long-term-thinking logs.
