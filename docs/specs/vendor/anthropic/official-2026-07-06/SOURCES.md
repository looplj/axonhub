# Anthropic official docs snapshot

Snapshot date: 2026-07-06

Sources fetched from official Anthropic docs and saved under this directory.

| Source URL | Local file | Notes |
|---|---|---|
| https://docs.anthropic.com/en/api/messages.md | `messages-api-raw.md` | Complete raw markdown for `POST /v1/messages`; authoritative local source for request/response schema. |
| https://docs.anthropic.com/en/api/messages | `messages-api.md`, `messages.md`, `messages_view_raw` | Dynamic/rendered captures retained for comparison; can be incomplete if the page shell is returned. |
| https://docs.anthropic.com/en/api/messages-streaming.md | `messages-streaming-raw.md` | Complete raw markdown for Messages streaming events. |
| https://docs.anthropic.com/en/docs/build-with-claude/tool-use | `tool-use.md` | Rendered tool-use guide capture. |
| https://docs.anthropic.com/en/docs/build-with-claude/tool-use.md | `tool-use-raw.md` | Raw markdown tool-use guide capture. |
| https://docs.anthropic.com/en/docs/build-with-claude/extended-thinking.md | `extended-thinking-raw.md` | Raw markdown for extended/adaptive thinking and preservation rules. |
| https://docs.anthropic.com/en/docs/build-with-claude/prompt-caching.md | `prompt-caching-raw.md` | Raw markdown for prompt caching and `cache_control`. |
| https://docs.anthropic.com/en/docs/build-with-claude/vision.md | `vision-raw.md` | Raw markdown for image input blocks. |
| https://docs.anthropic.com/en/docs/build-with-claude/files.md | `files-raw.md` | Raw markdown for file/document references. |
| https://docs.anthropic.com/en/docs/build-with-claude/citations.md | `citations-raw.md` | Raw markdown for citations in text/document/search result blocks. |
| https://docs.anthropic.com/en/docs/agents-and-tools/mcp-connector.md | `mcp-connector-raw.md` | Raw markdown for MCP connector, `mcp_servers`, and `mcp_toolset`. |
| https://docs.anthropic.com/llms.txt | `llms.txt` | Anthropic docs LLM index. |
| https://docs.anthropic.com/llms-full.txt | `llms-full.txt` | Anthropic docs full LLM export. |
| https://docs.anthropic.com/sitemap.xml | `sitemap.xml` | Sitemap snapshot used for discovery. |

## Capture notes

- `smart-search fetch https://docs.anthropic.com/en/api/messages` returned a dynamic page in earlier attempts. The complete API reference was fetched from the official raw markdown endpoint `https://docs.anthropic.com/en/api/messages.md`.
- Claude Messages protocol is not OpenAI Chat with renamed fields. Keep `system`, content blocks, tool use/tool result, thinking blocks, and MCP connector semantics separate during implementation.
