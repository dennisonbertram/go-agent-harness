# Plan: Replace Default Toolset with read/write/edit/bash

## Context

- Problem: Current default toolset (`list_files`, `read_file`, `run_go_test`) does not match requested POC interface.
- User impact: The harness cannot expose the expected editing/command capabilities for downstream GUI/TUI clients.
- Constraints:
  - Keep tools workspace-scoped and bounded.
  - Preserve deterministic harness loop behavior.
  - Follow strict TDD and update docs/logs.

## Scope

- In scope:
  - Replace default tool registration with `read`, `write`, `edit`, and `bash`.
  - Add/adjust tests for each tool behavior and safety guardrails.
  - Update README/tool documentation to reflect renamed/changed tools.
- Out of scope:
  - Permission escalation workflows.
  - Persistent audit log storage outside in-memory run history.

## Test Plan (TDD)

- New failing tests to add first:
  - `read` returns file content and enforces workspace boundaries.
  - `write` creates/overwrites files inside workspace.
  - `edit` applies exact find/replace and errors when target text is missing.
  - `bash` executes bounded shell commands in workspace and blocks dangerous patterns.
- Existing tests to update:
  - Replace old tool-name assertions in `internal/harness/tools_test.go`.
- Regression tests required:
  - Path traversal attempts rejected for `read`, `write`, and `edit`.
  - `bash` rejects unsupported commands and reports non-zero exit status.

## Implementation Checklist

- [x] Define acceptance criteria in tests.
- [x] Write failing tests first.
- [x] Implement minimal code changes.
- [x] Refactor while tests remain green.
- [x] Update docs and indexes.
- [x] Update engineering/system/observational logs as needed.
- [x] Run full test suite.
- [ ] Merge branch back to `main` after tests pass.

## Risks and Mitigations

- Risk: `bash` tool could become too permissive for a coding harness.
- Mitigation: enforce deny-list patterns, workspace working directory confinement, timeout bounds, and no-shell-escape behavior beyond `/bin/bash -lc` within workspace.
