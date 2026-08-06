# Cross-Surface Impact Map: Issue #1221 fresh PTY acceptance

## Task

- Task / issue: #1221, parent #1088.
- Plan link: `2026-08-06-issue-1221-pty-fresh-plan.md`.
- Owner: `internal/acceptance/ptyrunner`.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry points: `Run`, `ptyCommandArgsForOS`, and real PTY tests in
  `internal/acceptance/ptyrunner`.
- Source of truth: runner owns fake turns, daemon process group, `script(1)`,
  VT rendering, and artifact correlation; TUI/server own product behavior.
- Data flow: fake turns -> harnessd HTTP/SSE -> `script` PTY -> harnesscli ->
  raw terminal -> VT milestone screens -> run/conversation/message probes.
- Search evidence: `rg -n 'ptyCommandArgs|RunEvidence|startAndComplete|search'
  internal/acceptance/ptyrunner cmd/harnesscli/tui`.
- Conclusion: extend the existing launcher; do not duplicate an ad-hoc shell
  driver or bypass public HTTP/SSE boundaries.

## Config, API, CLI, and Tools

- User-facing config: None.
- Defaults/fallbacks: launcher retains 30 rows, 100 columns on BSD and Linux.
- Env/config: fixture-only daemon variables remain inside ArtifactRoot.
- API/wire: existing run, run-events, and conversation-message reads only.
- CLI/tools: real `harnesscli -tui`, normal prompt submission, and `/search`.
- Errors: zero/missing geometry is invalid runner evidence, never a TUI fix.

## Persistence and Compatibility

- Schemas/migrations: None.
- Compatibility: continuation runner and command registry are unchanged.
- Mixed rollout: None; tests are local/CI infrastructure only.

## Lifecycle, Security, and Reliability

- Lifecycle: wait for each semantic render/run boundary; retain PTY exit
  checks; terminate only runner-owned daemon/process groups.
- Security/privacy: no credentials; fake content only; private mode-0700 root;
  no user HOME/workspace writes.
- Recovery: failed runs retain caller-owned artifacts; rollback is revert-only.

## Product and Integration Surfaces

- Server/runtime: exercised through real isolated HTTP/SSE only; unchanged.
- TUI: real typed prompts and `/search`, visible via VT interpretation.
- Native/web: None; #1089/native GUI remains separate.
- Provider/catalog: fake provider only; no model routing change.
- Automation: cron/callback explicitly out of scope.

## Deployment and Operations

- Deployment: None.
- Diagnostics: hashed raw terminal, keystrokes, SSE, API/store artifacts.
- Rollback: revert isolated runner/docs PR; no persisted production data.
- Runbooks: acceptance inventory/logs document evidence boundary.

## Regression Tests

- First red: fresh runner test references absent `RunFreshConversation`.
- New tests: launcher geometry and actual fresh two-turn/search PTY path.
- Edge/failure: runner rejects a non-geometry launch and observes PTY exit.
- Real proof: focused normal/race runner test with disposable binaries.
- Commands: `go test ./internal/acceptance/ptyrunner -count=1`, race variant,
  and `TMPDIR=/private/tmp ./scripts/test-regression.sh`.

## Documentation and Handoff

- Specs before code: this map and plan.
- After code: plan checklist, engineering/observational/system/long-term logs
  and plans/logs indexes.
- Release notes: None; no product behavior changes.

## Warning Check

Every surface is mapped; all unaffected surfaces explicitly state None or a
separate owning issue.
