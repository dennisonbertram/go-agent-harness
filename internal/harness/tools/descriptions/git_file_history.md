Show the commit timeline for a specific file or directory, tracing how it changed over time. Optionally follows file renames. Use this tool to understand the evolution of a file, when it was created, major changes, or who has been working on it.

Parameters:
- path (required): File or directory path relative to workspace root.
- max_commits (optional, integer 1-200, default 20): Maximum number of commits to return.
- follow (optional, boolean, default true): When true, follows the file across renames using git log --follow.
- show_diffs (optional, boolean, default false): When true, includes the diff for each commit. Increases output size significantly.
- since (optional): Limit to commits after this date (YYYY-MM-DD) or ref.

Returns a JSON object with:
- file: the path queried
- follow: whether rename following was enabled
- commits: array of {hash, short_hash, author_name, author_email, date, subject, body, diff (only if show_diffs=true)}
- total_commits: number of commits returned
- truncated: whether results were cut off at max_commits

Note: When show_diffs=true, diff output per commit is truncated at 4096 bytes.
