# Tool-Driven Tests - Responses API

This directory contains OpenAI Responses API integration tests that exercise explicit tool usage rather than plain Q&A.

## Tests Included

1. **TestResponsesWebSearchReturnsCitations** - Uses the built-in `web_search` tool and validates:
   - `web_search_call` items are present
   - included search sources are present
   - `output_text.annotations` contains `url_citation`
   - citation URL/title/index fields are populated

## Running the Tests

```bash
# Run all tests in this directory
go test -v

# Run the web search test
go test -v -run TestResponsesWebSearchReturnsCitations
```
