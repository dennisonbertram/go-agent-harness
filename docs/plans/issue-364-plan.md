# Issue #364: Slash Command Consolidation Plan

## Problem
Command execution is split across three locations:
1. `cmd_parser.go::NewCommandRegistry()` — 5 stubs with "not yet implemented" (dead, only used in tests)
2. `model.go::buildCommandRegistry()` — 9 stubs returning empty results (also dead)
3. `model.go::Update()` switch statement (lines ~951-1025) — actual execution logic

Dead code: orphaned `case "provider":` in Update switch and View switch (command not registered).

## Changes

### `cmd/harnesscli/tui/cmd_parser.go`
- Delete `NewCommandRegistry()` entirely (only used in tests, has "not yet implemented" stubs)

### `cmd/harnesscli/tui/model.go`
1. **Refactor `buildCommandRegistry()`**: Move ALL side-effect logic from the Update switch into handler closures that capture `m *Model`
2. **Simplify `Update()`**: Delete the ~80-line switch on `cmd.Name`. Keep parse, dispatch, status check only.
3. **Add "provider" to `buildCommandRegistry()`**: It's in the switch but not registered (orphaned)
4. View switch "provider" case stays (will work once registered)

### `cmd/harnesscli/tui/cmd_parser_test.go`
- Update tests that use `NewCommandRegistry()` to use an empty registry or the model's registry
- Add tests:
  - All expected commands are registered
  - No "not yet implemented" text in any handler output
  - `/provider` is registered and functional

## Commit
Single: `fix(#364): consolidate slash command execution into registry handlers`

## Verification
- `go test ./cmd/harnesscli/... -timeout 60s`
- `go test ./cmd/harnesscli/... -race -timeout 60s`
