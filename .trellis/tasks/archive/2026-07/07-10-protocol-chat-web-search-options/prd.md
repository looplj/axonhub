# PRD — Chat `web_search_options`

## Goal

实现 OpenAI Chat Completions top-level `web_search_options` 的 same-protocol preservation，并避免把它误当成 OpenAI Responses hosted `web_search` tool 或 Anthropic server tool。

## Required Behavior

1. Chat -> Chat 保留 top-level `web_search_options`。
2. object 的 known 与 unknown nested values一起 preserve。
3. 不将它转成 `tools[]`、Responses `web_search`、Anthropic native web tool。
4. 跨协议方向必须 explicit unsupported/lossy。

## Acceptance Criteria

- red fixture 证明现有路径丢字段。
- Chat same-protocol request body 保留 value。
- `tools[]` 不因该字段发生变化。
- Responses/Anthropic direction 无 fake map。
- targeted tests 和 diff check 通过。

