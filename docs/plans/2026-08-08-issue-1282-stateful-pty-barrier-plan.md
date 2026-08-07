# Plan: Issue #1282 stateful PTY barrier evidence

## Context

- Governing issue: #1282.
- Problem: an ad-hoc sleep-driven terminal probe sent stateful slash commands after API completion without proving that the first assistant reply had rendered.
- User impact: API persistence could be mistaken for a visible TUI turn.
- Constraints: acceptance driver only; no TUI, server, provider, or persistence behavior change.

## Scope

- In: canonical owned-PTY collector, immutable frame barriers, API/store probe, and a real 100x30 stateful command batch.
- Out: product renderer changes, timeout increases, sleep-based readiness, and crediting an API-only success.

## Test Plan (TDD)

- First red: real PTY test invokes the stateful batch and requires a sealed first-reply frame before title/dashboard/workflow/tasks/undo/plugins/quit frames.
- Green: implement the batch with `freshFrameCollector.beginAction` and `waitAndSeal*` for every action.
- Regression: focused normal/race and repository regression gate.

## Implementation Checklist

- [x] Record source and failed ad-hoc probe.
- [x] Add first failing real-PTY acceptance test.
- [x] Implement the smallest acceptance-only driver.
- [x] Run focused normal/race and full regression (85.2% coverage; zero uncovered functions).

## Risks and Mitigations

- Risk: an overlay can hide a correct reply. Mitigation: seal the reply frame before writing any slash command.
- Risk: an API-complete run is treated as UI proof. Mitigation: require both rendered frame and API/store probe.
