# Cross-Surface Impact Map: Issue #1136

## Task and ownership

- Issue: #1136 immutable submitted-run timeout authority.
- Source of truth: a `RunSubmission` created by its owning `RunSession`.
- Search evidence: `RunSubmission`, `consumeTimeoutCancellation`,
  `cancelTimedOutSubmission`, and `submissionStreamTasks` under `macapp/`.

## Surfaces

- Native model: private owner UUID plus reset/load generation and lifecycle
  form the A-only capability. A package-visible ticket with a fileprivate
  initializer prevents public callers from constructing pre-deadline authority.
  ToolWalk-only submission binds the immutable duration; `markStarted` derives
  the deadline. `RunSession.submissionTimeoutGate(for:)` is the only package
  gate and refuses pre-deadline or duplicate minting.
- ToolWalk: consumes that ticket only through the GoCodeUI gate. Its
  timeout is transport-only; it
  cannot alter B/C selection, transcript, controls, or cancellation state.
  `waitForTerminal` receives only a poll interval; the configured timeout is
  bound before start and cannot be contradicted by a later wait argument.
- HTTP/API: unchanged existing `POST /v1/runs/{A}/cancel` endpoint only.
- Persistence, harness, TUI, schema, CLI, providers: none; search found no
  changed contract or stored state.

## Reliability, test, and rollback

- Concurrent A SSE, B/C conversation events, delayed ACK, timeout, and reset
  are scoped by immutable handle. Terminal/failure/reset/load make later A
  dispatch impossible; displacement deliberately does not.
- Gated URLProtocol integration uses actual `RunSession.submit()` and proves
  no ticket/action before deadline; B -> C -> A sends exactly one A cancel;
  duplicate, terminal, failure, and reset consume attempts fail; zero B/C
  actions; and reset stops both concurrent A/C event streams. The test-only
  internal RunSession clock seam freezes the same monotonic source used by
  `markStarted` and the gate, removing wall-clock sleep races.
- Rollback is the stacked native PR; no data migration or server rollback.
