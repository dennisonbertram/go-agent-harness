# Impact Map: Issue #1158 conversation history watermark foundation

## Task

- Task / issue: #1158 conversation messages plus event watermark snapshot.
- Plan link: `2026-08-04-issue-1158-conversation-watermark-plan.md`.
- Owner: isolated #1158 implementation slice.
- Status: implemented and verified through focused normal/race plus the full
  repository regression; awaiting commit, PR, hosted checks, and merge.

## Current Ownership, Callers, and Data Flow

- Entry point: `GET /v1/conversations/{id}/messages` calls
  `Runner.ConversationMessages`, while `/events` independently calls
  `Runner.SubscribeConversationFrom`.
- Source of truth: `Runner.completeRun` publishes the completed message mirror
  and conversation store; `eventJournal` persists/fans out exact event IDs;
  `conversationSequence` plus `conversationEventMu` already serialize event
  publication/replay for each conversation.
- Consumers: the TUI `fetchConversationMessagesCmd` decodes history into
  `ConversationHistoryMsg`; #1148's unmerged selected-conversation stream will
  consume the cursor when opening initial SSE.
- Searches: `rg -n "ConversationMessages|SubscribeConversationFrom|Last-Event-ID|ConversationHistoryMsg" internal cmd`; inspection found no existing
  paired snapshot or cursor field and no other product client decoding this
  endpoint into a strict response struct.
- Ownership conclusion: the pair belongs in Runner, not the HTTP handler or
  TUI, because only Runner owns both event ordering and conversation state.

## Config, API, CLI, and Tools

- Config/defaults/environment: None; no feature flag or saved setting changes.
- API: add optional string `last_event_id` beside `messages`; endpoint, request,
  status, and error shapes remain otherwise unchanged.
- CLI/TUI: `ConversationHistoryMsg` carries the decoded cursor. No command or
  slash-command syntax changes in this foundation slice.
- Tools/catalog/providers: None; search shows no tool schema or provider route
  consumes the conversation messages response.

## Persistence and Compatibility

- Schema/migration: None. The cursor is an existing event ID from the run event
  store, paired process-locally with the completed message snapshot in
  `conversationMessageWatermarks`.
- Recovery: when no process-local pair exists, the API returns `""` and
  requests full replay. A historical `run.completed` is insufficient proof
  after undo/rewind/compaction because those mutations are not transactionally
  versioned with the event store.
- Compatibility: old clients ignore the additive field; new clients decode an
  omitted old-server field as empty. Third-party/no-event-store runners retain
  empty-cursor full replay.

## Lifecycle, Security, and Reliability

- Concurrency: snapshot publication/read acquires the per-conversation sequence
  lock then `conversationEventMu`, matching event publication lock order. It
  must not independently sample messages and events. If another run lifetime
  overlaps, there is no safe single cursor for interleaved events absent from
  this run's message slice, so publication records an empty cursor.
- Copy semantics: outbound messages use `copyMessages`; caller mutation cannot
  rewrite Runner state.
- Auth/tenancy: existing `runs:read` and `blockConversationCrossTenant` checks
  stay before the snapshot call. In-process durable cursor sampling applies the
  completing run's tenant filter and cannot resolve another tenant's event ID.
- Failure: event-store failure or uncertain recovery yields an empty cursor,
  preferring duplicate-safe client reconciliation over event loss.

## Product and Integration Surfaces

- Server/runtime: Runner snapshot method and `/messages` response only.
- TUI: additive decode/carrying field only; #1148 owns bridge/reducer adoption,
  bounded exact-ID dedupe, and same-text scheduled-continuation rendering.
- Web/macOS: repository search found no strict consumer of this response; JSON
  addition is compatible. Native live proof remains the parent matrix gate.
- External automation: callback/cron execution is unchanged.
- UX/accessibility/motion: None in this foundation slice; no rendered layout or
  interaction changes.

## Deployment and Operations

- Order: merge #1158 before rebasing/completing #1148 so the client lifecycle
  consumes the authoritative cursor rather than content heuristics.
- Observability: plan and durable logs record empty-cursor fallback and the
  exact synchronization boundary; no new secrets/log payloads.
- Rollback: revert code/tests/docs; no persistence migration or repair.

## Regression Tests

- First red: Runner lacks paired snapshot/cursor; API lacks `last_event_id`;
  TUI message lacks the field.
- Acceptance: completed snapshot cursor, in-flight non-advancement, inverted
  overlap empty fallback, restart-after-undo empty fallback, additive response,
  tenant/scope denial, and new/old server decode.
- Edge/security: unknown conversation 404, foreign tenant hidden, omitted field
  empty, no event reader empty, message copy isolation.
- Integration: #1148 later proves initial conversation SSE sends this value as
  `Last-Event-ID` and exact-ID bounded dedupe preserves same-text callbacks.
- Commands: focused normal x10 and race x5 passed; complete affected package
  normal and race suites passed for `internal/harness`, `internal/server`, and
  `cmd/harnesscli/tui`. The non-concurrent full regression also passed at
  85.5% coverage with zero uncovered functions.

## Documentation and Handoff

- Before code: issue, plan, impact map.
- After code: plan checklist plus engineering, observational, system, and
  long-term-thinking logs; plan/log indexes.
- Public/release/training docs: None until #1148 and live matrices establish
  user-visible behavior.

## Warning Check

- Every surface is mapped. `None` entries include search-based rationale.
