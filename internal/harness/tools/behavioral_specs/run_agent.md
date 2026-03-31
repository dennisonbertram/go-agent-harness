## When to Use
- Delegating a well-scoped, self-contained sub-task that needs its own tool access
- Tasks that require reading multiple files and producing a focused output
- Parallel exploration tasks that can run independently

## When NOT to Use
- Tasks that require back-and-forth interaction with the parent — use inline tool calls instead
- Simple one-step operations — call the tool directly
- Tasks where the parent needs to inspect intermediate results before the subagent finishes
- Creating subagents just to avoid doing work in the current context

## Behavioral Rules
1. Don't Peek: never inspect a subagent's intermediate state; wait for its terminal output
2. Don't Race: don't start multiple subagents that write to the same file or resource
3. Fork for isolation: use run_agent when the task needs a clean workspace context
4. Use dedicated tools: if a dedicated tool (grep, glob, read) can accomplish the task, use it directly
5. Scope the prompt clearly: a subagent prompt should contain all context needed; it cannot ask the parent for clarification
6. Limit nesting: avoid spawning subagents from within subagents beyond 2 levels of depth

## Common Mistakes
- **OverDelegation**: Spawning a subagent to grep a file instead of calling grep directly
- **RacingWriters**: Spawning two subagents that both modify the same file — the last write wins
- **VaguePrompt**: Giving the subagent a prompt like "figure out what's wrong" without specifying files, goals, or success criteria
- **PeekingAtState**: Reading job output mid-run expecting partial results instead of waiting for completion

## Examples
### WRONG
Run an agent with prompt "look at the codebase and fix things" — too vague, no success criteria.

### RIGHT
Run an agent with a prompt that specifies: the exact files to examine, the specific behavior to verify or fix, the expected output format, and any relevant constraints (e.g., allowed tools).
