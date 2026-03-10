List recent conversations stored in the harness conversation history.

Returns lightweight metadata for each conversation: ID, title, creation time, last-updated time, and message count. No message content is returned — use search_conversations to find messages by keyword.

**Parameters**
- `limit` (integer, optional) — maximum number of conversations to return. Defaults to 20, capped at 100.
- `offset` (integer, optional) — number of conversations to skip for pagination. Defaults to 0.

**Returns**
JSON object with a `conversations` array. Each entry contains:
- `id` — unique conversation ID
- `title` — auto-generated or user-provided title (may be empty)
- `created_at` — RFC3339 timestamp of first message
- `updated_at` — RFC3339 timestamp of last message
- `message_count` — total number of messages in the conversation

**Notes**
- Results are ordered by most-recently-updated first.
- This tool is read-only and parallel-safe.
