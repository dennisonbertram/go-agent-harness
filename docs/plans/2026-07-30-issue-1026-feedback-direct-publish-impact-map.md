# Cross-Surface Impact Map: Direct GitHub Feedback Publication

## Task

- Task / issue: #1026
- Plan link: `2026-07-30-issue-1026-feedback-direct-publish-plan.md`
- Owner: Codex
- Status: implemented and verified; promotion pending

## Current Ownership, Callers, and Data Flow

- Entry points: pending TUI image chips plus `/feedback <request>`.
- Owning packages/types/functions and source of truth:
  - `components/inputarea.Attachment` owns pending local attachment paths and
    cleanup;
  - `paste_image.go` owns clipboard-to-chip conversion;
  - `executeFeedbackCommand` and `buildFeedbackBundle` own capture;
  - `feedback_context.go` owns parsing, screenshot validation, issue Markdown,
    and external `gh` execution;
  - `Model.Update` owns slash dispatch and async result application.
- Callers, consumers, events, and downstream data: input submit -> command
  registry -> synchronous bundle/sidecar snapshot -> async GitHub release
  upload -> issue create -> result message -> selective chip cleanup/status.
- Similar abstractions searched: ordinary run image encoding, transcript export,
  service logs, GitHub issue composer, GitHub release assets, top-level CLI
  routing.
- Search commands/evidence: repository-wide `rg` for feedback, attachments,
  image/media fields, command dispatch, `ClearAttachments`, issue creation, and
  release upload; current official GitHub REST and `gh` docs via Context7.
- Duplication/ownership conclusion: reuse pending chips and the feedback builder;
  no second attachment state or diagnostics format.

## Config, API, CLI, and Tools

- User-facing config added or changed: None.
- Defaults / fallbacks: `/feedback` publishes; `--local` opts out.
- Environment variables, config files, or saved settings touched: None.
- Endpoints, request fields, response fields, or server wiring affected: None.
- CLI commands, tools, wire formats, or integrations affected: `/feedback`
  syntax/help; external `gh release view/create/upload` and non-web
  `gh issue create`.
- Error states / validation changes: release provisioning, asset upload, and
  issue creation get stage-specific recovery errors; existing image validation
  remains.

## Persistence and Compatibility

- Schemas, migrations, caches, generated data, or ownership changes: no
  application schema. Durable image sidecars are added beside feedback zips.
  GitHub gains one dedicated prerelease/tag and uniquely named assets.
- Backward/forward compatibility and versioning: `--issue` and `--screenshot`
  remain accepted; `--local` preserves prior local-only behavior. Existing
  single-image bundle member names remain stable when one image is captured.
- Partial rollout and mixed-version behavior: no server dependency; old clients
  keep their existing local/browser behavior.

## Lifecycle, Security, and Reliability

- Concurrency, cancellation, retries, cleanup, and resource ownership: snapshot
  files synchronously; publish in a Bubble Tea command; preserve local evidence
  on failure; selectively remove original chips on success.
- Authentication, authorization, permissions, trust, privacy, and secrets:
  reuse authenticated `gh`; require issue and release/content write access;
  retain text redaction; upload raw image pixels and the redacted bundle as
  explicitly directed.
- Failure modes, recovery, idempotency, and data repair: release creation is
  idempotent; asset names are unique; failed stages retain local paths and
  retryable chips; no partially uploaded artifact is called a created issue.

## Product and Integration Surfaces

- Server/runtime: None — feedback captures runtime evidence read-only.
- TUI/web/macOS/other clients: TUI behavior only; no `/feedback` surface exists
  in web/macOS.
- Provider/model/tool catalog and routing: None — feedback images never enter a
  model request.
- External systems and automation: GitHub release assets and issues in the
  canonical go-code repository.
- UX states, keyboard/focus/accessibility/motion: existing attachment chips and
  Enter submission; no browser/modal/focus transfer; status covers local,
  publishing, success URL, and recovery.

## Deployment and Operations

- Deployment/migration order and feature flags: normal CLI release; first use
  provisions the asset prerelease.
- Logs, metrics, traces, alerts, and support diagnostics: local status plus
  created issue URL; diagnostic bundle remains the support artifact.
- Rollback triggers and recovery steps: revert default/publisher; keep prior
  issues/assets valid; optionally delete the dedicated release to remove stored
  evidence.
- Runbooks and operator docs: TUI usage and CLI reference only.

## Regression Tests

- Characterization and first expected red test: attached chip is invisible to
  current `executeFeedbackCommand`, and current GitHub handoff includes `--web`.
- New acceptance tests required: attachment capture, default publish,
  `--local`, multiple images, release create/reuse/upload, inline Markdown,
  direct issue URL, failure recovery, and selective cleanup.
- Edge, negative, failure, lifecycle, and security tests: missing/malformed
  images, 10 MiB limit, duplicate/captured paths, release view/create errors,
  upload errors, issue-create errors, and chips added during publication.
- Integration/e2e/real-path proof: fake `gh` command sequence plus a real TUI
  capture that creates a live issue with rendered image and downloadable zip.
- Cross-surface regressions to guard: ordinary image-run submission,
  active-run state, command registry snapshots, and local-only fallback.
- Exact targeted and full commands:
  - `go test ./cmd/harnesscli/tui/... -run 'Feedback|Attachment' -count=1`
  - `go test ./cmd/harnesscli/... -count=1`
  - `go test -race ./cmd/harnesscli/... -count=1`
  - `./scripts/test-regression.sh`

## Documentation and Handoff

- Specs/public docs before code: plan, impact map, long-term intent.
- Implementation notes/logs/indexes after code: plans index, engineering,
  system, observational logs, TUI docs/reference, and snapshots.
- Training/onboarding/release notes: document attach -> request -> created issue
  and the `--local` escape hatch.

## Warning Check

- No heading is blank. Unaffected server, database, provider, web, and macOS
  surfaces have explicit rationale above.
