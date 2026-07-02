# OpenRouter 三协议能力基准清单

> 唯一数据源：`docs/specs/openrouter-openapi.yaml`（OpenRouter 官方 OpenAPI spec）。
> 用途：后续所有协议转换（入站 client→canonical、出站 canonical→provider）改动的**唯一规范参照底稿**。
> 生成方式：Ruby YAML 解析 + 人工核对 schema 行号，非脚本批量推断。
> 范围：chat_completions / anthropic_messages / openai_responses 三协议的请求、内容、工具、流式、响应。
> 本文件只记录**规范说了什么**，不记录 AxonHub 实现现状（实现审计见 `master-conversion-table.md` 及三份 `audit-*.md`）。
>
> **版本说明**：本地 yaml 为 6/29 下载版。线上最新版（7/1 拉取 `https://openrouter.ai/openapi.yaml`）有一个结构差异——ChatFunctionTool anyOf 不再包含 `FusionServerTool_OpenRouter`（本地仍含）。其余 schema 名与字段完全一致。本清单已据此修正：Chat 工具按线上最新版 10 种计（不含 fusion），Responses/Messages 仍含 fusion。

---

## 1. 三协议顶层请求字段交叉表

共 60 个字段。✅ = 该协议定义了此字段；— = 未定义。

| 字段 | Chat | Messages | Responses | 备注 |
|---|---|---|---|---|
| `background` | — | — | ✅ | bool nullable |
| `cache_control` | ✅ | ✅ | ✅ | $ref AnthropicCacheControlDirective（type:ephemeral + ttl） |
| `context_management` | — | ✅ | — | 含 edits 数组：clear_tool_uses / clear_thinking / compact 三类 |
| `debug` | ✅ | — | ✅ | $ref ChatDebugOptions（echo_upstream_body 等，streaming only） |
| `fallbacks` | — | ✅ | — | 回退模型列表（≤3，与 models 互斥） |
| `frequency_penalty` | ✅ | — | ✅ | double, -2..2 |
| `image_config` | ✅ | — | ✅ | $ref ImageConfig（provider 专属图像参数，aspect_ratio/quality 等） |
| `include` | — | — | ✅ | []ResponseIncludesEnum（file_search_call.results 等 5 种） |
| `input` | — | — | ✅ | $ref Inputs（string \| array<items>） |
| `instructions` | — | — | ✅ | string nullable（Responses 的 system prompt 入口） |
| `logit_bias` | ✅ | — | — | map<string,double>（spec 允许浮点） |
| `logprobs` | ✅ | — | — | bool |
| `max_completion_tokens` | ✅ | — | — | integer |
| `max_output_tokens` | — | — | ✅ | integer nullable（Responses 的限长入口） |
| `max_tokens` | ✅ | ✅ | — | integer（Chat 已弃用但仍定义；Messages 必填） |
| `max_tool_calls` | — | — | ✅ | integer nullable（agent loop 限制） |
| `messages` | ✅ | ✅ | — | Chat=ChatMessages[]，Messages=MessagesMessageParam[] |
| `metadata` | ✅ | ✅ | ✅ | Chat=map<string,string>；Messages={user_id}；Responses=$ref RequestMetadata |
| `min_p` | ✅ | — | — | double（采样阈值，非所有 provider 支持） |
| `modalities` | ✅ | — | ✅ | Chat=[text\|image\|audio]，Responses=[text\|image] |
| `model` | ✅ | ✅ | ✅ | string，三协议必填 |
| `models` | ✅ | ✅ | ✅ | 候选模型列表（多模型路由） |
| `output_config` | — | ✅ | — | $ref MessagesOutputConfig（effort/format/task_budget） |
| `parallel_tool_calls` | ✅ | — | ✅ | bool nullable（Messages 用 tool_choice.disable_parallel_tool_use 替代） |
| `plugins` | ✅ | ✅ | ✅ | 9 种插件（auto-router/moderation/web/web-fetch/file-parser/response-healing/context-compression/pareto-router/fusion） |
| `presence_penalty` | ✅ | — | ✅ | double, -2..2 |
| `previous_response_id` | — | — | ✅ | string nullable（stateful 续接） |
| `prompt` | — | — | ✅ | $ref StoredPromptTemplate（提示模板引用） |
| `prompt_cache_key` | — | — | ✅ | string nullable |
| `provider` | ✅ | ✅ | ✅ | $ref ProviderPreferences（allow_fallbacks/data_collection/ignore/max_price/only/order/quantizations/sort/zdr 等 13 子字段） |
| `reasoning` | ✅ | — | ✅ | Chat={effort,summary}；Responses=$ref ReasoningConfig{effort,max_tokens,summary,enabled?} |
| `reasoning_effort` | ✅ | — | — | enum: max/xhigh/high/medium/low/minimal/none/null |
| `repetition_penalty` | ✅ | — | — | double（非所有 provider 支持） |
| `response_format` | ✅ | — | — | oneOf: text/json_object/json_schema/grammar/python |
| `route` | ✅ | ✅ | ✅ | $ref DeprecatedRoute |
| `safety_identifier` | — | — | ✅ | string nullable |
| `seed` | ✅ | — | — | integer |
| `service_tier` | ✅ | ✅ | ✅ | Chat/Responses enum: auto/default/flex/priority/scale/null；Messages plain string |
| `session_id` | ✅ | ✅ | ✅ | string ≤256（body 优先于 header；粘性路由 + 缓存命中 + 可观测分组） |
| `speed` | — | ✅ | — | enum: fast/standard/null |
| `stop` | ✅ | — | — | string \| array<string>≤4 |
| `stop_sequences` | — | ✅ | — | array<string> |
| `stop_server_tools_when` | ✅ | ✅ | ✅ | array<StopServerToolsWhenCondition>（step_count_is/max_cost 等，OR 逻辑） |
| `store` | — | — | ✅ | **const:false**（规范在此端点只允许假值） |
| `stream` | ✅ | ✅ | ✅ | bool |
| `stream_options` | ✅ | — | — | $ref ChatStreamOptions（仅 include_usage） |
| `system` | — | ✅ | — | string \| array<AnthropicTextBlockParam> |
| `temperature` | ✅ | ✅ | ✅ | double |
| `text` | — | — | ✅ | $ref TextExtendedConfig（format + verbosity） |
| `thinking` | — | ✅ | — | oneOf: enabled{budget_tokens,display} / disabled / adaptive{display} |
| `tool_choice` | ✅ | ✅ | ✅ | 三协议各自不同，见 §4 |
| `tools` | ✅ | ✅ | ✅ | 三协议各自不同，见 §3 |
| `top_a` | ✅ | — | — | double |
| `top_k` | ✅ | ✅ | ✅ | integer |
| `top_logprobs` | ✅ | — | ✅ | integer (0-20) |
| `top_p` | ✅ | ✅ | ✅ | double |
| `trace` | ✅ | ✅ | ✅ | $ref TraceConfig |
| `truncation` | — | — | ✅ | enum: auto/disabled/null |
| `user` | ✅ | ✅ | ✅ | string ≤256 |

