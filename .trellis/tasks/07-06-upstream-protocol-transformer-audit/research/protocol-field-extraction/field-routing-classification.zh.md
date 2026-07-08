# 字段中文含义与处理路径分类

本文件是实现前的字段分类表：每个顶层 request/response 字段、stream/event 字段都要先决定走哪条路径，不能边改边猜。

## 处理路径枚举

| 路径 | 含义 | 适用场景 |
|---|---|---|
| Common / `llm.Request` | 进入统一公共请求模型 | 多协议都有稳定等价语义的字段 |
| Native struct | 进入对应协议 native request/response struct | 官方协议字段，同协议必须保真 |
| `TransformerMetadata` | 转换恢复提示/桥接元数据 | 字段不适合进 common，但转换后还要恢复 |
| `ProviderExtensions` | 协议私有 sidecar | 不应该序列化进 common JSON 的协议私有数据 |
| Raw fallback | 保存原始 JSON 片段 | known/unmodeled variant，同协议回放 |
| Provider-specific outbound | 具体 provider 出站层处理 | OpenAI-compatible 但并非所有 provider 都支持的字段 |
| Lossy diagnostic + drop | 记录损失后丢弃 | 目标协议无等价语义 |
| Deliberate unsupported | 设计性不支持 | 例如多候选 `n` 这类会改变 Hub 统一语义的字段 |

## OpenAI Chat request 字段分类

