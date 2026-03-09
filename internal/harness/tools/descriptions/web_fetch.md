Fetch the content of a single web page by URL.

This is a simple wrapper that retrieves the text content of a web page. Use this tool when you need the raw content of a specific URL without any analysis step.

For fetching with analysis, use agentic_fetch instead. For general HTTP requests with timeout/size control, use fetch.

Parameters:
- url (required): The HTTP or HTTPS URL to fetch.

Returns a JSON object with:
- url: the requested URL
- content: the page content as text