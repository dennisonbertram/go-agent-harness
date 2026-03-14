Reset your conversation transcript and start a new context segment within the same run.

Use this tool when your conversation history has grown very long, contains information that is no longer relevant, or when you want to focus on a new phase of work without the overhead of prior context.

**What happens when you call this tool:**
- Your conversation history is cleared and a fresh transcript begins.
- The `persist` payload you provide is saved to observational memory and re-injected as the opening message of the new segment, so you can carry forward facts, state, or progress notes.
- Your step counter, cost, run ID, and observational memory all continue uninterrupted.
- The runner emits a `context.reset` SSE event so callers can track segment boundaries.

**What to include in `persist`:**
Include any information you will need to continue work effectively: current task, key decisions made, files modified, next steps, etc. This is a free-form JSON object — structure it however is most useful.

**Parameters:**
- `persist` (required): A JSON object containing the information you want to carry forward into the new context segment.
