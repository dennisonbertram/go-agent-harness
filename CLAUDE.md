# CLAUDE.md

## Project

Go-based agent harness — an event-driven HTTP backend that runs LLM tool-calling loops. Supports CLI and GUI frontends. Works locally and remotely.

## Architecture

- **Entry points**: `cmd/harnessd/` (HTTP server), `cmd/harnesscli/` (CLI client), `cmd/coveragegate/` (coverage tool)
- **Core loop**: `internal/harness/runner.go` — deterministic step loop: LLM → tool calls → execute → repeat
- **Tools**: `internal/harness/tools/` — 30+ tools (read, write, bash, grep, git, LSP, MCP, etc.)
- **API**: REST with SSE streaming. Endpoints in `internal/server/http.go`
- **Prompts**: `internal/systemprompt/` + `prompts/` directory — modular YAML-driven composition
- **Memory**: `internal/observationalmemory/` — SQLite-backed per-conversation memory with reflection
- **Provider**: `internal/provider/openai/` — OpenAI adapter (Anthropic planned)
- **Pricing**: `internal/provider/pricing/` — token-to-USD cost tracking

## Key Config

- `OPENAI_API_KEY` (required)
- `HARNESS_WORKSPACE`, `HARNESS_MODEL` (gpt-4.1-mini), `HARNESS_ADDR` (:8080), `HARNESS_MAX_STEPS` (8)

## Engineering Rules

### TDD is mandatory
1. Write failing tests first
2. Implement minimal code to pass
3. Run full suite before commit: `go test ./...` and `go test ./... -race`
4. Coverage gate: 80% minimum, no 0% functions
5. Use `./scripts/test-regression.sh` before merge

### Worktree workflow
- All implementation in dedicated git worktree branches
- Merge via `./scripts/verify-and-merge.sh <branch> "./scripts/test-regression.sh" main`
- Never commit directly to main

### Commit policy
- Only commit files changed for the current task
- Tests must pass before commit
- Every bug gets: engineering-log entry + regression test + GitHub issue

### Task completion
- Always state `Task status: DONE` or `Task status: NOT DONE` with blocker
- Do not suggest follow-up work unless required for the current task

## Documentation

- Bootstrap: `AGENTS.md` (read this first for full onboarding)
- Critical context: `docs/context/critical-context.md`
- Design docs: `docs/design/`
- Research: `docs/research/`
- Plans: `docs/plans/` (use `PLAN_TEMPLATE.md`)
- Logs: `docs/logs/engineering-log.md` (append to this)
- Runbooks: `docs/runbooks/`
- Master index: `docs/INDEX.md`

## Intent Precedence

When uncertain, resolve by:
1. Command intent (what the request explicitly asks)
2. User intent (what outcome matters most)

## Common Commands

```bash
go test ./...                              # run tests
go test ./... -race                        # race detector
./scripts/test-regression.sh               # full regression + coverage gate
./scripts/verify-and-merge.sh <branch> "./scripts/test-regression.sh" main  # merge
go run ./cmd/harnessd                      # start server
go run ./cmd/harnesscli                    # CLI client
```

## Active Issues / Roadmap

See GitHub issues for current work. Key tracks:
- Interaction: multi-turn continuation, mid-run steering
- Cost: deferred tools, cost ceilings, unlimited steps
- Platform: persistence layer, workspace abstraction, auth, multi-provider
- Observability: SSE event audit, streaming tool output
