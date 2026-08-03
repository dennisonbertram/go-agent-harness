# Logs Index

- 2026-08-03 — Issue #1135 deterministic cron recovery fixture evidence is
  recorded in the engineering, observational, system, and long-term-thinking
  logs.

- 2026-08-03 — Issue #1132 compaction-after-wait fixture synchronization
  evidence is recorded in the engineering, observational, system, and
  long-term-thinking logs.

- 2026-08-03 — Issue #1124 deterministic retry-wait callback fixture evidence
  is recorded in the engineering, observational, and system logs.

- 2026-08-03 — Issue #1120 deterministic blocked-heartbeat callback fixture
  evidence is recorded in the engineering, observational, and system logs.

- 2026-08-03 — Issue #1117 deterministic callback claim fixture evidence is
  recorded in the engineering, observational, and system logs.

- `engineering-log.md`: Record of implementation changes, bug fixes, tests added, and decisions.
- `observational-log.md`: Runtime/behavior observations, anomalies, and hypotheses.
- `system-log.md`: System map and interaction-level documentation between components.
- `long-term-thinking-log.md`: Command intent and user intent ledger used to resolve ambiguity and define success.

Issue-specific entries are dated within each durable log; Issue #1108 records
the native durable-message reconciliation fixture ordering repair.

Current operational additions: Issue #1096 documents deterministic modelstore
Keychain tests separately from the opt-in macOS host-live mutation lane.
Issue #1112 records the authenticated cron assembly race-timeout classification
and the test-only bcrypt-cost isolation that preserves production semantics;
PR #1113 is merged to `main` as the #1106 rebase baseline.

Issue #1106 documents mixed-version callback dispatch fencing, bootstrap-only
crash recovery provenance, and eventual pre-claim contention progress.
