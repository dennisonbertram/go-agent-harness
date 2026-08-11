# Cross-Surface Impact Map: Issue #1222 semantic working-memory results

## Task

- Task / issue: #1222 core working-memory JSON result adapter.
- Plan link: `2026-08-06-issue-1222-working-memory-json-plan.md`.
- Owner: harness core tools.
- Status: implemented; full regression pending.

## Current Ownership, Callers, and Data Flow

- Entry points: `WorkingMemoryTool` accepts agent calls; `harnessd` registers
  it with a SQLite store and sends its result into run events/SSE.
- Source of truth: `internal/workingmemory` deliberately stores canonical JSON
  as `string`; `Snippet` embeds that same text for context injection.
- Consumers: tool-result messages, event payloads, HTTP/SSE transcript clients,
  and same-conversation continuations.
- Search evidence: `rg -n "working_memory|MemoryStore|SQLite|same.*conversation" internal cmd -g '*.go'` identified core tool, memory stores, harnessd wiring, and API acceptance seams.
- Conclusion: storage ownership remains unchanged; one presentation adapter is
  required at get/list.

## Config, API, CLI, and Tools

- Config/defaults/environment: None.
- API/server: no endpoint or envelope change; nested `value` and list entries
  now retain their JSON types.
- CLI/tools: only existing `working_memory` get/list results change; set,
  delete, validation, and tool metadata are unchanged.
- Error states: missing key shape is preserved; malformed legacy rows serialize
  as normal strings.

## Persistence and Compatibility

- Schema/migration: None. `entry_json` remains canonical JSON text.
- Compatibility: valid old rows decode semantically; malformed old rows retain
  read access rather than causing invalid tool output.
- Mixed version/rollback: readers on older binaries still see stored text;
  reverting code needs no data action.

## Lifecycle, Security, and Reliability

- Concurrency/cancellation/retries: None; the adapter copies into result maps
  after existing store calls.
- Auth/privacy/secrets: None; scope and permission checks are untouched.
- Failure/recovery: `json.Valid` makes malformed persistence a safe string
  fallback rather than a serialization failure.

## Product and Integration Surfaces

- Server/runtime: real harnessd same-conversation continuation is covered.
- TUI/web/macOS: no client source changes; they consume improved SSE payloads.
- Provider/catalog/routing: fake provider is test-only; production routing is
  untouched.
- External automation/UX: None beyond transcript fidelity.

## Deployment and Operations

- Rollout: ordinary code deployment; no flag or ordered migration.
- Diagnostics: existing tool-call-completed SSE output reveals semantic value.
- Rollback/recovery: revert one PR; no durable repair.
- Runbooks: no operator procedure changes required.

## Regression Tests

- Characterization/red: focused core get/list values failed before the adapter
  because output was double encoded.
- New coverage: strings, objects, arrays, number, bool, null, malformed legacy
  rows, unchanged not-found envelope, SQLite close/reopen/snippet, and real
  HTTP + continuation + SSE.
- Exact targeted commands: `GOCACHE=/private/tmp/gocode-1222-go-build
  TMPDIR=/private/tmp go test ./internal/harness/tools/core ...`; `go test
  ./internal/workingmemory`; and `go test ./cmd/harnessd -run
  '^TestWorkingMemoryAPISSEContinuationPreservesSemanticJSON$' -v`.
- Full command: `TMPDIR=/private/tmp ./scripts/test-regression.sh` passed:
  normal, race, coverage, and coverage gate (85.3% total, zero uncovered
  functions).

## Documentation and Handoff

- No public spec update; plan, map, logs, and indexes provide handoff.
- PR closes #1222 and includes exact red/green/acceptance evidence.

## Warning Check

- Every affected surface is recorded; unmodified surfaces state why they are
  unaffected rather than being left blank.