| 字段 | 中文含义 | 作者 upstream | 当前分支 | 推荐处理路径 | 什么时候可丢/应诊断 |
|---|---|---:|---:|---|---|
| `metadata` | 元数据键值对，供请求/响应携带业务侧附加信息。 | 有 | 有 | 直接 common llm.Request ↔ Chat native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `top_logprobs` | 返回每个输出 token 的候选 logprob 数量。 | 有 | 有 | 直接 common llm.Request ↔ Chat native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `temperature` | 采样温度，控制随机性。 | 有 | 有 | 直接 common llm.Request ↔ Chat native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `top_p` | 核采样概率阈值。 | 有 | 有 | 直接 common llm.Request ↔ Chat native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `user` | 终端用户标识，旧字段，部分协议用来风控/缓存分桶。 | 有 | 有 | 直接 common llm.Request ↔ Chat native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `safety_identifier` | 安全风控用稳定用户标识，替代 user 的部分用途。 | 有 | 有 | 直接 common llm.Request ↔ Chat native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `prompt_cache_key` | 提示缓存分桶 key，用于提升缓存命中。 | 有 | 有 | 直接 common llm.Request ↔ Chat native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `service_tier` | 服务层级/优先级容量选择。 | 有 | 有 | 直接 common llm.Request ↔ Chat native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `prompt_cache_retention` | 提示缓存保留策略，例如内存或 24h。 | 缺 | 缺 | 补 Chat native request 字段；同协议保真；跨协议需 lossy diagnostic 或显式桥接。 | 同协议不应丢；跨到不支持协议时诊断后丢弃或 provider-specific。 |
| `messages` | Chat/Claude 消息数组。 | 有 | 有 | 直接 common llm.Request ↔ Chat native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `model` | 模型 ID。 | 有 | 有 | 直接 common llm.Request ↔ Chat native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `modalities` | 输出模态列表，例如 text/audio/image。 | 有 | 有 | 直接 common llm.Request ↔ Chat native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `verbosity` | 输出详细程度。 | 有 | 有 | 直接 common llm.Request ↔ Chat native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `reasoning_effort` | 推理努力程度。 | 有 | 有 | 直接 common llm.Request ↔ Chat native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `max_completion_tokens` | Chat 最大 completion token 数，包含推理 token。 | 有 | 有 | 直接 common llm.Request ↔ Chat native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `frequency_penalty` | 频率惩罚，降低重复 token。 | 有 | 有 | 直接 common llm.Request ↔ Chat native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `presence_penalty` | 存在惩罚，鼓励新主题。 | 有 | 有 | 直接 common llm.Request ↔ Chat native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `web_search_options` | Chat 内置 web search 配置。 | 缺 | 缺 | 补 Chat native request 字段；同协议保真；跨协议需 lossy diagnostic 或显式桥接。 | 同协议不应丢；跨到不支持协议时诊断后丢弃或 provider-specific。 |
| `response_format` | Chat 输出格式配置，例如 text/json_schema/json_object。 | 有 | 有 | 直接 common llm.Request ↔ Chat native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `audio` | Chat 顶层音频输出配置。 | 缺 | 缺 | 补 Chat native request 字段；同协议保真；跨协议需 lossy diagnostic 或显式桥接。 | 同协议不应丢；跨到不支持协议时诊断后丢弃或 provider-specific。 |
| `store` | 是否存储输出供平台后续使用。 | 有 | 有 | 直接 common llm.Request ↔ Chat native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `stream` | 是否流式返回。 | 有 | 有 | 直接 common llm.Request ↔ Chat native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `stop` | 停止序列配置。 | 有 | 有 | 直接 common llm.Request ↔ Chat native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `logit_bias` | 对指定 token 的 logit 偏置。 | 有 | 有 | Chat native struct；必要时映射 common 字段。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `logprobs` | 是否返回 logprob。 | 有 | 有 | 直接 common llm.Request ↔ Chat native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `max_tokens` | 旧版最大输出 token 字段。 | 有 | 有 | 直接 common llm.Request ↔ Chat native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `n` | Chat 一次生成几个候选。 | 缺 | 缺 | 设计性不支持/可丢弃，但必须文档化；如要兼容需新增字段和测试。 | 可设计性丢弃：`n` 会改变返回多候选语义，Hub 当前统一模型偏单候选。 |
| `prediction` | Chat 预测内容/静态内容提示，用于加速已知输出场景。 | 缺 | 缺 | 补 Chat native request 字段；同协议保真；跨协议需 lossy diagnostic 或显式桥接。 | 同协议不应丢；跨到不支持协议时诊断后丢弃或 provider-specific。 |
| `seed` | 采样随机种子。 | 有 | 有 | 直接 common llm.Request ↔ Chat native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `stream_options` | 流式返回配置。 | 有 | 有 | 直接 common llm.Request ↔ Chat native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `tools` | 可供模型调用的工具定义。 | 有 | 有 | 直接 common llm.Request ↔ Chat native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `tool_choice` | 工具选择策略。 | 有 | 有 | 直接 common llm.Request ↔ Chat native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `parallel_tool_calls` | 是否允许并行工具调用。 | 有 | 有 | 直接 common llm.Request ↔ Chat native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `function_call` | 旧版函数调用选择字段，已被 tool_choice 替代。 | 缺 | 缺 | 旧版兼容字段：优先转换到 tools/tool_choice；不能表达时 raw same-protocol 或明确丢弃。 | 可在新版路径降级/丢弃，但应优先转换为 tools/tool_choice，并保留旧版兼容测试。 |
| `functions` | 旧版函数定义字段，已被 tools 替代。 | 缺 | 缺 | 旧版兼容字段：优先转换到 tools/tool_choice；不能表达时 raw same-protocol 或明确丢弃。 | 可在新版路径降级/丢弃，但应优先转换为 tools/tool_choice，并保留旧版兼容测试。 |

## OpenAI Chat response 字段分类

| 字段 | 中文含义 | 作者 upstream | 当前分支 | 推荐处理路径 | 什么时候可丢/应诊断 |
|---|---|---:|---:|---|---|
| `id` | 对象唯一 ID。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `choices` | Chat 候选输出列表。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `created` | Chat 创建时间戳。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `model` | 模型 ID。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `service_tier` | 服务层级/优先级容量选择。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `system_fingerprint` | 模型后端系统指纹。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `object` | 对象类型标识。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `usage` | 用量统计。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |

## OpenAI Responses request 字段分类

