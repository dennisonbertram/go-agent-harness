Report the current context window usage and message composition.

Returns an object with:
- `estimated_context_tokens`: estimated token count for all messages in the current conversation
- `message_count`: total number of messages
- `tool_call_count`: number of assistant messages containing tool calls
- `tool_result_count`: number of tool-role messages (tool results)
- `user_message_count`: number of user-role messages
- `assistant_message_count`: number of assistant-role messages
- `system_message_count`: number of system-role messages
- `has_compact_summary`: whether any message is a compacted summary
- `recommendation`: a brief recommendation based on context pressure (e.g., "context is healthy", "consider compacting")

Use this tool to understand context window pressure before deciding whether to invoke compact_history.