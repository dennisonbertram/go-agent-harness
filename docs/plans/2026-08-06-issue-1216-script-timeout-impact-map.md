# Cross-Surface Impact Map: Issue #1216 script timeout

## Task

- Task / issue: #1216 script tool timeout under race/load.
- Plan link: `2026-08-06-issue-1216-script-timeout-plan.md`.
- Owner: `internal/harness/tools/script.makeScriptHandler`.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry points: `LoadScriptTools` creates each handler with `makeScriptHandler`.
- Owning source: `internal/harness/tools/script/loader.go` derives the timeout,
  starts the executable, supplies JSON stdin, collects stdout/stderr, and maps
  exit results to the public handler error.
- Consumers: the registry invokes the returned `tools.Handler`; the runner and
  API/TUI/native clients consume its unchanged string/error result.
- Similar abstraction: `internal/harness/tools/exec_group_unix.go` owns a
  process-group `Cancel` plus bounded `WaitDelay` for other command runners.
- Search evidence: `rg -n "makeScriptHandler|TestScriptHandler_Timeout|CommandContext|Setpgid|process group" internal/harness/tools`.
- Conclusion: one Unix process-lifecycle seam owns the defect; reuse its
  cancellation model without changing public tool discovery or invocation.

## Config, API, CLI, and Tools

- User-facing config: none; `timeout_seconds` keeps its current meaning.
- Defaults/settings: no additions or fallback changes.
- API/CLI/wire formats: none; handler result and timeout error text stay stable.
- Error/validation: preserve existing start, non-zero, stderr, and timeout paths.

## Persistence and Compatibility

- Schemas/caches/generated data: none.
- Compatibility: no request, response, or saved-tool format changes.
- Mixed rollout: safe; local process supervision changes are atomic per handler.

## Lifecycle, Security, and Reliability

- Lifecycle: handler context is sole cancellation owner; it kills the script's
  process group and bounds pipe-drain wait after the deadline.
- Security/privacy: the existing HOME/PATH-only environment stays unchanged.
- Failure/recovery: timeout returns promptly, descendants die, and normal
  exits retain stderr/error mapping. No retry or data repair applies.

## Product and Integration Surfaces

- Server/runtime: script tool calls cannot leave an active run waiting on an
  inherited stdio pipe after timeout.
- TUI/web/macOS: no direct UI change; they receive the existing tool timeout
  result promptly through normal run events.
- Provider/catalog/external automation: tool metadata and routing unchanged.
- UX/accessibility: none; no controls or rendering change.

## Deployment and Operations

- Deployment: ordinary binary rollout; no migration or flag.
- Observability: existing timeout error names the tool and configured duration.
- Rollback: revert this isolated PR if process behavior regresses.
- Runbooks: no operator procedure changes.

## Regression Tests

- First red: real script starts a PID-recorded descendant holding inherited
  stdio; old direct-child `CommandContext` cancellation makes `Wait` linger.
- Acceptance: return before five seconds after the configured timeout and prove
  the descendant is gone; the existing basic timeout test retains one second.
- Edge/failure: retain standard timeout, stderr/non-zero, and external-context
  cancellation behavior; normal and race stress cover scheduling.
- Real-path proof: executable shell script, real PID/process group, no mock.
- Commands: focused `go test ./internal/harness/tools/script -run
  TestScriptHandler_Timeout -count=20`, race equivalent, package lifecycle
  tests, then `TMPDIR=/private/tmp ./scripts/test-regression.sh`.

## Documentation and Handoff

- Specs/public docs: none; behavior is existing and unchanged at the contract.
- Logs/indexes: add engineering-log outcome and plans index entries in this PR.
- Training/release notes: none.
