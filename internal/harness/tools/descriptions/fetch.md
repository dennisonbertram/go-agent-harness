Fetch the contents of a remote URL (HTTP or HTTPS) and return the response body as text.

Use this tool to retrieve web pages, API responses, or any text-based content from a remote HTTP/HTTPS endpoint. The response body is returned as a string in the result.

This tool is READ-ONLY — it does NOT save content to disk. If you need to save a downloaded file to the workspace, use the **download** tool instead.

Do NOT use this tool for local files. Use **read** for local/workspace file access.

Parameters:
- url (required): Full HTTP or HTTPS URL to fetch (e.g. "https://api.example.com/data").
- format (optional): Hint for how to interpret the response (e.g. "json", "html", "text"). Does not change the request — it is passed through in the result for downstream use.
- timeout_seconds (optional): Request timeout in seconds (1–120, default 20).
- max_bytes (optional): Maximum response body size in bytes (1–1048576, default 131072). Responses exceeding this are truncated.

Returns a JSON object with:
- url: the requested URL
- status_code: HTTP status code
- content_type: the Content-Type header from the response
- content: the response body (text)
- truncated: true if the response was cut short at max_bytes