# Cross-Surface Impact Map: Issue #1110

## Task

- Task / issue: #1110 test fixture synchronization for notify-parent activation.
- Plan link: `2026-08-03-issue-1110-notify-parent-activation-test-plan.md`.
- Owner: Codex.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry points: `Runner.StartRun`; `notify_parent_activation_test.go`.
- Owning packages/types/functions and source of truth: `Runner.StartRun`
  activates the named deferred tool before asynchronous execution;
  `RegistryActivation` owns the transient activation lifetime.
- Callers, consumers, events, and downstream data: first provider
  `CompletionRequest.Tools` is the behavioral boundary; terminal cleanup makes
  a later direct registry read intentionally non-authoritative.
- Similar abstractions searched: gated and capturing providers in
  `runner_compact_instruction_test.go`, `callback_bridge_test.go`, and
  `runner_terminal_store_test.go`.
- Search commands/evidence: `rg -n "notify_parent|ParentContextHandoff|gated|captur" internal/harness`.
- Duplication/ownership conclusion: add one small local test provider; no
  production seam needs modification.

## Config, API, CLI, and Tools

- User-facing config added or changed: None; test-only.
- Defaults / fallbacks: None.
- Environment variables, config files, or saved settings touched: None.
- Endpoints, request fields, response fields, or server wiring affected: None.
- CLI commands, tools, wire formats, or integrations affected: no change;
  test inspects existing provider tool request.
- Error states / validation changes: None.

## Persistence and Compatibility

- Schemas, migrations, caches, generated data, or ownership changes: None.
- Backward/forward compatibility and versioning: None.
- Partial rollout and mixed-version behavior: None; tests only.

## Lifecycle, Security, and Reliability

- Concurrency, cancellation, retries, cleanup, and resource ownership: the
  provider gate supplies a real in-flight observation point and releases to
  prove normal terminal cleanup; no sleep-based timing.
- Authentication, authorization, permissions, trust, privacy, and secrets:
  None; test uses no credentials or external services.
- Failure modes, recovery, idempotency, and data repair: test now distinguishes
  correct terminal cleanup from missing pre-execution activation.

## Product and Integration Surfaces

- Server/runtime: test covers the Runner-to-provider request boundary only.
- TUI/web/macOS/other clients: None changed; they consume existing run events.
- Provider/model/tool catalog and routing: provider fixture captures the
  existing `notify_parent` definition; no catalog/routing change.
- External systems and automation: None.
- UX states, keyboard/focus/accessibility/motion: None.

## Deployment and Operations

- Deployment/migration order and feature flags: None.
- Logs, metrics, traces, alerts, and support diagnostics: deterministic test
  failure names missing first-request activation rather than post-terminal state.
- Rollback triggers and recovery steps: revert this test-only PR if it causes
  an unexpected test failure; production behavior is untouched.
- Runbooks and operator docs: test/log evidence only.

## Regression Tests

- Characterization and first expected red test: old instant provider's
  post-`StartRun` state is race-prone under `-race`; capture its repeated run.
- New acceptance tests required: gated first-request positive + terminal
  activation cleanup.
- Edge, negative, failure, lifecycle, and security tests: retained no-parent
  and empty-parent negatives; terminal cleanup after gate release.
- Integration/e2e/real-path proof: adjacent parent handoff persistence test.
- Cross-surface regressions to guard: tools included in the actual first
  provider request, not a transient state observed after terminal completion.
- Exact targeted and full commands: focused normal/race repetitions, adjacent
  parent-handoff test, `./scripts/test-regression.sh`.

## Documentation and Handoff

- Specs/public docs before code: this plan and map; no public docs change.
- Implementation notes/logs/indexes after code: engineering and observational
  log entries plus plans index.
- Training/onboarding/release notes: None; no product behavior changes.
