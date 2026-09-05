# Profile Authoring

## What Is a Profile

A profile is a named, reusable subagent configuration stored as a TOML file. It
specifies which model to use, how many steps the agent may take, what tools it
may call, and what its system prompt says. Profiles are the primary mechanism
for tailoring subagent behavior to a specific task class.

Use a profile when:

- You spawn the same type of subagent repeatedly (e.g., always a read-only
  researcher or always a GitHub automation agent).
- You want to enforce a tool allowlist so the agent cannot call tools outside
  its scope.
- You need cost or step ceilings that differ from server defaults.
- You want a task-specific system prompt prepended to every run.

A profile is optional. When no profile is specified, the run inherits the
server's startup defaults (typically the `full` profile).

---

## Profile TOML Schema

A profile file has an optional top-level `extends` key (declares a parent
profile name for inheritance; the child inherits unresolved fields from the
base) plus four sections: `[meta]`, `[runner]`, `[tools]`, and `[permissions]`.
The `[mcp_servers]` section is optional (`internal/profiles/profile.go:19-35`).

### `[meta]` — Identity and metadata

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | **required** | Unique identifier. Kebab-case recommended (e.g. `code-reviewer`). Must not conflict with built-in names when creating via API. |
| `description` | string | **required** | Human-readable summary of the profile's purpose. |
| `version` | int | `1` | Monotonically increasing version counter. |
| `created_at` | string | `""` | ISO date string (e.g. `"2026-03-20"`). Informational only. |
| `created_by` | string | `""` | Who created this profile: `"built-in"`, `"agent"`, `"user"`, or `"api"`. |
| `efficiency_score` | float | `0.0` | Last recorded efficiency score (0.0–1.0). Updated by efficiency analysis; do not set manually. |
| `review_count` | int | `0` | Number of efficiency reviews performed. |
| `review_eligible` | bool | `false` | Set to `true` for user-created profiles that should participate in efficiency review. Always `false` for built-ins. |

### `[runner]` — Execution parameters

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `model` | string | `""` | Model to use for runs under this profile (e.g. `"gpt-4.1-mini"`, `"claude-opus-4-6"`). Empty inherits server default. |
| `max_steps` | int | `0` | Maximum tool-use steps per run. `0` means no limit (server default applies). |
| `max_cost_usd` | float | `0.0` | Per-run cost ceiling in USD. `0.0` means no profile-level ceiling. |
| `system_prompt` | string | `""` | System prompt injected at the start of every run using this profile. Empty means no profile-level system prompt. |
| `reasoning_effort` | string | `""` | Reasoning effort hint forwarded to the provider. Valid values: `"low"`, `"medium"`, `"high"`. Empty means provider default. |
| `max_turns` | int | `0` | Maximum assistant LLM turns per run. `0` means no limit. |

### `[tools]` — Tool allowlist

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `allow` | []string | `[]` | List of tool names the agent may call. An **empty slice means all tools are allowed**. Use an explicit list to restrict the agent to a named subset. |

### `[permissions]` — Sandbox policy

An omitted permission field inherits request/runner defaults. An explicitly
present `false` denies that capability for selected ordinary runs; an explicit
`true` permits it subject to the request's own safety policy.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `allow_bash` | bool | `false` | Permit bash/shell tool calls. |
| `allow_file_write` | bool | `false` | Permit file-write tool calls. |
| `allow_net_access` | bool | `false` | Permit network access. |
| `allowed_commands` | []string | `[]` | Parsed profile metadata, but **not yet enforced for ordinary selected-profile runs**. Do not rely on it as an allowlist; use `allow_bash = false` or an explicit run permission rule until command-level profile enforcement ships. |

### Selected-profile precedence for ordinary runs

