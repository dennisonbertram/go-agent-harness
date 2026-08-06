# Impact Map: Issue #1229 VT wrap-pending transitions

## Task

- Issue: #1229. Plan: `2026-08-06-issue-1229-vt-wrap-state-plan.md`. Status: implementation.

## Current Ownership, Callers, and Data Flow

- `vtBuffer` in `internal/acceptance/ptyrunner/runner.go` is the sole parser state; `currentScreen` feeds fresh and continuation PTY evidence. Search evidence: issue #1229 `rg` seam.

## Config, Persistence, Product, and Operations

- None: no configuration, API, persistence, client, provider, deployment, or permissions change. Retained 0600 PTY artifacts remain diagnostics.

## Lifecycle, Compatibility, and Tests

- Parser state is invocation-local. Compatibility requires exact DEC semantics: selective reset/preserve and independent alternate buffers. Tests cover cursor/erase, BS/TAB, SGR/combining, J3, 1049/47, real PTY, race, and full regression.

## Documentation and Handoff

- Update engineering log/index and PR evidence after green gates; rollback is one acceptance/parser commit revert.
