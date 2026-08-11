# Plan: Issue #1174 TUI `/init` SSE completion write

## Context

- Governing GitHub issue: #1174.
- Problem: a real HTTP/SSE `run.completed` finalizes `/init` output but does not persist it.
- User impact: the transcript can show generated instructions that the next turn cannot consume.
- Constraints: one TUI/workspace-write slice; no server, provider, profile, or user-global configuration changes.

## Scope

- In scope: bind `/init` to its accepted run ID, consume final `assistant.message`, write only a matching successful terminal, and commit atomically with a re-stat conflict fence.
- Out of scope: harness protocol, model/tool behavior, profile storage, and GUI changes.

## Documentation Contract

- Feature status: implemented locally; pending independent review and hosted CI.
- Public docs affected: none; this repairs an existing slash command.
- Spec docs to update before code: this plan and its impact map.
- Implementation notes to add after code: durable logs and indexes.

## Test Plan (TDD)

- New failing tests: an actual `assistant.message` followed by `SSEDoneMsg{run.completed}` writes exact content; failed/fatal terminals (including an unbound startup failure and malformed successful response without `run_id`), trimmed accepted run identities, exhausted reconnect loss, confirmed local Ctrl+C/Escape cancellation, and an appearing file never write or overwrite.
- Existing tests: synthetic completion retains matching-run behavior; confirmed replacement preserves mode.
- Regression: `go test ./cmd/harnesscli/tui -run TestInitCommand -count=1`; race equivalent; `./scripts/test-regression.sh`.

## Implementation Checklist

- [x] Verify issue contract and current source ownership.
- [x] Record plan and cross-surface impact before implementation.
- [x] Add and observe expected-red SSE/conflict tests.
- [x] Bind pending write to accepted run identity and target snapshot.
- [x] Atomically commit only matching successful output.
- [x] Update logs/indexes and complete focused, race, and full verification.

## Risks and Mitigations

- Risk: a late/foreign terminal writes another run's content. Mitigation: exact pending run-ID comparison.
- Risk: a file created during generation is overwritten. Mitigation: re-stat before commit and reject unconfirmed appearance.
- Risk: partial output corrupts instructions. Mitigation: temp write, file sync/close, rename, directory sync, and cleanup on error.
