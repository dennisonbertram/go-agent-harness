# Logs Index

- 2026-08-04 — Issue #1158 records the Runner-owned conversation message/event
  watermark boundary, tenant-safe additive API, old-server TUI fallback, and
  #1148 client-lifecycle dependency in all durable logs.

- 2026-08-04 — Issue #1156 records test-owned MCP HTTP transport isolation,
  global cleanup coupling, and the unchanged production auth boundary in all
  durable logs.
- 2026-08-04 — Issue #1148 selected-conversation TUI SSE lifecycle, replay,
  and terminal ordering evidence is recorded in all durable logs.
- 2026-08-04 — Issue #1149 authenticated, tenant-safe cron execution-history
  API evidence is recorded in all durable logs.
- 2026-08-04 — Issue #1153 deterministic durable cron dispatch polling and
  cancellation coverage evidence is recorded in all durable logs.
- 2026-08-04 — Issue #1152 records callback-isolated harnessd lifecycle
  fixture readiness and race-baseline evidence in the engineering,
  observational, system, and long-term-thinking logs.

- 2026-08-03 — Issue #1144 deterministic transient callback heartbeat fixture
  evidence is recorded in the engineering, observational, system, and
  long-term-thinking logs.

- 2026-08-03 — Issue #1140 harnessd matrix listener identity evidence is
  recorded in the engineering, observational, system, and long-term-thinking
  logs.

- 2026-08-03 — Issue #1141 callback deadline-release fixture evidence is
  recorded in the engineering, observational, system, and long-term-thinking
  logs.

- 2026-08-03 — Issue #1135 deterministic cron recovery fixture evidence is
  recorded in the engineering, observational, system, and long-term-thinking
  logs.

- 2026-08-03 — Issue #1132 compaction-after-wait fixture synchronization
  evidence is recorded in the engineering, observational, system, and
  long-term-thinking logs.

- 2026-08-03 — Issue #1124 deterministic retry-wait callback fixture evidence
  is recorded in the engineering, observational, and system logs.
- 2026-08-03 — Issue #1136 records immutable submission timeout capability,
  one-shot dispatch, and reset/load all-stream detachment separately from #1133.

- 2026-08-03 — Issue #1133 corrects the prior displaced-result wording:
  displacement revokes controls but does not end A outcome observation.

- 2026-08-03 — Issue #1130 submission-local outcome ownership, deterministic
  barriers, and ToolWalk timeout ordering are recorded in all durable logs.

- 2026-08-03 — Issue #1128 native submitted-run ownership evidence is recorded
  in the engineering, observational, system, and long-term logs.

- 2026-08-03 — Issue #1125 native Stop/steer/ToolWalk ownership evidence is
  recorded in the engineering, observational, system, and long-term logs.

- 2026-08-03 — Issue #1122 native interactive-state ownership evidence is
  recorded in the engineering, observational, system, and long-term logs.

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
