# Known Issues

## AllowedTools Lost on ContinueRun (GitHub #524)

**Severity:** HIGH / Security
**Found:** 2026-03-31
**Status:** Open

`ContinueRun` in `runner.go` does not copy `allowedTools` from the source run state. This means per-run tool restrictions are silently dropped after the first message in a multi-turn conversation. The continuation run defaults to `nil` (unrestricted), giving the agent access to all tools.

**Additionally:** Conversation history contains tool call results from prior runs, so the LLM "remembers" having tools even if they're filtered on the current run. This is a secondary exploit vector — even if `allowedTools` were properly propagated, old tool calls in history reinforce the model's belief that it has access.

**Workaround:** None currently. The `allowed_tools` API field is effectively broken for multi-turn conversations.

**Fix:** Snapshot `allowedTools` in `ContinueRun`'s `contState` initialization, and consider sanitizing conversation history when tool restrictions change between runs.