When a client selects a profile (for example through TUI `/profiles`), the
server composes it before run admission. An explicit run request wins for
model, max-step/turn/cost budgets, system prompt, and reasoning effort. A
profile's non-empty tool allowlist is an upper bound: request `allowed_tools`
may narrow it but cannot add a tool. Explicit profile file-write and network
denials are enforced by registered tool action at both offer and dispatch
(`ActionWrite`, `ActionFetch`, and the dual `ActionDownload`). External MCP
RPC tools and `connect_mcp` setup are conservatively classified as
`ActionFetch`, so a network-denied profile blocks them before an untrusted
server is called. The same action and
profile policy carries into `ContinueRun`; bash is denied by its distinct tool
policy. A continuation `allowed_tools` override may narrow a selected profile's
non-empty tool allowlist but cannot replace or widen it. This keeps an API
caller from using a broad request to escape a profile the operator selected.
Recipe members are evaluated against the same direct-call gates before any
recipe handler executes: selected-profile bounds, active skills, fine-grained
permission rules, and operator approval all apply to each member. Allowing
`run_recipe` never implicitly allows its `bash`, write, fetch, or other member
tools. Pre-tool hooks also see and may deny or rewrite each substituted member
argument; member approvals use stable indexed identities, including recipes
whose step names are duplicate or omitted. As with direct calls, a member that
fails its profile, allowlist, or active-skill gate never reaches a hook.

No selected profile means no composition change. Startup and subagent profile
paths retain their existing profile semantics.

### Top-level fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `isolation_mode` | string | `""` | Workspace isolation backend: `"none"`, `"worktree"`, `"container"`, `"vm"`. Empty inherits server defaults. |
| `cleanup_policy` | string | `""` | Workspace lifecycle after run completes: `"keep"`, `"delete"`, `"delete_on_success"`. Empty inherits server defaults. |
| `base_ref` | string | `""` | Git ref to use as base for worktree-backed runs (e.g. `"main"`). Empty inherits runner defaults. |
| `result_mode` | string | `""` | How child/subagent output is formatted: `"summary"`, `"full"`, `"structured"`. Empty inherits defaults. |

---

## Resolution Tiers

The loader resolves a profile by name using three tiers, highest priority first:

1. **Project-level** — `.harness/profiles/<name>.toml` relative to the working
   directory where `harnessd` started.
2. **User-global** — `~/.harness/profiles/<name>.toml` on the machine running
   `harnessd`.
3. **Built-in** — profiles embedded in the binary at compile time
   (`internal/profiles/builtins/`).

When the same name appears in multiple tiers, the highest-priority tier wins.
This lets you override a built-in profile for a specific project without
changing the global configuration.

---

## Built-In Profiles Catalog

The following profiles are embedded in the binary and available on every
deployment.

| Name | Purpose | Tools | max_steps | max_cost_usd |
|------|---------|-------|-----------|--------------|
| `full` | Default — all tools available | (all) | 30 | $2.00 |
| `researcher` | Read-only analysis, no writes | `read`, `grep`, `glob`, `ls`, `web_search`, `web_fetch` | 10 | $0.25 |
| `reviewer` | Code review, strictly no writes | `read`, `grep`, `glob`, `ls`, `git_diff` | 10 | $0.25 |
| `file-writer` | Code changes to specific files | `read`, `write`, `edit`, `apply_patch`, `bash` | 15 | $0.50 |
| `bash-runner` | Script execution, pipeline tasks | `bash` | 10 | $0.25 |
| `github` | GitHub automation: issues, PRs, repo management | `bash`, `read` | 20 | $0.50 |

Built-in profiles are read-only. They cannot be modified or deleted via the API.

---

## Creating a Profile

### Option 1: Write a TOML file directly

Create `.harness/profiles/my-profile.toml` (project-level) or
`~/.harness/profiles/my-profile.toml` (user-global):

```toml
[meta]
name = "my-profile"
description = "Focused Go linter that only reads source files"
version = 1
created_at = "2026-03-20"
created_by = "user"
review_eligible = true

[runner]
model = "gpt-4.1-mini"
max_steps = 8
max_cost_usd = 0.15
system_prompt = "You are a Go code linter. Analyze code for style issues. Never write files."

[tools]
allow = ["read", "grep", "glob", "ls"]
```

The profile is available immediately — no server restart required.

### Option 2: Via the HTTP API

```bash
curl -s -X POST http://localhost:8080/v1/profiles/my-profile \
  -H "Content-Type: application/json" \
  -d '{
    "description": "Focused Go linter",
    "model": "gpt-4.1-mini",
    "max_steps": 8,
    "max_cost_usd": 0.15,
    "system_prompt": "You are a Go code linter. Analyze code for style issues. Never write files.",
    "allowed_tools": ["read", "grep", "glob", "ls"]
  }'
```

