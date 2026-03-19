Spawn a subagent using a named profile and wait for its result.

A profile is a reusable configuration: tool allowlist, model, max_steps, system prompt, and cost ceiling. Profiles reduce token overhead by constraining the subagent's option space.

Built-in profiles:
- "github"       — bash + read, 20 steps, gh CLI specialist
- "file-writer"  — read/write/edit/bash, 15 steps, code file modifications
- "researcher"   — read/grep/glob/ls/web_search, 10 steps, read-only analysis
- "bash-runner"  — bash only, 10 steps, script execution
- "reviewer"     — read/grep/glob/ls/git_diff, 10 steps, code review (no writes)
- "full"         — all tools, 30 steps, general purpose (default when no profile given)

Parameters:
- task (required): The task for the subagent. Be specific — it has no parent context.
- profile (optional): Profile name to use. Defaults to "full".
- model (optional): Override the profile's model for this call.
- max_steps (optional): Override the profile's max_steps for this call.
