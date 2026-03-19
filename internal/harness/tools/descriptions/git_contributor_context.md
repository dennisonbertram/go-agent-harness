Show the top contributors for a file or directory, ranked by commit count. Use this tool to answer "who owns this code?", "who should review this?", or "who has been most active in this area?".

Parameters:
- path (optional): File or directory path relative to workspace root. When omitted, shows contributors for the entire repository.
- max_authors (optional, integer 1-20, default 10): Maximum number of authors to return.
- since (optional): Limit to commits after this date (YYYY-MM-DD) or ref.

Returns a JSON object with:
- path: the path queried (or "" for the whole repo)
- authors: array of {name, email, commit_count} sorted by commit_count descending

Note: Authors are grouped by email address, not display name, to handle name variations.