**必填项**：Chat=`messages`；Messages=`model`+`messages`+`max_tokens`；Responses=无显式 required（model 实际必填）。

---

## 2. 协议独有/语义不等价字段速查

下列字段虽跨协议同名，但语义或载体不同，转换时不可直接等价映射：

| 概念 | Chat | Messages | Responses | 差异点 |
|---|---|---|---|---|
| 限长 | `max_tokens`(弃用)+`max_completion_tokens` | `max_tokens`(必填) | `max_output_tokens` | 三种命名，语义相同 |
| 停止序列 | `stop`(string\|array≤4) | `stop_sequences`(array) | —(无顶级) | Responses 协议层面无 stop |
| 系统提示 | 并入 messages[0] role=system | 顶层 `system`(string\|array) | 顶层 `instructions`(string) | 三种入口 |
| 消息主体 | `messages` | `messages` | `input` | Responses 用 input 承载 |
| 推理控制 | `reasoning`{effort,summary} + 平铺 `reasoning_effort` | `thinking`{type,budget_tokens,display} + `output_config`{effort,format,task_budget} | `reasoning`{effort,max_tokens,summary,enabled?} | 三套不同结构 |
| 并行工具 | 顶层 `parallel_tool_calls`(bool) | `tool_choice.disable_parallel_tool_use`(bool) | 顶层 `parallel_tool_calls`(bool) | Messages 独有子开关 |
| 缓存提示 | 顶层 `cache_control` | 顶层 `cache_control` + 块级 cache_control | 顶层 `cache_control` | Messages 最完整 |
| 会话分组 | `session_id`(body) | `session_id`(body) | `session_id`(body) | 三协议均 body 优先于 header |
| 用户身份 | `user`(string) | `user`(string) + `metadata.user_id` | `user`(string) | Messages 双通道 |
| 输出格式 | `response_format`(5种) | —(无原生) | `text.format`+`text.verbosity` | Chat 最丰富 |
| 速度 | — | `speed`(fast/standard) | — | Messages 独有 |

