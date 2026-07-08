# Canonical protocol sources selected by search

- Snapshot date: 2026-07-06
- Scope: choose the most official and most complete local source set for protocol conversion/audit work across:
  - OpenAI Responses
  - OpenAI Chat Completions
  - Anthropic Claude Messages
- Search evidence directory: `.agent/summary/smart-search-evidence/protocol-canonical-search/`

## Selection result

| Protocol | Most official machine-readable source | Most complete human-readable source | Local canonical files |
|---|---|---|---|
| OpenAI Responses | `https://platform.openai.com/docs/static/api-definition.yaml` | `https://developers.openai.com/api/reference/resources/responses/` plus existing platform create snapshot | `openai-api-definition.fetch.yaml`, `openai-responses-reference.exa.md`, `openai-responses-create.platform-snapshot.md` |
| OpenAI Chat Completions | `https://platform.openai.com/docs/static/api-definition.yaml` | `https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create` | `openai-api-definition.fetch.yaml`, `openai-chat-completions-create.developers-snapshot.md`, `openai-chat-completions-create.developers-snapshot.html` |
| Anthropic Claude Messages | No separate OpenAPI file found through this search; official raw Markdown is the best canonical source | `https://docs.anthropic.com/en/api/messages.md` plus streaming/MCP raw docs | `anthropic-messages-api.official-raw.md`, `anthropic-messages-streaming.official-raw.md`, `anthropic-mcp-connector.official-raw.md` |

## Why these were selected

### OpenAI Responses

Search evidence:

- `.agent/summary/smart-search-evidence/protocol-canonical-search/openai-responses-exa.json`
- `.agent/summary/smart-search-evidence/protocol-canonical-search/openai-api-definition-fetch.json`

Decision:

1. The OpenAI static API definition is the most official machine-readable protocol source. The local copy contains both `/responses` and `operationId: createResponse` markers, plus Responses-specific fields such as `context_management`.
2. The Developers reference page is the best human-readable source. Exa retrieved a large current reference extract containing `post /responses`, `### Body Parameters`, `context_management`, and `tool_choice`.
3. The existing platform create snapshot is retained because it is larger than the Exa text extract and includes detailed create/response schema sections useful for line-level audit.

### OpenAI Chat Completions

Search evidence:

- `.agent/summary/smart-search-evidence/protocol-canonical-search/openai-chat-completions-exa.json`
- `.agent/summary/smart-search-evidence/protocol-canonical-search/openai-api-definition-fetch.json`

Decision:

1. The same OpenAI static API definition is the most official machine-readable source. The local copy contains `/chat/completions`, `operationId: createChatCompletion`, and Chat-specific fields such as `web_search_options`.
2. The full Developers create page is the best human-readable source for Body Parameters and response schema. The saved markdown copy contains `POST/chat/completions`, `# Create chat completion`, `### Body Parameters`, and `web_search_options`.
3. Exa search also surfaced `https://platform.openai.com/docs/static/api-definition.yaml`; this is more protocol-spec-like than individual rendered pages.

### Anthropic Claude Messages

Search evidence:

- `.agent/summary/smart-search-evidence/protocol-canonical-search/anthropic-messages-exa.json`
- `.agent/summary/smart-search-evidence/protocol-canonical-search/anthropic-openapi-exa.json`
- `.agent/summary/smart-search-evidence/protocol-canonical-search/anthropic-messages-raw-exa.json`
- `.agent/summary/smart-search-evidence/protocol-canonical-search/anthropic-messages-md-fetch.json`
- `.agent/summary/smart-search-evidence/protocol-canonical-search/anthropic-messages-streaming-md-fetch.json`

Decision:

1. This search did not identify a better official OpenAPI/YAML schema for Anthropic Messages.
2. The official raw Markdown endpoint `https://docs.anthropic.com/en/api/messages.md` is the most complete canonical source found for `POST /v1/messages` request/response schema.
3. Streaming and MCP connector behavior are protocol-relevant but live on separate official pages, so `messages-streaming.md` and `mcp-connector.md` are included as canonical companion files.

## Failed or insufficient candidates

| Candidate | Result | Reason not selected as canonical by itself |
|---|---|---|
| Direct `curl https://platform.openai.com/docs/static/api-definition.yaml` | Returned Cloudflare/HTML in this environment | Replaced by `smart-search fetch` content saved as `openai-api-definition.fetch.yaml`. |
| Direct `curl https://developers.openai.com/api/reference/resources/responses/` | Returned `Forbidden` in this environment | Exa current text extract and existing official platform snapshot were used instead. |
| Direct `curl https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create` | Returned `Forbidden` in this environment | Existing full Developers markdown/html snapshots were copied into this canonical directory. |
| Direct `curl https://docs.anthropic.com/en/api/messages.md` | Returned a Claude region/unavailable HTML page in this environment | Replaced by `smart-search fetch`/existing raw Markdown snapshot. |
| `https://developers.openai.com/api/reference/llms-full.txt` | Official but reference-summary oriented | Useful index/reference export, but not enough for full create Body Parameters. |
| `https://developers.openai.com/api/reference/resources/responses/methods/create.md` | Short Stainless summary only | Not full create schema; do not use as baseline. |

## Verification performed

Local text-marker checks after writing canonical files:

- `openai-api-definition.fetch.yaml`
  - contains `/chat/completions`
  - contains `operationId: createChatCompletion`
  - contains `/responses`
  - contains `operationId: createResponse`
  - contains `web_search_options`
  - contains `context_management`
- `openai-chat-completions-create.developers-snapshot.md`
  - contains `POST/chat/completions`
  - contains `# Create chat completion`
  - contains `### Body Parameters`
  - contains `web_search_options`
- `openai-responses-reference.exa.md`
  - contains `post /responses`
  - contains `### Body Parameters`
  - contains `context_management`
- `anthropic-messages-api.official-raw.md`
  - contains `**post** /v1/messages`
  - contains `### Body Parameters`
- `anthropic-mcp-connector.official-raw.md`
  - contains `mcp_servers`

Note: `PyYAML` is not installed in this environment, so the OpenAI YAML was not parsed structurally in this pass. The verification above is text-marker based.
