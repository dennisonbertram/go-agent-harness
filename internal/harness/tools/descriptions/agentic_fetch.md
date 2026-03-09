Fetch and analyze web content with optional delegated reasoning.

This tool combines a URL fetch with a sub-agent analysis step. Provide a prompt describing what you want to learn or extract; optionally provide a URL whose content will be fetched and included as context for the analysis.

Parameters:
- prompt (required): The question or instruction for the sub-agent to reason about.
- url (optional): An HTTP/HTTPS URL to fetch. Its content is passed to the sub-agent along with the prompt.

Returns a JSON object with:
- prompt: the original prompt
- url: the fetched URL (if provided)
- content: the raw fetched content (if a URL was provided)
- analysis: the sub-agent's analysis result