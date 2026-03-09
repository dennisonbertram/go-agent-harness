Query the built-in model catalog to list, filter, and inspect available LLM models. This tool reads from an in-memory catalog of all configured providers and their models — you do NOT need to call any external API, read any files, or use bash to discover models. Just call this tool directly.

Use this tool whenever you need to:
- See what LLM models or providers are available
- Find models with specific capabilities (tool calling, vision, streaming, reasoning)
- Compare models by cost tier, speed tier, or context window size
- Get detailed metadata about a specific model

Actions (set via the "action" parameter):
- "list" (default): Return all models, optionally filtered. Combine any filters below (AND logic).
- "providers": Return a summary of each configured provider (name, model count, base URL).
- "info": Return full details for one model. Requires both "provider" and "model_id".

Filter parameters (used with action="list"):
- provider: Filter by provider key (e.g. "openai", "anthropic").
- tool_calling: Set true/false to filter by tool-calling support.
- streaming: Set true/false to filter by streaming support.
- reasoning: Set true/false to filter by reasoning mode.
- modality: Filter by modality tag (e.g. "text", "vision").
- speed_tier: Filter by speed tier (e.g. "fast", "medium", "slow").
- cost_tier: Filter by cost tier (e.g. "low", "medium", "high").
- best_for: Filter by best-for tag (e.g. "coding", "chat").
- strength: Filter by strength tag (e.g. "reasoning", "instruction-following").
- min_context: Minimum context window size in tokens (integer).

Returns a JSON object with the action performed and the matching results.

Examples:
- List all models: {"action": "list"}
- List providers: {"action": "providers"}
- Models with vision: {"action": "list", "modality": "vision"}
- Cheap models with tool calling: {"action": "list", "tool_calling": true, "cost_tier": "low"}
- Info on a specific model: {"action": "info", "provider": "openai", "model_id": "gpt-4.1-mini"}