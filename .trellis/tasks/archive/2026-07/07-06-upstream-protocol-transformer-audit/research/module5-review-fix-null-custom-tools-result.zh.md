# Module 5 审查修复：explicit null / Chat custom tools / deprecated defer 说明

## 背景

Module 5 Chat request slice 的三路审查返回后，不能进入下一模块。需要先修复或明确处理以下问题：

1. `llm/transformer/openai/chat_extensions.go` 是未跟踪文件，提交时有漏提交风险。
2. OpenAI Chat 官方 nullable 顶层字段中，`temperature:null`、`top_p:null`、`top_logprobs:null` 不保真。
3. OpenAI Chat 官方 `tools` 的 `CustomToolChatCompletions` 形态没有 raw same-protocol 保真，也不应 fake-map 到 common `llm.Tool`。
4. `functions` / `function_call` 只做 raw same-protocol replay，需要明确这是 compatibility/defer 决策，不是跨协议转换。

## 修复策略

继续沿用作者架构：

```text
OpenAI Chat native Request
  -> common llm.Request
  -> ProviderExtensions.OpenAIChat sidecar
  -> PlatformOpenAI only raw replay
```

没有把 OpenAI Chat 专属字段加入 common `llm.Request`，也没有扩大共享 `RequestFromLLM`。

新增内部机制：

- `Request.RawTopLevelFields map[string]json.RawMessage`，`json:"-"`，仅 OpenAI Chat native model 内部使用。
- `Request.MarshalJSON()` 在最终出站 JSON 上 overlay `RawTopLevelFields`。
- `captureOpenAIChatRequestRawTopLevelFields()` 在 inbound unmarshal 后，从原始 body 捕获需要精确保真的字段。
- `openAIChatRawReplayFieldNames` 是保守白名单，不做 unknown top-level passthrough。

白名单字段：

```text
prompt_cache_retention
temperature
top_p
top_logprobs
n
web_search_options
prediction
audio

tools
tool_choice
functions
function_call
```

注释明确说明：

- 这是 same-protocol replay list。
- 覆盖 explicit null、union payload、deprecated function fields、modern custom tools。
- 跨协议转换或 diagnostics 属于 LossyDowngrade slice。
- 当前模块不能 fake-map unsupported fields 到 common fields。

## TDD 证据

新增红测先失败：

```text
TestInboundTransformer_TransformRequest_PreservesOpenAIChatExplicitNullSamplingFields
TestOutboundTransformer_TransformRequest_PreservesOpenAIChatExplicitNullSamplingFieldsForOpenAIOnly
TestInboundTransformer_TransformRequest_PreservesOpenAIChatCustomToolsRawFields
TestOutboundTransformer_TransformRequest_PreservesOpenAIChatCustomToolsRawFieldsForOpenAIOnly
```

失败表现：

- inbound `ProviderExtensions.OpenAIChat` 为 nil / fields 缺失；
- outbound body 中 `temperature/top_p/top_logprobs/tools/tool_choice` 为空；
- custom tools 一度被错误放入 common `req.Tools`，随后加断言并修复。

最终目标测试通过：

```bash
cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm
go test ./transformer/openai -run 'Test(InboundTransformer_TransformRequest_PreservesOpenAIChatExplicitNullSamplingFields|InboundTransformer_TransformRequest_PreservesOpenAIChatCustomToolsRawFields|OutboundTransformer_TransformRequest_PreservesOpenAIChatExplicitNullSamplingFieldsForOpenAIOnly|OutboundTransformer_TransformRequest_PreservesOpenAIChatCustomToolsRawFieldsForOpenAIOnly)$' -count=1
```

结果：

```text
ok  github.com/looplj/axonhub/llm/transformer/openai
```

Module 5 相关目标测试通过：

```bash
go test ./transformer/openai -run 'Test(InboundTransformer_TransformRequest_PreservesOpenAIChatExplicitNullSamplingFields|InboundTransformer_TransformRequest_PreservesOpenAIChatCustomToolsRawFields|OutboundTransformer_TransformRequest_PreservesOpenAIChatExplicitNullSamplingFieldsForOpenAIOnly|OutboundTransformer_TransformRequest_PreservesOpenAIChatCustomToolsRawFieldsForOpenAIOnly|InboundTransformer_TransformRequest_PreservesOpenAIChatNativeTopLevelFields|InboundTransformer_TransformRequest_PreservesDeprecatedOpenAIChatFunctionFields|InboundTransformer_TransformRequest_PreservesOpenAIChatChoiceAndPromptCacheFields|OutboundTransformer_TransformRequest_PreservesOpenAIChatNativeTopLevelFieldsForOpenAIOnly|OutboundTransformer_TransformRequest_PreservesDeprecatedOpenAIChatFunctionFieldsForOpenAIOnly|OutboundTransformer_TransformRequest_PreservesOpenAIChatChoiceAndPromptCacheFieldsForOpenAIOnly)$' -count=1
```

结果：

```text
ok  github.com/looplj/axonhub/llm/transformer/openai
```

包级验证通过：

```bash
go test ./transformer/openai -count=1
```

结果：

```text
ok  github.com/looplj/axonhub/llm/transformer/openai
```

差异检查通过：

```bash
git -C /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean diff --check
```

结果：退出码 0。

## 字段矩阵复查

用官方 OpenAI 字段文档与当前 OpenAI Chat `Request` 模型对照：

```text
missing_official_in_model = []
extra_model_not_official = [reasoning_budget, reasoning_summary]
```

解释：

- 官方 Chat request 顶层字段当前在 OpenAI native model 中无遗漏。
- `reasoning_budget` / `reasoning_summary` 是项目既有扩展字段，不是本修复新增。

## 自审结论

当前审查反馈已处理：

- explicit null：`temperature:null`、`top_p:null`、`top_logprobs:null` 已通过 raw overlay same-protocol 回放。
- custom tools：`tools` / `tool_choice` 的 Chat custom 形态已 raw 保存和 OpenAI-only 回放。
- fake mapping：custom tool 不进入 common `req.Tools` / `req.ToolChoice`；测试明确断言。
- deprecated defer：代码注释说明 deprecated function fields 属于 same-protocol compatibility replay，跨协议转换/诊断留给 LossyDowngrade。
- untracked risk：`chat_extensions.go` 仍显示为未跟踪，必须在 Module 5 最终提交时一起 stage/commit；下一轮 review patch 必须继续显式包含该文件。

## 下一步

重新生成包含未跟踪文件的 review patch，重新触发 Module 5 code-review / architecture review。若通过，再 stage 包含 `chat_extensions.go` 的完整 Module 5 变更并提交 checkpoint。
