Connect to a new MCP (Model Context Protocol) server mid-session and register its tools for immediate use.

Use this tool when you need to integrate with an MCP server that was not configured at startup. The server's tools will be discovered and registered into the active session, making them available from the next tool call onward.

Only HTTP/SSE MCP servers are supported (not stdio). The server must expose a standard MCP HTTP endpoint.

Parameters:
- url (required unless command provided): HTTP or HTTPS URL of the MCP server endpoint (e.g. "http://localhost:3000/mcp"). The server must implement the MCP HTTP/SSE transport.
- server_name (optional): A short display name for this server (e.g. "my-server"). If omitted, a name is derived from the URL. Must contain only alphanumeric characters, hyphens, and underscores. The name is used as a prefix for the registered tool names (e.g. "mcp_my_server_tool_name").

Returns a JSON object with:
- server_name: the name used for this server
- tools_registered: list of tool names that were registered
- count: number of tools registered

Errors:
- Returns an error if the URL is empty or uses an unsupported scheme
- Returns an error if a server with the same name is already connected
- Returns an error if the MCP server cannot be reached or returns an invalid response
- Returns an error if no tools were discovered on the server
