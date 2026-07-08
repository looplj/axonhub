# Module 5 Slice 5.2：OpenAI Chat deprecated functions / function_call 保真结果

## 目标

保留 OpenAI Chat Completions 原生但已废弃的顶层字段：

- `functions`
- `function_call`

要求：

- 入站 OpenAI Chat 请求转换到 `llm.Request` 时不丢失字段。
- 出站仅在 `PlatformOpenAI` 的 Chat same-protocol 路径回放这些字段。
- 不把这两个字段加入共享 `llm.Request`。
- 不修改共享 `RequestFromLLM`，避免影响 Gemini/OpenRouter/DeepSeek/ZAI/Moonshot/Doubao/Copilot 等 OpenAI-compatible provider。

## 代码路径

实现沿用 Slice 5.1 建立的 Chat 原生扩展 seam：

- `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/provider_extensions.go`
  - `ProviderExtensions.OpenAIChat`
  - `OpenAIChatRequestExtensions.RawTopLevelFields`
  - `EnsureOpenAIChatProviderExtensions`
  - clone 逻辑
- `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/model.go`
  - `Request.Functions json.RawMessage`
  - `Request.FunctionCall json.RawMessage`
- `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/chat_extensions.go`
  - `preserveOpenAIChatRequestExtensions`
  - `applyOpenAIChatRequestExtensions`
- `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/inbound_convert.go`
  - `Request.ToLLMRequest()` 入站保存
- `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/outbound.go`
  - `PlatformOpenAI` 分支出站回放

## TDD 证据

先添加红测，确认当前行为确实丢字段：

```text
TestInboundTransformer_TransformRequest_PreservesDeprecatedOpenAIChatFunctionFields
失败：ProviderExtensions.OpenAIChat 为 nil

TestOutboundTransformer_TransformRequest_PreservesDeprecatedOpenAIChatFunctionFieldsForOpenAIOnly
失败：openAIBody["functions"] / ["function_call"] 为空
```

实现后目标测试通过：

```bash
cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm
go test ./transformer/openai -run 'Test(InboundTransformer_TransformRequest_PreservesDeprecatedOpenAIChatFunctionFields|OutboundTransformer_TransformRequest_PreservesDeprecatedOpenAIChatFunctionFieldsForOpenAIOnly)$' -count=1
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

差异格式检查通过：

```bash
git -C /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean diff --check
```

结果：退出码 0。

## codebase-memory CLI 证据

已使用 CLI 增量刷新当前 clean worktree：

```bash
codebase-memory-mcp cli --json index_repository '{"repo_path":"/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean","mode":"fast"}'
```

结果摘要：

```text
pipeline.route path=incremental
incremental.done
project: Users-asuan-AI-axonhub-worktrees-responses-top-level-preservation-clean
status: indexed
```

`RequestFromLLM` 调用面经图谱确认仍然很宽，不能把 Chat 原生字段放进共享 builder：

```text
RequestFromLLM direct callers include:
- openai outbound
- gemini/openai outbound
- openrouter outbound
- doubao outbound
- deepseek outbound
- zai outbound
- moonshot outbound
- copilot outbound
```

## 自审结论

通过。

审查项：

- 字段归属：`functions/function_call` 属于 OpenAI Chat 原生请求字段，保存在 `OpenAIChat.Request.RawTopLevelFields`，没有污染 `llm.Request`。
- 出站策略：只在 `PlatformOpenAI` 调用 `applyOpenAIChatRequestExtensions`，Google/OpenAI-compatible 路径测试明确断言不输出。
- 架构一致性：沿用作者的 inbound -> common llm -> outbound 框架，只加深 `ProviderExtensions` seam。
- 死代码检查：移除了未使用的 `openAIChatNativeTopLevelFieldNames` 列表，避免保留无用结构。
- 验证范围：当前 slice 的目标测试、openai transformer 包测试、diff check 均通过。

## 下一步

继续 Module 5 后续 Chat emission policy slice：检查官方 Chat 字段矩阵中剩余未覆盖/有争议字段，按同样规则决定：

1. Chat 原生字段是否应进入 native model/raw extension；
2. 是否只允许 OpenAI same-protocol 回放；
3. 是否需要显式 lossy downgrade 诊断而不是假映射。
