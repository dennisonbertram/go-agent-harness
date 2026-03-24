# Open SWE-Inspired Trigger/Context Adoption Impact Map

## Task

- Task / issue:
  - Plan how `go-agent-harness` should adopt the useful parts of `langchain-ai/open-swe`.
- Plan link:
  - `docs/plans/2026-03-22-open-swe-trigger-context-adoption-plan.md`
- Owner:
  - TBD
- Status:
  - Proposed

## Config

- User-facing config added or changed:
  - Repo `AGENTS.md` auto-loading should be on by default when a workspace/repo root is known.
  - Trigger adapters will likely need explicit enable/disable config per source system.
  - Workspace backend selection may gain a request/profile-level selector.
- Defaults / fallbacks:
  - No external trigger metadata -> existing run behavior remains unchanged.
  - No root `AGENTS.md` -> no prompt section added.
  - No explicit workspace backend -> retain current default behavior.
- Environment variables, config files, or saved settings touched:
  - GitHub App / webhook settings for GitHub-first rollout.
  - Later Slack and Linear credentials/secrets.
  - Possibly profile or server config for workspace backend defaults.
- Migration / backward-compatibility notes:
  - Existing direct HTTP API clients must continue to work without change.
  - Existing prompts must remain valid when repo-local instructions are absent.

## Server API

- Endpoints, request fields, response fields, or server wiring affected:
  - New webhook or trigger-ingestion endpoints are likely.
  - Existing `/v1/runs/{id}/steer` and `/v1/runs/{id}/continue` remain the primary follow-up API.
  - New source/context metadata may be added to run creation paths.
- Provider/model resolution or registry changes:
  - None for the first slice. This proposal is trigger/context oriented, not provider/model oriented.
- Error states / validation changes:
  - Invalid signatures must fail closed.
  - Missing repo mapping or unsupported trigger payloads must return explicit validation errors.
  - Follow-up routing must distinguish active vs completed runs deterministically.

## TUI State

- Slash commands, overlays, selection state, routing, or status bar changes:
  - None in the first slice. The initial work is server/integration-facing.
- Persisted client state or local config changes:
  - None initially.
- Keyboard/navigation implications:
  - None initially.

## Regression Tests

- New acceptance tests required:
  - Repo `AGENTS.md` prompt injection.
  - GitHub trigger ingestion and deterministic thread routing.
  - Follow-up dispatch to `SteerRun` vs `ContinueRun`.
  - Source-context hydration from issue/comment payloads.
  - Workspace backend selection fallback behavior.
- Existing tests to update:
  - `internal/systemprompt`
  - `internal/server`
  - `internal/harness`
  - `internal/workspace`
- Cross-surface regressions to guard:
  - Direct API runs must remain unchanged.
  - Trigger metadata must not bypass auth/validation.
  - Trigger adapters must not create a second conversation-routing model that diverges from runner truth.

## Warning Check

- Config impact is real because credentials, trigger enablement, and workspace defaults will need clear policy.
- Server API impact is real because new trigger ingestion paths and metadata wiring will be added.
- TUI impact is intentionally `None` for the first slice because no UI behavior is proposed yet.
- Regression-test impact is significant and should be addressed before any implementation starts.
