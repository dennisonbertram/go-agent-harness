# Plan: Preserve Child Exit Across the Initial Workflow Write

## Context

- Governing GitHub issue: #1076.
- Problem: `SourceManager.runSourceWorkflow` returns immediately when encoding
  the initial `start` message fails, so it skips `cmd.Wait` and the existing
  terminal-outcome resolver. A child that already exited non-zero can therefore
  be reported only as `broken pipe`, without its bounded stderr diagnostic.
- User impact: real source-workflow failures lose their actionable child exit
  and stderr evidence, and the scheduling-dependent path intermittently rejects
  the accepted hosted race baseline.
- Constraints: ship separately before rebasing #1070; preserve timeout and
  semantic protocol-error precedence, standalone initial-write errors, stderr
  bounds, successful results, exact-once wait/reap, and process-group cleanup.

## Scope

- In scope: one deterministic child-exit-before-initial-write integration
  regression, outcome-resolver controls, and the smallest change that routes an
  initial write failure through bounded cleanup, wait, and shared arbitration.
- Out of scope: protocol redesign, retries or sleeps, ignoring pipe failures,
  timeout changes, stderr-bound changes, #973's invalid-protocol/nil-map defects,
  and every #1070 terminal-publication file.

## Documentation Contract

- Feature status: implemented and fully verified locally; closing PR and hosted
  checks pending parent promotion.
- Public docs affected: none; this corrects existing runtime failure provenance.
- Spec docs before code: this plan and its linked impact map.
- Implementation notes after code: engineering, observational, system, and
  long-term logs plus the plans index and active-plan tracker.

## Test Plan (TDD)

- First integration red: hold execution after `cmd.Start`; a real child writes
  stderr and exits status 7; an OS-released advisory lock proves exit before the
  initial write proceeds. Require process-exit diagnostics, bounded stderr, and
  a reaped PID instead of raw EPIPE.
- Resolver red: add `initialWriteErr` cases proving deadline and semantic
  protocol errors remain earlier, non-zero wait plus bounded stderr beats the
  write error, and a standalone write error remains visible.
- Cleanup-causality red: a live child closes stdin and remains alive; require
  the initial EPIPE to remain primary when that failure triggers process-group
  SIGKILL, while still reaping the child exactly once.
- Controls: later close-only errors, missing result, successful result, stderr
  truncation, and the existing real-child timeout/protocol paths remain pinned.
- Focused stress: lifecycle integration and resolver normal/race at
  `-count=100`.
- Package gates: complete `internal/workflow` normal and race.
- Repository gates: `make test-race`, then unchanged foreground
  `./scripts/test-regression.sh` with Command Line Tools, `/private/tmp`, and an
  isolated Go cache.
- Hosted gates: closing PR `test-fast` and `test-race`; then rebase #1070 and
  rerun its complete accepted gates.

## Cross-Surface Impact Map

- See `2026-07-31-issue-1076-workflow-initial-write-exit-impact-map.md`.

## Implementation Checklist

- [x] Link contract-complete bug #1076 before implementation.
- [x] Record current ownership, callers, sources of truth, and search evidence.
- [x] Write this plan and the impact map before production code.
- [x] Add and capture the deterministic failing lifecycle regression.
- [x] Add and capture failing resolver controls.
- [x] Add and capture the live-child cleanup-causality regression found in review.
- [x] Route the initial write failure through cleanup, exact-once wait, and the
  existing resolver.
- [x] Confirm focused stress, workflow package, CI-equivalent race, and full
  repository regression gates.
- [x] Update logs and documentation status with current exact evidence.
- [x] Commit locally with #1076 linkage; do not push or merge.
- [ ] Parent opens one closing PR with `Closes #1076` and confirms hosted gates.
- [ ] Parent merges the baseline repair, rebases #1070, and reruns its gates.

## Risks and Mitigations

- Risk: cleanup-induced errors could hide timeout, semantic protocol, or true
  process-exit evidence.
- Mitigation: pin the complete precedence table and retain one shared resolver.
- Risk: the lifecycle regression could still depend on scheduler timing.
- Mitigation: coordinate with a FIFO and an advisory lock released only by
  process exit; do not use sleeps or probabilistic repetition for the red.
- Risk: fixing the early return could leak or double-wait a child.
- Mitigation: keep one linear close/wait path, assert the real PID is reaped,
  and stress normal/race execution.
- Risk: after the initial EPIPE requests SIGKILL, that same wait signal cannot
  prove whether this cleanup or an indistinguishable concurrent actor delivered
  it.
- Mitigation: classify SIGKILL after a successful cleanup request as cleanup
  evidence and preserve the earlier EPIPE; any natural exit status, including
  exit 7, remains primary. Exact pre-kill signal provenance would require a
  broader WNOWAIT/process-supervision redesign excluded by #1076.
- Risk: diagnostics could expose unbounded child output.
- Mitigation: retain `limitedWriter` and `boundedString` and keep the existing
  truncation control green.

## Verification Evidence

- First semantic red: the deterministic exited-child test returned
  `write |1: broken pipe` instead of exit status 7 plus bounded stderr; the
  standalone resolver control returned `exited without a result`.
- Review red: the live-child test returned
  `workflow "initial-write-live-child" exited: signal: killed` instead of the
  initial EPIPE; its resolver control failed the same precedence contract.
- Focused green: both real lifecycle branches, the full resolver table, and the
  real CommandContext timeout passed together. Both child PID assertions proved
  the process was reaped.
- Focused stress: both lifecycle branches plus resolver/stderr controls passed
  normal `-count=100` in 84.986s and race `-count=100` in 90.588s.
- Package: complete `internal/workflow/...` passed normal in 13.719s and race
  in 16.534s.
- CI-equivalent race: `make test-race` passed every configured package;
  `internal/workflow` passed in 23.720s.
- Full repository regression: the unchanged foreground non-PTY
  `./scripts/test-regression.sh` passed normal, full race, and coverage;
  `coveragegate: PASS (total=85.6%, min=80.0%, zero-functions=0)`.
- Launch evidence: tmux/redirect attempts were excluded because macOS
  `security` opened the controlling terminal. The accepted run used the managed
  non-PTY foreground process with Command Line Tools, `/private/tmp`, and the
  isolated Go cache.
