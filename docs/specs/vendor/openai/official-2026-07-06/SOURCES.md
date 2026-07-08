# OpenAI official docs snapshot

Snapshot date: 2026-07-06

Sources fetched from official OpenAI domains and saved under this directory. Exa discovery evidence is stored under `.agent/summary/smart-search-evidence/protocol-exa/`.


## LLM export indexes

| Source URL | Local file | Notes |
|---|---|---|
| https://developers.openai.com/api/llms.txt | `api-llms.txt` | Broad OpenAI API docs/reference curated index. Exa identified this as the better discovery entrypoint for combined docs. |
| https://developers.openai.com/api/llms-full.txt | `api-llms-full-smart-search.md`, `api-llms-full-smart-search-unescaped.md` | Combined docs/reference export fetched through `smart-search fetch` after direct `curl` returned `Forbidden`. Useful for guides and feature docs; not a replacement for the full rendered create-method parameter pages. |
| https://developers.openai.com/api/reference/llms.txt | `llms.txt` | API reference-only curated index. |
| https://developers.openai.com/api/reference/llms-full.txt | `llms-full.txt` | API reference-only combined export. It lists endpoint pages but does not include the full create-method parameter body for Responses/Chat. |

## API reference snapshots

| Source URL | Local file | Notes |
|---|---|---|
| https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create | `_official_chat_completions_create.md`, `_official_chat_completions_create.html` | Complete `POST /chat/completions` create method page. This is the authoritative local source for Chat body parameters and response schema. |
| https://developers.openai.com/api/reference/resources/chat | `_official_chat_resources.md`, `_official_chat_resources.html` | Chat resource/schema index page; useful but not sufficient by itself for the complete create body table. |
| https://platform.openai.com/docs/api-reference/chat/create | `chat-completions-create-reference.md`, `_try_chat-create.md`, `_try_chat-methods-create.md`, `_try_chat-overview.md` | Earlier platform/API reference captures retained for comparison. |
| https://platform.openai.com/docs/api-reference/responses | `responses-api-reference.md`, `_try_responses-create.md`, `_try_responses-create-response.md` | Responses API reference captures. `_try_responses-create.md` contains the create body and response/event schema sections used by the baseline. |
| https://developers.openai.com/api/reference/resources/responses/ | Exa result: `.agent/summary/smart-search-evidence/protocol-exa/openai-responses-exa.json` | Exa retrieved a complete text extract for the Responses reference page, including `post /responses` and Body Parameters. |

## Guides and feature docs

| Source URL | Local file | Notes |
|---|---|---|
| https://platform.openai.com/docs/guides/tools-tool-search | `tools-tool-search.md` | Tool Search / lazy tool loading guide. |
| https://platform.openai.com/docs/guides/tools-connectors-mcp | `tools-connectors-mcp.md` | MCP connector/tool guide. |
| https://platform.openai.com/docs/guides/streaming-responses | `streaming-responses.md` | Streaming guide, including Responses event semantics and Chat streaming contrast. |
| https://platform.openai.com/docs/guides/function-calling | `function-calling.md` | Function/custom tool guide and tool calling concepts. |
| https://platform.openai.com/docs/guides/migrate-to-responses | `migrate-to-responses.md` | Migration guide comparing Chat Completions and Responses. |

## Capture notes

- Some direct fetches against documentation pages can return a short dynamic shell instead of the complete markdown. Do not treat such short fetches as complete protocol sources.
- Direct `.md` method URLs such as `https://developers.openai.com/api/reference/resources/responses/methods/create.md` currently return only a short Stainless summary, not the full schema table. Do not use those stubs as the create-method baseline.
- Exa discovery found `https://developers.openai.com/api/llms-full.txt` as the broad combined docs/reference export. Direct `curl` returned `Forbidden`; `smart-search fetch` succeeded via Firecrawl and the content was saved locally.
- The full Chat create method source is the `developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create` page, not only the shorter Chat resource page.
- `client_metadata` is not listed in these public OpenAI protocol snapshots; keep it in Codex compatibility/provider-extension documentation unless a public OpenAI source confirms it.
