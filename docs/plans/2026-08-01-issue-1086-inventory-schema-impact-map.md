# Cross-Surface Impact Map: Issue #1086

## Task

- Task / issue: #1086, authoritative tool and TUI-command inventories with a
  versioned evidence schema.
- Plan link: `2026-08-01-issue-1086-inventory-schema-plan.md`.
- Owner: acceptance infrastructure.
- Status: implemented and focused normal/race verified; full promotion gates
  remain.

## Current Ownership, Callers, and Data Flow

- Entry points: the paired `harness.Registry.DefinitionsWithMetadata()` /
  `ToolsetResolutionSnapshot()` contract and `tui.NewCommandRegistry().All()`.
- Owning sources: `internal/harness/registry.go`,
  `internal/harness/tools_default.go`,
  `internal/harness/tools/deferred/mcp.go`, and
  `cmd/harnesscli/tui/cmd_parser.go`.
- Consumers: later #1087--#1090 runners and existing ToolWalk migration work;
  this slice adds no production caller.
- Search evidence: `rg -n "NewDefaultRegistry|DefinitionsWithMetadata|NewCommandRegistry|ToolWalk|Verdict|ui-walk-tools" internal cmd macapp scripts`.
- Conclusion: registry snapshots, rather than a parallel list, are the source
  of truth; manual rows may add intent/postcondition only.

## Config, API, CLI, and Tools

- Config: no new user configuration; callers supply already-resolved runtime
  capability observations.
- API/CLI/tools: `GET /v1/tools` adds stable `owner` and `condition` fields to
  each existing row plus paired configured/observed unavailable toolset arrays;
  names, tiers, tags, parameters, authorization, and method behavior are
  unchanged. The read-only compiler command consumes those fields and renders
  a report for later operator tooling.
- Error states: invalid duplicate/unknown identities, missing availability
  reason, evidence-only pass records, absent/null resolver arrays, missing TUI
  provenance, incomplete unidentified resolution, incomplete native
  applicability overlays, and incomplete native proof environments are rejected.

## Persistence and Compatibility

- Persistence: report artifacts only; no product database, migration, or cache.
- Compatibility: the `/v1/tools` wire change is additive. Tool names,
  descriptions, tiers, command names, aliases, and ToolWalk formats remain
  untouched.
- Mixed versions: evidence schema v2 includes the full inventory hash and an
  optional suite hash so a future runner rejects incomparable reports. The v1
  singular-probe/untyped-artifact draft was never shipped and is intentionally
  not accepted as a weak compatibility fallback.

## Lifecycle, Security, and Reliability

- Concurrency: compiler and Registry copy snapshots deterministically; a typed
  per-call discovery error binds unavailable evidence without a mutable
  global-last-result race. Generic discovery failures without provider names
  retain an explicit incomplete bit that makes the HTTP boundary return 503.
  No daemon, port, timer, or workspace ownership.
- Security: compiled data contains no secrets. Artifact paths are caller-owned
  opaque references; no user-home/config values are serialized.
- Recovery: validation fails closed; unavailable capabilities require a stable
  reason rather than an implicit omission.

## Product and Integration Surfaces

- Server/runtime: reads the resolved production tool registry only.
- TUI/macOS: reads TUI command entries. Canonical names and every alias produce
  distinct TUI invocation requirements. Native applicability is a complete,
  source-referenced, inventory-hash-bound suite overlay; native passes require
  screenshot, AX, raw event, API/store, build, daemon, and isolated-workspace
  proof. No rendered UI behavior changes.
- Provider/model/tool catalog: default-registry groups attach provenance where
  their real registration condition is evaluated. Discovered and runtime MCP
  tools retain their logical server identity; unavailable dynamic condition
  resolution is represented as a supplied availability record, not guessed
  into individual tool names.
- External systems: none; later live/external lanes remain opt-in.
- UX/accessibility: none in this slice.

## Deployment and Operations

- Deployment: additive library only; no flag, rollout, or migration.
- Diagnostics: deterministic inventory hash, owner, condition, and Markdown
  report make drift and not-applicability inspectable.
- Rollback: revert this child; it has no product-state impact.
- Runbooks: document how future runners must validate the schema and reconcile
  the generated report with live registries.

## Regression Tests

- Characterization/red: inventory and evidence tests are added before package
  implementation and initially fail to compile.
- Acceptance: default resolved registry plus built-in command registry produce
  unique deterministic rows and a report containing their hash.
- Negative: omitted, duplicate, unknown, invalid unavailable, and
  postcondition-free pass cases fail. Missing aliases, incomplete multi-probe
  evidence, absent artifact digests/redaction declarations, fabricated local
  runtime IDs, undeclared synthetic scenarios, and missing required negative
  cases also fail. Missing/unknown native mappings, native cases for explicit
  N/A rows, missing cases for native-available rows, incomplete native artifact
  bundles, and non-isolated environment metadata also fail.
- Focused commands completed: normal tests for
  `./internal/acceptance/inventory`, `./cmd/acceptance-inventory`,
  `./internal/harness`, `./internal/harness/tools/deferred`, and
  `./internal/server`; focused provenance/reconciliation/concurrency tests
  across the same packages under `-race`.
- The authoritative logged-in foreground `./scripts/test-regression.sh` passes
  the final v2 candidate normal, full race, and coverage at 85.7% with zero
  uncovered functions.
  Swift is not applicable because no `macapp/` or ToolWalk schema/consumer
  changed in this repair.

## Documentation and Handoff

- Before code: this map and plan.
- After code: testing guidance, engineering/system/observational logs, and
  plans/runbooks/logs indexes.
- Handoff: #1087--#1090 consume `inventory` rows and evidence validation;
  #1010 remains the independent cron/callback convergence gate.
