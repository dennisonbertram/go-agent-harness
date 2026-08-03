# Cross-Surface Impact Map: Issue #1120

## Ownership and Data Flow

- Owner/caller: the one fixture in
  `internal/harness/tools/delayed_callback_retry_red_test.go` drives two SQLite
  handles through the unchanged `CallbackManager` and `SQLiteCallbackStore`.
- Test-only evidence wrappers observe `ExtendLease` entry and the exact
  `ReleaseLease` token. Search evidence: `rg "BlockingHeartbeat|blockingLease|ReleaseLease" internal/harness/tools`.

## Surface Assessment

- Config/API/CLI/tools, persistence/schema, compatibility, providers/models,
  TUI/web/native UI, deployment, and external services: none. No production
  package changes.
- Lifecycle/concurrency: the fixture now establishes starter -> blocked
  heartbeat -> second process-fence rejection -> deadline cancellation -> exact
  durable release; it does not alter that production sequence.
- Security/privacy: none; temporary SQLite databases and synthetic IDs only.

## Verification and Handoff

- Historical red: Sol's hosted race observation establishes the old fixture's
  90 ms scheduling gap; local pre-change race x200 passed and is recorded as
  non-reproduction rather than a waived baseline.
- Green gates: focused normal/race x100 and stress, tools package normal/race,
  then isolated full regression. PR must state the #1119 stack and `Closes #1120`.
