Retrieve a suggest-only efficiency report for a named agent profile.

The report summarises aggregate run history (step counts, cost, success rate, top tools) and provides text suggestions for how the profile could be refined. Suggestions are NEVER auto-applied — they are guidance only and must be reviewed by a human or dedicated review workflow before any profile change is made.

**Input**
- `profile_name` (required) — the name of the profile to inspect (e.g. `researcher`, `github`, `full`).

**Output** — JSON object with:
- `profile_name` — the profile queried
- `generated_at` — ISO-8601 timestamp
- `run_count` — number of runs recorded
- `avg_steps` — mean step count across runs
- `avg_cost_usd` — mean cost in USD
- `success_rate` — fraction of runs that completed successfully (0.0–1.0)
- `top_tools` — most-used tools, descending order
- `suggestions` — list of suggest-only refinement hints (may be empty for healthy profiles)
- `has_history` — false when fewer than 3 runs have been recorded (suggestions are suppressed)

**Important**: when `has_history` is false, suggestions will contain a single "not enough history" message. Wait until at least 3 runs have been recorded before acting on efficiency data.
