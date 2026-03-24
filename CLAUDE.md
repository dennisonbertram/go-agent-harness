# CLAUDE.md

This repository is a Go coding harness with a streamed run API, a CLI smoke-test client, and a growing catalog of local and optional remote tools.

## Current Source Of Truth

- The canonical implementation details are in `internal/server`, `internal/harness`, `internal/config`, `cmd/harnessd`, and `cmd/harnesscli`.
- The public-facing docs should stay aligned with the current routes, run request fields, event names, tool catalog, and environment variables.
- If a docs change reveals a mismatch, update the docs rather than preserving stale prose.

## Provider Note

- OpenAI is the primary provider path.
- Anthropic provider support exists in the provider catalog and should not be described as merely planned.

## Operational Reminder

- Keep `docs/logs/long-term-thinking-log.md` in sync with any durable intent or success-criteria changes.
- Keep `docs/runbooks/` aligned with the current CLI and server behavior.
- For a new worktree, run `scripts/bootstrap-worktree.sh <task-slug>` first. It creates the worktree, downloads dependencies, builds local binaries, writes a sourceable env file, and can start `harnessd` in tmux when requested.
