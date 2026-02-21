Fetch raw content from a URL without any AI processing.

Inputs:
- query (required): The URL to fetch (http/https only).

Notes:
- If the response is HTML, it is converted to Markdown.
- The response is limited to 5MB. If larger, it is truncated.
