# Tool Roadmap

Status legend: `planned` | `in_progress` | `implemented` | `deferred`

## Canonical Mapping

| Crush capability | Harness canonical tool | Status |
|---|---|---|
| `view` | `read` (with `file_path`, `offset`, `limit`) | `implemented` |
| `edit` | `edit` | `implemented` |
| `multiedit` | `apply_patch` (`edits[]` mode) | `implemented` |
| `write` | `write` | `implemented` |
| `bash` | `bash` | `implemented` |
| `job_output` | `job_output` | `implemented` |
| `job_kill` | `job_kill` | `implemented` |
| `ls` | `ls` | `implemented` |
| `glob` | `glob` | `implemented` |
| `grep` | `grep` | `implemented` |
| `fetch` | `fetch` | `implemented` |
| `download` | `download` | `implemented` |
| `todos` | `todos` | `implemented` |
| observational memory control | `observational_memory` | `implemented` (local sqlite + local coordinator mode) |
| `sourcegraph` | `sourcegraph` | `implemented` (requires endpoint config) |
| `lsp_diagnostics` | `lsp_diagnostics` | `implemented` (requires `gopls`) |
| `lsp_references` | `lsp_references` | `implemented` (requires `gopls`) |
| `lsp_restart` | `lsp_restart` | `implemented` |
| `list_mcp_resources` | `list_mcp_resources` | `implemented` (requires MCP registry integration) |
| `read_mcp_resource` | `read_mcp_resource` | `implemented` (requires MCP registry integration) |
| dynamic `mcp_*` | dynamic `mcp_<server>_<tool>` | `implemented` (requires MCP registry integration) |
| `agent` | `agent` | `implemented` (requires agent runner integration) |
| `agentic_fetch` | `agentic_fetch` | `implemented` (requires agent runner + web fetcher integration) |
| `web_search` | `web_search` | `implemented` (requires web fetcher integration) |
| `web_fetch` | `web_fetch` | `implemented` (requires web fetcher integration) |

## Notes

- Tools are registered from a single catalog in `internal/harness/tools/catalog.go`.
- New tools should be added as a new file (or directory for larger tools) in `internal/harness/tools/` and a catalog entry.
- Not-yet-wired external dependencies (MCP, agent runner, web fetcher, sourcegraph endpoint) are not placeholders; they are fully implemented tool contracts gated by real dependency presence.
