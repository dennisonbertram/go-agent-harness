Search commit history by keyword in commit messages and/or diff content (pickaxe search). Use this tool to answer questions like "when was X added?", "which commits changed Y?", or "who removed Z?".

Parameters:
- query (required): Literal string to search for. The LLM should extract key terms from natural-language questions before calling this tool.
- mode (optional): Search scope. "message" searches commit messages only (git log --grep). "pickaxe" searches diff content only (git log -S). "both" (default) runs both and deduplicates results.
- path (optional): Limit search to a specific file or directory path (relative to workspace root).
- max_results (optional, integer 1-100, default 20): Maximum number of commits to return.
- since (optional): Limit to commits after this date (YYYY-MM-DD) or ref (e.g. "HEAD~50", "v1.0").

Returns a JSON object with:
- commits: array of {hash, short_hash, author_name, author_email, date, subject, body, match_type}
- total_found: total number of matching commits before truncation
- truncated: whether results were cut off at max_results
- query: the search term used
- mode: the search mode used

Note: This tool uses literal string matching. For semantic search, extract key terms before calling.
