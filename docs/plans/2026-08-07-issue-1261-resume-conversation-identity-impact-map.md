# Cross-Surface Impact Map: Issue #1261 Resume Continuation Conversation Identity

## Task

- Task / issue: #1261 — retain continuation conversation identity after `/resume`.
- Plan link: `2026-08-07-issue-1261-resume-conversation-identity-plan.md`.
- Owner: isolated #1261 PR.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry points: `Server.handleRunContinue`, `continueRunCmd`, `RunStartedMsg`, and `Model.Update` terminal conversation bridge start.
- Source of truth: `harness.Run.ConversationID`; `ContinueRunWithOptions` creates child run C with inherited conversation P.
- Consumers: the blank TUI reducer selects a conversation, then `startConversationSSE` constructs `/v1/conversations/{conversation}/events`.
- Search evidence: `rg -n "handleRunContinue|continueRunCmd|RunStartedMsg|startConversationSSE" internal/server cmd/harnesscli/tui`; source test and production locations are recorded in #1261.
- Conclusion: preserve P through the existing command/reducer protocol; do not infer it from C.

## Config, API, CLI, and Tools

- User-facing config: none.
- API: additive `conversation_id` field on existing accepted continuation response.
- CLI: `/resume` obtains and carries the identity; fresh run behavior remains unchanged.
- Errors: old servers require a child-run GET resolution; missing/unresolvable identity is explicit failure rather than unsafe C fallback.

## Persistence and Compatibility

- Schema/migration/cache: none.
- Compatibility: new client accepts old response by querying `GET /v1/runs/C`; old clients ignore additive field. Mixed version never silently uses C as conversation ID.
- Rollout: server-first is ideal but client compatibility guards old server responses.

## Lifecycle, Security, and Reliability

- Lifecycle: selected-conversation stream starts only against authoritative P; terminal response retains existing run SSE behavior.
- Security/privacy: uses authenticated existing run GET and conversation SSE routes; no new permissions or data fields.
- Recovery: a failed legacy lookup returns a user-visible continuation error and does not start a false stream.

## Product and Integration Surfaces

- Server/runtime: continuation JSON only.
- TUI: direct reducer and two SSE lifecycle paths.
- GUI/native: no source change; consumes durable conversation behavior independently.
- Provider/model/tool catalog/external automation: none; no scheduler changes.
- UX: no false "run not found" warning after a successful continuation.

## Deployment and Operations

- Deployment: additive server response; deploy server then client where possible.
- Observability: TUI error names identity resolution instead of misreporting a conversation 404 as a missing run.
- Rollback: revert only this PR; continuation persistence and scheduler state are unchanged.
- Runbooks: no operator workflow change.

## Regression Tests

- First red: inherited parent conversation absent from server continue response; TUI endpoint uses child ID before repair.
- Acceptance: response/client/reducer/legacy lookup and no-false-warning lifecycle tests.
- Edge/failure: fresh run still falls back to run ID; mismatched existing selected conversation is preserved; legacy lookup error fails closed.
- Real proof: 100x30 blank-TUI `/resume` against a child C in parent P, assistant reply visible and no 404.
- Commands: targeted normal/race server and TUI packages; `TMPDIR=/private/tmp GOCACHE=/private/tmp/gocode-1261-cache ./scripts/test-regression.sh`.

## Documentation and Handoff

- Specs/public docs before code: plan and impact map only; public documentation is unchanged.
- After code: durable logs/index and PR evidence with exact PTY artifact path.
- Training/onboarding/release notes: none.

## Warning Check

Every surface is mapped; unaffected scheduler/native GUI/provider/schema surfaces are explicitly identified above.
