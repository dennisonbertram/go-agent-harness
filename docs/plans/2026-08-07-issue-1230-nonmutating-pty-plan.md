# Plan: Issue #1230 causal non-mutating TUI PTY evidence

## Context

- Governing GitHub issue: #1230 (parent #1088).
- Problem: the official direct-PTY runner proves fresh chat and resume/continue,
  but cannot causally drive the first bounded informational slash-command batch.
- User impact: a command is not proven by reducer tests; the real TUI must show
  the intended result after the actual key sequence and before the next input.
- Constraints: an isolated fake daemon, one direct PTY master reader, immutable
  frames, owner-only cleanup, and no production TUI/server behavior changes.

## Scope

- In scope: `/help`, `/cost`, `/stats`, `/config`, `/context`, `/doctor`,
  `/permissions`, `/search` plus Escape, an unknown command, and real
  `/resume` and `/continue` continuations against deterministic completed runs.
- Out of scope: mutating or destructive commands, the remaining #1088 catalog,
  GUI, core/deferred tools, cron/callback timers, and product fixes.

## Documentation Contract

- Feature status: implemented and regression-verified locally; independent
  review and merge are pending.
- Public docs affected: None; this is test-only acceptance infrastructure.
- Spec docs before code: this plan and its impact map.
- Implementation notes after code: logs, runbook, artifact guidance, and indexes.

## Test Plan (TDD)

- First red: a direct-PTY scenario contract requires one sealed visible frame
  for every listed command before it writes the next input.
- Existing tests: preserve fresh conversation and continuation evidence.
- Regression: focused normal/race acceptance tests, live fake-daemon run, then
  `TMPDIR=/private/tmp GOCACHE=/private/tmp/go-code-gocache ./scripts/test-regression.sh`.

## Cross-Surface Impact Map

See `2026-08-07-issue-1230-nonmutating-pty-impact-map.md`.

## Implementation Checklist

- [x] Verify #1088 scope and the merged direct-PTY runner.
- [x] Create PR-sized child issue #1230 with contract and impact analysis.
- [x] Record plan and impact map before source changes.
- [x] Add failing causal batch contract.
- [x] Implement the smallest runner extension.
- [x] Run focused normal/race and retain a live batch artifact bundle.
- [x] Run the full regression gate (PASS: 85.0% total coverage, zero uncovered
  functions).
- [x] Update logs/runbook/indexes.
- [ ] Push a reviewable `Closes #1230` PR.

## Risks and Mitigations

- Risk: output is observed only retrospectively. Mitigation: the sole collector
  seals a frame before each subsequent keystroke.
- Risk: overlays leave focus captured. Mitigation: explicit Escape frames and
  composer restoration assertions.
- Risk: fixture state drifts across commands. Mitigation: one owned fake
  daemon, deterministic turns, and direct API/SSE/store probes.

## Local outcome

- The first red exposed an acceptance-only stats label mismatch: the canonical
  rendered header is `Activity (last 7 days)`, not `Activity (Week)`. The
  strengthened predicate now also requires its toggle hint, one total run, and
  zero total cost before sealing; Escape is sealed before `/config` input.
- `/resume` and `/continue` share a one-shot source-run contract. The alias
  therefore targets the completed `/resume` child, whose exact terminal state
  is independently probed before typing. The live normal/race batch proves
  three distinct same-conversation runs and exactly one assistant/completed
  event for each continuation child.
