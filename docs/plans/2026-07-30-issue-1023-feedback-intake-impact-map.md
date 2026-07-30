# Cross-Surface Impact Map: Anytime Contextual Feedback Intake

## Task

- Task / issue: #1023
- Plan link: `2026-07-30-issue-1023-feedback-intake-plan.md`
- Owner: Codex
- Status: implemented and merged in PR #1025

## Current Ownership, Callers, and Data Flow

- Entry points: TUI `/feedback`; possible top-level `harnesscli feedback` and
  wrapper routing if the shared contract remains single-source.
- Owning packages/types/functions and source of truth:
  - `cmd/harnesscli/tui/feedback.go` owns the existing zip format;
  - `cmd/harnesscli/tui/cmd_parser.go` owns slash dispatch and arguments;
  - `tui.Model` owns active run/conversation/transcript/workspace state;
  - `cmd/harnesscli/service.go` owns daemon log path conventions.
- Callers, consumers, events, and downstream data: command registry -> feedback
  execution -> local zip -> optional `gh` issue or browser draft -> user-visible
  status.
- Similar abstractions searched: transcript exporter, run image attachments,
  service logs, GitHub profiles/skills, `symphd`, wrapper subcommands.
- Search commands/evidence: repository-wide `rg` for `feedback`, `screenshot`,
  `attachment`, `issue`, `github`, transcript, run ID, conversation ID, and log
  paths; official GitHub REST documentation via Context7.
- Duplication/ownership conclusion: extend the existing feedback builder; do
  not introduce a second diagnostics archive or a harnessd feedback endpoint.

## Config, API, CLI, and Tools

- User-facing config added or changed: None.
- Defaults / fallbacks: local bundle only; no screenshot; no issue submission.
- Environment variables, config files, or saved settings touched:
  `HARNESS_ROLLOUT_DIR` remains optional; canonical service logs are discovered
  from existing defaults.
- Endpoints, request fields, response fields, or server wiring affected: None.
- CLI commands, tools, wire formats, or integrations affected: `/feedback`
  arguments and bundle members; supported `gh issue create`/`--web` handoff.
- Error states / validation changes: explicit invalid screenshot, bundle write,
  missing `gh`, unauthenticated GitHub, repository-resolution, and browser-open
  errors.

## Persistence and Compatibility

- Schemas, migrations, caches, generated data, or ownership changes: additive
  zip members only; no database or migration.
- Backward/forward compatibility and versioning: retain existing members and
  no-argument behavior; add a bundle schema version to the context manifest.
- Partial rollout and mixed-version behavior: old clients ignore new zip
  members; new clients continue reading old feedback archives manually.

## Lifecycle, Security, and Reliability

- Concurrency, cancellation, retries, cleanup, and resource ownership: snapshot
  copied TUI values without changing run state; close/remove partial archives on
  local write failure; do not delete successful bundles after GitHub failure.
- Authentication, authorization, permissions, trust, privacy, and secrets:
  reuse redaction for all text; issue submission is explicit; screenshots are
  raw user-selected evidence; reject symlinks and non-regular files; bound size.
- Failure modes, recovery, idempotency, and data repair: repeated invocations
  create timestamped bundles/issues; local bundle path is always recoverable;
  browser draft remains user-controlled and is not reported as a created issue.

## Product and Integration Surfaces

- Server/runtime: read-only metadata and logs only; no runtime change.
- TUI/web/macOS/other clients: TUI primary; shell entry point only if shared;
  macOS UI deferred.
- Provider/model/tool catalog and routing: None — current selected model is
  metadata only.
- External systems and automation: GitHub CLI/browser only when explicit; the
  draft targets canonical `dennisonbertram/go-code` even when the TUI workspace
  is another repository.
- UX states, keyboard/focus/accessibility/motion: status covers success, local
  success plus external failure, and validation failure; no new motion/layout.

## Deployment and Operations

- Deployment/migration order and feature flags: normal CLI release; no flag or
  migration.
- Logs, metrics, traces, alerts, and support diagnostics: bounded service logs
  become archive evidence; no remote telemetry.
- Rollback triggers and recovery steps: revert new command behavior; existing
  archives remain readable.
- Runbooks and operator docs: update slash-command/CLI reference and diagnostic
  privacy guidance.

## Regression Tests

- Characterization and first expected red test: current no-argument feedback
  tests stay green while new request/context members initially fail.
- New acceptance tests required: request, active state, transcript/session
  context, screenshot validation, issue generation, and partial success.
- Edge, negative, failure, lifecycle, and security tests: secret canaries in
  every text source; malformed/oversized/symlink images; missing rollout/logs;
  missing or failing `gh`; empty request.
- Integration/e2e/real-path proof: live TUI invocation during an active or
  failed run, archive inspection, and supported GitHub handoff.
- Cross-surface regressions to guard: slash registry snapshots, wrapper routing
  if added, transcript export unchanged.
- Exact targeted and full commands:
  - `go test ./cmd/harnesscli/tui/... -count=1`
  - `go test ./cmd/harnesscli/... -count=1`
  - `go test -race ./cmd/harnesscli/... -count=1`
  - `./scripts/test-regression.sh`

## Documentation and Handoff

- Specs/public docs before code: this plan and impact map.
- Implementation notes/logs/indexes after code: plans index, active plan,
  engineering log, CLI reference, and any affected operator index.
- Training/onboarding/release notes: document local-first privacy and the
  supported browser handoff for binary issue evidence.

## Warning Check

- No heading is blank. Unaffected server, database, provider, and macOS
  surfaces have explicit rationale above.