| 字段 | 中文含义 | 作者 upstream | 当前分支 | 推荐处理路径 | 什么时候可丢/应诊断 |
|---|---|---:|---:|---|---|
| `metadata` | 元数据键值对，供请求/响应携带业务侧附加信息。 | 有 | 有 | common llm.Request ↔ Responses native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `top_logprobs` | 返回每个输出 token 的候选 logprob 数量。 | 有 | 有 | common llm.Request ↔ Responses native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `temperature` | 采样温度，控制随机性。 | 有 | 有 | common llm.Request ↔ Responses native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `top_p` | 核采样概率阈值。 | 有 | 有 | common llm.Request ↔ Responses native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `user` | 终端用户标识，旧字段，部分协议用来风控/缓存分桶。 | 有 | 有 | common llm.Request ↔ Responses native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `safety_identifier` | 安全风控用稳定用户标识，替代 user 的部分用途。 | 有 | 有 | common llm.Request ↔ Responses native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `prompt_cache_key` | 提示缓存分桶 key，用于提升缓存命中。 | 有 | 有 | common llm.Request ↔ Responses native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `service_tier` | 服务层级/优先级容量选择。 | 有 | 有 | common llm.Request ↔ Responses native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `prompt_cache_retention` | 提示缓存保留策略，例如内存或 24h。 | 有 | 有 | Responses native request struct + TransformerMetadata/ProviderExtensions 按需恢复。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `previous_response_id` | Responses 续接上一条 response 的 ID。 | 有 | 有 | Responses native request struct + TransformerMetadata/ProviderExtensions 按需恢复。 | 同协议不应丢；跨 Chat/Claude 无等价时 diagnostic + drop/bridge。 |
| `model` | 模型 ID。 | 有 | 有 | common llm.Request ↔ Responses native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `reasoning` | 推理配置，例如 effort、summary、encrypted_content 等。 | 有 | 有 | common llm.Request ↔ Responses native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `background` | Responses 后台运行开关。 | 有 | 有 | Responses native request struct + TransformerMetadata/ProviderExtensions 按需恢复。 | 同协议不应丢；跨 Chat/Claude 无等价时 diagnostic + drop/bridge。 |
| `max_tool_calls` | Responses 最大工具调用次数。 | 有 | 有 | Responses native request struct + TransformerMetadata/ProviderExtensions 按需恢复。 | 同协议不应丢；跨 Chat/Claude 无等价时 diagnostic + drop/bridge。 |
| `text` | Responses 文本输出配置，例如格式/JSON schema。 | 有 | 有 | Responses native request struct + TransformerMetadata/ProviderExtensions 按需恢复。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `tools` | 可供模型调用的工具定义。 | 有 | 有 | common llm.Request ↔ Responses native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `tool_choice` | 工具选择策略。 | 有 | 有 | common llm.Request ↔ Responses native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `prompt` | Responses 存储提示模板引用/变量。 | 缺 | 有 | 补 Responses native/opaque request 字段；同协议保真；跨协议不静默映射。 | 同协议不应丢；跨 Chat/Claude 无等价时 diagnostic + drop/bridge。 |
| `truncation` | Responses 截断策略。 | 有 | 有 | Responses native request struct + TransformerMetadata/ProviderExtensions 按需恢复。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `input` | Responses 输入，可能是字符串或 typed input item 列表。 | 有 | 有 | Responses native request struct + TransformerMetadata/ProviderExtensions 按需恢复。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `include` | 要求响应额外包含哪些输出数据，例如 logprobs、检索结果、图片 URL、encrypted reasoning。 | 有 | 有 | Responses native request struct + TransformerMetadata/ProviderExtensions 按需恢复。 | 同协议不应丢；跨 Chat/Claude 无等价时 diagnostic + drop/bridge。 |
| `parallel_tool_calls` | 是否允许并行工具调用。 | 有 | 有 | common llm.Request ↔ Responses native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `store` | 是否存储输出供平台后续使用。 | 有 | 有 | common llm.Request ↔ Responses native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `instructions` | Responses 顶层指令。 | 有 | 有 | Responses native request struct + TransformerMetadata/ProviderExtensions 按需恢复。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `stream` | 是否流式返回。 | 有 | 有 | common llm.Request ↔ Responses native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `stream_options` | 流式返回配置。 | 有 | 有 | common llm.Request ↔ Responses native struct。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `conversation` | Responses 服务端 conversation 挂载/续接。 | 缺 | 缺 | 补 Responses native/opaque request 字段；同协议保真；跨协议不静默映射。 | 同协议不应丢；跨 Chat/Claude 无等价时 diagnostic + drop/bridge。 |
| `context_management` | Responses 上下文管理配置，例如 compaction 阈值。 | 缺 | 缺 | 补 Responses native/opaque request 字段；同协议保真；跨协议不静默映射。 | 同协议不应丢；跨 Chat/Claude 无等价时 diagnostic + drop/bridge。 |
| `max_output_tokens` | Responses 最大输出 token 数。 | 有 | 有 | Responses native request struct + TransformerMetadata/ProviderExtensions 按需恢复。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |

