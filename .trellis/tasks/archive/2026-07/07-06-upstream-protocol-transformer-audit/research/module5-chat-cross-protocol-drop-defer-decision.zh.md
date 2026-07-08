# Module 5 决策：OpenAI Chat raw 字段的跨协议 drop/defer

## 决策状态

Accepted for Module 5 Chat request emission policy.

## 背景

Module 5 的目标是：

- 保留 OpenAI Chat same-protocol 请求字段；
- 不把 OpenAI Chat 专属字段意外发给所有 OpenAI-compatible provider；
- 不把协议专属字段加入 common `llm.Request`；
- 不做未验证的跨协议假映射。

`implement.md` 要求 unsupported fields 需要 explicit diagnostic 或 documented omission。当前 Module 5 不实现 LossyDowngrade diagnostics，因为 Phase 5 已单独定义为跨协议 loss 可观察化模块。因此本文件作为 Module 5 的 durable documented omission / defer 记录。

## 当前 same-protocol raw replay 字段

OpenAI Chat same-protocol raw replay 白名单由 `llm/transformer/openai/chat_extensions.go` 的 `openAIChatRawReplayFieldNames` 维护：

```text
prompt_cache_retention
temperature
top_p
top_logprobs
stream_options
n
web_search_options
prediction
audio

tools
tool_choice
functions
function_call
```

这些字段满足至少一类条件：

1. 官方 Chat-native 字段不属于 stable common `llm.Request` 语义；
2. 字段是 union/object，typed model 不能完整表达所有官方变体；
3. 字段需要 explicit null 保真；
4. 字段是 deprecated compatibility field，只应 OpenAI Chat same-protocol replay；
5. 字段包含 Chat custom tool 形态，不能 fake-map 到 common function tool。

## 跨协议行为

当目标不是 OpenAI Chat `PlatformOpenAI` 时，本模块不回放这些 raw fields。

当前直接覆盖的非 OpenAI Chat OpenAI-compatible target 是 `PlatformGoogle`，测试明确断言这些 raw fields 不进入 Google body：

- `web_search_options`
- `prediction`
- `audio`
- `functions`
- `function_call`
- `n`
- `prompt_cache_retention`
- `temperature:null`
- `top_p:null`
- `top_logprobs:null`
- `stream_options.include_obfuscation`
- `tools` custom shape
- `tool_choice` custom shape

保留的 common 字段仍按 common model 正常输出。例如 `stream_options.include_usage` 是 common-owned 字段，Google 路径仍可输出 `{"include_usage":true}`；OpenAI Chat raw replay 额外保留 `include_obfuscation`。

## 为什么不在 Module 5 做跨协议转换

### deprecated `functions` / `function_call`

`functions` / `function_call` 理论上可以映射到 modern `tools` / `tool_choice` 的部分 function 形态，但当前 Module 5 不做该映射，原因：

- 本模块目标是 same-protocol fidelity 和 provider blast-radius 控制；
- legacy function fields 与 modern tool fields 的行为差异需要单独兼容矩阵；
- 如果目标 provider 不是 OpenAI Chat，静默转换可能改变请求语义；
- 后续 LossyDowngrade diagnostics 应记录 source field、target protocol、drop/defer reason、severity。

当前决策：

```text
OpenAI Chat -> OpenAI Chat: raw same-protocol replay
OpenAI Chat -> non-OpenAI Chat compatible provider: documented omission, diagnostics deferred to Phase 5 LossyDowngrade
```

### Chat custom `tools` / custom `tool_choice`

OpenAI Chat custom tools 不是 common `llm.Tool` 的 function tool。当前不做 fake mapping：

- inbound custom tools 不进入 `req.Tools`；
- inbound custom tool_choice 不进入 `req.ToolChoice`；
- same-protocol raw replay 保留完整 JSON；
- non-OpenAI target 不回放 raw custom tool shape。

当前决策：

```text
OpenAI Chat custom tools -> OpenAI Chat: raw same-protocol replay
OpenAI Chat custom tools -> common llm.Tool: not represented
OpenAI Chat custom tools -> non-OpenAI Chat compatible provider: documented omission, diagnostics deferred to Phase 5 LossyDowngrade
```

