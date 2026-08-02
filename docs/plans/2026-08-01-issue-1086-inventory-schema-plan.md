# Plan: Issue #1086 authoritative inventory and evidence schema

## Context

- Governing GitHub issue: #1086, child of #1085.
- Problem: the runtime tool and TUI command registries can drift from ToolWalk
  fixtures without a machine-checkable omission signal or a proof-oriented result
  contract.
- User impact: later API, PTY, and rendered-native acceptance runners need a
  stable, source-derived set of work items and cannot treat tool narration as
  success.
- Constraints: additive infrastructure and `/v1/tools` provenance fields only;
  no runner implementation, UI behavior, registry names, aliases, or
  external/live action changes.

## Scope

- In scope: compile resolved `harness.Registry` and `tui.CommandRegistry` into
  a deterministic inventory; validate an explicit evidence schema and manual
  intent cases against that inventory; render a deterministic Markdown report;
  document the handoff contract.
- Out of scope: executing cases, PTY or macOS automation, migrating ToolWalk
  consumers, product behavior repairs, and #1010 cron/callback convergence.

## Documentation Contract

- Feature status: `in implementation`.
- Public docs affected: none; the report and contract are operator/developer
  infrastructure, not a public product promise.
- Spec docs to update before code: this plan and its impact map.
- Implementation notes to add after code: engineering, system, and
  observational logs plus testing runbook and indexes.

## Test Plan (TDD)

- New failing tests to add first: resolved tool/command inventory includes
  owner, condition, aliases and deterministic hash; an eligible unresolved case
  fails completeness; unavailable capabilities require a stable reason; a pass
  with only a completed event and no postcondition is rejected.
- Existing tests to update: `/v1/tools` response coverage, runtime MCP catalog
  coverage, and the live-command HTTP fixture must assert authoritative
  owner/condition metadata.
- Regression tests required: duplicate identities, unknown rows, invalid
  outcomes, missing IDs/artifacts/cleanup/timing, and live default-registry/TUI
  reconciliation.

## Cross-Surface Impact Map

See `2026-08-01-issue-1086-inventory-schema-impact-map.md`.

## Implementation Checklist

- [x] Define acceptance criteria in tests and link the structured issue.
- [x] Record registry and command source-of-truth search evidence.
- [x] Complete the cross-surface impact map before implementation.
- [x] Record focused red evidence.
- [x] Implement the smallest compiler/schema/renderer and registry-owned tool
  provenance.
- [x] Add contract documentation and update indexes/logs.
- [x] Run targeted normal/race verification.
- [x] Repair independent-review fail-open findings for evidence identity,
  applicability, configured/unavailable provenance matching, canonical hashing,
  validated rendering, and live daemon propagation.
- [x] Repair exact-head review findings: command provenance now originates in
  `CommandEntry`, resolver evidence is mandatory at both HTTP/CLI boundaries,
  unknown-provider discovery is explicitly incomplete and fails closed, and
  the command entrypoint plus error unwrapping have behavioral coverage.
- [x] Add selected-surface completeness validation for independent API, TUI,
  and native runners without filtering or re-hashing the compiled inventory.
- [x] Add v2 PTY evidence semantics: canonical and every alias are separate
  inventory-derived invocation variants; local versus conversation evidence
  controls runtime-ID requirements; passes require all typed postconditions and
  typed, digested artifacts with explicit redaction declarations.
- [x] Add inventory-bound suite contracts for required runner-owned unknown-
  command and invalid-form scenarios, with stable IDs, strict completeness,
  suite-hash evidence provenance, and suite-aware rendering.
- [x] Add an inventory-hash-bound native applicability overlay. Every resolved
  item is explicitly native-available or not-applicable with source references
  and UX rationale; missing, duplicate, and unknown mappings fail closed.
- [x] Require native PASS records to carry typed screenshot, accessibility
  snapshot, raw SSE/event, and API/store artifacts plus exact build, bundle,
  daemon, and isolated-workspace metadata.
- [x] Evaluate Swift applicability: no `macapp/` or ToolWalk schema/consumer
  changed, so `swift test` is not applicable to this repair.
- [x] Rerun the authoritative foreground full regression and coverage gate on
  the final v2 schema candidate: normal, full race, and coverage passed at
  85.7% with zero uncovered functions.
- [ ] Commit only the #1086 slice for frontier review; this repair lane is
  explicitly not authorized to commit, push, or merge.

## Risks and Mitigations

- Risk: a second hand-maintained catalog drifts. Mitigation: compile names,
  tiers, aliases, and descriptions from the registries supplied at runtime.
- Risk: event/narration-only proof is recorded as a pass. Mitigation: validator
  requires an explicit expected postcondition and observed probe evidence, and
  the renderer revalidates raw records against their authoritative cases.
- Risk: unavailable external capability is silently skipped. Mitigation: a
  typed not-applicable resolution requires a stable machine-readable reason;
  configured and observed records cross `/v1/tools` together and match their
  full normalized resolver tuple.
- Risk: flat core/deferred slices erase why a conditional or dynamic tool was
  registered. Mitigation: every builder append carries owner and condition in
  `catalogTool`; runtime MCP and hot-reload mutations stamp the same Registry
  metadata without tool-name inference maps.
- Risk: later runners overclaim GUI coverage. Mitigation: native applicability
  is an inventory-bound reviewed overlay rather than a runner decision. A
  native PASS requires the complete typed proof bundle and isolated environment
  metadata; this slice still has no runner/pass producer.
- Risk: one canonical command test is reused as alias proof. Mitigation: aliases
  compile into separate stable invocation IDs and completeness/evidence/report
  keys include the invocation.
- Risk: negative tests become an untracked parallel inventory. Mitigation:
  suite scenarios are typed, explicitly runner-owned, hash-bound to the full
  inventory, and all declarations are required; they never become tool/command
  items.
- Risk: an MCP implementation reports a generic discovery failure without
  provider names. Mitigation: Registry records an explicit incomplete state and
  `/v1/tools` returns 503 rather than authoritative-looking empty arrays.
