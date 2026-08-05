# Cross-Surface Impact Map: Issue #1188 selected profile policy

## Task

- Task / issue: #1188, ordinary TUI selected-profile policy.
- Plan link: `2026-08-05-issue-1188-selected-profile-policy-plan.md`.
- Owner: Codex.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry points: `cmd/harnesscli/tui/api.go` sends `RunRequest.ProfileName`; `/v1/runs` passes it to `Runner.StartRun`.
- Source of truth: `internal/profiles.Profile.ApplyValues`; `internal/harness.Runner.startRun` owns ordinary request admission and `runPreflight` owns isolation/MCP.
- Consumers: provider completion requests, filtered/call-gated tools, permission evaluator, workspace preflight, and same-conversation continuations.
- Search evidence: `rg "ProfileName|ApplyValues|AllowedTools|SystemPrompt|MaxSteps" cmd/harnesscli/tui internal/harness internal/profiles` on 2026-08-05 showed ordinary `startRun` lacks ApplyValues while startup/subagent loaders already use it.
- Conclusion: compose once at ordinary Runner admission, before model/prompt/state creation; do not duplicate policy in TUI.

## Config, API, CLI, and Tools

- API wire shape unchanged: existing `profile` field becomes truthful.
- TUI `/profiles` selection now affects next normal submission. Existing explicit API request fields retain documented precedence.
- Tool policy: profile allowlist is an upper bound; explicit allowed tools can only narrow it. Profile disabled categories remain denied.

## Persistence and Compatibility

- No schema/migration/cache change. Old callers with no profile remain byte-for-byte default behavior.
- Mixed versions degrade only by old servers retaining prior behavior; no incompatible request shape.

## Lifecycle, Security, and Reliability

- Composition occurs before validation/persistence/state publication and copies slices/rules to avoid aliasing.
- Profile restrictions cannot be widened by request values. Model/prompt/budget are explicit-request overrides; invalid profile load remains non-fatal like current isolation/MCP behavior.
- No retry, timer, or cleanup change; profile workspace/MCP preflight remains existing owner.

## Product and Integration Surfaces

- Server/runtime: ordinary run admission gains policy composition.
- TUI: no new UI; picker behavior aligns with its existing label.
- macOS/web: existing `profile` API callers inherit correct behavior.
- Provider/catalog: profile model/reasoning selection flows through existing resolution; no catalog changes.
- External systems: no change.

## Deployment and Operations

- No flag/migration. Logs retain current profile name; support can reproduce with profile TOML and request body.
- Rollback: revert scoped composition only. No repair required.

## Regression Tests

- Characterization: current profile isolation/MCP and startup/subagent tests remain.
- Expected red: ordinary selected profile's values are absent before composition.
- New: profile defaults, explicit override precedence, safety intersection/denial, second conversation turn, fake-provider TUI/API path.
- Commands: focused normal/race `go test ./internal/harness ./cmd/harnesscli/tui`; final `TMPDIR=/private/tmp ./scripts/test-regression.sh`.

## Documentation and Handoff

- Update `docs/runbooks/profile-authoring.md`, plan/log indexes, four logs, and GitHub PR test-first evidence.
