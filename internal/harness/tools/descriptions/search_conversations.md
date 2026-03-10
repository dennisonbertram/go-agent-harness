Search conversation history using full-text search and return matching snippets.

Runs a full-text search across all stored conversation messages and returns short excerpts (snippets) showing where the query matched. This allows the agent to find relevant past discussions without loading entire message transcripts.

**Parameters**
- `query` (string, required) — the search terms to look for. Supports FTS5 query syntax (phrase matching with quotes, NOT/OR/AND operators).
- `limit` (integer, optional) — maximum number of results to return. Defaults to 10, capped at 50.

**Returns**
JSON object with a `results` array. Each entry contains:
- `conversation_id` — ID of the conversation containing the match
- `role` — role of the message author (`user`, `assistant`, or `tool`)
- `snippet` — short excerpt of the matching message content, showing context around the match

**Notes**
- Returns an empty results array (not an error) when no messages match the query.
- Results are ordered by FTS relevance score.
- This tool is read-only and parallel-safe.
- To get full conversation metadata for a result, pass the `conversation_id` to `list_conversations` with `limit=1`.
