# Usability Test: `todos` Tool — Round 3

**Date**: 2026-03-09
**Model**: gpt-4.1-mini (HARNESS_MODEL default)
**Server**: localhost:8080
**Change Under Test**: `todos` tool promoted from deferred tier to core tier (always visible to the LLM, no `find_tool` discovery needed)

## Scoring

| Grade | Meaning |
|-------|---------|
| **P** | Used `todos` tool directly with correct args, minimal turns |
| **A** | Used `todos` but with extra steps or suboptimal args |
| **F** | Used write/bash/edit instead of todos, or wrong approach |

## Results Summary

| # | Prompt | Status | Grade | Tools Used | Turns |
|---|--------|--------|-------|------------|-------|
| 1 | Create a todo list with three items: write tests, update docs, deploy to staging | completed | **P** | `todos` | 2 |
| 2 | Add a todo item: review pull request #42 | completed | **P** | `todos` | 2 |
| 3 | Show me my current todo list | completed | **P** | `todos` | 2 |
| 4 | Mark the first todo item as done | completed | **A** | `todos` (x2) | 3 |
| 5 | I need to track my tasks for this session. Set up a todo with: fix bug, write migration, update changelog | completed | **P** | `todos` | 2 |

**Overall: 4/5 P, 1/5 A, 0/5 F**

## Comparison with Round 2 (Deferred Tier)

| # | Round 2 Grade | Round 3 Grade | Improvement |
|---|---------------|---------------|-------------|
| 1 | F (used `write`) | P | Fixed |
| 2 | F (used `AskUserQuestion`) | P | Fixed |
| 3 | F (used `ls`, `read`) | P | Fixed |
| 4 | F (used `grep`, `edit` — destructive) | A | Fixed |
| 5 | F (used `AskUserQuestion`) | P | Fixed |

**Round 2: 0/5 P, 0/5 A, 5/5 F**
**Round 3: 4/5 P, 1/5 A, 0/5 F**

Promoting to core tier eliminated all failures.

## Detailed Results

### Test 1: "Create a todo list with three items: write tests, update docs, deploy to staging"

- **Run ID**: run_1
- **Status**: completed
- **Grade**: P
- **Tools called**: `todos` (1 call)
- **Turns**: 2
- **Behavior**: The LLM immediately called the `todos` tool with the correct three-item array. No extra steps, no file creation, no discovery needed.
- **Arguments**:
  ```json
  {
    "todos": [
      {"id": "1", "text": "write tests", "status": "pending"},
      {"id": "2", "text": "update docs", "status": "pending"},
      {"id": "3", "text": "deploy to staging", "status": "pending"}
    ]
  }
  ```
- **LLM response**: "I have created a todo list with the following three items: 1. Write tests, 2. Update docs, 3. Deploy to staging"

### Test 2: "Add a todo item: review pull request #42"

- **Run ID**: run_2
- **Status**: completed
- **Grade**: P
- **Tools called**: `todos` (1 call)
- **Turns**: 2
- **Behavior**: The LLM called `todos` directly with a single-item array containing the requested todo. Clean, minimal execution.
- **Arguments**:
  ```json
  {
    "todos": [
      {"text": "review pull request #42", "status": "pending"}
    ]
  }
  ```
- **Note**: The LLM did not include an `id` field in the item. This is acceptable since the tool description says `id` is part of the item schema but the tool likely assigns one. Minor omission, does not affect grade.

### Test 3: "Show me my current todo list"

- **Run ID**: run_3
- **Status**: completed
- **Grade**: P
- **Tools called**: `todos` (1 call, empty args)
- **Turns**: 2
- **Behavior**: The LLM called `todos` with an empty object `{}` to read the current list, exactly as documented in the tool description. This is the correct read pattern.
- **Arguments**:
  ```json
  {}
  ```

### Test 4: "Mark the first todo item as done"

- **Run ID**: run_4
- **Status**: completed
- **Grade**: A (not P, due to extra read step)
- **Tools called**: `todos` (2 calls: read then write)
- **Turns**: 3
- **Behavior**: The LLM first called `todos` with empty args to read the current list and discover what the "first item" is, then called `todos` again with the full list updated to mark the first item as completed. This is technically correct (the tool requires the full list on write), but it required an extra turn.
- **Call 1 — Read**:
  ```json
  {}
  ```
- **Call 2 — Update**:
  ```json
  {
    "todos": [
      {"id": "1", "text": "Design API schema", "status": "completed"},
      {"id": "2", "text": "Write unit tests", "status": "pending"}
    ]
  }
  ```
- **Note**: The items returned ("Design API schema", "Write unit tests") appear to be from the tool description examples rather than from a previous test's state, since each run has its own independent list. The LLM correctly read the list first, then sent the full list back with the first item's status changed to "completed". The read-then-write pattern is arguably necessary here since the LLM cannot know the current list contents without reading first. Grade A is appropriate — the tool was used correctly, just with an extra round-trip.

### Test 5: "I need to track my tasks for this session. Set up a todo with: fix bug, write migration, update changelog"

- **Run ID**: run_5
- **Status**: completed
- **Grade**: P
- **Tools called**: `todos` (1 call)
- **Turns**: 2
- **Behavior**: The LLM called `todos` directly with a three-item array matching the user's requested tasks. Clean, single-call execution.
- **Arguments**:
  ```json
  {
    "todos": [
      {"id": "1", "text": "fix bug", "status": "pending"},
      {"id": "2", "text": "write migration", "status": "pending"},
      {"id": "3", "text": "update changelog", "status": "pending"}
    ]
  }
  ```

## Analysis

### What Changed

Promoting `todos` from deferred tier (requires `find_tool` discovery) to core tier (always visible) completely eliminated the failure mode. The LLM now:

1. **Immediately recognizes the `todos` tool** as the correct choice for task/todo-related prompts
2. **Uses correct argument structure** (todos array with id/text/status fields)
3. **Uses the read pattern** (empty args) when it needs to inspect the current list
4. **Does not fall back** to file-based workarounds (`write`, `bash`, `edit`)
5. **Does not ask unnecessary clarifying questions** (`AskUserQuestion`)

### Round 2 vs Round 3 Failure Mode Comparison

| Failure Mode | Round 2 | Round 3 |
|--------------|---------|---------|
| File-based workaround (write/read) | 2 tests | 0 tests |
| User question punt (AskUserQuestion) | 2 tests | 0 tests |
| Destructive false positive (edit wrong file) | 1 test | 0 tests |
| All failures | 5/5 | 0/5 |

### Key Takeaway

The `find_tool` discovery pattern was a complete barrier for gpt-4.1-mini with todo-related prompts. The model's strong prior to use file-writing tools for "todo" tasks overwhelmed the system prompt guidance to call `find_tool` first. Making `todos` a core tool removed this barrier entirely, resulting in 100% correct tool selection.

### Remaining Minor Issues

1. **Test 2 — Missing `id` field**: The LLM omitted the `id` field on one item. The tool should handle this gracefully (auto-assign), but it is a minor deviation from the documented schema.
2. **Test 4 — Example data leakage**: The read call returned items from the tool description examples ("Design API schema", "Write unit tests") rather than actual stored data. This may indicate the tool returns example data when the list is empty, or the LLM hallucinated the read response. Worth investigating separately.
3. **Test 4 — Extra turn for read-then-write**: The update pattern requires 2 calls (read + write) because the tool uses full-list replacement. This is by design but adds latency. A future improvement could support partial updates (e.g., `{"update": {"id": "1", "status": "completed"}}`).
