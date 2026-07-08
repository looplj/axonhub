# 字段触发条件分类：哪些字段正常不会触发

本文件回答：官方协议里有很多字段，但不是每次请求都会出现。实现前必须区分“必须保留”和“正常不会触发”。

## 核心原则

字段是否出现取决于三个来源：

1. **客户端请求是否发送**：request 字段不会由模型凭空生成。
2. **上游服务端是否启用对应能力**：server-side tool/MCP/code interpreter/web search 等只有配置后才会产生事件。
3. **模型实际是否走到该能力**：即使配置了工具，模型没调用也不会产生 tool call / stream event。

## MCP 特别区分

| 类型 | 谁执行/发现工具 | 典型字段/事件 | 是否普通 Codex 本地 MCP 会触发 |
|---|---|---|---|
| Codex 本地 MCP / 懒加载 | Codex 客户端/本地 MCP runtime | `tool_search`, `defer_loading`, `tool_search_call`, `tool_search_output`, `additional_tools`, `namespace` | 会，这是 Codex 客户端工具发现/懒加载路径 |
| OpenAI Responses server-side MCP connector | OpenAI 上游服务端连接远程 MCP | `tools[].type=mcp`, `response.mcp_call.*`, `response.mcp_list_tools.*` | 通常不会，除非请求明确把 OpenAI server-side MCP tool 发给上游 |
| Anthropic MCP connector | Anthropic 上游服务端连接远程 MCP | `mcp_servers`, `tools[].type=mcp_toolset` | 通常不会，除非请求明确使用 Anthropic MCP connector 参数 |

结论：用户说“使用服务器端 MCP”时，要先问清是 OpenAI Responses server-side MCP、Anthropic MCP connector，还是 Codex 本地 MCP。三者字段不同，不能互相自动映射。

## 一般不会触发 / 需要显式 opt-in 的 request 字段

| 协议 | 字段 | 触发条件 | 普通聊天是否出现 | Hub 处理建议 |
|---|---|---|---:|---|
| OpenAI Responses | `conversation` | 客户端使用 OpenAI server-side Conversations，把 response 挂到 conversation | 否 | 同协议 native 保留；跨协议 diagnostic/drop |
| OpenAI Responses | `previous_response_id` | 客户端使用 Responses 状态续接上一条 response | 否 | 同协议/common 保留；跨协议 diagnostic/drop |
| OpenAI Responses | `context_management` | 客户端显式要求服务端 compaction/context management | 否 | 同协议 native/opaque 保留；跨协议 diagnostic/drop |
| OpenAI Responses | `background` | 客户端请求后台异步 response | 否 | 同协议保留；跨协议 diagnostic/drop |
| OpenAI Responses | `prompt` | 客户端使用 OpenAI prompt template / stored prompt | 否 | 同协议 native 保留；跨协议 diagnostic/drop |
| OpenAI Responses | `include` | 客户端要求额外返回 logprobs、检索结果、图片 URL、encrypted reasoning 等 | 否 | 同协议保留；跨协议 diagnostic/drop |
| OpenAI Responses | `max_tool_calls` | 客户端限制工具调用次数 | 否 | 同协议保留；跨协议 diagnostic/drop |
| OpenAI Responses | `tools[].type=mcp` | 客户端启用 OpenAI server-side MCP connector | 否 | 同协议 native/raw 保留；不要自动转 Anthropic MCP |
| OpenAI Responses | `tools[].type=code_interpreter` | 客户端启用 OpenAI server-side code interpreter | 否 | 同协议 native/raw 保留；跨协议 diagnostic/drop |
| OpenAI Responses | `tools[].type=file_search` | 客户端启用 OpenAI server-side file search/vector store | 否 | 同协议 native/raw 保留；跨协议 diagnostic/drop |
| OpenAI Chat | `web_search_options` | 客户端使用 Chat 内置 web search | 否 | Chat native/provider-specific；跨协议 diagnostic/drop |
| OpenAI Chat | `prediction` | 客户端提供已知/预测输出内容用于加速 | 否 | Chat native；跨协议 diagnostic/drop |
| OpenAI Chat | top-level `audio` | 客户端要求 Chat 音频输出 | 否 | Chat native；跨协议需要目标支持 |
| OpenAI Chat | `n` | 客户端要求多个候选 | 否 | 当前可设计性不支持；若支持需改 response/stream/usage |
| Anthropic | `container` | 客户端复用/指定 Claude container，常见于 code execution | 否 | Anthropic native 保留；跨协议 diagnostic/drop |
| Anthropic | `inference_geo` | 客户端指定推理地域 | 否 | Anthropic native 保留；跨协议 diagnostic/drop |
| Anthropic | `mcp_servers` | 客户端启用 Anthropic server-side MCP connector | 否 | Anthropic native/provider extension；不要自动转 OpenAI MCP |
| Anthropic | `tools[].type=mcp_toolset` | 客户端引用 Anthropic `mcp_servers` 中的远程 MCP server | 否 | Anthropic native/raw 保留；跨协议 diagnostic/drop |
| Anthropic | `output_config` | 客户端使用 Claude structured output / effort / task budget | 否 | Anthropic native/metadata；跨协议只映射可等价部分 |
| Anthropic | `thinking` | 客户端启用 Claude extended thinking | 取决于模型/配置 | Anthropic native；跨协议与 OpenAI reasoning 只能有限桥接 |

