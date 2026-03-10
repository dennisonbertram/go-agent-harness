Manage skill pack discovery and activation.

A skill pack is a named collection of domain-specific instructions and tool constraints that can be loaded on demand to specialize the agent for a particular workflow.

## Actions

### list
List all available skill packs in the registry.

Parameters: none additional

Example: `{"action": "list"}`

### search
Search for skill packs matching a keyword query (searches name, description, category, and tags).

Parameters:
- `query` (string, required): keyword(s) to search for

Example: `{"action": "search", "query": "deployment"}`

### activate
Activate a skill pack by name. Validates prerequisites (required CLI tools and environment variables), then loads the pack's instructions into the current conversation.

Parameters:
- `name` (string, required): the pack name to activate

Example: `{"action": "activate", "name": "railway-deploy"}`

## Notes

- Activating a pack injects its instructions into the conversation as a meta-message.
- If a pack has unmet prerequisites (missing CLI tools or env vars), activation will fail with a clear error listing what is missing.
- Use `search` to discover packs for a specific domain before activating.