## OpenAI Responses response 字段分类

| 字段 | 中文含义 | 作者 upstream | 当前分支 | 推荐处理路径 | 什么时候可丢/应诊断 |
|---|---|---:|---:|---|---|
| `metadata` | 元数据键值对，供请求/响应携带业务侧附加信息。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `top_logprobs` | 返回每个输出 token 的候选 logprob 数量。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `temperature` | 采样温度，控制随机性。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `top_p` | 核采样概率阈值。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `user` | 终端用户标识，旧字段，部分协议用来风控/缓存分桶。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `safety_identifier` | 安全风控用稳定用户标识，替代 user 的部分用途。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `prompt_cache_key` | 提示缓存分桶 key，用于提升缓存命中。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `service_tier` | 服务层级/优先级容量选择。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `prompt_cache_retention` | 提示缓存保留策略，例如内存或 24h。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `previous_response_id` | Responses 续接上一条 response 的 ID。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 同协议不应丢；跨 Chat/Claude 无等价时 diagnostic + drop/bridge。 |
| `model` | 模型 ID。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `reasoning` | 推理配置，例如 effort、summary、encrypted_content 等。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `background` | Responses 后台运行开关。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 同协议不应丢；跨 Chat/Claude 无等价时 diagnostic + drop/bridge。 |
| `max_tool_calls` | Responses 最大工具调用次数。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 同协议不应丢；跨 Chat/Claude 无等价时 diagnostic + drop/bridge。 |
| `text` | Responses 文本输出配置，例如格式/JSON schema。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `tools` | 可供模型调用的工具定义。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `tool_choice` | 工具选择策略。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `prompt` | Responses 存储提示模板引用/变量。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 同协议不应丢；跨 Chat/Claude 无等价时 diagnostic + drop/bridge。 |
| `truncation` | Responses 截断策略。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `id` | 对象唯一 ID。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `object` | 对象类型标识。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `status` | Responses 状态。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `created_at` | 创建时间戳。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `completed_at` | 完成时间戳。 | 缺 | 缺 | 补 native response 字段或在聚合层派生；若只是便捷字段可由 aggregator 生成。 | 响应同协议应保留；若 common response 无等价，native response/metadata 保留。 |
| `error` | 错误对象。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `incomplete_details` | 未完成原因详情。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `output` | Responses 输出 item 列表。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `instructions` | Responses 顶层指令。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `output_text` | Responses 便捷聚合文本输出。 | 缺 | 缺 | 补 native response 字段或在聚合层派生；若只是便捷字段可由 aggregator 生成。 | 响应同协议应保留；若 common response 无等价，native response/metadata 保留。 |
| `usage` | 用量统计。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `parallel_tool_calls` | 是否允许并行工具调用。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `conversation` | Responses 服务端 conversation 挂载/续接。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 同协议不应丢；跨 Chat/Claude 无等价时 diagnostic + drop/bridge。 |
| `max_output_tokens` | Responses 最大输出 token 数。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |

## Anthropic Messages request 字段分类