---

## 3. 工具定义（tools）

### 3.1 function tool 字段

| 协议 | schema | 字段 |
|---|---|---|
| Chat | ChatFunctionTool inline(type=function) | cache_control, function{name,description,parameters,strict}, type |
| Messages | inline(type=custom) | cache_control, description, input_schema, name, type |
| Responses | FunctionTool | description, name, parameters, strict, type |

注意：Chat 的 function tool 嵌套在 `function` 对象里；Messages 用 `input_schema`（非 `parameters`）；Responses 用扁平 `parameters`。

### 3.2 三协议 server tool type 完整列表

**Chat（ChatFunctionTool anyOf，共 10 种；线上最新版）**：
- `function`（内联函数工具）
- `openrouter:advisor`（AdvisorServerTool_OpenRouter）
- `openrouter:bash`（BashServerTool）
- `openrouter:datetime`（DatetimeServerTool）
- `openrouter:image_generation`（ImageGenerationServerTool_OpenRouter）
- `openrouter:experimental__search_models`（ChatSearchModelsServerTool）
- `openrouter:subagent`（SubagentServerTool_OpenRouter）
- `openrouter:web_fetch`（WebFetchServerTool）
- `openrouter:web_search`（OpenRouterWebSearchServerTool）
- `web_search` / `web_search_preview` / `web_search_preview_2025_03_11` / `web_search_2025_08_26`（ChatWebSearchShorthand，字符串简写）

> 注：本地 6/29 版本的 ChatFunctionTool 额外含 `openrouter:fusion`（FusionServerTool_OpenRouter），但线上 7/1 最新版已移除。Responses/Messages 仍含 fusion。

**Messages（tools anyOf，共 13 种）**：
- `custom`（内联函数工具，用 input_schema）
- `bash_20250124`（name=bash）
- `text_editor_20250124`（name=str_replace_editor）
- `web_search_20250305`（name=web_search；含 allowed_domains/blocked_domains/max_uses/user_location）
- `web_search_20260209`（name=web_search；额外含 allowed_callers）
- `advisor_20260301`（name=advisor；含 allowed_callers/caching/defer_loading/max_uses/model）
- `openrouter:bash` / `openrouter:datetime` / `openrouter:image_generation` / `openrouter:experimental__search_models` / `openrouter:web_fetch` / `openrouter:web_search`（OpenRouter server tools）
- unknown type 兜底（properties.type:string，required:type）

**Responses（tools anyOf，共 26 种）**：
- `function`（FunctionTool）
- `web_search_preview`（Preview_WebSearchServerTool）
- `web_search_preview_2025_03_11`（Preview_20250311_WebSearchServerTool）
- `web_search`（Legacy_WebSearchServerTool）
- `web_search_2025_08_26`（WebSearchServerTool）
- `file_search`（FileSearchServerTool）
- `computer_use_preview`（ComputerUseServerTool）
- `code_interpreter`（CodeInterpreterServerTool）
- `mcp`（McpServerTool）
- `image_generation`（ImageGenerationServerTool）
- `local_shell`（CodexLocalShellTool）
- `shell`（ShellServerTool）
- `apply_patch`（ApplyPatchServerTool）
- `custom`（CustomTool）
- `openrouter:advisor` / `openrouter:subagent` / `openrouter:datetime` / `openrouter:fusion` / `openrouter:image_generation` / `openrouter:experimental__search_models` / `openrouter:web_fetch` / `openrouter:web_search` / `openrouter:apply_patch` / `openrouter:bash` / `openrouter:shell`（OpenRouter server tools）

