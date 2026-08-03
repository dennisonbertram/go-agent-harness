# Cross-Surface Impact Map: Issue #1122

## Task

- Task / issue: #1122 native interactive-state ownership.
- Plan link: `2026-08-03-issue-1122-interactive-state-plan.md`.
- Owner: Codex.
- Status: in implementation; stacked on #1118 `34219699`.

## Current Ownership, Callers, and Data Flow

- Entry points: conversation SSE reaches `RunSession.apply`; Chat and ToolWalk
  render pending state and invoke `approve`, `deny`, and `answer`.
- Source of truth: `RunSession.currentRunID`; `Transcript.pendingApproval` and
  `pendingPlan`; `RunSession.pendingQuestions` (already carries `runID`).
- Search evidence: `rg -n "PendingApproval|PendingPlan|pendingQuestions|currentRunID|approve" macapp -g'*.swift'`.
- Conclusion: pending approvals/plans lack identity, while actions currently
  resolve a fresh `currentRunID`, allowing displaced UI to address B.

## Config, API, CLI, and Tools

- Config/defaults/environment: None.
- API/wire: no server change; native client continues existing run-specific
  endpoints.
- CLI/tools: ToolWalk captures each affordance origin before action; no tool
  catalog/routing change.
- Error validation: mismatched expected/current/pending owner is a no-op before
  a Task or network call.

## Persistence and Compatibility

- No schemas, migrations, caches, or stored wire shapes change.
- Native in-memory structs gain run identity derived from the already present
  SSE event `run_id`; no compatibility migration is needed.
- Mixed client rollout is safe because server endpoints are unchanged.

## Lifecycle, Security, and Reliability

- Selection, terminal retirement, fallback, clear/reset synchronously discard
  stale A pending state and invalidate asynchronous request ownership.
- This prevents a human decision intended for A from authorizing B; no auth or
  secret boundary changes.
- Preserve #994 control generation/lifecycle ACK semantics; foreign terminals
  retain selected B interactions.

## Product and Integration Surfaces

- Server/runtime: None beyond existing SSE and action endpoints.
- macOS: Chat and ToolWalk bind visible action to captured run identity.
- TUI/web: None; their state owners are separate.
- Provider/catalog/automation: None.
- UX/accessibility: stale affordances disappear synchronously; valid controls
  retain their labels and disabled states.

## Deployment and Operations

- No migration or flag. Deploy with #1118 after its base merges and checks pass.
- Existing action endpoint logs expose any unexpected network request; no new
  metrics needed.
- Rollback: revert this stacked native-only commit; server behavior remains
  untouched.

## Regression Tests

- Expected red: pending A survives selected B and stale action can hit B.
- Acceptance: approval, plan, input A-to-B; selected terminal/no fallback;
  foreign terminal preserves B; action endpoint capture is zero for stale UI.
- Commands: focused `swift test --package-path macapp --filter RunSessionExternalControlTests`, full `swift test --package-path macapp`, and `./scripts/test-regression.sh`.

## Documentation and Handoff

- No public docs before code; plan/map and durable logs/indexes after code.
- PR records red/green commands, stack base, and no-merge handoff.
