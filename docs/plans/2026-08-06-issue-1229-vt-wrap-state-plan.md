# Plan: Issue #1229 VT wrap-pending transitions

## Context

- Governing GitHub issue: #1229 (follow-up to #1221/#1228).
- Problem: exact-width pending wrap survives parser transitions that must reset
  or preserve it according to the corrected terminal contract.
- Scope: parser-local state plus table-driven regressions; no runtime/API/UI change.

## Test Plan (TDD)

- Add the issue’s table-driven reset/preserve transition regressions first.
- Run focused normal/race, repeated real fresh PTY, then external-cache full regression.

## Implementation Checklist

- [x] Read issue correction and parser ownership.
- [ ] Record red table-driven regressions.
- [ ] Repair only specified parser transitions.
- [ ] Run required gates and update durable logs/indexes.

## Risks and Mitigations

- Risk: blanket reset breaks valid deferred wrap. Mitigation: encode reset and
  preserve cases separately, including alternate-buffer round trips.