| 字段 | 中文含义 | 作者 upstream | 当前分支 | 推荐处理路径 | 什么时候可丢/应诊断 |
|---|---|---:|---:|---|---|
| `max_tokens` | 旧版最大输出 token 字段。 | 有 | 有 | Anthropic native MessageRequest；与 common 字段可桥接则桥接。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `messages` | Chat/Claude 消息数组。 | 有 | 有 | Anthropic native MessageRequest；与 common 字段可桥接则桥接。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `model` | 模型 ID。 | 有 | 有 | Anthropic native MessageRequest；与 common 字段可桥接则桥接。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `container` | Claude container 标识/返回信息，用于代码执行等容器能力。 | 缺 | 缺 | 补 Anthropic native/opaque request 字段；同协议保真；跨协议诊断。 | 同协议不应丢；跨协议无等价时 diagnostic + drop。 |
| `inference_geo` | Claude 推理地理区域。 | 缺 | 缺 | 补 Anthropic native/opaque request 字段；同协议保真；跨协议诊断。 | 同协议不应丢；跨协议无等价时 diagnostic + drop。 |
| `metadata` | 元数据键值对，供请求/响应携带业务侧附加信息。 | 有 | 有 | Anthropic native MessageRequest；与 common 字段可桥接则桥接。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `output_config` | Claude 输出配置，例如 structured output schema、effort、task_budget。 | 有 | 有 | Anthropic native MessageRequest；与 common 字段可桥接则桥接。 | 同协议不应丢；跨协议无等价时 diagnostic + drop。 |
| `service_tier` | 服务层级/优先级容量选择。 | 有 | 有 | Anthropic native MessageRequest；与 common 字段可桥接则桥接。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `stop_sequences` | Claude 自定义停止序列。 | 有 | 有 | Anthropic native MessageRequest；与 common 字段可桥接则桥接。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `stream` | 是否流式返回。 | 有 | 有 | Anthropic native MessageRequest；与 common 字段可桥接则桥接。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `system` | Claude 顶层 system prompt。 | 有 | 有 | Anthropic native MessageRequest；与 common 字段可桥接则桥接。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `temperature` | 采样温度，控制随机性。 | 有 | 有 | Anthropic native MessageRequest；与 common 字段可桥接则桥接。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `thinking` | Claude extended thinking 配置或 thinking 内容块。 | 有 | 有 | Anthropic native MessageRequest；与 common 字段可桥接则桥接。 | 同协议不应丢；跨协议无等价时 diagnostic + drop。 |
| `tool_choice` | 工具选择策略。 | 有 | 有 | Anthropic native MessageRequest；与 common 字段可桥接则桥接。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `tools` | 可供模型调用的工具定义。 | 有 | 有 | Anthropic native MessageRequest；与 common 字段可桥接则桥接。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `top_k` | Claude/部分 provider top-k 采样。 | 有 | 有 | Anthropic native MessageRequest；与 common 字段可桥接则桥接。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `top_p` | 核采样概率阈值。 | 有 | 有 | Anthropic native MessageRequest；与 common 字段可桥接则桥接。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |

## Anthropic Message response 字段分类

| 字段 | 中文含义 | 作者 upstream | 当前分支 | 推荐处理路径 | 什么时候可丢/应诊断 |
|---|---|---:|---:|---|---|
| `id` | 对象唯一 ID。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `container` | Claude container 标识/返回信息，用于代码执行等容器能力。 | 缺 | 缺 | 补 native response 字段或在聚合层派生；若只是便捷字段可由 aggregator 生成。 | 同协议不应丢；跨协议无等价时 diagnostic + drop。 |
| `content` | 消息/响应内容块数组。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `model` | 模型 ID。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `role` | 消息角色。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `stop_details` | Claude 结构化停止/拒绝详情。 | 缺 | 缺 | 补 native response 字段或在聚合层派生；若只是便捷字段可由 aggregator 生成。 | 响应同协议应保留；若 common response 无等价，native response/metadata 保留。 |
| `stop_reason` | Claude 停止原因。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `stop_sequence` | 触发停止的具体序列。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `type` | 对象或事件类型。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |
| `usage` | 用量统计。 | 有 | 有 | 协议 native response struct + TransformResponse/TransformStream。 | 公共字段不应丢；若目标协议不支持，必须记录 lossy。 |

## Stream/Event 字段分类