Response on success (201 Created):

```json
{"status": "created", "name": "my-profile"}
```

Profile mutation is enabled by the normal `harnessd` startup path. By default
it writes to `~/.harness/profiles`; to exercise or operate it without touching
that directory, start the daemon with an **absolute** isolated directory:

```bash
HARNESS_PROFILES_DIR=/absolute/path/to/profiles harnessd
```

Relative values are rejected at startup. The override changes only the
user-global writable tier; project profiles continue to resolve from
`$HARNESS_WORKSPACE/.harness/profiles`, ahead of the isolated user tier and
built-ins. POST, PUT, and DELETE to `/v1/profiles/{name}` require the
`runs:write` scope. Attempting to create a profile with a built-in name returns
409 Conflict.

### Option 3: Via the `create_profile` tool (from inside a run)

When running inside the harness, the `create_profile` tool (TierDeferred)
creates a profile programmatically. Use `find_tool create_profile` to discover
it, then call it with the same fields as the HTTP API.

---

## Validating a Profile

### Via GET /v1/profiles/{name}

A 200 response confirms the profile loaded successfully. A 404 means it was not
found in any resolution tier.

```bash
curl -s http://localhost:8080/v1/profiles/my-profile | python3 -m json.tool
```

Example response:

```json
{
  "name": "my-profile",
  "description": "Focused Go linter",
  "model": "gpt-4.1-mini",
  "max_steps": 8,
  "max_cost_usd": 0.15,
  "allowed_tools": ["read", "grep", "glob", "ls"],
  "allowed_tool_count": 4,
  "source_tier": "user",
  "created_by": "user"
}
```

### Via the `validate_profile` tool (from inside a run)

The `validate_profile` tool (TierDeferred) performs a dry-run validation of a
profile definition without writing any files. Use it to check a profile
JSON/TOML payload before committing it.

---

## Listing Available Profiles

```bash
curl -s http://localhost:8080/v1/profiles | python3 -m json.tool
```

Response:

```json
{
  "profiles": [
    {
      "name": "full",
      "description": "Default — all tools available",
      "model": "gpt-4.1-mini",
      "allowed_tool_count": 0,
      "source_tier": "built-in"
    },
    {
      "name": "my-profile",
      "description": "Focused Go linter",
      "model": "gpt-4.1-mini",
      "allowed_tool_count": 4,
      "source_tier": "user"
    }
  ],
  "count": 7
}
```

`allowed_tool_count: 0` means all tools are allowed (empty allow list = no restriction).

---

## Example: Custom Profile with Isolation

A profile that runs in a git worktree and cleans up on success:

```toml
[meta]
name = "isolated-patcher"
description = "Applies patches in a git worktree, cleans up on success"
version = 1
created_at = "2026-03-20"
created_by = "user"
review_eligible = true

[runner]
model = "gpt-4.1-mini"
max_steps = 20
max_cost_usd = 1.00
system_prompt = "Apply the requested patch precisely. Verify tests pass before completing."

[tools]
allow = ["read", "write", "edit", "apply_patch", "bash", "grep", "glob"]

isolation_mode = "worktree"
cleanup_policy = "delete_on_success"
base_ref = "main"
result_mode = "structured"
```

---

## Notes

- Profile names must be valid kebab-case identifiers. The validator rejects names
  with path separators or characters that could escape the profiles directory.
- The `[permissions]` section zero value means "no override". An explicit `false`
  is tracked separately from an absent field (`BashSpecified`/`FileWriteSpecified`/
  `NetAccessSpecified`, `internal/profiles/profile.go:93-108`) and denies that
  capability; an omitted field inherits from the request or server defaults. See
  the `[permissions]` table above.
- Efficiency scores and review counts in `[meta]` are managed by the efficiency
  analysis system. Do not set them manually in authored profiles.
- Built-in profile TOML files live at `internal/profiles/builtins/` in the
  source tree. They are embedded in the binary at compile time and cannot be
  modified at runtime.
