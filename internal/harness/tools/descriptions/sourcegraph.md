Search code across repositories using a Sourcegraph instance. Use this tool when you need to search code beyond the current workspace — for example, searching across an organization's repositories, finding usages of a function in other projects, or running Sourcegraph-specific query syntax (repo:, lang:, file:, etc.).

This tool connects to an external Sourcegraph instance. For searching code within the current workspace, use the grep tool instead.

Parameters:
- query (required): A Sourcegraph search query string. Supports Sourcegraph query syntax including filters like repo:, lang:, file:, type:, and boolean operators. Examples: "repo:myorg/myrepo lang:go func main", "ioutil.ReadAll lang:go", "repo:myorg/ type:diff author:alice".
- count: Maximum number of results to return (1-200, default 20). Use smaller values for broad queries to avoid excessive output.
- context_window: Number of characters of surrounding context to include with each match (0-2000, default 0). Increase this to see code around each match.
- timeout_seconds: Maximum time to wait for results (1-60 seconds, default 15). Increase for complex queries across many repositories.

Returns a JSON object with:
- status_code: HTTP status code from the Sourcegraph instance
- response: The raw response body from Sourcegraph containing search results

Tips:
- Use repo: filters to narrow searches to specific repositories or organizations.
- Use lang: filters to restrict results to a specific programming language.
- Combine with count to control result volume — start small and increase if needed.
- If the Sourcegraph instance is not configured, this tool will return an error. Fall back to grep for local workspace searches.