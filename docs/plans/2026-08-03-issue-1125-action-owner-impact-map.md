# Cross-Surface Impact Map: Issue #1125

## Ownership and Data Flow

- Chat captures `currentRunID` for Stop/Composer; ToolWalk snapshots the timeout
  owner; `RunSession` validates it before local mutation, Task creation, or HTTP.
- Search evidence: `rg -n "run.cancel|run.steer|currentRunID|ToolWalk" macapp
  -g '*.swift'` located exactly the three stale action paths.

## API, Persistence, Compatibility

- No API/wire/schema/persistence change. Existing run-specific endpoints remain.
- Legacy dynamic APIs remain for programmatic compatibility; visible clients use
  expected-run overloads. Mixed client rollout is safe.

## Lifecycle, Clients, and Operations

- Stale actions cannot clear B's draft, local-force-stop B, or send B traffic.
- #994 delayed ACK and external-run isolation remain in the same RunSession path.
- Only macOS and ToolWalk change; TUI, harness, providers, deployment, and public
  docs are unaffected by source search. Rollback is a native-only revert.

## Tests

- A-to-B stale Stop, steer, and ToolWalk timeout assert zero B endpoints;
  legitimate B cancel/steer retains positive endpoint evidence.
