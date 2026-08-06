# Cross-Surface Impact Map: Issue #1214 source-workflow protocol handshake

## Task and ownership

- Task / issue: #1214 — deterministic invalid-protocol-after-result fixture.
- Plan: `2026-08-06-issue-1214-workflow-protocol-handshake-plan.md`.
- Owner: `internal/workflow/source_test.go` fixture
  `TestSourceManagerRunWorkflowFailsOnInvalidProtocolAfterResult`.
- Status: in implementation.

## Current flow and searched evidence

- Entry: `runWorkflowToTerminal` starts the authored source workflow through
  `SourceManager.runSourceWorkflow`.
- Existing fixture: raw `fmt.Println` writes a result then a log and never
  reads stdin.
- Runtime behavior: `source.go` writes `{"type":"start"...}` before serving
  stdout; `workflowsdk.Main` reads that frame before executing the callback and
  writes the valid result.
- Search: `rg -n -i 'invalid json|invalid.*protocol|protocol.*error|EPIPE|sourceWorkflow'
  internal/workflow --glob '*_test.go'` and inspection of `source.go` and
  `pkg/workflowsdk/sdk.go` identify this fixture as the sole scope.

## Surface analysis

- Production/runtime/API/CLI/TUI/web/macOS: None. No non-test source changes.
- Protocol compatibility: unchanged. The fixture now follows the established
  start handshake before deliberately exercising the existing late-message
  rejection.
- Persistence/config/deployment/security: None. No endpoint, credential, file,
  or process-management behavior changes.
- Concurrency/lifecycle: test startup gains a causal protocol acknowledgement;
  timeout, cleanup, process-group, and terminal-error arbitration remain owned
  by existing production code.
- #1209/native scenarios: None; no file under that slice is modified.

## Verification and handoff

- Acceptance: terminal run is failed specifically for `message after terminal
  result`, including repeated Linux-compatible normal/race execution.
- Full gate: `TMPDIR=/private/tmp ./scripts/test-regression.sh`.
- Rollback: revert test-and-doc only commit.
