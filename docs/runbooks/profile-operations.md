# Profile Operations

## Choosing the Right Profile

### Via the `recommend_profile` tool (from inside a run)

The `recommend_profile` tool uses keyword matching on a task description and
returns the best built-in profile. No LLM inference is performed — the
recommendation is deterministic and free.

Use `find_tool recommend_profile` to discover the tool, then call it:

```json
{"task": "Review the authentication module for security issues"}
```

Response:

```json
{
  "profile_name": "reviewer",
  "reason": "Task matched keyword \"review\" — using reviewer profile (review/audit task).",
  "confidence": "high"
}
```

The recommender applies rules in this order (first match wins):

| Keywords | Recommended Profile |
|----------|-------------------|
| `review`, `audit`, `check`, `analyze` | `reviewer` |
| `research`, `search`, `find`, `investigate` | `researcher` |
| `bash`, `shell`, `script`, `run command` | `bash-runner` |
| `write file`, `create file`, `edit file` | `file-writer` |
| `github`, `pull request`, ` pr `, `issue` | `github` |
| (no match) | `full` |

The recommendation is advisory only. It does not override an explicit profile
choice. If you already know which profile to use, call `run_agent` directly
without consulting `recommend_profile`.

### Via the HTTP API

```bash
# List all profiles with metadata
curl -s http://localhost:8080/v1/profiles | python3 -m json.tool

# Inspect a specific profile's tool allowlist
curl -s http://localhost:8080/v1/profiles/researcher | python3 -m json.tool
```

Choose based on:

- **Task scope**: read-only tasks → `researcher` or `reviewer`; write tasks → `file-writer`
- **Cost sensitivity**: `researcher`, `reviewer`, `bash-runner` cap at $0.25; `full` caps at $2.00
- **Tool needs**: check `allowed_tools` in the profile summary to confirm required tools are present

---

## Starting harnessd with a Profile

Pass `--profile` to `harnessd` at startup to set server-wide defaults:

```bash
./harnessd --profile full
```

The named profile's `[runner]` fields become the server defaults for all runs
that do not specify their own model, max_steps, or cost ceiling. Environment
variables (`HARNESS_MODEL`, `HARNESS_MAX_STEPS`, etc.) take higher precedence
and override the profile at startup.

If `--profile` is omitted, `harnessd` starts with compiled-in defaults and no
profile-level system prompt.

### Environment variable layering order (highest to lowest priority)

1. `HARNESS_*` environment variables
2. Project-level `.harness/config.toml`
3. User-global `~/.harness/config.toml`
4. `--profile` startup profile
5. Compiled-in defaults

---

## Setting a Profile in a Run Request

Include `"profile"` in the POST `/v1/runs` body to select a profile for that
specific run:

```bash
curl -s -X POST http://localhost:8080/v1/runs \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Search the codebase for all usages of the old API and summarize findings.",
    "profile": "researcher"
  }' | python3 -m json.tool
```

The profile is resolved using the three-tier loader at request time. If the
profile name is not found in any tier, the run is rejected with an error rather
than silently falling back.

Run-level fields (`model`, `max_steps`, `max_cost_usd`) in the request body
take precedence over the profile's values when both are specified.

---

## Using the `-prompt-profile` Flag in harnesscli

`harnesscli` accepts `-prompt-profile` for prompt routing (model selection
hints), not for the agent profile system. This is a separate mechanism used by
the prompt routing layer.

```bash
./harnesscli -base-url http://localhost:8080 \
  -prompt "Analyze auth.go for SQL injection vulnerabilities" \
  -prompt-profile "openai_gpt5"
```

To start the TUI, use `-tui`:

```bash
./harnesscli -base-url http://localhost:8080 -tui
```

---

## Viewing Efficiency Reports

An efficiency report summarises aggregate run history for a profile and provides
suggest-only refinement hints. Reports require at least 3 completed runs before
suggestions are generated.

### Via the `get_efficiency_report` tool (from inside a run)

Use `find_tool get_efficiency_report` to discover it, then call:

```json
{"profile_name": "researcher"}
```

Example response:

```json
{
  "profile_name": "researcher",
  "generated_at": "2026-03-20T14:00:00Z",
  "run_count": 12,
  "avg_steps": 7.4,
  "avg_cost_usd": 0.08,
  "success_rate": 0.92,
  "top_tools": ["read", "grep", "glob"],
  "suggestions": [],
  "has_history": true
}
```

When `has_history` is `false`, the `suggestions` array contains a single
`"Not enough history"` message. Do not act on efficiency data until `has_history`
is `true`.

Suggestions are **never auto-applied**. They are guidance for human or automated
review only.

### Efficiency score formula

The per-run efficiency score is deterministic:

```
efficiency = 1.0 / (1.0 + steps * 0.1 + cost_usd * 10.0)
```

Scores range from 0.0 to 1.0. Higher is better. When a run's efficiency score
falls below 0.6, the runner emits a `profile.efficiency_suggestion` SSE event
on the run's event stream.

---

## Profile Lifecycle Management via API

All mutation endpoints require `runs:write` scope. GET endpoints require
`runs:read`. The `profilesDir` must be configured on the server for mutations to
be enabled (returns 501 otherwise).

### Create a profile

```bash
curl -s -X POST http://localhost:8080/v1/profiles/go-linter \
  -H "Content-Type: application/json" \
  -d '{
    "description": "Lint Go files for style issues",
    "model": "gpt-4.1-mini",
    "max_steps": 8,
    "max_cost_usd": 0.15,
    "system_prompt": "You are a Go linter. Read and analyze only. Never write files.",
    "allowed_tools": ["read", "grep", "glob", "ls"]
  }'
```

Returns 201 on success. Returns 409 Conflict if the name matches a built-in
profile.

### Update a profile

Only user-created profiles can be updated. Built-in profiles return 403.

```bash
curl -s -X PUT http://localhost:8080/v1/profiles/go-linter \
  -H "Content-Type: application/json" \
  -d '{
    "description": "Lint Go files for style and correctness",
    "max_steps": 12
  }'
```

Fields omitted from the PUT body retain their existing values. Returns 200 on
success.

### Delete a profile

```bash
curl -s -X DELETE http://localhost:8080/v1/profiles/go-linter
```

Returns 200 on success. Returns 403 if the name is a built-in profile. Returns
404 if no user file exists for that name.

### List all profiles

```bash
curl -s http://localhost:8080/v1/profiles | python3 -m json.tool
```

The response includes profiles from all three resolution tiers (project, user,
built-in). Duplicate names are deduplicated; the highest-priority tier wins.

---

## Using Profiles with run_agent (Tool Call)

When a parent agent calls `run_agent` to spawn a subagent, the `profile` field
selects the profile:

```json
{
  "task": "Search internal/harness for all TODO comments and list them.",
  "profile": "researcher"
}
```

If the named profile does not exist, `run_agent` fails closed — it returns an
error rather than silently using a fallback. This prevents accidental unconstrained
runs when a typo occurs in a profile name.

---

## Notes

- The `--profile` flag at `harnessd` startup sets server-wide defaults. It is
  separate from the `"profile"` field in a run request, which is per-run.
- Profile names are case-sensitive. `"Researcher"` and `"researcher"` are
  different names.
- There is no hot-reload mechanism. File-based profile changes (TOML files
  added or modified on disk) are picked up on the next `LoadProfile` call
  without a server restart.
- The `recommend_profile` tool only knows about built-in profiles. It will not
  recommend a user-created profile, even if one exists.