### 3.3 ImageGenerationServerTool 完整字段（Responses 原生）

type=image_generation，必填 type。字段：

| 字段 | 类型 | 枚举/说明 |
|---|---|---|
| `type` | string | `image_generation`（必填） |
| `model` | string | enum: gpt-image-1 / gpt-image-1-mini（x-speakeasy-unknown-values: allow） |
| `input_image_mask` | object | {file_id:string, image_url:string} |
| `background` | string | enum: transparent / opaque / auto |
| `input_fidelity` | string? | enum: high / low / null |
| `moderation` | string | enum: auto / low |
| `output_compression` | integer | |
| `output_format` | string | enum: png / webp / jpeg |
| `partial_images` | integer | |
| `quality` | string | enum: low / medium / high / auto |
| `size` | string | enum: 1024x1024 / 1024x1536 / 1536x1024 / auto |

注意：OpenRouter 版 `openrouter:image_generation`（ImageGenerationServerTool_OpenRouter）结构不同，用 `parameters` 嵌套（$ref ImageGenerationServerToolConfig）。

### 3.4 McpServerTool 完整字段（Responses）

type=mcp。字段：allowed_tools, authorization, connector_id, headers, require_approval, server_description, server_label, server_url, type。

### 3.5 CustomTool 字段（Responses）

type=custom。字段：description, format, name, type。用于 apply_patch 等 freeform-grammar 工具。

---

## 4. tool_choice 三协议清单

### Chat（ChatToolChoice anyOf）
- `none` / `auto` / `required`（string）
- ChatNamedToolChoice：`{type:"function", function:{name:string}}`
- ChatServerToolChoice：`{type:"openrouter:web_search" | "web_search" | "web_search_preview" | ...}`（OpenRouter 扩展，直接命名 server tool）

### Messages（tool_choice oneOf）
- `{type:"auto", disable_parallel_tool_use?:bool}`
- `{type:"any", disable_parallel_tool_use?:bool}`
- `{type:"none"}`
- `{type:"tool", name:string, disable_parallel_tool_use?:bool}`

### Responses（OpenAIResponsesToolChoice anyOf）
- `auto` / `none` / `required`（string）
- `{type:"function", name:string}`
- `{type:"web_search_preview" | "web_search_preview_2025_03_11"}`（type-only 服务端选择）
- ToolChoiceAllowed：`{type:"allowed_tools", mode:"auto"|"required", tools:[{name,type}]}`（多选数组形式）
- `{type:"apply_patch"}`
- `{type:"shell"}`

---

## 5. 消息/内容载体

### 5.1 Chat messages（ChatMessages，role 判别）

| role | schema | 独有字段 |
|---|---|---|
| system | ChatSystemMessage | name |
| user | ChatUserMessage | name |
| developer | ChatDeveloperMessage | name |
| assistant | ChatAssistantMessage | audio, images, reasoning, reasoning_details, refusal, tool_calls, name |
| tool | ChatToolMessage | tool_call_id |

Chat content parts（ChatContentItems，type 判别）：`text` / `image_url` / `input_audio` / `input_video`(legacy) / `video_url` / `file`。
- ChatContentText 额外有 cache_control。
- ChatAssistantMessage.reasoning_details 是 array<ReasoningDetailUnion>（summary/encrypted/text 三种）。

### 5.2 Messages content blocks（MessagesMessageParam.content）

content = string \| array<block>，block 类型：

