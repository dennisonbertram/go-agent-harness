# Plan: Issue #1268 PTY EOF Drain Before Cleanup

## Context

- Governing GitHub issue: #1268.
- Problem: the real acceptance runner's successful quit paths close the PTY master after `Cmd.Wait` but before the collector sees EOF. Closing a master can cancel an in-flight read and discard the child’s terminal tail, making retained visual evidence incomplete.
- User impact: a TUI acceptance run can report raw-empty or incomplete final output despite a completed child, weakening the GUI/TUI/harness evidence required by the epic.
- Constraints: retain Linux slave-close `EIO` normalization; do not add sleeps, increase timeouts, or change production harness/TUI behavior.

## Scope

- In scope: the successful finalization ordering in fresh and non-mutating acceptance PTY runners, plus the Linux PTY integration fixture and durable logs.
- Out of scope: abort/error cleanup ordering, production `harnesscli` behavior, daemon/API/SSE semantics, tool implementations, and timeout policy.

## Documentation Contract

- Feature status: `implemented` in this closing PR, pending merge.
- Public docs affected: none; this is acceptance-runner evidence integrity.
- Spec docs to update before code: this plan and its impact map.
- Implementation notes to add after code: durable engineering, observational, system, and intent logs with index entry.

## Test Plan (TDD)

- First changed regressions: `TestCloseMasterBeforeProcessCleanup` fails to compile before the ordered cleanup helper exists and then pins master-close before process cleanup; `TestLinuxPTYSlaveCloseDrainsAsCleanEOF` waits with `context.Background`, asserts final child bytes, and only then permits deferred master cleanup.
- Existing tests to preserve: the injected `n > 0` plus `EIO` collector contract and arbitrary-read-error failure contract.
- Regression tests required: repeated focused normal/race package runs, package normal/race, and `./scripts/test-regression.sh`.

## Cross-Surface Impact Map

- See `2026-08-07-issue-1268-pty-eof-drain-impact-map.md`.

## Implementation Checklist

- [x] Verify structured issue, source ownership, and both affected success paths.
- [x] Record plan and impact map before source change.
- [x] Change final successful order to child wait, collector EOF, final frame sealing, deferred master cleanup.
- [x] Update Linux integration fixture to retain final bytes before cleanup.
- [x] Preserve error/abort fast-close behavior and existing EIO contract coverage.
- [x] Update durable logs/indexes and execute focused through full regression gates.
- [ ] Commit, push, and open one closing PR for #1268.

## Risks and Mitigations

- Risk: waiting for EOF could mask a broken collector. Mitigation: use the existing bounded caller context and retain non-EIO read-error propagation.
- Risk: abort cleanup waits on a process before unblocking its collector. Mitigation: one helper explicitly closes the master before process cleanup; the ordering is unit tested, while success does not invoke it until after EOF/frame sealing.
