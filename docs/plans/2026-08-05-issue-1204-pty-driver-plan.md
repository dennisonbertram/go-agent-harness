# Plan: Issue #1204 real PTY continuation evidence

## Context

- Governing GitHub issue: #1204.
- Problem: reducer/package tests and a scripted fake completion do not prove that an actual `harnesscli -tui` terminal renders a resumed or continued reply.
- User impact: a green fake-only or raw-byte fixture could mask a broken interactive continuation path.
- Constraints: test/acceptance infrastructure only; use a fake provider, disposable binaries, and caller-owned artifacts. Do not change real-provider behavior or user configuration.

## Scope

- In scope: a real Unix PTY drives `/resume <source-run>` and `/continue <source-run>` after a source reply; it records typed keystrokes, raw terminal bytes, ANSI/VT-interpreted visible screen, child SSE, and API/store correlation.
- In scope: an explicit fake-turn `deltas` JSON seam so the real daemon emits the child `assistant.message.delta` required by the acceptance contract.
- Out of scope: provider routing, production TUI semantics, public API schema, persistence migrations, macOS GUI, and web clients.

## Documentation Contract

- Feature status: in implementation; acceptance evidence and regression verification are still required.
- Public docs affected: none; this is internal test infrastructure.
- Spec docs to update before code: this plan and `2026-08-05-issue-1204-pty-driver-impact-map.md`.
- Implementation notes to add after code: durable logs and their indexes, with actual command results and retained-artifact locations.

## Test Plan (TDD)

- First red: a real PTY `/resume` or `/continue` cannot establish a same-conversation child run, visible continuation reply, one child delta, terminal SSE, and durable API/store linkage.
- Acceptance: build disposable `harnessd` and `harnesscli`; start a fake-only daemon; complete a source turn; type each command through `script(1)`; require a distinct child run in the source conversation and the rendered reply.
- Fixture regressions: wait for source text rather than startup escape bytes; interpret the alternate screen/ANSI updates rather than raw substring matching; retain the last visible frame before Bubble Tea's blank redraw; preserve UTF-8 wide and combining glyph cell behavior; establish explicit terminal geometry.
- Portability regressions: Darwin retains its direct `script -q /dev/null …` form; Linux uses util-linux `script -q -c <one POSIX-quoted child command> /dev/null`. A real `script(1)` sentinel must run on the host, and an early TUI child exit must fail while either semantic rendered-screen readiness or post-input child-run discovery is in progress.
- Real fixture synchronization: child-input readiness is a sentinel-owned private file rather than a transcript marker, because terminal-capture buffering is not an input-readiness guarantee under full-suite scheduling.
- Artifact/cleanup: constrain fake turns, SQLite files, HOME, logs, terminal/SSE/API probes, and keystrokes to a mode-0700 caller-owned artifact root; hash retained evidence; terminate the daemon process group and close PTY/file handles on every path. Caller/test policy decides when retained evidence is removed.
- Verification: focused normal and race test for `./internal/acceptance/ptyrunner`, then the repository regression gate. Record exact results before status becomes implemented.

## Implementation Checklist

- [x] Verify issue contract, current acceptance ownership, and source/test seam.
- [x] Record plan and cross-surface impact map before implementation completion.
- [x] Add the fake-only scripted-delta seam and its focused contract coverage.
- [x] Add real PTY `/resume` and `/continue` acceptance coverage with correlated artifacts.
- [x] Add deterministic ANSI/alternate-screen, blank-redraw, Unicode, and geometry fixture coverage.
- [x] Add Darwin/Linux `script(1)` argv and early-child-exit regressions without unsafe shell interpolation.
- [x] Run and record focused normal/race verification: `TMPDIR=/private/tmp GOCACHE=$PWD/.gocache go test ./internal/acceptance/ptyrunner -count=1` (pass, 19.578s) and `TMPDIR=/private/tmp GOCACHE=$PWD/.gocache go test -race ./internal/acceptance/ptyrunner -count=1` (pass, 20.743s).
- [x] Run and record repository regression verification: `TMPDIR=/private/tmp GOCACHE=$PWD/.gocache ./scripts/test-regression.sh` passed (normal, race, coverage `85.3%`, zero uncovered functions).
- [x] Update durable logs and plan/log indexes; do not publish public behavior until verification completes.

## Risks and Mitigations

- Risk: raw terminal bytes preserve text no longer visible. Mitigation: parse the current VT screen and retain the last frame containing the required reply.
- Risk: a source run or unrelated conversation is mistaken for the continuation. Mitigation: require distinct source/child run IDs, a shared conversation ID, child SSE lifecycle, and independent API/store probe.
- Risk: acceptance leaves state in a checkout or user home. Mitigation: disposable binaries and an explicit artifact root own all daemon state; shutdown is process-group scoped.
- Risk: BSD and util-linux `script(1)` parse command arguments differently, allowing Ubuntu to exit before a TUI exists. Mitigation: select the documented form by OS, quote every child argument as one POSIX shell command on Linux, and surface child exit during readiness rather than masking it with polling.