| 协议 | 事件/schema | 中文含义 | 作者 upstream 显式覆盖 | 当前分支显式覆盖 | 推荐处理路径 |
|---|---|---|---:|---:|---|
| `openai_responses` | `response.audio.delta` | Responses 音频输出/转写流式事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.audio.done` | Responses 音频输出/转写流式事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.audio.transcript.delta` | Responses 音频输出/转写流式事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.audio.transcript.done` | Responses 音频输出/转写流式事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.code_interpreter_call_code.delta` | Responses code interpreter 工具调用状态/代码增量事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.code_interpreter_call_code.done` | Responses code interpreter 工具调用状态/代码增量事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.code_interpreter_call.completed` | Responses code interpreter 工具调用状态/代码增量事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.code_interpreter_call.in_progress` | Responses code interpreter 工具调用状态/代码增量事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.code_interpreter_call.interpreting` | Responses code interpreter 工具调用状态/代码增量事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.completed` | Responses 生命周期状态事件。 | 有 | 有 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.content_part.added` | Responses stream 事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.content_part.done` | Responses stream 事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.created` | Responses 生命周期状态事件。 | 有 | 有 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.custom_tool_call_input.delta` | Responses custom tool 输入增量/完成事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.custom_tool_call_input.done` | Responses custom tool 输入增量/完成事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `error` | Responses stream 事件。 | 有 | 有 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.failed` | Responses 生命周期状态事件。 | 有 | 有 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.file_search_call.completed` | Responses file search 工具调用状态事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.file_search_call.in_progress` | Responses file search 工具调用状态事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.file_search_call.searching` | Responses file search 工具调用状态事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.function_call_arguments.delta` | Responses function call 参数增量/完成事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.function_call_arguments.done` | Responses function call 参数增量/完成事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.image_generation_call.completed` | Responses image generation 工具调用状态/部分图像事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.image_generation_call.generating` | Responses image generation 工具调用状态/部分图像事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.image_generation_call.in_progress` | Responses image generation 工具调用状态/部分图像事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.image_generation_call.partial_image` | Responses image generation 工具调用状态/部分图像事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.in_progress` | Responses 生命周期状态事件。 | 有 | 有 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.incomplete` | Responses 生命周期状态事件。 | 有 | 有 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.mcp_call_arguments.delta` | Responses MCP 工具调用/列工具状态事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.mcp_call_arguments.done` | Responses MCP 工具调用/列工具状态事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.mcp_call.completed` | Responses MCP 工具调用/列工具状态事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.mcp_call.failed` | Responses MCP 工具调用/列工具状态事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.mcp_call.in_progress` | Responses MCP 工具调用/列工具状态事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.mcp_list_tools.completed` | Responses MCP 工具调用/列工具状态事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.mcp_list_tools.failed` | Responses MCP 工具调用/列工具状态事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.mcp_list_tools.in_progress` | Responses MCP 工具调用/列工具状态事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.output_item.added` | Responses stream 事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.output_item.done` | Responses stream 事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.output_text.annotation.added` | Responses 文本输出增量/完成事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.queued` | Responses 生命周期状态事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.reasoning_summary_part.added` | Responses reasoning/summary 文本增量事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.reasoning_summary_part.done` | Responses reasoning/summary 文本增量事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.reasoning_summary_text.delta` | Responses reasoning/summary 文本增量事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.reasoning_summary_text.done` | Responses reasoning/summary 文本增量事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.reasoning_text.delta` | Responses reasoning/summary 文本增量事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.reasoning_text.done` | Responses reasoning/summary 文本增量事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.refusal.delta` | Responses stream 事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.refusal.done` | Responses stream 事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.custom_tool_call_input.done` | Responses custom tool 输入增量/完成事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.output_text.delta` | Responses 文本输出增量/完成事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.output_text.done` | Responses 文本输出增量/完成事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.web_search_call.completed` | Responses web search 工具调用状态事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.web_search_call.in_progress` | Responses web search 工具调用状态事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.web_search_call.searching` | Responses web search 工具调用状态事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.create` | Responses stream 事件。 | 有 | 有 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.create` | Responses stream 事件。 | 有 | 有 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_responses` | `response.custom_tool_call_input.done` | Responses custom tool 输入增量/完成事件。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_chat` | `ChatCompletionMessageToolCallChunk` | Chat Completions 流式 chunk/schema。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_chat` | `ChatCompletionStreamOptions` | Chat Completions 流式 chunk/schema。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_chat` | `ChatCompletionStreamResponseDelta` | Chat Completions 流式 chunk/schema。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `openai_chat` | `CreateChatCompletionStreamResponse` | Chat Completions 流式 chunk/schema。 | 缺/泛化 | 缺/泛化 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `anthropic` | `message_start` | Anthropic Messages SSE 事件或 delta 类型。 | 有 | 有 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `anthropic` | `content_block_start` | Anthropic Messages SSE 事件或 delta 类型。 | 有 | 有 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `anthropic` | `content_block_delta` | Anthropic Messages SSE 事件或 delta 类型。 | 有 | 有 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `anthropic` | `content_block_stop` | Anthropic Messages SSE 事件或 delta 类型。 | 有 | 有 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `anthropic` | `message_delta` | Anthropic Messages SSE 事件或 delta 类型。 | 有 | 有 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `anthropic` | `message_stop` | Anthropic Messages SSE 事件或 delta 类型。 | 有 | 有 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `anthropic` | `ping` | Anthropic Messages SSE 事件或 delta 类型。 | 有 | 有 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `anthropic` | `error` | Anthropic Messages SSE 事件或 delta 类型。 | 有 | 有 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `anthropic` | `text_delta` | Anthropic Messages SSE 事件或 delta 类型。 | 有 | 有 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `anthropic` | `input_json_delta` | Anthropic Messages SSE 事件或 delta 类型。 | 有 | 有 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `anthropic` | `thinking_delta` | Anthropic Messages SSE 事件或 delta 类型。 | 有 | 有 | stream 聚合/转换层处理；不能用 request 字段替代。 |
| `anthropic` | `signature_delta` | Anthropic Messages SSE 事件或 delta 类型。 | 有 | 有 | stream 聚合/转换层处理；不能用 request 字段替代。 |

