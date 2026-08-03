# Cross-Surface Impact Map: Issue #1136

## Task and ownership

- Issue: #1136 immutable submitted-run timeout authority.
- Source of truth: a `RunSubmission` created by its owning `RunSession`.
- Search evidence: `RunSubmission`, `consumeTimeoutCancellation`,
  `cancelTimedOutSubmission`, and `submissionStreamTasks` under `macapp/`.

## Surfaces

- Native model: private owner UUID plus reset/load generation and lifecycle
  form an unforgeable, one-shot A cancellation capability.
- ToolWalk: invokes only the handle API. Its timeout is transport-only; it
  cannot alter B/C selection, transcript, controls, or cancellation state.
- HTTP/API: unchanged existing `POST /v1/runs/{A}/cancel` endpoint only.
- Persistence, harness, TUI, schema, CLI, providers: none; search found no
  changed contract or stored state.

## Reliability, test, and rollback

- Concurrent A SSE, B/C conversation events, delayed ACK, timeout, and reset
  are scoped by immutable handle. Terminal/failure/reset/load make later A
  dispatch impossible; displacement deliberately does not.
- Gated URLProtocol integration uses actual `RunSession.submit()` and proves
  B -> C -> A sends exactly one A cancel, zero B/C actions, and reset stops
  both concurrent A/C event streams.
- Rollback is the stacked native PR; no data migration or server rollback.
