List files and directories in the workspace. This is the primary tool for viewing directory contents — use it instead of running shell commands like `ls`, `dir`, or `find` via bash.

Use this tool whenever you need to see what files or subdirectories exist at a given path, explore project structure, or enumerate directory contents (including hidden files like .gitignore or .env).

Parameters:
- path: Relative path inside the workspace to list (e.g. "src", "internal/server"). Defaults to "." (workspace root).
- recursive: Set to true to walk the directory tree and list all nested files and subdirectories, not just immediate children. Required when using the depth parameter. Default false.
- depth: Maximum depth of recursion when recursive is true (e.g. depth=2 lists entries up to 2 levels deep). Only effective when recursive=true. Default 0 (unlimited depth).
- include_hidden: Set to true to include hidden files and directories (names starting with "."). By default, hidden entries are excluded.
- max_entries: Maximum number of entries to return (1-2000). Default 200. If the listing is truncated, the response indicates this.

Returns a JSON object with:
- path: the listed directory (relative to workspace root)
- entries: sorted array of relative file/directory paths
- truncated: true if max_entries was reached before all entries were listed

Tips:
- For a quick overview of a directory, just provide path with no other options.
- To explore project structure, use recursive=true with a depth limit (e.g. recursive=true, depth=2).
- To see dotfiles and hidden config, set include_hidden=true.
- If you only need to find files matching a name pattern (e.g. all *.go files), consider using the glob tool instead.