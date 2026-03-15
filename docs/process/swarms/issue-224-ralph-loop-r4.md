# Ralph Loop R4 — Forensics Code Review
**Date:** 2026-03-12
**Branch:** issue-19-bidirectional-mcp (forensics work)
**Commit reviewed:** 1bc5d36 (fixes from R3: recorderMu, terminal sealing bypass, ThinkingDelta leaks, shallow payload clone)
**Files reviewed:** runner.go, runner_forensics_test.go, events.go, types.go

## Test Gate

```
go test ./internal/harness/... -race
```

Result: ALL PASS (3.467s, race-clean)

## Review Files

- Pass 1 (Adversarial): `code-reviews/issue-217-r4-pass1-adversarial-20260312-174355.md`
- Pass 2 (Skeptical): `code-reviews/issue-217-r4-pass2-skeptical-20260312-174602.md`
- Pass 3 (Correctness): `code-reviews/issue-217-r4-pass3-correctness-20260312-174238.md`

---

## Verdicts

| Pass | Perspective | APPROVED |
|------|-------------|----------|
| 1 | Adversarial Security Researcher | NO |
| 2 | Skeptical Power User | NO |
| 3 | Correctness Auditor | NO |

**Total: 0 of 3 APPROVED**

---

## CRITICAL Findings

### C1. Recorder pre-terminal event drop (MEDIUM in R3, escalated to CRITICAL by P2)
**Classification: PRE-EXISTING (partially fixed in R3 for recorderMu race, but root TOCTOU remains)**

Both Pass 1 (item 7, reported MEDIUM) and Pass 2 (item 1, escalated to CRITICAL) independently identified the same issue:

A non-terminal event goroutine can capture `rec := state.recorder` under `r.mu`, then be preempted. The terminal event goroutine can subsequently acquire `r.mu`, detach the recorder, close it, and set `recorderClosed=true`. When the non-terminal goroutine then acquires `recorderMu`, it sees `recorderClosed==true` and skips `Record()`. The event appears in `state.events` (in-memory) but is absent from the JSONL rollout file.

**Impact:** JSONL rollout is not a faithful ledger. Sequence gaps possible.

**Fix:** Per-run recorder goroutine + channel; terminal close triggers channel close + drain wait.

---

### C2. Message shallow copy — ToolCalls slice aliasing
**Classification: PRE-EXISTING**

Pass 2 (item 2, HIGH) and Pass 3 (item 2, CRITICAL) both flag that `GetRunMessages()`, `ConversationMessages()`, and `completeRun()` all use `append([]Message(nil), state.messages...)` which copies the `Message` struct but shares the `ToolCalls []ToolCall` backing array.

External callers can mutate `msgs[i].ToolCalls[j].Name` and corrupt internal runner state or stored conversation history. Race detector will flag this under concurrent access.

**Impact:** Transcript integrity breach. Race condition.

**Fix:** Deep-copy `ToolCalls` when exporting or storing messages.

---

### C3. CompactRun lost-update / execute() stale local messages
**Classification: PRE-EXISTING**

Pass 3 (item 1, CRITICAL) and Pass 2 (item 4, MEDIUM) flag that `CompactRun()` calls `r.setMessages(runID, newMessages)` but `execute()` has its own local `messages` slice and calls `r.setMessages(runID, messages)` on the next step, overwriting the compaction.

**Impact:** Manual compaction is nondeterministically ineffective. Critical state invariant broken.

**Fix:** `execute()` must reload `state.messages` as source of truth each step (under lock/compactMu), or use a versioned message replacement mechanism.

---

### C4. deepCloneValue does not clone structs/pointers — payload aliasing
**Classification: PRE-EXISTING (partially addressed in R3 for shallow map/slice case, struct pointer case not addressed)**

Pass 1 (item 6, MEDIUM), Pass 2 (item 3, HIGH), and Pass 3 (item 3, CRITICAL) all independently flag that `deepCloneValue` handles maps and slices but returns structs and pointers as-is. Payloads include `CompletionUsage{...}` (with `*int` pointer fields) via `recordAccounting()`. Subscribers can share pointer targets and mutate each other's copies.

**Impact:** Forensic isolation invariant violated. Subscriber cross-contamination possible.

**Fix:** Convert accounting structs to `map[string]any` before emitting (marshal/unmarshal or manual mapping).

---

## HIGH Findings

### H1. ConversationID tenant isolation (Pass 1, CRITICAL — scoped to multi-tenant deployment)
**Classification: PRE-EXISTING**

