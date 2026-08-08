# Plan: Issue #1280 shared same-daemon acceptance lifecycle

## Context

- Governing issue: #1280, child of #1279 and epic #1000.
- Problem: API/SSE inventory acceptance and PTY acceptance each own a fake daemon, so they cannot establish one scheduled continuation across both surfaces.
- User impact: later cron/callback proof can bind API, SSE, and TUI evidence to one daemon identity.
- Constraints: tooling only; no scheduling semantics, GUI work, production runtime change, or #1010 completion claim.

## Scope

- In: one owned inherited-listener daemon, isolated workspace/stores, source/config provenance, public API/SSE URL, PTY settings, controlled teardown.
- Out: cron/callback execution, remote cronsd, native proof, server API changes, and closing #1279/#1010.

## Documentation Contract

- Feature status: implemented pending regression/review.
- Public docs affected: None; internal acceptance tooling only.
- Spec docs before code: this plan and impact map.
- Notes after code: engineering and long-term logs.

## Test Plan (TDD)

- First failing tests: API/SSE/PTY same-daemon identity, source mismatch before spawn, and occupied-listener non-interference.
- Preserved red: `go test ./internal/acceptance/scheduledlifecycle -run '^TestStart' -count=1` failed with undefined `Start` and `Config`.
- Required gates: focused normal/race then `./scripts/test-regression.sh`.

## Cross-Surface Impact Map

See [2026-08-08-issue-1280-same-daemon-lifecycle-impact-map.md](2026-08-08-issue-1280-same-daemon-lifecycle-impact-map.md).

## Implementation Checklist

- [x] Structured issue and architecture search evidence.
- [x] TDD red test.
- [x] Minimal lifecycle implementation.
- [x] Docs/logs/indexes.
- [x] Focused normal/race, including independent-review regressions for child exit, multi-observer completion, resource-environment scrubbing, and executable provenance.
- [x] Full regression after review repair (`./scripts/test-regression.sh`: PASS, 85.1%, zero uncovered functions).
- [x] P1 cleanup sequencing regressions: pre-exited child receives no signal; stubborn child is reaped after SIGKILL before `Close` returns.
- [ ] Reviewable PR with `Closes #1280`; no merge in this slice.

## Risks and Mitigations

- Listener collision or unrelated teardown: inherit the exact reserved TCP descriptor and signal only the recorded child process group.
- Cross-daemon evidence: expose one returned URL for API/SSE and PTY attachment, recording provenance with artifacts.