## Anthropic MCP connector companion 字段分类

| 字段 | 中文含义 | 推荐处理路径 | 什么时候可丢/应诊断 |
|---|---|---|---|
| `mcp_servers` | Anthropic MCP connector 远程 MCP 服务器定义。 | Anthropic native/provider extension/raw same-protocol；不要自动映射到 OpenAI Responses `mcp`。 | 同协议不应丢；跨协议无等价时 lossy diagnostic + drop。 |
| `tools[].type=mcp_toolset` | Anthropic MCP toolset 工具变体，引用 mcp_servers 中的服务器。 | Anthropic native/provider extension/raw same-protocol；不要自动映射到 OpenAI Responses `mcp`。 | 同协议不应丢；跨协议无等价时 lossy diagnostic + drop。 |
| `mcp_servers[].name` | MCP server 名称。 | Anthropic native/provider extension/raw same-protocol；不要自动映射到 OpenAI Responses `mcp`。 | 同协议不应丢；跨协议无等价时 lossy diagnostic + drop。 |
| `mcp_servers[].url` | MCP server URL。 | Anthropic native/provider extension/raw same-protocol；不要自动映射到 OpenAI Responses `mcp`。 | 同协议不应丢；跨协议无等价时 lossy diagnostic + drop。 |
| `mcp_servers[].authorization_token` | MCP server 鉴权 token。 | Anthropic native/provider extension/raw same-protocol；不要自动映射到 OpenAI Responses `mcp`。 | 同协议不应丢；跨协议无等价时 lossy diagnostic + drop。 |
| `mcp_servers[].tool_configuration` | MCP server 工具过滤/配置。 | Anthropic native/provider extension/raw same-protocol；不要自动映射到 OpenAI Responses `mcp`。 | 同协议不应丢；跨协议无等价时 lossy diagnostic + drop。 |

## 作者一般丢弃的字段类型

| 类型 | 作者当前表现 | 应不应该丢 |
|---|---|---|
| 目标协议确实不支持的字段 | 例如 Chat builder 过滤非 function tool | 可以丢，但必须确认不是协议漂移；应加 lossy diagnostic |
| 旧版 deprecated 字段 | `function_call` / `functions` 未建模 | 可转换到新版 `tools/tool_choice`；不能转换时同协议 raw 或文档化丢弃 |
| 改变统一语义的字段 | `n` 多候选当前未支持 | 可以设计性不支持，但要明确不支持原因 |
| Provider 私有扩展 | 当前经常靠 metadata/raw 兜底 | 同协议不该丢；跨协议默认不透传 |
| Stream 细粒度事件 | OpenAI Responses 很多事件未显式覆盖 | 不应简单丢；要看 aggregator 是否泛化保真，否则补 stream 层 |
| 无等价服务端状态字段 | `conversation`、`previous_response_id`、Claude `container` 等 | 同协议保留，跨协议只能诊断/桥接，不应静默伪映射 |
