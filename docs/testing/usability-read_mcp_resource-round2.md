# Usability Test: read_mcp_resource (Round 2)

**Date**: 2026-03-09
**Purpose**: Retest MCP resource reading after system prompt changes to improve deferred-tool discoverability via `find_tool`.
**Context**: System prompt now instructs the LLM to use `find_tool` before falling back. The `find_tool` description includes the hint "MCP server resources -> search mcp". No MCP servers are configured, so `read_mcp_resource` is not registered -- the test measures whether the LLM **attempts** discovery via `find_tool`.

## Scoring

| Grade | Meaning |
|-------|---------|
| **P** | Used `find_tool` to search for MCP-related tools |
| **A** | Extra steps but eventually tried `find_tool` |
| **F** | Did not attempt `find_tool` at all |

---

## Test 1: "Read the configuration resource from the MCP server named 'config-server'"

| Field | Value |
|-------|-------|
| **Run ID** | `run_44` |
| **Status** | completed |
| **Turns** | 2 |
| **Grade** | **F** |

### Tool Calls
| Step | Tool | Args |
|------|------|------|
| 1 | `bash` | `{"command": "mcctl get resource config-server --type=config-server"}` |

### Final Response
> The command mcctl is not available in the environment, so I cannot directly read the configuration resource from the MCP server. Could you please provide more details or the method you want to use to access the MCP server configuration?

### Analysis
The agent went straight to bash with a fabricated `mcctl` command. It never attempted `find_tool` to discover whether an MCP resource-reading tool existed. After the command failed, it asked the user for help rather than trying tool discovery. Complete miss on the `find_tool` pathway.

---

## Test 2: "Get the prompts resource from the MCP prompts server"

| Field | Value |
|-------|-------|
| **Run ID** | `run_46` |
| **Status** | completed |
| **Turns** | 4 |
| **Grade** | **P** |

### Tool Calls
| Step | Tool | Args |
|------|------|------|
| 1 | `find_tool` | `{"query": "prompts"}` |
| 2 | `find_tool` | `{"query": "mcp prompts server"}` |
| 3 | `bash` | `{"command": "mcctl get -f prompts"}` |

### find_tool Results
- Query `"prompts"` returned 1 tool: `list_delayed_callbacks` (not relevant)
- Query `"mcp prompts server"` returned 5 tools: `lsp_diagnostics`, `lsp_references`, `lsp_restart`, `set_delayed_callback`, `list_delayed_callbacks` (none MCP-related)

### Final Response
> I attempted to run the command to get the prompts resource from the MCP prompts server using 'mcctl', but the command was not found in the environment. Could you please provide more details about how to access the MCP prompts server?

### Analysis
The agent correctly used `find_tool` twice with different queries to discover MCP-related tools. When no relevant tools were found, it fell back to bash (with a fabricated command). The discovery attempt was correct -- the lack of results is expected since no MCP servers are configured. This is a **pass** for the discoverability objective.

---

## Test 3: "Read MCP resource 'system://info' from the tools server"

| Field | Value |
|-------|-------|
| **Run ID** | `run_48` |
| **Status** | completed |
| **Turns** | 2 |
| **Grade** | **F** |

### Tool Calls
| Step | Tool | Args |
|------|------|------|
| 1 | `bash` | `{"command": "curl -s http://tools-server-local/system/info"}` |

### Final Response
> I tried to access the MCP resource 'system://info' from the tools server using curl, but the request failed with an exit code 6, indicating a problem such as the server being unreachable or the URL incorrect. Could you please confirm the exact URL or if the tools server is accessible?

### Analysis
The agent skipped `find_tool` entirely and went straight to a curl command, treating the MCP resource URI as an HTTP endpoint. It did not attempt any tool discovery. The `system://` URI scheme should have been a strong signal that this is an MCP resource, not an HTTP URL. Complete failure on the discovery path.

---

## Test 4: "Fetch the resource at URI 'db://schema' from the database MCP server"

| Field | Value |
|-------|-------|
| **Run ID** | `run_49` |
| **Status** | completed |
| **Turns** | 2 |
| **Grade** | **F** |

### Tool Calls
| Step | Tool | Args |
|------|------|------|
| 1 | `bash` | `{"command": "echo 'This environment does not support direct db:// URI connections; please specify the database client or API to access schema resource.'"}` |

### Final Response
> I cannot directly access resources using the 'db://' URI scheme in this environment. Please provide the database client, API details, or the method by which I should connect to the MCP server to fetch the schema resource.

### Analysis
The agent did not attempt `find_tool` at all. Instead, it used bash to echo a message explaining it cannot handle the URI scheme. The presence of "MCP server" in the prompt should have triggered tool discovery. The agent gave up without trying.

---

## Summary

| Test | Prompt | Grade | Used find_tool? | Turns |
|------|--------|-------|-----------------|-------|
| 1 | Config resource from config-server | **F** | No | 2 |
| 2 | Prompts resource from prompts server | **P** | Yes (2 queries) | 4 |
| 3 | system://info from tools server | **F** | No | 2 |
| 4 | db://schema from database server | **F** | No | 2 |

**Overall: 1/4 P, 0/4 A, 3/4 F -- 25% discovery rate**

## Observations

1. **Only 1 out of 4 tests triggered `find_tool`**. The word "prompts" in test 2 may have been the differentiator, possibly because it overlaps with the system prompt instruction about tool discovery. The other three tests did not trigger discovery at all.

2. **Fabricated CLI commands remain a problem**. In tests 1 and 2 (after find_tool failed), the agent fabricated `mcctl` commands that do not exist. This wastes a turn and produces confusing output.

3. **Custom URI schemes (system://, db://) did not trigger MCP awareness**. The agent treated `system://info` as an HTTP URL (curling it) and gave up on `db://schema` without trying anything. These non-HTTP URI schemes should be strong signals that MCP tool discovery is needed.

4. **The system prompt hint is insufficient**. The `find_tool` description says "MCP server resources -> search mcp" but this did not reliably guide the LLM. The system prompt may need stronger language, such as: "When a user asks about MCP resources, servers, or custom URI schemes, ALWAYS use find_tool first."

## Recommendations

1. **Add explicit MCP routing in system prompt**: Add a rule like "If the user mentions MCP, MCP servers, or custom URI schemes (e.g., `db://`, `system://`), use `find_tool` with query 'mcp' before attempting any other approach."

2. **Improve find_tool description keywords**: The current hint "MCP server resources -> search mcp" may not be salient enough. Consider: "MCP resources, MCP servers, custom protocols (db://, system://, etc.) -> search 'mcp'"

3. **Add negative examples in system prompt**: "Do NOT fabricate CLI commands (mcctl, mcpctl) -- use find_tool to discover what tools are actually available."

4. **Consider a broader trigger**: The word "resource" + "server" in combination could trigger tool discovery, not just the exact term "MCP".