### explicit null fields

`temperature:null`、`top_p:null`、`top_logprobs:null`、`prompt_cache_retention:null` 在 OpenAI Chat same-protocol 中需要保真。但 null 与缺失在 common pointer 字段中无法区分。

当前决策：

```text
OpenAI Chat explicit null -> OpenAI Chat: raw same-protocol replay
OpenAI Chat explicit null -> non-OpenAI Chat compatible provider: documented omission unless field is common-owned and represented by structured common value
```

### `stream_options.include_obfuscation`

`stream_options.include_obfuscation` 是 OpenAI Chat stream_options 的官方 nested field，不在 common `llm.StreamOptions` 中。

当前决策：

```text
OpenAI Chat stream_options.include_obfuscation -> OpenAI Chat: raw same-protocol replay
OpenAI Chat stream_options.include_usage -> common StreamOptions.IncludeUsage, can be emitted to compatible providers
OpenAI Chat stream_options.include_obfuscation -> non-OpenAI Chat compatible provider: documented omission, diagnostics deferred to Phase 5 LossyDowngrade
```

## 后续 Phase 5 要求

Phase 5 LossyDowngrade diagnostics 应读取 `ProviderExtensions.OpenAIChat.Request.RawTopLevelFields`，为非 OpenAI Chat target 生成诊断，至少包含：

- source protocol: `openai_chat`
- source field: raw field name
- target protocol/platform
- reason: unsupported / no verified semantic mapping / custom union shape / explicit null not representable
- severity

## 不是当前模块目标

Module 5 不做以下事情：

- 不建立全协议 universal AST；
- 不扩大 common `llm.Request`；
- 不把 OpenAI Chat custom tools fake-map 成 common function tools；
- 不把 deprecated `functions` / `function_call` 静默改写成 modern `tools` / `tool_choice`；
- 不为所有 OpenAI-compatible provider 透传 OpenAI Chat raw fields；
- 不实现 LossyDowngrade diagnostics。

## 2026-07-07 复审补充：field-aware raw replay policy

架构复审要求 raw replay 不能把不同 field ownership 压成一个无条件覆盖 bucket。Module 5 采用 field-aware merge：

1. **raw-only Chat fields** 可以 OpenAI Chat same-protocol 直接回放：
   - `prompt_cache_retention`
   - `n`
   - `web_search_options`
   - `prediction`
   - `audio`
   - `functions`
   - `function_call`

2. **common-owned explicit-null fields** 只在 raw literal 为 `null` 时回放：
   - `temperature`
   - `top_p`
   - `top_logprobs`

   如果 raw 中是非 null 值，而 common structured 字段已经存在，common structured 字段优先，避免 raw fallback 覆盖中间层对 common 字段的合法调整。

3. **nested mixed-ownership field `stream_options`** 做对象级合并：
   - `include_usage` 是 common-owned，输出以 `llm.StreamOptions.IncludeUsage` 为准；
   - `include_obfuscation` 是 OpenAI Chat raw-only nested field，可从 raw replay 合并；
   - raw `include_usage` 不覆盖 common `include_usage`。

4. **tool union fields `tools` / `tool_choice`** 只在 structured common output 不存在时 raw replay：
   - function tools / named function tool_choice 属于 common-owned structured path；
   - Chat custom tool shape 属于 raw-only same-protocol path；
   - 如果 common structured output 已存在，不用 raw 整字段覆盖，避免把 function tool common 语义替换为 stale raw payload。

## 2026-07-07 复审补充：Phase 5 diagnostics 读取规则

Phase 5 LossyDowngrade diagnostics 应在 provider-specific emission filter 之前读取 `ProviderExtensions.OpenAIChat.Request.RawTopLevelFields` 判定 dropped fields，不能只从最终 outbound body 反推。原因：

- 非 OpenAI Chat target 会在 Module 5 emission policy 阶段过滤 OpenAI Chat raw fields；
- 如果 Phase 5 只看最终 body，字段已经消失，无法区分“源请求没有该字段”和“字段因跨协议不支持被 drop”；
- 正确诊断来源应是 provider extension 中保留的 source ownership 信息。
