# Issue #1009 — Cross-Surface Impact Map

## Task

- Task / issue: #1009 macOS scheduled-task lifecycle and controls.
- Plan link: `2026-08-04-issue-1009-macapp-task-lifecycle-plan.md`.
- Owner: Codex implementation worktree.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry points: `internal/server/http_tasks.go` creates `Task`; existing
  `http_cron.go` and callback action handler own mutation; macOS
  `HarnessKit/ClientTasks.swift` decodes and `ActivityView.swift` renders.
- Source of truth: cron client/status/executions and callback manager state;
  the server calculates task action availability. `ProjectSession.refreshActivity`
  owns macOS reconciliation.
- Search evidence: `rg -n "TaskInfo|/v1/tasks|TaskAction|Activity" internal cmd macapp`.
- Conclusion: extend the common task DTO rather than duplicate a parallel
  scheduled-task listing; task-specific fields remain optional.

## Config, API, CLI, and Tools

- No configuration, CLI command, tool schema, or provider routing change.
- Additive `/v1/tasks` fields: schedule timing, latest execution state/run/error
  and callback due time; existing cron/callback endpoints remain action targets.
- Mac client sends optional row `updated_at` as its opaque raw server string in
  scoped cron action JSON while preserving empty legacy action bodies; server maps stale/invalid action
  state to 409. Errors are retained as `HarnessError` and no client-side
  permission inference is trusted.

## Persistence and Compatibility

- No migration. Cron values originate from persisted cron storage; callback
  `updated_at` is now selected/scanned from its existing durable column for
  each lifecycle transition.
- Older server payloads omit all new optionals; unknown kind/state/action raw
  values preserve display rather than causing decode failures.
- Mixed-version clients use existing coarse row fields and show no absent action.
- No-store callback managers retain terminal map rows for the all-state task
  API, but their `byConv` compatibility index remains active-only so capacity
  accounting and agent-facing conversation lists do not regress.

## Lifecycle, Security, and Reliability

- No new goroutines/timers. Activity polling remains bounded to the visible view.
- Existing runs:read/runs:write and tenant checks enforce authority. UI refreshes
  after either action success or failure so stale action sets are not retained.
  Cron pause/resume require the matching current state and an optional CAS
  version; linked run controls require reducer-admitted live event evidence.
- Server-safe errors are surfaced; delete needs confirmation; cancel/pause/resume
  depend only on advertised actions.

## Product and Integration Surfaces

- Server/runtime: task union gets lifecycle projection only.
- macOS: typed models, action client, task detail/controls, VoiceOver labels,
  linked-run navigation/selection where available.
- TUI/web and external systems: none; existing API remains backward compatible.
- Provider/model/tool catalog: none; rendering never fabricates assistant output.

## Deployment and Operations

- Deploy server before clients (additive reads); rollback clients preserves
  read-only task display. Roll back server fields without breaking clients.
- API task rows provide operator diagnostics; no new metrics or secrets.

## Regression Tests

- Red: task lifecycle projection and typed Swift action request/decode tests.
- New tests: cron active/paused lifecycle, exact nine-digit `updated_at`
  list-to-action preservation plus stale truncated-token CAS rejection,
  stale/current CAS action state,
  no-store callback cancel/fire/shutdown terminal timestamp and all-state task
  visibility with legacy active-list exclusion,
  callback durable update-time projection/transition, versioned/legacy action
  bodies, active/terminal/missing linked-run navigation, reconciliation after
  success/failure, unknown task values, accessible labels.
- Exact commands: `go test ./internal/server -run 'TestTasks|TestCron' -count=1`,
  race equivalent, `swift test --package-path macapp`, and full regression.

## Documentation and Handoff

- Update plan/log indexes and engineering log once implemented. No public
  assertion of full native proof until #1010 uses an exact current artifact.
