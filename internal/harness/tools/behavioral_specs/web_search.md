## When to Use
- Looking up current events, recent library versions, or live documentation
- Finding API documentation that may have changed since training cutoff
- Researching a topic where freshness matters
- Verifying that a library or package exists and its current version

## When NOT to Use
- Generating code — use your training knowledge and the codebase context
- Finding files in the local workspace — use glob or grep
- Answering questions about the current codebase — use read/grep/glob
- Tasks where local context is sufficient

## Behavioral Rules
1. Prefer local context (grep, read, glob) over web search for questions about the current codebase
2. Use web search when information is time-sensitive (release notes, CVEs, current API changes)
3. Verify fetched content with agentic_fetch for full documentation pages when snippets are insufficient

## Common Mistakes
- **SearchInsteadOfRead**: Using web search to find out what's in a local file
- **StaleQuery**: Searching for information that is in the codebase itself (e.g., searching for function signatures that are in the local source)

## Examples
### WRONG
Web search for the signature of a function defined in the current repository.

### RIGHT
Use grep to search the local codebase for the function name, then read the file.
