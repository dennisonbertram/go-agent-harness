Run Go language-server diagnostics on a file or the entire workspace using gopls check. Returns compiler errors, type errors, unused imports, undeclared names, and other static-analysis findings that the Go language server detects.

Use this tool when you need to check Go source code for compilation errors, type mismatches, missing imports, unused variables, or other diagnostics that a compiler or language server would report. This is the preferred way to validate whether Go code compiles cleanly — do NOT shell out to gopls directly via bash.

Parameters:
- file_path (optional): Relative path to a single .go file to check (e.g. "internal/server/http.go"). If omitted, checks the entire workspace ("./...").

Returns a JSON object with:
- output: The gopls check output — one diagnostic per line in the format "file:line:col: message", or empty if no issues found.
- exit_code: 0 if no diagnostics, non-zero if issues were found or gopls encountered an error.
- timed_out: true if the check exceeded the 30-second timeout.

Tips:
- For a quick health check on a single file after editing, pass its path. For a broad sweep, omit file_path.
- An empty output with exit_code 0 means the code is clean.
- If gopls is not installed in the environment, the tool returns an error — this is an infrastructure issue, not a code issue.
- This tool only works on Go source files. Non-Go files (e.g. .md, .yaml) will produce unhelpful or empty output.