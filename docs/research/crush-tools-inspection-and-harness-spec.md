# Crush Tools Inspection and Harness Spec

**Date:** 2026-03-05  
**Scope:** Inspect `charmbracelet/crush` tool surface and define a practical compatibility spec for `go-agent-harness`.
**Source commit:** `fae0f2e82da57a0e0335d86b417a819121f4e69f` (GitHub `main` on 2026-03-05).

## Source Evidence

- `internal/agent/coordinator.go` (`buildTools`)
- `internal/agent/tools/*.go` and `internal/agent/tools/*.md`
- `internal/agent/agent_tool.go`
- `internal/agent/agentic_fetch_tool.go`

## 1) Crush Tool Surface (Observed)

## Core coding/runtime tools

- `bash(description, command, working_dir?, run_in_background?)`
- `job_output(shell_id, wait?)`
- `job_kill(shell_id)`
- `download(url, file_path, timeout?)`
- `edit(file_path, old_string, new_string, replace_all?)`
- `multiedit(file_path, edits[])`
- `fetch(url, format, timeout?)`
- `glob(pattern, path?)`
- `grep(pattern, path?, include?, literal_text?)`
- `ls(path?, ignore?, depth?)`
- `sourcegraph(query, count?, context_window?, timeout?)`
- `todos(todos[])`
- `view(file_path, offset?, limit?)`
- `write(file_path, content)`

## LSP tools

- `lsp_diagnostics(file_path?)`
- `lsp_references(symbol, path?)`
- `lsp_restart(name?)`

## MCP tools

- `list_mcp_resources(mcp_name)`
- `read_mcp_resource(mcp_name, uri)`
- Dynamic MCP tools at runtime as `mcp_<server>_<tool>` with server-provided schema.

## Sub-agent tools

- `agent(prompt)`
- `agentic_fetch(prompt, url?)`
- `agentic_fetch` sub-agent toolset: `web_search(query, max_results?)`, `web_fetch(url)`, plus local `glob/grep/sourcegraph/view`.

## 2) Permission and Safety Model (Observed)

Approval-gated actions in Crush:

- `bash` -> action `execute` (skips approval for commands matching internal safe read-only list).
- `download` -> action `download`.
- `edit`, `multiedit`, `write` -> action `write`.
- `fetch`, `agentic_fetch` -> action `fetch`.
- `ls` -> action `list` (only when listing outside working directory).
- `view` -> action `read` (only outside working directory; skill docs exempted).
- `list_mcp_resources` -> action `list`.
- `read_mcp_resource` -> action `read`.
- dynamic `mcp_*` tools -> action `execute`.

Non-gated tools in current Crush implementation:

- `grep`, `glob`, `sourcegraph`, `todos`, `job_output`, `job_kill`, `lsp_*`, `agent`.

Safety behaviors worth copying:

- `bash` blocks high-risk command families (network/admin/package-management/system-modification).
- `write`/`edit`/`multiedit` enforce "read before write" and stale-read checks via file tracker.
- `view` enforces max size for non-skill text and model capability checks for images.
- `fetch` and `download` validate protocol and apply explicit timeout/size bounds.
- `multiedit` supports partial success and returns failed edit metadata.

## 3) Behavior Contracts Per Tool (Condensed)

- `bash`: foreground/background execution; long-running commands can be moved to background; use `job_output` and `job_kill`.
- `edit`: strict exact-match replacement semantics; can create file (`old_string=""`) or delete (`new_string=""`).
- `multiedit`: ordered edit sequence against evolving file state; partial-apply semantics.
- `view`: returns numbered content, diagnostics, and image responses for supported formats.
- `todos`: structured task-state update (`pending|in_progress|completed`) persisted per session.
- `agentic_fetch`: either URL analysis path or search-first path with temporary workspace and auto-approved child session.

## 4) Harness Spec for `go-agent-harness`

## 4.1 Canonical tool descriptor

```yaml
tool:
  name: string
  description: string
  category: [core, lsp, mcp, orchestration, web]
  mutating: boolean
  parallel_safe: boolean
  permission:
    required: boolean
    action: [read, write, list, execute, fetch, download]
    scope: [path, workspace, session]
  input_schema: json-schema
  output_mode: [text, image, media, metadata]
  defaults:
    timeout_ms: number
    max_results: number
```

## 4.2 Crush-compatible tool names to preserve

Keep these names for easier model prompt transfer:

- `bash`, `job_output`, `job_kill`
- `download`, `edit`, `multiedit`, `write`, `view`
- `glob`, `grep`, `ls`, `fetch`, `sourcegraph`
- `todos`
- `lsp_diagnostics`, `lsp_references`, `lsp_restart`
- `list_mcp_resources`, `read_mcp_resource`
- `agent`, `agentic_fetch`, `web_search`, `web_fetch`

## 4.3 Phase implementation plan

## Phase 1 (MVP parity for coding)

- `view`, `grep`, `glob`, `ls`
- `write`, `edit`, `multiedit`
- `bash`, `job_output`, `job_kill`
- `fetch`, `download`
- Permission actions: `read`, `write`, `list`, `execute`, `fetch`, `download`.

## Phase 2 (developer-quality parity)

- `todos`
- `lsp_diagnostics`, `lsp_references`, `lsp_restart`
- `sourcegraph`
- stale-read protection and diff metadata on file writes.

## Phase 3 (extensibility and orchestration)

- `list_mcp_resources`, `read_mcp_resource`, dynamic `mcp_*`
- `agent`, `agentic_fetch`
- sub-agent web tools (`web_search`, `web_fetch`).

## 4.4 Minimal JSON schema examples for our implementation

`bash`:

```json
{
  "type": "object",
  "properties": {
    "description": { "type": "string" },
    "command": { "type": "string" },
    "working_dir": { "type": "string" },
    "run_in_background": { "type": "boolean" }
  },
  "required": ["description", "command"],
  "additionalProperties": false
}
```

`view`:

```json
{
  "type": "object",
  "properties": {
    "file_path": { "type": "string" },
    "offset": { "type": "integer", "minimum": 0 },
    "limit": { "type": "integer", "minimum": 1 }
  },
  "required": ["file_path"],
  "additionalProperties": false
}
```

`multiedit`:

```json
{
  "type": "object",
  "properties": {
    "file_path": { "type": "string" },
    "edits": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "old_string": { "type": "string" },
          "new_string": { "type": "string" },
          "replace_all": { "type": "boolean" }
        },
        "required": ["old_string", "new_string"],
        "additionalProperties": false
      }
    }
  },
  "required": ["file_path", "edits"],
  "additionalProperties": false
}
```

## 5) Direct Implementation Guidance for This Repo

1. Match Crush parameter names exactly (`snake_case`) where practical.
2. Implement permission gating as first-class middleware with `action` and `path`.
3. Separate mutating vs non-mutating tool execution lanes (retry policy and concurrency differ).
4. Add file-read tracking before enabling `write`/`edit` in autonomous mode.
5. Keep orchestration tools (`agent`, `agentic_fetch`) behind feature flags until core tool reliability is stable.

## 6) Acceptance Criteria for "Crush-like" Tooling

1. Tool names and schemas are wire-compatible for Phase 1 tools.
2. Permission prompts include action, path, and tool-specific params.
3. Background command lifecycle works end-to-end (`bash` -> `job_output`/`job_kill`).
4. File mutation tools reject stale writes and emit structured diff metadata.
5. MCP static + dynamic tool registration works without restarting the harness.
