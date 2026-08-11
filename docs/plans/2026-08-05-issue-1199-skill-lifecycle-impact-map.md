# Issue #1199 Impact Map

Task: synchronous, durable authored-skill create/verify lifecycle. Plan: `2026-08-05-issue-1199-skill-lifecycle-plan.md`. Status: implementation.

- Ownership/data flow: `create_skill` writes `SKILL.md`; `skillListerAdapter` owns reload and verification persistence; `skills.Registry` remains the visible source. Search: `rg create_skill|UpdateSkillVerification|WriteVerification`.
- Config/API/tools: no new config or wire format. Deferred tool, core `skill verify`, and HTTP all use the adapter verification method.
- Persistence/compatibility: existing YAML fields and O_EXCL creation preserved; write then reload prevents watcher-dependent visibility; restart reload reads durable metadata.
- Lifecycle/security: reload errors fail the call after a disk write and never claim success; no auth, permissions, secrets, or pack semantics change.
- Product/deployment: harnessd only; no TUI/GUI changes. Packs retain `PackRegistry` ownership and are not loaded as authored skills.
- Tests: focused deferred/core/harnessd, normal/race/full gate. Acceptance requires fake-provider create→verify→activate same conversation with watching disabled.
- Rollback/docs: revert adapter/create wiring; no migration. Logs and indexes updated.
