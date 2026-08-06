# Plan: Issue #1208 owned native fake-provider scenarios

## Context

- Governing issue: #1208 (parent #1089; depends on merged #1206 owner lifecycle).
- Problem: the owned lifecycle starts an isolated fake-provider app and daemon, but has no declarative contract for the first rendered tool, cron, or callback conversations.
- User impact: an eventual operator run needs exact, reviewable scenario inputs and artifact/correlation expectations; it must not treat lifecycle readiness as GUI proof.
- Constraints: no app launch, foreground control, AX/OCR interaction, TCC prompt, existing-process discovery, attachment, reuse, or termination in this slice.

## Scope

- In scope: deterministic fake-provider turn manifests for a core-tool two-message conversation, cron continuation, and exactly-once callback continuation; native evidence artifact/correlation preflight; unit tests and operator documentation.
- Out of scope: executing the scenarios, producing a PASS manifest, changing GoCode UI/server behavior, real-provider access, or accepting macOS permissions.

## Documentation Contract

- Feature status: implemented (non-interactive support only).
- Public docs affected: none; this is internal acceptance infrastructure.
- Spec docs to update before code: this plan, impact map, and native acceptance runbook.
- Implementation notes after code: engineering/observational/system logs and indexes.

## Test Plan (TDD)

- First failing test: a scenario manifest missing a required typed artifact or using a duplicate path must fail preflight before any owner lifecycle is selected.
- Existing tests: native GUI validator and owner preflight tests remain the containment baseline.
- Regression tests: focused normal/race Go tests, headless Swift tests, and the repository regression gate.

## Cross-Surface Impact Map

See `2026-08-06-issue-1208-native-scenarios-impact-map.md`.

## Implementation Checklist

- [x] Verify current `origin/main` includes #1206 before implementation.
- [x] Define scope and cross-surface analysis.
- [x] Add the failing scenario-manifest preflight tests.
- [x] Add minimal deterministic scenario support and fake turns.
- [x] Bind the owner path to preflight only; do not launch it in this slice.
- [x] Update runbook, logs, and indexes.
- [x] Run focused, Swift, and full regression gates.

## Risks and Mitigations

- Risk: static fixtures get mislabeled as rendered proof. Mitigation: preflight emits no evidence and the runbook states that only an explicitly authorized later driver can create artifacts/PASS records.
- Risk: dynamic IDs are not correlated across evidence. Mitigation: the manifest requires one nonce and distinct screenshot, AX, SSE, and API/store artifact paths per scenario, with runtime run/conversation placeholders that the future driver must resolve consistently.
- Review repair: traversal and symlinked write targets now fail at the fixture write boundary, not only during manifest validation. Scenario preflight validates the exact `ls`, `cron_create`, and `set_delayed_callback` arguments plus their required continuation wording rather than only counting fake turns; cron schedules use the same parser-backed contract as `cron_create`.
