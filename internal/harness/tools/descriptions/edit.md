Edit a workspace file by replacing text. Use this tool to make targeted changes to existing files — replace specific strings or code blocks with new content. Requires the file to already exist; for creating new files, use the write tool instead.

Parameters:
- path: relative file path inside the workspace (required)
- old_text: the exact text to find and replace (required)
- new_text: the replacement text (required)
- replace_all: if true, replace all occurrences; otherwise replace only the first match (default: false)
- expected_version: optional version hash for optimistic concurrency — if provided and the file has changed since you last read it, the edit is rejected with a stale_write error

Supports multi-line old_text and new_text for replacing code blocks, function bodies, or multi-line strings. The old_text must match exactly (including whitespace and newlines).