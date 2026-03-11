Compact the conversation history to reduce context window pressure.

Parameters:
- mode (required): One of "strip", "summarize", or "hybrid"
  - strip: Remove tool_call and tool_result messages outside the keep_last window. Zero LLM cost. Fastest option.
  - summarize: Replace compaction zone with a single LLM-generated summary. Higher quality but costs one LLM call.
  - hybrid: Strip tool_call metadata; summarize tool_results exceeding 500 estimated tokens; keep small tool results and all user/assistant text.
- keep_last (optional, default 4): Number of recent turns to preserve intact. Minimum 2.

Returns an object with:
- before_tokens: estimated token count before compaction
- after_tokens: estimated token count after compaction
- turns_compacted: number of turns that were compacted
- summary: the generated summary text (only for summarize and hybrid modes)

Use context_status first to assess whether compaction is needed. Prefer "strip" for quick cleanup and "hybrid" when you want to preserve important context.
