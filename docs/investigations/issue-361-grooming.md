# Issue #361 Grooming: Deployment Profile + Live Smoke Suite

## Already Addressed?
**PARTIAL** — Profile infrastructure exists (`internal/profiles/builtins/full.toml` etc.), but no designated golden path and no automated smoke suite.

## Evidence
- 6 embedded TOML profiles including `full.toml`
- Manual smoke test doc exists (`docs/testing/harness-smoke-test-post-rename-2026-03-18.md`)
- No scripted/automated smoke suite found
- No "supported golden path" designation in docs

## Clarity
UNCLEAR (4/10) — "selected profile" is undefined. Which profile is golden? Which optional subsystems (memory, MCP, S3) belong in smoke path?

## Acceptance Criteria
VAGUE — "selected profile" and "golden path" need definition. Suggested:
- Use `full.toml` with conversation + run persistence, no optional extras
- Bash/Go smoke script covering: start, health, model-discovery, run-creation, event-stream, one-tool-call, persistence

## Scope
NOT ATOMIC — depends hard on #362 (provider bootstrap). Cannot define golden path until startup semantics are fixed.

## Blockers
- **Hard blocker: #362** must land first

## Recommended Labels
- `needs-clarification`
- `medium`
- `blocked`
