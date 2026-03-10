Inspect a file's metadata, type, and content preview without reading the entire file. Returns structured information including size, MIME type, encoding (text vs binary), and either a line-based text preview or a hex dump for binary files.

Use this tool when you need to understand what a file is before deciding how to process it — for example, to check whether a file is text or binary, determine its MIME type, see its size, or get a quick preview of its contents. This is especially useful for unfamiliar files, large files where reading the whole content would be wasteful, or binary files where the read tool would return garbled output.

Parameters:
- path (required): Relative file path inside the workspace (e.g. "assets/logo.png", "internal/server/http.go").
- preview_lines: For text files, the number of lines to include in the preview (1–100). Default 20. Ignored for binary files.
- hex_bytes: For binary files, the number of bytes to include in the hex dump (1–1024). Default 256. Ignored for text files.

Returns a JSON object with:
- path: the inspected file path (relative to workspace root)
- size_bytes: file size in bytes
- size_human: human-readable file size (e.g. "4.2 KB", "1.3 MB")
- mime_type: detected MIME type (e.g. "text/plain", "image/png", "application/octet-stream")
- encoding: "utf-8" for valid UTF-8 text files, "binary" otherwise
- extension: file extension including the dot (e.g. ".go", ".png"), or empty string if none
- preview: (text files only) the first N lines of the file as a string
- preview_lines: (text files only) number of lines included in the preview
- total_lines: (text files only) total number of lines in the file
- hex_preview: (binary files only) hex dump of the first N bytes
- truncation_warning: present only when the preview was truncated, explains what was omitted

This tool inspects metadata and provides previews only. It does NOT modify files, read entire file contents, search within files, or list directories.