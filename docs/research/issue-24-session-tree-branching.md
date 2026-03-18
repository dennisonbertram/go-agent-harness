# Issue #24: Session Tree Branching and Conversation Forking

**Date**: 2026-03-14
**Status**: Research
**Depends on**: #7 (persistence layer)

---

## 1. What Is Session Tree Branching?

Session tree branching is the ability to fork a conversation at any step and
explore multiple independent continuations. Instead of a single linear sequence
of messages, a branched conversation forms a tree: each node is a message, each
node's children are branches taken from that point.

**From the user's perspective (Pi/omp model):**

- `/fork` at any step creates a new branch starting from the current message
- `/tree` command shows the full conversation tree with branch labels
- Branches can be labeled and navigated like git branches
- Each branch is a complete conversation in its own right; they share a common
  prefix up to the fork point

**Why it matters:**

- You can recover from a bad LLM turn without starting over
- A/B test different prompts at the same point ("what if I told the agent to
  use approach X instead of Y?")
- Human-in-the-loop steering: pause at step N, fork, explore multiple agent
  paths in parallel, pick the best
- Prevents context contamination: each branch has isolated message state

**Pi/omp implementation:**

Sessions serialize as JSONL files. Each message has `id` and `parentId` fields.
A branch is just a chain of messages whose `parentId` points up the tree. The
tree viewer reconstructs the branching structure by following `parentId` links.

---

## 2. Current State in This Codebase

### How Conversation History Is Stored

From `internal/harness/runner.go`:

```go
type Runner struct {
    // ...
    runs            map[string]*runState
    conversations   map[string][]Message    // flat slice per conversation
    conversationOwners map[string]conversationOwner
}
```

Conversation history is stored as a **flat `[]Message` slice** keyed by
`conversation_id`. This is purely linear — there is no tree structure, no
parent pointers, no branching concept.

When a run completes, `completeRun()` writes the full message slice into
`r.conversations[convID]`. When the next run starts with the same
`conversation_id`, `loadConversationHistory()` reads the entire slice and
appends the new user prompt.

For multi-turn continuation, `ContinueRun()` creates a new `Run` with the
same `ConversationID`. The new run inherits all prior messages through the
`loadConversationHistory()` call in `execute()`.

**Key constraint**: the conversation ID is a flat key. There is no notion of
"branch A" vs "branch B" under the same conversation — they would overwrite
each other.

### What Forking Would Require

To implement forking at step N in a conversation with K messages total (N < K):

1. **Identify the fork point**: which message index or message ID to branch from
2. **Copy the prefix**: messages 0..N become the shared prefix
3. **Create a new conversation ID** for the fork: `conv_123:fork:step-5`
4. **Write the prefix to the new conversation**: so the forked run starts with
   the prefix history
5. **Start a new run** against the forked conversation ID with a new prompt

This is mechanically feasible today using `StartRun` with a new
`ConversationID` and a pre-populated history — but there is no API surface to
do it cleanly and no built-in concept of "this is a fork of conversation X at
step N."

### Rollout Recorder / JSONL

The rollout system (`internal/rollout/recorder.go`) already records events to
`<dir>/<date>/<run_id>.jsonl`. Each event has `ts`, `seq`, `type`, and `data`
fields. This is the closest analog to Pi's JSONL session format.

However, the rollout files are **per-run**, not per-conversation. A branching
model would need events linked by `parentId` across run files, or a
consolidated conversation JSONL file with branch pointers.

The rollout comment says: "The file is compatible with standard JSONL readers
and can be grepped, replayed, or forked without additional tooling." The replay
infrastructure (issues #211-215) is already planned and aligns with what
forking needs.

### Forensics / Replay System

Issues #211-215 define a replay system that rehydrates run state from JSONL
event logs. This is directly useful for branching:

- To branch at step N, replay events 0..N, then diverge
- The rollout format is the durable substrate for branch storage
- A `ForkRun` API would trigger a replay up to the fork point, then start a
  new execution from there

---

## 3. Technical Approaches

### Approach A: Copy-on-Fork with Shared Prefix

- Store the message prefix once, indexed by a content hash or message ID
- Each branch records only its divergent messages, plus a pointer to the prefix
- On load: resolve prefix + branch messages

**Pros**: Storage efficient (prefix stored once, not N times)
**Cons**: Requires a new storage schema; `copyMessages()` semantics are
non-trivial when the prefix is shared across branches; merge complexity

**Estimate**: ~3-4 days implementation + 1 day tests

### Approach B: Full Copy Per Branch (Recommended for MVP)

- When forking at step N, copy messages 0..N into a new `conversation_id`
- New runs against that ID extend independently
- No shared state; branches are fully independent conversations

**Pros**: Dead simple — uses existing `conversations` map with no schema change;
`ContinueRun` + `StartRun` semantics already work; no GC complexity

**Cons**: Storage duplicates the prefix for every branch (acceptable in practice
because conversation histories are small)

**Estimate**: ~1-2 days — mostly API surface + a `ForkRun(runID, step)` method

### Approach C: Event-Sourced Branches

- Add a `fork` event type to the JSONL rollout format
- Each event gets a `parentEventID` field
- The replay system reconstructs any branch by following the event chain

**Pros**: Clean forensic audit trail; branches are explicit in the event log;
matches Pi's JSONL-with-parentId model exactly

**Cons**: Requires changes to the rollout schema and replay infrastructure;
more complex than needed for MVP; depends on issues #211-215 being implemented
first

**Estimate**: ~5-7 days; blocked on #211-215

---

## 4. Use Cases

### A/B Testing Different Prompts

```
conv-001: [user: "write a parser"] → [assistant: "here's approach X"]
fork at step 1 → conv-001:branch-a, conv-001:branch-b

branch-a: user steers toward recursive descent
branch-b: user steers toward regex-based approach
→ compare outputs
```

### Human-in-the-Loop Course Correction

```
Run pauses at step 3; agent proposed deleting the wrong file.
→ ForkRun(runID, step=3)
→ New run starts with message history up to step 3
→ User steers differently: "don't delete, rename instead"
```

This is the most high-value use case: recovering from a destructive agent
action without restarting from the beginning.

### Exploring Multiple Tool Call Paths

When an agent makes an irreversible tool call (bash, write), forking before
that call lets you compare outcomes with and without the call.

---

## 5. Interaction with Existing Systems

### Compaction

If a conversation was compacted before the fork point, the pre-compaction
messages are gone from the in-memory store. The branch would start from
compacted history, not original history.

This means:
- Fork before compaction = full fidelity branch
- Fork after compaction = branch from summarized history (still useful but
  loses earlier context)

The rollout JSONL files are unaffected by compaction (they capture events at
recording time). So Approach C (event-sourced) would preserve pre-compaction
branching capability.

### Observational Memory

Per the grooming doc, branches could either:
- **Share memory**: both branches draw on the same per-conversation memory
- **Have independent memory**: each branch has its own memory scope

The simpler and more correct choice is **shared memory for the prefix, isolated
for branch-specific observations**. This aligns with Approach B: the new
`conversation_id` for the branch gets its own memory scope automatically.

### ConversationOwner Scoping (#221)

The current ownership check validates `(tenant_id, agent_id)` against a
`conversation_id`. Fork-created conversation IDs would need to inherit
ownership from the parent conversation, or be created in the same tenant/agent
scope.

### Persistence (#7)

Without a persistence layer, branches are in-memory only and lost on restart.
The SQLite ConversationStore already has `LoadMessages` / `SaveConversationWithCost`
— branching would write the prefix copy into this store with a new ID.

---

## 6. Proposed Minimal API

```go
// ForkRunRequest specifies the fork parameters.
type ForkRunRequest struct {
    // SourceRunID is the run to fork from.
    SourceRunID string
    // AtStep specifies the fork point (1-based).
    // 0 means fork at the end of the run (equivalent to ContinueRun).
    AtStep int
    // NewPrompt is the user message to inject at the fork point.
    NewPrompt string
}

// ForkRun creates a new branch of a conversation starting from step AtStep
// of the source run. Returns the new run and a new conversation_id.
func (r *Runner) ForkRun(req ForkRunRequest) (Run, string, error)
```

**Behavior**:
1. Look up `SourceRunID` state
2. Slice `state.messages` to include messages 0..AtStep-1 (the prefix)
3. Generate a new `conversation_id` (e.g., `original_conv_id + ":fork:" + uuid`)
4. Write the prefix into `r.conversations[newConvID]`
5. Call `StartRun` with the new `conversation_id` and `NewPrompt`
6. Return the new run and `newConvID`

---

## 7. Implementation Complexity Estimate

| Approach | Effort | Dependencies | Risk |
|----------|--------|--------------|------|
| A (copy-on-fork, shared prefix) | 3-4 days | #7 (store) | Medium |
| **B (full copy, simple)** | **1-2 days** | **None** | **Low** |
| C (event-sourced) | 5-7 days | #211-215 (replay) | High |

**Recommendation**: Approach B as MVP. It reuses the existing `conversations`
map and `ConversationStore`, requires no new schema, and delivers the core use
cases. Approach C can be layered on top once replay infrastructure exists.

---

## 8. Storage Overhead Analysis

For a 50-message conversation (~25KB at 500 chars/message):
- Each fork duplicates the prefix: if forking at step 25, that's ~12.5KB extra
- For 10 branches from the same point: 125KB overhead
- This is negligible for the typical use case

For long conversations (500+ messages, ~250KB):
- Forking early wastes significant storage
- Shared-prefix (Approach A) becomes worthwhile above ~10 branches
- The SQLite store can handle this comfortably

---

## 9. Relationship to Pi JSONL Format

Pi's session JSONL: each line has `{id, parentId, role, content, ...}`. The
`parentId` field is the branching mechanism — it points to the message that
preceded this one. A fork creates a new message with the same `parentId` as
the branching-off point, creating a tree structure.

The go-agent-harness rollout JSONL has `{ts, seq, type, data}` but no
`parentId`. To align with Pi's format, either:
- Add `parent_run_id` to rollout entries (tracked as `previousRunID` in
  `runState` already), or
- Add a `fork_point_step` field to the rollout header event

The `previousRunID` field is already set in `ContinueRun` and emitted in
`run.started` events — this is a direct analog to Pi's `parentId`. Fork support
would extend this naturally.

---

## 10. Open Questions

1. **Branch deletion**: should branches be deleteable? If so, what happens to
   sub-branches? (Git answer: rebase or orphan)

2. **Branch listing**: what API surfaces the list of branches under a
   conversation? A `/api/v1/conversations/:id/branches` endpoint?

3. **Memory scoping**: should forked branches share observational memory from
   the pre-fork messages, or start fresh?

4. **Compaction and branching**: if we compress the prefix, can we still
   display the full tree? Only if the JSONL rollout is preserved.

5. **Merge semantics**: the grooming doc asks "what does merge mean?" For MVP,
   merge is undefined — branches are independent. Merging (cherry-pick turns)
   is a future enhancement.
