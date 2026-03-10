Grep (search) file contents using text or regex patterns. Recursively searches all files under the given path (defaults to workspace root). Skips binary files and .git directories automatically.

Use this tool when you need to find occurrences of a string, identifier, error message, or pattern across the codebase. Supports literal text search (default), case-insensitive search, and full regular expressions.

Parameters:
- query (required): The search string or regex pattern.
- path: Relative path within the workspace to search. Can be a directory (searched recursively) or a single file. Defaults to "." (entire workspace).
- regex: Set to true to interpret query as a Go-flavour regular expression (e.g. "func\s+\w+" to find function definitions). Default false.
- case_sensitive: Set to true to require exact case matching. Default false (case-insensitive).
- literal_text: Set to true to force literal matching even if the query looks like a regex. Default false.
- max_matches: Maximum number of matching lines to return (1-2000). Default 200.

Returns a JSON object with:
- query: the original query
- matches: array of {path, line_number, line} objects
- truncated: true if max_matches was reached before all files were searched

Tips:
- For simple keyword searches, just provide query. The default case-insensitive literal search handles most needs.
- Use regex=true with patterns like "TODO|FIXME|HACK" to find multiple markers at once.
- Narrow the search with path (e.g. path="internal/server") to get faster, more relevant results.
- If you get too many results, add path scoping or increase specificity of the query rather than raising max_matches.