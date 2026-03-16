Read the contents of a file from the workspace. Returns the file text, a version hash for conflict detection, and whether the output was truncated.

Use this tool when you need to inspect or review the contents of a specific, known file. You must provide the file path relative to the workspace root.

Parameters:
- path (required): Relative file path inside the workspace (e.g. "go.mod", "internal/server/http.go").
- file_path: Alias for path — either may be used.
- offset: Zero-based line offset to start reading from (e.g. offset=9 starts at line 10). Omit to read from the beginning.
- limit: Maximum number of lines to return from the offset. Omit to read to the end of the file (subject to max_bytes).
- max_bytes: Maximum bytes to read (1–1048576). Defaults to 16 384. Content beyond this limit is truncated.
- hash_lines: If true, each line is prefixed with a 12-character hex hash of its content, formatted as `[abc123def456] 42→content of line`. Use this when you need to reference specific lines by hash for reliable hash-addressed edits. The hash is stable: it identifies a line by content regardless of surrounding changes.

When offset or limit is provided, the response includes a "lines" array with line numbers and text for precise navigation.

This tool reads a single file whose path you already know. Do NOT use it to search for text across files (use grep), list directory contents (use glob or bash ls), or discover which files exist (use glob).