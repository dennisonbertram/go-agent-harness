Read the contents of a resource exposed by a connected MCP (Model Context Protocol) server. Returns the resource body as text along with the server name and URI that was read.

Use this tool when you need to retrieve data from an external MCP server resource identified by a URI. The MCP server must already be connected and registered with the harness. To discover which resources are available on a server, use list_mcp_resources first.

This tool reads resources over the MCP protocol. Do NOT use this tool to read local workspace files (use the read tool instead). Do NOT use bash commands like "mcp fetch" or "mcpctl" -- use this tool directly.

Parameters:
- mcp_name (required): The registered name of the MCP server to read from (e.g. "my-server", "prompt-store"). This must match the name used when the server was configured.
- uri (required): The MCP resource URI to read (e.g. "config://settings", "data://users", "prompts://system-prompt"). The URI scheme and path depend on what the MCP server exposes.

Returns a JSON object with:
- mcp_name: the server name that was queried
- uri: the resource URI that was read
- content: the resource body as text

Both mcp_name and uri are required. The tool will return an error if either is empty or if the MCP server is not found.