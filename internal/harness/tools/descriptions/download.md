Download a file from an HTTP/HTTPS URL and save it to a workspace path.

Use this tool whenever you need to save remote content to a file — it handles the HTTP request, creates parent directories automatically, and writes the response body to the specified path. It works for any content type including text, JSON, HTML, images, and other binary data.

IMPORTANT: Do NOT use bash with curl/wget to download files. Use this tool instead — it is safer, workspace-aware, and enforces size limits.

If you only need to read remote content without saving to a file, use fetch instead (fetch returns the content in the tool result without writing to disk).

Parameters:
- url (required): Full HTTP or HTTPS URL to download (e.g. "https://example.com/data.json").
- file_path (required): Workspace-relative path where the downloaded content will be saved. Parent directories are created automatically (e.g. "data/responses/output.json").
- timeout_seconds (optional): Request timeout in seconds (1–120, default 20).
- max_bytes (optional): Maximum download size in bytes (1–5242880, default 1048576). Downloads exceeding this limit are truncated.

Returns a JSON object with:
- url: the requested URL
- file_path: the workspace-relative path where the file was saved
- bytes_written: number of bytes written to disk
- status_code: HTTP status code
- content_type: the Content-Type header from the response
- truncated: true if the download was cut short at max_bytes
- version: file version hash for optimistic concurrency