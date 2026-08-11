# Plan: Issue #1222 semantic working-memory tool results

## Context

- Governing GitHub issue: #1222.
- Problem: the stores correctly retain canonical JSON text, but the core
  `working_memory` tool serialized that text as a second JSON string. A value
  set as `"api-memory-value"` was returned to a continuation as
  `"\\\"api-memory-value\\\""`.
- User impact: agents cannot reliably carry typed working-memory values across
  API/SSE conversation turns.
- Constraints: preserve storage and snippet behavior; keep the missing-key
  result shape; malformed legacy rows must remain readable.

## Scope

- In scope: decode valid stored JSON only at the core-tool get/list result
  boundary; tests for scalar, object, list, malformed, SQLite reopening, and
  real same-conversation API/SSE behavior.
- Out of scope: schema changes, migrations, TUI/native UI changes, cron or
  callback behavior, and storage-format rewrites.

## Documentation Contract

- Feature status: `implemented`.
- Public docs affected: None; the public API keeps its existing tool name and
  response envelope while correcting the JSON value type.
- Implementation notes: this plan, impact map, engineering log, long-term
  intent ledger, and indexes record the adapter boundary.

## Test Plan (TDD)

- First red: core get/list tests showed strings, objects, arrays, numbers,
  booleans, and null returned as JSON strings rather than their stored types.
- Regression: malformed legacy entries fall back to strings; not-found output
  remains `{"found":false,"key":"missing","value":""}`; SQLite reopen
  preserves canonical JSON and snippets; a real harnessd continuation exposes
  the semantic result through SSE.

## Cross-Surface Impact Map

- See [impact map](2026-08-06-issue-1222-working-memory-json-impact-map.md).

## Implementation Checklist

- [x] Link the structured issue and record architecture search evidence.
- [x] Create and reconcile the impact map before production code.
- [x] Add focused failing tool-boundary tests.
- [x] Implement the smallest result adapter without altering storage.
- [x] Add SQLite reopen and real harnessd API/SSE continuation tests.
- [x] Update durable logs and indexes.
- [x] Run the complete repository regression gate: normal, race, coverage, and
  coverage gate passed (85.3% total; zero uncovered functions).

## Risks and Mitigations

- Risk: treating malformed historical text as raw JSON would break every tool
  response. Mitigation: use `json.Valid`; invalid rows remain JSON strings.
- Risk: changing the store would alter snippets and persistence compatibility.
  Mitigation: adapter is limited to `WorkingMemoryTool` output construction.
- Rollback: revert this isolated adapter/tests/docs commit; SQLite data needs
  no migration or repair.
