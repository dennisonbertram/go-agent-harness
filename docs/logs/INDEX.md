# Logs Index

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
open PR #1113 awaits independent review and merge.