| type | schema/inline | 关键字段 |
|---|---|---|
| `text` | AnthropicTextBlockParam | text, cache_control, citations |
| `image` | AnthropicImageBlockParam | source, cache_control |
| `document` | AnthropicDocumentBlockParam | source, title, context, cache_control, citations |
| `tool_use` | inline | id, name, input, cache_control |
| `tool_result` | inline | tool_use_id, content(string\|array), is_error, cache_control |
| `thinking` | inline | thinking, signature |
| `redacted_thinking` | inline | data |
| `server_tool_use` | inline | id, name, input, cache_control |
| `web_search_tool_result` | inline | tool_use_id, content(array\|error), cache_control |
| `search_result` | AnthropicSearchResultBlockParam | source, title, content, cache_control, citations |
| `compaction` | inline | content(nullable string), cache_control |
| advisor_tool_result | MessagesAdvisorToolResultBlock | (ref) |

tool_result.content 内可含 `tool_reference`（{type, tool_name}）。

### 5.3 Responses input items（Inputs = string \| array<item>）

item 类型（anyOf，30+ 种）：

**输入侧**：
- `message`（EasyInputMessage：content, role, type, phase）
- `message`（InputMessageItem：content, id, role, type）
- `reasoning`（ReasoningItem）
- `function_call`（FunctionCallItem）
- `function_call_output`（FunctionCallOutputItem）
- `apply_patch_call`（ApplyPatchCallItem）
- `apply_patch_call_output`（ApplyPatchCallOutputItem）
- `custom_tool_call`（CustomToolCallItem）
- `custom_tool_call_output`（CustomToolCallOutputItem）
- `mcp_list_tools` / `mcp_approval_request` / `mcp_approval_response` / `mcp_call`（MCP 系列）
- `local_shell_call` / `local_shell_call_output`
- `shell_call` / `shell_call_output`
- `item_reference`（ItemReferenceItem）
- `compaction`（CompactionItem）

**输出回放侧（Output 系列，可出现在 input 中做历史续接）**：
- OutputMessage（message，含 output_text/refusal content）
- OutputReasoning（reasoning，含 reasoning_text + summary）
- OutputFunctionCall（function_call）
- OutputCustomToolCall（custom_tool_call）
- OutputWebSearchCall / OutputFileSearchCall / OutputImageGenerationCall / OutputCodeInterpreterCall / OutputComputerCall
- OutputDatetimeItem / OutputWebSearchServerTool / OutputCodeInterpreterServerTool / OutputFileSearchServerTool / OutputImageGenerationServerTool / OutputBrowserUseServerTool / OutputBashServerTool / OutputTextEditorServerTool / OutputApplyPatchServerTool / OutputWebFetchServerTool / OutputToolSearchServerTool / OutputMemoryServerTool / OutputMcpServerTool / OutputSearchModelsServerTool / OutputFusionServerTool / OutputAdvisorServerTool / OutputSubagentServerTool

InputText 字段：text, type=input_text。
InputImage 字段：detail, image_url, type=input_image。
InputFile 字段：file_data, file_id, file_url, filename, type=input_file。

---

## 6. 流式事件

### 6.1 Responses StreamEvents（discriminator: type，40+ 种）

| 类别 | 事件 type |
|---|---|
| 生命周期 | response.created, response.in_progress, response.completed, response.incomplete, response.failed |
| 输出项 | response.output_item.added, response.output_item.done |
| 内容块 | response.content_part.added, response.content_part.done |
| 文本 | response.output_text.delta, response.output_text.done, response.output_text.annotation.added |
| 拒绝 | response.refusal.delta, response.refusal.done |
| 函数调用 | response.function_call_arguments.delta, response.function_call_arguments.done |
| 自定义工具 | response.custom_tool_call_input.delta, response.custom_tool_call_input.done |
| 推理 | response.reasoning_text.delta, response.reasoning_text.done, response.reasoning_summary_part.added, response.reasoning_summary_part.done, response.reasoning_summary_text.delta, response.reasoning_summary_text.done |
| 图像生成 | response.image_generation_call.in_progress, .generating, .partial_image, .completed |
| Web 搜索 | response.web_search_call.in_progress, .searching, .completed |
| Apply Patch | response.apply_patch_call_operation_diff.delta, .done |
| Fusion | response.fusion_call.in_progress, .completed, .analysis.in_progress, .analysis.completed, .panel.added, .panel.delta, .panel.completed, .panel.failed, .panel.reasoning.delta |
| 其他 | error, response.debug |