`StartRun()` accepts user-controlled `ConversationID` with no tenant/agent scope enforcement. In a multi-tenant service, an attacker can load another conversation's transcript by guessing/obtaining a UUID.

**Assessment for current scope:** This is a valid architectural concern but is pre-existing and outside the scope of the R3 forensics fixes. The harness currently does not implement multi-tenancy. Recommend tracking as a separate security issue.

---

### H2. emit() shallow payload copy before redaction can mutate caller data (Pass 2 H3, Pass 3 H2)
**Classification: PRE-EXISTING (the R3 fix addressed the shallow clone going INTO storage; the incoming payload alias before redaction was not addressed)**

`emit()` shallow-copies `payload` into `enriched` (copies top-level keys only), then calls `redaction.RedactPayload(enriched)`. If redaction mutates nested values in-place, caller-owned nested structures are corrupted. The test `TestEmitDoesNotMutateCallerPayload` only validates injected top-level keys.

**Fix:** Deep-clone caller payload before any pipeline mutation.

---

### H3. CompactRun TOCTOU — can compact after terminal (Pass 3 H1)
**Classification: PRE-EXISTING**

`CompactRun()` checks `status` under `r.mu.RLock()`, releases, then proceeds to mutate messages. Run can transition to terminal between check and mutation.

**Fix:** Re-check terminal status under write lock before applying message replacement.

---

### H4. Rollout recorder JSONL ordering not guaranteed (Pass 3 H3)
**Classification: PRE-EXISTING**

Even when no drop occurs, concurrent non-terminal emits release `r.mu` at different times and race to `recorderMu`, so JSONL lines may arrive out-of-order relative to assigned sequence numbers.

**Fix:** Single per-run writer goroutine with ordered channel queue.

---

## MEDIUM Findings (selected)

| # | Finding | Classification |
|---|---------|----------------|
| M1 | Global `r.mu` held during redaction+clone+fanout — DoS/latency amplification (P1 #5) | PRE-EXISTING |
| M2 | No context propagation to provider/tools/hooks — context.Background() everywhere (P2 #5) | PRE-EXISTING |
| M3 | Hook.Name() can panic without recovery (P2 #6) | PRE-EXISTING |
| M4 | ContinueRun doesn't preserve per-run policy (MaxSteps, ReasoningEffort) (P2 #7) | PRE-EXISTING |
| M5 | nil map semantics not preserved by deepCloneValue (P3 M1) | PRE-EXISTING |
| M6 | Redaction unguarded — no mandatory redaction when RolloutDir enabled (P1 H2) | PRE-EXISTING |

---

## NEW vs PRE-EXISTING Classification

All CRITICAL and HIGH findings are **PRE-EXISTING** issues in the codebase. None were introduced by the R3 fixes (commit 1bc5d36). The R3 fixes correctly addressed:
- Recorder close race via `recorderMu` — addressed, but deeper sequencing/drain issue remains (C1 above)
- Terminal sealing bypass — addressed
- ThinkingDelta leaks when CaptureReasoning=false — addressed
- Shallow payload clone (map/slice level) — addressed, struct/pointer level not yet addressed (C4)

The reviewers surfaced deeper layered issues that the R3 fixes did not fully resolve (C1, C4) as well as pre-existing orthogonal issues (C2, C3, H1-H4).

---

## Recommendation

**DO NOT MERGE.** Fix and re-review required.

Priority order:
1. **C4** — Convert `CompletionUsage` struct to `map[string]any` in `recordAccounting()` before emitting (small, targeted fix, closes the deepCloneValue struct pointer gap)
2. **C2** — Deep-copy `ToolCalls` slice in `GetRunMessages()`, `ConversationMessages()`, `completeRun()`
3. **C1** — Implement per-run recorder channel + drain-on-close to fix JSONL completeness guarantee
4. **C3** — Fix `execute()` to reload `state.messages` as source of truth each step
5. **H2** — Deep-clone caller payload at `emit()` entry before redaction pipeline

Issues H1 (tenant isolation), H3 (CompactRun TOCTOU), H4 (JSONL ordering) and all MEDIUM items can be deferred to separate issues given scope.

---

## R4 Summary

- Tests: PASS (race-clean)
- Review verdict: 0/3 APPROVED
- New CRITICAL/HIGH findings introduced by R3 fixes: 0
- Pre-existing CRITICAL/HIGH findings surfaced: 4 CRITICAL, 4 HIGH
- Action: Fix C1-C4 + H2 before R5
