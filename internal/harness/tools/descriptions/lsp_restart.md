Restart a language server process by name. Use this when the language server
(e.g. gopls) becomes unresponsive, returns stale diagnostics, or needs a
fresh start after configuration changes.

Parameters:
- name (string, optional): The language server to restart. Defaults to "gopls"
  if omitted. Common values: "gopls", "rust-analyzer", "typescript-language-server".

Returns a JSON object with {"restarted": true, "name": "<server_name>"}.

Prefer this tool over manually killing and restarting the language server
process via bash.