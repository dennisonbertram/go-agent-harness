# Cross-Surface Impact Map: Issue #1252 explicit model-facing cron mode

## Task

- Task / issue: #1252 fail closed when conversational cron mode is omitted.
- Plan link: `2026-08-07-issue-1252-cron-explicit-mode-plan.md`.
- Owner: Codex.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry point: `internal/harness/tools/deferred.CronCreateTool` receives model JSON.
- Source of truth: its JSON schema and handler map explicit shell/harness input to `CronCreateJobRequest`.
- Downstream: scoped cron client persists a job; `cmd/harnessd` dispatches explicit harness jobs through `cronRunStarter` into the stored conversation.
- Search evidence: `rg 'CronCreateTool|execution_type|ScopedHarnessJobContinues' internal cmd` found the deferred boundary and the existing composed harness continuation test.
- Conclusion: validation belongs only in the deferred model tool; duplicating it in persistence or REST would change excluded operator surfaces.

## Config, API, CLI, and Tools

- User-facing config: none.
- Defaults / fallbacks: remove only model-tool omitted-mode fallback; explicit `shell` and `harness` remain.
- Endpoints/CLI: None — REST cron create and operator tools retain their existing contracts.
- Tool wire format: `execution_type` becomes required; harness additionally requires prompt, shell additionally requires command.
- Errors: omitted mode returns an actionable error before `CreateJob` is called.

## Persistence and Compatibility

- Schemas/migrations/caches: None.
- Compatibility: persisted shell jobs and explicit REST shell jobs retain their stored execution type/config and behavior.
- Mixed versions: new agents provide explicit mode; an older model call is rejected safely rather than creating a non-conversational job.

## Lifecycle, Security, and Reliability

- Lifecycle: harness path continues to use stored immutable tenant/agent/conversation/job/execution scope.
- Security/privacy: no new fields accepted from model arguments; existing scope derivation stays authoritative.
- Failure/recovery: omission fails before persistence or scheduler registration; no cleanup/data repair is needed.

## Product and Integration Surfaces

- Server/runtime: unchanged except explicit harness jobs are already dispatched through the existing path.
- TUI: must visibly receive the one child assistant message and permit a later chat turn in the same conversation.
- Web/macOS: no source change; native equivalent is recorded as the final #1010 matrix requirement, not fabricated here.
- Provider/model/tool catalog: the tool schema and embedded description are updated together.
- External automation: none.

## Deployment and Operations

- Deployment: no migration or feature flag.
- Observability: actionable tool validation response; cron history continues to distinguish shell output/no run from harness run ID.
- Rollback: revert only deferred schema/handler/description change; existing persisted jobs require no action.
- Runbooks: none beyond evidence in the PR.

## Regression Tests

- First red: `TestCronCreateRequiresExplicitExecutionType` proves omitted mode is not passed to the client and schema lists it as required.
- Acceptance: explicit shell request retains command, output/history, and no child run; existing `TestEmbeddedCron_ScopedHarnessJobContinuesOwnedConversation` proves explicit harness child ID/scope/output.
- TUI: owned 100x30 fake-provider PTY with source manifest; if Go omits VCS fields, record clean HEAD/build command/SHA limitation and do not claim Go metadata provenance.
- Commands: focused `go test ./internal/harness/tools ./cmd/harnessd -run 'TestCronCreate|TestEmbeddedCron_(ScopedHarnessJobContinuesOwnedConversation|Shell)' -count=1`; then `./scripts/test-regression.sh`.

## Documentation and Handoff

- Specs/public docs before code: plan/map and embedded tool description.
- After code: engineering log and plans index.
- Training/release notes: none; tool description is the model-facing contract.

## Warning Check

- No blank surfaces. Excluded REST, persistence, callback, and native-source work are explicitly retained unchanged.
