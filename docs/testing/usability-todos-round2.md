# Usability Test: `todos` Tool — Round 2

**Date**: 2026-03-09
**Model**: gpt-4.1-mini (HARNESS_MODEL default)
**Server**: localhost:8080
**Goal**: Verify that system prompt changes cause the LLM to discover and use the deferred `todos` tool via `find_tool`, rather than falling back to `write`, `bash`, `read`, or `edit`.

## Scoring

| Grade | Meaning |
|-------|---------|
| **P** | Used `find_tool` -> discovered `todos` -> used it |
| **A** | Extra steps but eventually found and used `todos` |
| **F** | Never used `find_tool` or `todos`; fell back to generic tools |

## Results Summary

| # | Prompt | Status | Grade | Tools Used | Turns |
|---|--------|--------|-------|------------|-------|
| 1 | Create a todo list with three items: write tests, update docs, deploy to staging | completed | **F** | `write` | 2 |
| 2 | Add a todo item: review pull request #42 | failed | **F** | `AskUserQuestion` | 1 |
| 3 | Show me my current todo list | completed | **F** | `ls`, `read` | 3 |
| 4 | Mark the first todo item as done | completed | **F** | `grep`, `edit` | 3 |
| 5 | I need to track my tasks for this session. Set up a todo with: fix bug, write migration, update changelog | failed | **F** | `AskUserQuestion` | 1 |

**Overall: 0/5 P, 0/5 A, 5/5 F**

## Detailed Results

### Test 1: "Create a todo list with three items: write tests, update docs, deploy to staging"

- **Run ID**: run_55
- **Status**: completed
- **Grade**: F
- **Tools called**: `write`
- **Turns**: 2
- **Behavior**: The LLM immediately used the `write` tool to create a plain-text file `todo_list.txt` with a simple numbered list. It never called `find_tool` and never discovered the `todos` tool.
- **Arguments**:
  ```json
  {
    "path": "todo_list.txt",
    "content": "TODO List:\n1. Write tests\n2. Update docs\n3. Deploy to staging\n"
  }
  ```
- **LLM response**: "I have created a todo list with the three items: write tests, update docs, and deploy to staging. The list is saved in a file named 'todo_list.txt'."

### Test 2: "Add a todo item: review pull request #42"

- **Run ID**: run_7
- **Status**: failed (asked user a question, no response possible in non-interactive mode)
- **Grade**: F
- **Tools called**: `AskUserQuestion`
- **Turns**: 1
- **Behavior**: Instead of using `find_tool` to discover the `todos` tool, the LLM asked the user a clarifying question about confirming the TODO details. Since this is a non-interactive run, the run failed waiting for user input.
- **Arguments**:
  ```json
  {
    "questions": [{
      "header": "Confirm TODO Details",
      "question": "Please confirm the details for the TODO item to add.",
      "options": [
        {"label": "Review pull request #42", "description": "Review and provide feedback on pull request number 42."},
        {"label": "Other", "description": "Specify a different TODO item."}
      ],
      "multiSelect": false
    }]
  }
  ```

### Test 3: "Show me my current todo list"

- **Run ID**: run_52
- **Status**: completed
- **Grade**: F
- **Tools called**: `ls`, `read`
- **Turns**: 3
- **Behavior**: The LLM listed the workspace directory, then tried to read a `todo.json` file (presumably from a previous run or hallucinated). It never called `find_tool` and never discovered the `todos` tool.
- **LLM response**: "Your current todo list has one item: Title: 'delectus aut autem' - Completed: No" (this appears to be data from a prior test or stale file in the workspace).

### Test 4: "Mark the first todo item as done"

- **Run ID**: run_53
- **Status**: completed
- **Grade**: F
- **Tools called**: `grep`, `edit`
- **Turns**: 3
- **Behavior**: The LLM searched the workspace for unchecked markdown checkboxes (`- [ ]`), found one in `docs/operations/nightly-tasks.md`, and edited it to `- [x]`. This modified an unrelated operational checklist file rather than using the `todos` tool. It never called `find_tool`.
- **Arguments**:
  ```json
  // grep
  {"query": "- [ ]", "max_matches": 1}
  // edit
  {
    "path": ".claude/worktrees/agent-abef27a5/docs/operations/nightly-tasks.md",
    "old_text": "- [ ] Run regression suite ...",
    "new_text": "- [x] Run regression suite ..."
  }
  ```
- **Concern**: This is a **destructive false positive** — the LLM modified a real operational file that had nothing to do with the user's request.

### Test 5: "I need to track my tasks for this session. Set up a todo with: fix bug, write migration, update changelog"

- **Run ID**: run_54
- **Status**: failed (asked user a question, no response possible in non-interactive mode)
- **Grade**: F
- **Tools called**: `AskUserQuestion`
- **Turns**: 1
- **Behavior**: The LLM asked the user how they want to prioritize the tasks instead of creating the todo list. It never called `find_tool` or `todos`.

## Analysis

### Root Cause

The system prompt changes (adding `find_tool` guidance and "Todo/task tracking -> search todos" hint) had **zero effect** on LLM behavior. In all 5 test cases, the LLM:

1. **Never called `find_tool`** — not even once across all 5 runs
2. **Never discovered the `todos` tool** — it remains completely invisible to the LLM
3. **Used workaround tools** — `write` (create files), `read`/`ls` (browse filesystem), `grep`/`edit` (modify markdown checkboxes), or `AskUserQuestion` (punt to user)

### Failure Modes

| Mode | Tests | Description |
|------|-------|-------------|
| **File-based workaround** | 1, 3 | LLM creates/reads plain text or JSON files as "todo lists" |
| **User question punt** | 2, 5 | LLM asks clarifying questions instead of acting |
| **Destructive false positive** | 4 | LLM modifies unrelated files that happen to contain checkboxes |

### Why `find_tool` Is Not Being Called

Likely causes (in order of probability):

1. **Model does not understand `find_tool` semantics** — gpt-4.1-mini may not have enough context from the system prompt alone to understand that `find_tool` is the gateway to discovering additional tools. The tool description and system prompt hint may be too subtle.

2. **"Todo" triggers strong priors** — The word "todo" strongly activates file-writing behavior in the model. It defaults to creating text/JSON files because that pattern is overrepresented in training data.

3. **No negative signal for workarounds** — The system prompt tells the LLM to use `find_tool` but does not explicitly say "do NOT use write/bash to create todo files." Without a prohibition, the model's prior dominates.

4. **`find_tool` is low-salience** — Among 30+ tools, `find_tool` may not stand out enough. The model may not even attend to its description before selecting a more familiar tool.

### Recommendations

1. **Stronger system prompt prohibition**: Add explicit text like "When the user asks about todos, tasks, or task tracking, you MUST call `find_tool` first. Do NOT use `write`, `bash`, or `edit` to create todo files."

2. **Rename `find_tool` to something more discoverable**: Consider `discover_tools` or `search_available_tools` — a name that signals the action more clearly.

3. **Promote `todos` to a non-deferred tool**: If `todos` is a key capability, making it always-visible would eliminate the discovery problem entirely.

4. **Test with a stronger model**: gpt-4.1-mini may not have the reasoning capacity to follow the `find_tool` discovery pattern. Test with gpt-4.1 or gpt-4o to see if model capability is the bottleneck.

5. **Add `find_tool` to the tool-call examples in the system prompt**: Show a concrete example like "User asks about todos -> call find_tool('todos') -> use the discovered tool."