### 6.2 Messages StreamEvents（discriminator: type，8 种）

- message_start
- message_delta
- message_stop
- content_block_start
- content_block_delta
- content_block_stop
- ping
- error

---

## 7. 响应输出项与 namespace/caller

### 7.1 Responses 输出项中的 namespace

以下输出项均含可选 `namespace` 字段（描述："Namespace qualifier for tools registered as part of a namespace tool group (e.g. an MCP server)"）：

- `OpenAIResponseFunctionToolCall`（function_call，yaml:15119）
- `OpenAIResponseCustomToolCall`（custom_tool_call，yaml:15055）
- `OutputItemFunctionCall`（function_call，yaml:16521）
- `OutputItemCustomToolCall`（custom_tool_call，yaml:16103）
- 流式 OutputItemAddedEvent / OutputItemDoneEvent 中的 function_call / custom_tool_call item

namespace 是 **MCP 工具组归属限定符**，出现在输出侧（response），不在请求侧 tools 声明中。

### 7.2 Messages tool_use 中的 caller

`AnthropicToolUseBlock` 含必填 `caller` 字段（yaml:1107/1804），是 **发起方鉴别器**（direct / code_execution_*），与 namespace 语义不同。

### 7.3 Responses output_item.added/done 判别映射

OutputItemAddedEvent 和 OutputItemDoneEvent 的 item 用 type 判别，映射到 8 种：
message / reasoning / function_call / custom_tool_call / web_search_call / file_search_call / image_generation_call / apply_patch_call。

---

## 8. 推理控制三协议对照

| 维度 | Chat | Messages | Responses |
|---|---|---|---|
| effort 枚举 | max/xhigh/high/medium/low/minimal/none/null | output_config.effort: low/medium/high/xhigh/max/null | ReasoningConfig.effort（同 Chat 枚举） |
| budget | —(无原生) | thinking.budget_tokens | reasoning.max_tokens |
| summary | reasoning.summary: auto/concise/detailed/null | —(无等价) | reasoning.summary / generate_summary（同枚举） |
| display | — | thinking.display: summarized/omitted/null | — |
| enabled 开关 | — | thinking.type: enabled/disabled/adaptive | reasoning.enabled?（待确认） |
| task_budget | — | output_config.task_budget | — |
| format | — | output_config.format(json_schema) | — |
| 平铺快捷键 | reasoning_effort（=reasoning.effort 简写） | — | — |

ChatReasoningDetails（assistant message 内）：array<ReasoningDetailUnion>，三种：
- ReasoningDetailSummary：{format, id, index, summary, type}
- ReasoningDetailEncrypted：{data, format, id, index, type}
- ReasoningDetailText：{format, id, index, signature, text, type}

---

## 9. 关键约束与陷阱（转换时必查）

