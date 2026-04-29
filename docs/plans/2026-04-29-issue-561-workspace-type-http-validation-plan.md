# Plan: Issue #561 Workspace Type HTTP Validation

## Context

- Problem: `POST /v1/runs` can accept malformed or unsupported `workspace_type` values and return a queued run before workspace provisioning later fails through SSE events.
- User impact: CLI and HTTP clients that only inspect run creation see success even though the request was not satisfiable by the server.
- Constraints: Preserve the existing runner event flow for valid workspace configurations, keep validation narrow to run creation, and follow strict TDD.

## Scope

- In scope:
  - Reject unknown explicit `workspace_type` values at `POST /v1/runs` time with HTTP 400.
  - Reject explicit `worktree` requests when the runner lacks a configured base repository.
  - Return a `workspace_unsupported` error body with remediation text.
  - Keep runner-side validation as a guard for non-HTTP callers.
- Out of scope:
  - Redesigning workspace provisioning.
  - Changing profile-based workspace isolation resolution.
  - Making Docker or VM cloud readiness checks perform network or daemon calls during request validation.

## Documentation Contract

- Feature status: `implemented`
- Public docs affected: none.
- Spec docs to update before code: this plan and long-term intent log.
- Implementation notes to add after code: engineering/system logs updated after implementation.

## Test Plan (TDD)

- New failing tests to add first:
  - `POST /v1/runs` with an unknown explicit `workspace_type` returns HTTP 400 and `workspace_unsupported`.
  - `POST /v1/runs` with `workspace_type=worktree` and no configured repository returns HTTP 400 with a `HARNESS_WORKSPACE` remediation hint.
  - `POST /v1/runs` with `workspace_type=container` or `workspace_type=vm` on standalone `harnessd` returns HTTP 400 with a remediation hint instead of queueing.
  - A valid `workspace_type=local` request still returns HTTP 202 and later emits `workspace.provisioned`.
- Existing tests to update:
  - Runner workspace selection tests if the shared validation helper changes error wording.
- Regression tests required:
  - `go test ./internal/server -run TestPostRunWorkspaceType`
  - `go test ./internal/server ./internal/harness`
  - `./scripts/test-regression.sh`

## Implementation Checklist

- [x] Define acceptance criteria in tests.
- [x] Write failing HTTP tests first.
- [x] Implement minimal runner preflight validation available to the HTTP handler.
- [x] Map workspace validation failures to `workspace_unsupported` HTTP 400 responses.
- [x] Reject standalone-unconfigured `container` and `vm` workspace requests before run creation.
- [x] Preserve valid `workspace.provisioned` event flow.
- [x] Update engineering/system logs after implementation.
- [ ] Run targeted and regression validation. Targeted validation passed; full regression is blocked by the repository total-coverage gate.

## Validation Notes

- Passing targeted commands:
  - `TMPDIR=$PWD/.tmp/tmp GOCACHE=$PWD/.tmp/go-build go test ./internal/server -run TestPostRunWorkspaceType -count=1`
  - `TMPDIR=$PWD/.tmp/tmp GOCACHE=$PWD/.tmp/go-build go test ./internal/harness -run 'TestValidateWorkspaceType|TestRunRequest_WorkspaceType_(Unknown|WorktreeMissingRepoPath|Local)' -count=1`
  - `TMPDIR=$PWD/.tmp/tmp GOCACHE=$PWD/.tmp/go-build go test ./internal/checkpoints -count=1`
  - `TMPDIR=$PWD/.tmp/tmp GOCACHE=$PWD/.tmp/go-build go test ./internal/workingmemory -count=1`
  - `tmux new-session -d -s symphony-package-561 '... go test ./internal/server ./internal/harness -count=1 ...'`
  - `TMPDIR=$PWD/.tmp/tmp GOCACHE=$PWD/.tmp/go-build go test ./internal/workspace -run TestContainerWorkspace_Provision_Success -count=1`
  - `TMPDIR=$PWD/.tmp/tmp GOCACHE=$PWD/.tmp/go-build go test ./internal/checkpoints ./internal/harness ./internal/networks ./internal/workflows -run 'Test(ServiceResolutionHelpersAndStores|SQLiteStoreUpdatesAndQueriesWorkflowPending|CheckpointApprovalBrokerDenyResolvesPendingApproval|EngineExecutesSequentialRolesViaWorkflows|EngineDefinitionSubscriptionAndFailurePaths|SQLiteStorePersistsWorkflowRunsStepsAndEvents|WorkflowHelpersCoverFallbacks)' -count=1 -timeout=30s`
  - `TMPDIR=$PWD/.tmp/tmp GOCACHE=$PWD/.tmp/go-build go test ./internal/server -run 'Test(HandleGetCheckpoint|HandleWorkflowEventsStreamsHistory|MustJSONFallbacks|PostRunWorkspaceType)' -count=1 -timeout=30s`
- Blocked command:
  - `tmux new-session -d -s symphony-regression-561c '... ./scripts/test-regression.sh ...'` completed normal, race, and coverage package phases, then failed the total coverage gate with `72.7%`, below the required `80.0%`; no zero-coverage functions remain in the generated profile.

## Risks and Mitigations

- Risk: HTTP and runner validation drift apart.
- Mitigation: keep the capability check in the runner package and call it from HTTP before `StartRun`.
- Risk: Request-time validation performs slow or environment-sensitive Docker/VM checks.
- Mitigation: limit synchronous validation to deterministic configuration and registration checks.
