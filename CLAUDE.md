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
6. Zero tolerance for broken tests — pre-existing test failures must be fixed before merging new work; broken tests mask regressions

### Regression tests are mandatory
Every new feature or bug fix MUST include regression tests that validate:
1. **Concurrency safety** — any shared state (stores, schedulers, registries) must be tested under concurrent access with `-race`
2. **Error path coverage** — invalid inputs, connection failures, timeouts, malformed data must be tested, not just happy paths
3. **Boundary conditions** — limits, truncation points, empty inputs, max values must have explicit tests
4. **Environment/config edge cases** — empty env vars, invalid values, missing config must be tested (don't assume env vars fail gracefully)
5. **Integration seams** — where components interact (e.g., store + scheduler, server + client), test the realistic concurrent patterns
6. **Constraint enforcement** — unique constraints, foreign keys, status transitions must be tested for violation behavior
7. **Graceful degradation** — shutdown under load, partial failures, resource exhaustion must be validated

### Tool descriptions use `//go:embed`
- All tool descriptions live in `internal/harness/tools/descriptions/*.md` (one `.md` file per tool, named `{tool_name}.md`)
- Descriptions are loaded via `descriptions.Load("tool_name")` — see `descriptions/embed.go`
- When adding a new tool, create its description as a `.md` file in that directory — do NOT use inline string literals for the `Description` field
- This keeps descriptions editable, diffable, and separate from handler logic
- Tool descriptions should be self-contained — do NOT reference other tool names (tools may be enabled/disabled independently)

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

## GitHub

- GitHub user: `dennisonbertram`
- Before running ANY `gh` commands: `unset GITHUB_TOKEN && gh auth switch --user dennisonbertram`
- The `GITHUB_TOKEN` env var overrides `gh auth` — always unset it first

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
