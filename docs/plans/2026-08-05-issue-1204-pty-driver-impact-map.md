# Cross-Surface Impact Map: Issue #1204

## Task

- Task / issue: #1204 real PTY `/resume` and `/continue` acceptance evidence.
- Plan link: `2026-08-05-issue-1204-pty-driver-plan.md`.
- Owner: Codex. Status: in implementation; verification pending.

## Current Ownership, Callers, and Data Flow

- Entry points: `internal/acceptance/ptyrunner.Run`; `TestRealPTYResumeAndContinue`; `cmd/harnessd.loadFakeTurns`.
- Ownership: the PTY runner owns only disposable acceptance setup and evidence; harnessd owns fake-turn decoding; existing Runner/API/SSE/store paths remain the source of truth.
- Flow: fake source turn -> durable source run/conversation -> Darwin direct or util-linux `-c`-wrapped, POSIX-quoted `script(1)` child -> `harnesscli -tui -resume=<source>` -> typed slash command -> distinct child run -> delta/completed SSE -> current rendered screen and API/store probe.
- Search evidence: `rg -n 'resume|continue|deltas|assistant.message.delta|ArtifactRoot|renderedScreen' internal/acceptance/ptyrunner cmd/harnessd` identifies the new seam and no production-client owner.
- Duplication conclusion: `ptyrunner` is a narrow real-terminal adapter; it does not reimplement the TUI reducer, provider, or persistence.

## Config, API, CLI, and Tools

- User-facing config: none. Test-only environment pins loopback address, fake provider/turns, disposable stores/workspace/HOME, and existing prompts.
- Defaults/fallbacks: timeout defaults locally; command is strictly `resume` or `continue`; missing `script(1)` fails the acceptance run.
- API/CLI: consumes existing `/healthz`, run, event, and read APIs plus existing `-tui`, `-resume`, `/resume`, and `/continue`; no wire-schema change.
- Error/validation: missing required paths, bad command, missing PTY utility, an early `script` child exit during semantic screen readiness, readiness timeout, wrong event count, absent visible reply, or mismatched identity fail closed.

## Persistence and Compatibility

- Persistence: fake turns, logs, terminal transcript/screen, keystrokes, SSE, API/store probe, SQLite DBs, and HOME are artifact-root local; no migrations or production records.
- Compatibility: the additive fake-only `deltas` field preserves existing fixture files that omit it; real-provider payloads are untouched.
- Mixed versions: no rollout coordination; tests build the worktree binaries they exercise.

## Lifecycle, Security, and Reliability

- Lifecycle: readiness requires listener plus `/healthz`; source screen text gates input while the owned PTY child-exit channel fails promptly; child discovery gates correlation; `/quit` and process-group SIGTERM prevent orphan TUI/daemon descendants.
- Security/privacy: fake data only, loopback listener, auth disabled only inside the disposable daemon, mode-0700 artifact root and mode-0600 files; no user HOME or credentials.
- Failure/recovery: retain caller-owned artifacts for diagnosis, hash required evidence, close open handles, and leave deletion/retention to the caller/test policy.

## Product and Integration Surfaces

- Server/runtime: harnessd gains only fake fixture delta decoding; normal provider routing remains unchanged.
- TUI: exercised through its actual PTY rendering path; no TUI production code contract is changed.
- Web/macOS: None; neither launches the terminal client. Search/ownership shows no shared UI implementation is modified.
- Provider/model/tools: fake provider only; no model catalog, tool catalog, external egress, or credential path.

## Deployment and Operations

- Deployment: no flag, migration, or production rollout; normal CI/test execution consumes the package.
- Diagnostics: retained, SHA-256-bound terminal, screen, keystroke, SSE, and API/store artifacts connect visible output to lifecycle and durable state.
- Rollback: revert the acceptance package and fake fixture parsing; no data repair or operator runbook change.

## Regression Tests

- Characterization/first red: existing source/child lifecycle lacked real terminal proof and fake fixture deltas, so child stream/render assertions could not be made deterministically.
- Acceptance: both slash commands create a distinct same-conversation child, show the continuation on the interpreted screen, emit exactly one child delta and a completion, and produce all required digests.
- Edge/failure: malformed commands/paths, unavailable PTY utility, daemon health timeout, early PTY-child exit, missing render, ANSI erasure/alternate buffer, final blank redraw, and Unicode cell geometry fail rather than become false positives. A real host `script(1)` sentinel plus Linux argv/quoting regression protects both script implementations.
- Exact commands: `TMPDIR=/private/tmp GOCACHE=$PWD/.gocache go test ./internal/acceptance/ptyrunner -count=1` (pass, 19.578s); `TMPDIR=/private/tmp GOCACHE=$PWD/.gocache go test -race ./internal/acceptance/ptyrunner -count=1` (pass, 20.743s); `TMPDIR=/private/tmp GOCACHE=$PWD/.gocache ./scripts/test-regression.sh` (pass: normal, race, coverage `85.3%`, zero uncovered functions).

## Documentation and Handoff

- Specs: this map and linked plan describe internal, in-progress acceptance infrastructure only.
- Logs/indexes: four durable logs and both affected indexes record the boundary and remaining verification.
- Training/release notes: none; do not represent this as production behavior or real-provider proof.
