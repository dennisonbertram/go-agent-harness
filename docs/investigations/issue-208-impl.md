# Issue #208: Forensics — Tool Decision Tracing + Hook Mutation Tracing

## Status: DONE

## Summary

Implemented three opt-in forensic tracing features for the agent harness runner.
All features are disabled by default (false) and enabled via `RunnerConfig` fields.

---

## Part 1: Tool Decision Tracing (`TraceToolDecisions bool`)

**What it does:** After each LLM turn that returns tool calls, emits a `tool.decision`
SSE event listing which tools were available to the model and which tools it selected.
Also tracks a per-run sequential call counter (`call_1`, `call_2`, ...).

**New event:** `tool.decision`

Payload fields:
- `step` (int): current step number
- `call_sequence` (int): sequential call number within the run
- `call_sequence_id` (string): human-readable form, e.g. `"call_1"`
- `available_tools` ([]string): names of tools sent in the CompletionRequest
- `selected_tools` ([]string): names of tools the LLM chose to call

**Implementation:** Local `callSeq` counter in the `execute()` goroutine. Increments
only on steps that produce tool calls, so it is a true "tool call batch" counter.

---

## Part 2: Anti-Pattern Detection (`DetectAntiPatterns bool`)

**What it does:** Tracks `(tool_name, arguments)` pairs across all steps in a run.
When the same pair is seen 3 or more times, emits a `tool.antipattern` event.
The alert is emitted only once per unique pair (subsequent occurrences are silently
counted but not re-alerted).

**New event:** `tool.antipattern`

Payload fields:
- `type` (string): `"retry_loop"`
- `tool` (string): tool name
- `call_count` (int): number of times the pair has been seen (>= 3)
- `step` (int): step number where the threshold was first crossed

**Implementation:** Two local maps in `execute()`: `antiPatternCounts` (key -> count)
and `alreadyAlerted` (key -> bool). Key is `tool_name + "\x00" + arguments_json`.

---

## Part 3: Hook Mutation Tracing (`TraceHookMutations bool`)

**What it does:** Before each pre-tool-use hook runs, captures the current `callArgs`
as a before-snapshot. After the hook, compares before/after. Emits a `tool.hook.mutation`
event for any non-Allow outcome (Modify, Block, or Inject). Plain Allow (no change)
does not emit an event to avoid noise.

**New event:** `tool.hook.mutation`

Payload fields:
- `tool_call_id` (string): the LLM tool call ID
- `hook` (string): hook name
- `action` (string): one of `"Block"`, `"Modify"`, `"Inject"`, `"Allow"` (never "Allow" in practice — allow is a no-op)
- `args_before` (string): JSON args before hook ran
- `args_after` (string): JSON args after hook ran (empty for Block)

**Action classification** (in `tooldecision.ClassifyHookAction`):
- `Block`: hook denied the call
- `Modify`: args changed, both before and after are non-empty/non-null
- `Inject`: before was empty or "null", after has content
- `Allow`: args unchanged (never emitted as an event)

---

## Files Changed

### New Files

- `/internal/forensics/tooldecision/tooldecision.go` — types: `ToolDecisionSnapshot`,
  `AntiPatternAlert`, `HookMutation`, `HookMutationAction` constants, `ClassifyHookAction()`
- `/internal/forensics/tooldecision/tooldecision_test.go` — unit tests for all types
  and `ClassifyHookAction`
- `/internal/harness/runner_tooldecision_test.go` — integration tests using `stubProvider`
  + registry, covering all three parts

### Modified Files

- `/internal/harness/events.go` — added `EventToolDecision`, `EventToolAntiPattern`,
  `EventToolHookMutation` constants; updated `AllEventTypes()` slice
- `/internal/harness/types.go` — added `TraceToolDecisions`, `DetectAntiPatterns`,
  `TraceHookMutations` fields to `RunnerConfig`
- `/internal/harness/runner.go` — added `tooldecision` import; added forensic tracking
  state in `execute()`; added tool.decision emission after LLM response; added
  anti-pattern counting per tool call; added before/after arg capture in
  `applyPreToolUseHooks` with mutation event emission
- `/internal/harness/events_test.go` — updated `TestAllEventTypes_Count` from 46 to 49

---

## Test Coverage

All 3 parts have integration tests:
- `TestToolDecisionEventEmittedWhenEnabled` — verifies event fields
- `TestToolDecisionEventNotEmittedWhenDisabled` — default off
- `TestToolDecisionCallSequenceIncrementsAcrossSteps` — counter increases
- `TestToolDecisionNotEmittedWithoutToolCalls` — no event on text-only steps
- `TestAntiPatternRetryLoopDetected` — triggers at 3rd same-args call
- `TestAntiPatternNotDetectedWithDifferentArgs` — different args = no alert
- `TestAntiPatternNotDetectedWhenDisabled` — default off
- `TestAntiPatternAlertEmittedOnlyOnce` — idempotent
- `TestHookMutationEventEmittedOnModify` — Modify action with before/after
- `TestHookMutationEventEmittedOnBlock` — Block action
- `TestHookMutationNoEventForAllow` — plain allow = no event
- `TestHookMutationNotEmittedWhenDisabled` — default off
- `TestRunnerConfigForensicsDefaultsToFalse` — zero-value config check
- `TestAllEventTypesIncludesForensicsEvents` — AllEventTypes has new events

All tests pass: `go test ./internal/harness/... ./internal/forensics/... -race`
