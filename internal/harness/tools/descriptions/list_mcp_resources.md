List the resources available on a connected MCP (Model Context Protocol) server. Returns a list of resource URIs and metadata exposed by that server.

Use this tool to discover what resources an MCP server provides before reading them. Each returned resource includes a URI that can be passed to read_mcp_resource.

This tool queries MCP servers over the MCP protocol. Do NOT use bash commands like "mcpctl" or "mcp list" -- use this tool directly.

Parameters:
- mcp_name (required): The registered name of the MCP server to query (e.g. "my-server", "docs-server"). This must match the name used when the server was configured.

Returns a JSON object with:
- mcp_name: the server name that was queried
- resources: an array of resource objects with URIs and metadata