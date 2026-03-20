# Issue #364 Grooming: TUI Slash Command Consolidation

## Already Addressed?
**PARTIALLY** — Recent commit `4bbd760` removed `/provider` from registry, but core problem remains: command execution is split between a stub registry and a switch statement in the update loop.

## Evidence
- `cmd_parser.go:82-118` `NewCommandRegistry()` has "not yet implemented" placeholders (only used in tests now)
- `model.go:1414-1490` `buildCommandRegistry()` has same stub pattern
- `model.go:951-1025` has the ACTUAL execution logic in a parallel switch statement
- Dead code: `/provider` switch case remains (lines 1000-1009) but was removed from registry
- Three locations define "what commands exist": NewCommandRegistry, buildCommandRegistry, and the switch statement

## Clarity
GOOD — Problem is clear: two parallel paths (registry stubs + switch statement) for command execution.

## Acceptance Criteria
Clear from issue:
- One authoritative command execution path
- No "not yet implemented" fallbacks for supported commands
- Autocomplete/help and execution derive from same source
- Unit tests for registry contents, dispatch, and consistency

## Scope
ATOMIC — localized refactoring in `cmd_parser.go` and `model.go`.

## Blockers
NONE.

## Recommended Labels
- `well-specified`
- `small`
