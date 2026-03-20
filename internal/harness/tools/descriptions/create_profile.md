Create a new named agent profile in the user profiles directory.

A profile is a reusable subagent configuration that controls the model, max_steps, cost ceiling, system prompt, and allowed tools. Use this tool to define a new profile that can then be used with `run_agent`.

**Required fields:**
- `name` — unique identifier (kebab-case recommended, e.g. "code-reviewer", "data-analyst")
- `description` — human-readable description of what the profile is for

**Optional fields:**
- `model` — LLM model name (e.g. "gpt-4.1-mini", "claude-opus-4-6")
- `max_steps` — maximum steps the agent may take (default: 30)
- `max_cost_usd` — maximum spend in USD (default: 2.0)
- `system_prompt` — custom system prompt to prepend
- `allowed_tools` — list of tool names to restrict the agent to (empty = all tools)

**Errors returned:**
- Built-in profile names (github, researcher, reviewer, bash-runner, file-writer, full) are rejected — they are read-only.
- A profile with the given name that already exists in the user directory is rejected.
- Invalid profile names (containing path separators, empty strings) are rejected.
