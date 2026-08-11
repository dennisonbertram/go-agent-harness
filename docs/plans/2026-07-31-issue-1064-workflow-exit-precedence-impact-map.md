# Cross-Surface Impact Map: Issue #1064 Workflow Exit Precedence

## Task

- Task / issue: #1064, workflow child exit masked by stdin-close broken pipe.
- Plan: `2026-07-31-issue-1064-workflow-exit-precedence-plan.md`.
- Owner: Codex.
- Status: implemented and fully verified locally; hosted checks pending.

## Current Ownership, Callers, and Data Flow

- Entry point: `SourceManager.runSourceWorkflow`.
- Source of truth: after `serveProtocol`, the function owns `protocolErr`,
  `stdin.Close`'s `closeErr`, `cmd.Wait`'s `waitErr`, the run deadline, result,
  and stderr buffer.
- Callers/consumers: the source-workflow definition registered in `Engine`;
  terminal errors become failed workflow runs and are displayed by API/CLI/TUI,
  web, and macOS clients without client-specific arbitration.
- Similar abstractions searched: `rg -n
  "broken pipe|cmd.Wait|stdin|StdinPipe|process exited|exited" internal/workflow`.
  No second source-workflow process-outcome owner exists.
- Duplication conclusion: keep one arbitration seam beside
  `runSourceWorkflow`; do not add client-specific error handling.

## Config, API, CLI, and Tools

- Config/env/defaults: none.
- Endpoints/request/response/wire formats: unchanged.
- CLI/tools/integrations: only the terminal diagnostic selected from already
  captured errors changes.
- Error states: deadline first, protocol second, non-zero child exit third,
  stdin-close cleanup fourth, then missing-result/success.

## Persistence and Compatibility

- Schemas, migrations, caches, generated data: none.
- Compatibility: successful workflows, workflow protocol, and error types stay
  structurally unchanged; affected failures become more informative.
- Mixed-version behavior: none; arbitration is process-local.

## Lifecycle, Security, and Reliability

- Concurrency/lifecycle: addresses simultaneous child exit and stdin close
  without sleeps or scheduling assumptions.
- Cancellation/retries/cleanup: unchanged; stdin is still closed and the child
  is still waited exactly once.
- Auth/permissions/privacy/secrets: none.
- Reliability: bounded stderr remains capped at
  `maxWorkflowStderrBytes`; a close-only failure remains visible.

## Product and Integration Surfaces

- Server/runtime: corrected failed-run diagnostic.
- TUI/web/macOS/other clients: display the corrected existing error string; no
  code or UX-state changes.
- Provider/model/tool catalogs and routing: none.
- External automation: hosted workflow tests become scheduling-independent.
- Accessibility/focus/motion: none.

## Deployment and Operations

- Deployment/migrations/flags: none.
- Observability: the terminal error reports the process exit and bounded stderr
  instead of cleanup noise.
- Rollback: revert if timeout/protocol precedence, close-only reporting, or
  successful result behavior changes.
- Operator runbooks: none.

## Regression Tests

- First red: deterministic dual-error arbitration with synthetic broken pipe
  plus non-zero `waitErr`.
- Acceptance controls: timeout, protocol, wait, close-only, missing-result,
  success, and bounded stderr.
- Existing real-child controls:
  `TestSourceManagerRunWorkflowFailsOnProcessExit`,
  `TestSourceManagerRunWorkflowIncludesBoundedStderrOnProcessExit`, timeout,
  and malformed-protocol tests.
- Exact gates: focused normal/race `-count=100`; complete workflow normal/race;
  foreground non-TTY `./scripts/test-regression.sh`; hosted required checks.

## Documentation and Handoff

- Plans/specs: issue-specific plan and this map.
- Logs/indexes: engineering, observational, system, long-term, and plans index.
- Public/training/release docs: none because no public contract is added.

## Warning Check

- Every cross-surface heading is resolved. Unaffected surfaces are explicitly
  named with repository-search and data-flow rationale above.