1. **store = const:false**：Responses 规范在此端点只允许 `store:false`。发 `store:true` 属超规范。
2. **session_id body 优先于 header**：三协议均定义 body 值优先，max 256 字符。
3. **ChatFunctionTool 不是纯 function**：anyOf 含 9 种 OpenRouter server tools（线上最新版），不能假设 Chat tools 只有 function。
4. **Messages tools 有 unknown type 兜底**：未识别 type 不会报错，而是走兜底（properties.type:string）。
5. **disable_parallel_tool_use 仅 Messages 有**：是 Messages 控制并行的唯一手段；Chat/Responses 用顶层 parallel_tool_calls。
6. **namespace ≠ caller**：namespace 是 MCP 组归属（Responses 输出侧）；caller 是发起方鉴别（Messages tool_use block）。不可混用。
7. **namespace 仅在输出侧**：spec 无 `{type:"namespace"}` 请求声明，namespace 只出现在 function_call / custom_tool_call 输出项。
8. **ImageGenerationServerTool.model / input_image_mask**：是标准字段（gpt-image-1 / gpt-image-1-mini + mask 对象），不是扩展。
9. **logit_bias spec 允许 double**：map<string,double>，浮点值合法。
10. **response_format 仅 Chat 有**：5 种（text/json_object/json_schema/grammar/python）；Responses 用 text.format + text.verbosity；Messages 无原生等价。
11. **stop 在 Responses 不存在**：协议层面无顶级 stop/stop_sequences。
12. **ToolChoiceAllowed 是多选数组**：Responses 独有，`{type:"allowed_tools", mode, tools:[{name,type}]}`，canonical 单选无法承载。
13. **ChatServerToolChoice**：Chat 独有，`{type:"openrouter:web_search"}` 直接命名 server tool，非 function wrapper。
14. **stop_server_tools_when 三协议都有**：array<condition>，OR 逻辑，覆盖 max_tool_calls。
15. **metadata 结构不同**：Chat=flat map；Messages={user_id}；Responses=$ref RequestMetadata。
16. **modalities 枚举不同**：Chat 含 audio，Responses 只有 text/image。
17. **reasoning 对象 vs 平铺键**：Chat 可用 reasoning 对象或平铺 reasoning_effort（不能同时不同值）；Responses 只有 reasoning 对象。
18. **advisor_20260301 需要 model**：Messages 的 advisor 工具 required 含 model 字段。
19. **web_search 有 4 个版本**：web_search_20250305 / web_search_20260209（Messages）；web_search / web_search_preview / web_search_preview_2025_03_11 / web_search_2025_08_26（Responses/Chat shorthand）。
20. **compaction block**：Messages content 有 `{type:"compaction", content}` 块；Responses input 有 `CompactionItem`。

---

## 附录 A：OpenRouter server tools 跨协议对照

| server tool | Chat type | Messages type | Responses type |
|---|---|---|---|
| web search | `openrouter:web_search` + shorthand | `openrouter:web_search` + `web_search_20250305`/`web_search_20260209` | `openrouter:web_search` + `web_search`/`web_search_preview`/`web_search_preview_2025_03_11`/`web_search_2025_08_26` |
| image generation | `openrouter:image_generation` | `openrouter:image_generation` | `image_generation`(原生) + `openrouter:image_generation` |
| bash | `openrouter:bash` | `openrouter:bash` + `bash_20250124` | `openrouter:bash` |
| datetime | `openrouter:datetime` | `openrouter:datetime` | `openrouter:datetime` |
| web fetch | `openrouter:web_fetch` | `openrouter:web_fetch` | `openrouter:web_fetch` |
| search models | `openrouter:experimental__search_models` | `openrouter:experimental__search_models` | `openrouter:experimental__search_models` |
| advisor | `openrouter:advisor` | `openrouter:advisor` + `advisor_20260301` | `openrouter:advisor` |
| fusion | —(线上已移除) | —(无) | `openrouter:fusion` |
| subagent | `openrouter:subagent` | —(无) | `openrouter:subagent` |
| file search | —(无) | —(无) | `file_search` |
| computer use | —(无) | —(无) | `computer_use_preview` |
| code interpreter | —(无) | —(无) | `code_interpreter` |
| mcp | —(无) | —(无) | `mcp` |
| shell | —(无) | —(无) | `shell` + `local_shell` |
| apply patch | —(无) | —(无) | `apply_patch` + `custom` |
| text editor | —(无) | `text_editor_20250124` | —(无) |

## 附录 B：streaming 事件对照

| 维度 | Responses | Messages |
|---|---|---|
| 生命周期 | response.created/in_progress/completed/incomplete/failed | message_start/message_delta/message_stop |
| 内容块 | response.content_part.added/done | content_block_start/delta/stop |
| 文本 | response.output_text.delta/done/annotation.added | (经 content_block_delta) |
| 函数调用 | response.function_call_arguments.delta/done | (经 content_block_delta) |
| 自定义工具 | response.custom_tool_call_input.delta/done | —(无) |
| 推理 | response.reasoning_text.delta/done + summary 系列 | (经 content_block_delta, thinking block) |
| 服务端工具 | image_generation/web_search/apply_patch/fusion 各有专属事件 | —(无) |
| 其他 | error/debug/ping | error/ping |
