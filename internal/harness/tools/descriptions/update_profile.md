Update an existing user-created agent profile in the profiles directory.

Modifies the fields of a profile that already exists in the user profiles directory. Built-in profiles (github, researcher, reviewer, bash-runner, file-writer, full) cannot be updated — they are embedded in the binary and read-only.

**Required fields:**
- `name` — the name of the profile to update (must already exist in the user directory)

**Optional update fields (at least one should be provided):**
- `description` — new human-readable description
- `model` — new LLM model name
- `max_steps` — new maximum step count
- `max_cost_usd` — new cost ceiling in USD
- `system_prompt` — new system prompt
- `allowed_tools` — new list of allowed tool names (empty list = allow all tools)

**Errors returned:**
- Profile does not exist in the user directory (only user-created profiles can be updated).
- Profile name refers to a built-in profile (403 Forbidden).
- Invalid profile name.
