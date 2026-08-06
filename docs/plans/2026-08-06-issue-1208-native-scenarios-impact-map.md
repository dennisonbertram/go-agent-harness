# Cross-Surface Impact Map: Issue #1208 native scenarios

## Task

- Task / issue: #1208.
- Plan link: `2026-08-06-issue-1208-native-scenarios-plan.md`.
- Owner: native GUI acceptance lane.
- Status: in implementation; non-interactive support only.

## Current Ownership, Callers, and Data Flow

- Entry points: `cmd/native-gui-acceptance`, `scripts/run-native-gui-acceptance.sh`, and `internal/acceptance/nativegui`.
- Owning source of truth: #1206 `Owner` owns private roots/listeners/children; `nativegui.Manifest` validates later proof packs; #1086 `inventory.SuiteContract` binds native cases.
- Downstream flow: scenario fixture -> fake-provider turns -> future owned app driver -> screenshot/AX/SSE/API-store artifacts -> native manifest validator.
- Search evidence: `rg -n 'nativegui|native-gui-acceptance|HARNESS_FAKE_TURNS|ScenarioContract' internal cmd macapp docs` on merged `bdd25ebf`.
- Duplication conclusion: add scenario declarations beside native acceptance; do not reuse shared-state `macapp/scripts/live-test.sh`.

## Config, API, CLI, and Tools

- Config: no caller-selected URL, driver, manifest, bundle, cleanup selector, provider credential, or new public setting.
- API/CLI/tools: no production endpoint or tool change. The existing owner command remains opt-in-only; its internal preflight validates static support before any spawn.
- Errors: malformed scenario manifests fail deterministically before the owner is selected.

## Persistence and Compatibility

- Persistence: no production schema/migration/cache change; future artifacts live only under the owner private root.
- Compatibility: additive test infrastructure; existing lifecycle and proof manifests remain valid.
- Mixed rollout: none; no deployed component changes.

## Lifecycle, Security, and Reliability

- Lifecycle: no processes are started by unit preflight; later execution remains solely #1206 owner-recorded children and cleanup.
- Security: fixture uses fake provider only; nonce and typed artifact path requirements prevent accidental proof-pack mixing. No secrets or external network.
- Failure/recovery: preflight fails closed on missing/duplicate artifact contracts, bad scenario identity, or inconsistent dynamic correlation placeholders.

## Product and Integration Surfaces

- Server/runtime: fake turns exercise existing core tool/cron/callback routes only in a later owned run; no behavior change here.
- macOS UI: no UI code or GUI interaction; later driver must visibly verify Chat and Activity states.
- TUI/web: none; #1204 owns TUI proof.
- Provider/tool catalog: fixed fake-provider turns, including `ls`, `cron_create`, and `set_delayed_callback`; no catalog change.
- UX/accessibility: no focus, keyboard, AX, screenshot, motion, or TCC action. Future execution requires explicit foreground plus Accessibility and Screen Recording authorization.

## Deployment and Operations

- Deployment: none.
- Diagnostics: preflight exposes scenario ID/contract errors; later artifacts retain nonce/run/conversation correlation.
- Rollback: revert additive scenario support; no data migration or process repair.
- Runbook: update native GUI acceptance runbook and indexes.

## Regression Tests

- First red: missing required artifact and duplicate typed artifact path reject a scenario manifest.
- New tests: deterministic scenario IDs, fake turns, required unique screenshot/AX/SSE/API-store paths, and no live lifecycle invocation.
- Exact commands: `TMPDIR=/private/tmp go test ./internal/acceptance/nativegui -count=1`; same with `-race`; `cd macapp && TMPDIR=/private/tmp swift test`; `TMPDIR=/private/tmp ./scripts/test-regression.sh`.

## Documentation and Handoff

- Specs: plan/impact map/runbook before code.
- Implementation notes: logs and indexes after green code.
- Handoff: live runner remains pending explicit foreground-control authorization and any required Accessibility/Screen Recording permission acceptance.