## 只有模型实际调用工具后才会触发的 stream/response 字段

| 协议 | 字段/事件族 | 触发条件 | 没触发是否正常 |
|---|---|---|---:|
| OpenAI Responses | `response.mcp_list_tools.*` | 请求含 server-side MCP，且服务端正在列远程 MCP 工具 | 是 |
| OpenAI Responses | `response.mcp_call.*` | 请求含 server-side MCP，且模型调用了远程 MCP 工具 | 是 |
| OpenAI Responses | `response.web_search_call.*` | 请求含 web_search 工具，且模型实际搜索 | 是 |
| OpenAI Responses | `response.file_search_call.*` | 请求含 file_search 工具，且模型实际检索 | 是 |
| OpenAI Responses | `response.code_interpreter_call.*` | 请求含 code_interpreter，且模型实际执行代码 | 是 |
| OpenAI Responses | `response.image_generation_call.*` | 请求含 image_generation，且模型实际生成图像 | 是 |
| OpenAI Responses | `response.function_call_arguments.*` | 模型调用 function tool | 是 |
| OpenAI Responses | `response.custom_tool_call_input.*` | 模型调用 custom tool | 是 |
| OpenAI Responses | `response.reasoning_text.*` | 模型/请求允许暴露 reasoning text | 是 |
| OpenAI Responses | `response.reasoning_summary_*` | 请求/模型返回 reasoning summary | 是 |
| OpenAI Responses | `response.audio.*` | 请求音频输出 | 是 |
| Anthropic | `content_block_delta` + `input_json_delta` | Claude 正在流式生成 tool input JSON | 是 |
| Anthropic | `thinking_delta` / `signature_delta` | 请求启用 extended thinking 且流式返回 thinking/signature | 是 |
| Anthropic | `tool_use` content block | Claude 实际调用工具 | 是 |
| Anthropic | `server_tool_use` / web search result blocks | Anthropic server tools 被配置且被调用 | 是 |

## 不会由模型主动“触发”的 request 字段

这些字段只会由客户端发出，模型不会在输出中自动生成 request 字段：

- `conversation`
- `context_management`
- `background`
- `prompt`
- `include`
- `web_search_options`
- `prediction`
- `mcp_servers`
- `container`
- `inference_geo`

如果入站 body 没有这些字段，Hub 不应该凭空生成。

## 实现含义

1. **正常不触发 ≠ 可以不支持**：同协议透传时，客户端一旦发了就必须保留。
2. **server-side MCP ≠ Codex 本地 MCP**：OpenAI Responses MCP、Anthropic MCP connector、Codex local MCP lazy loading 三套字段不同。
3. **stream 事件没出现不一定是 bug**：只有对应能力配置且模型实际调用后才会出现。
4. **跨协议默认不自动映射工具生态字段**：例如 Anthropic `mcp_servers` 不自动转 OpenAI Responses `mcp`。
5. **实现优先级应按实际触发概率分层**：Codex 常用字段优先；server-side optional tool connector 字段可先 native/raw 保真，再逐步做语义桥接。
